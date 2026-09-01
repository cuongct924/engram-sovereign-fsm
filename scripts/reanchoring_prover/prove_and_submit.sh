#!/usr/bin/env bash
# Real end-to-end ZK re-anchoring pipeline (spec/README.md's §Re-anchoring
# via ZK-Proof of Recovery), driving REAL chain data through the REAL
# circuit -- not a synthetic fixture like scripts/e6_zk_reanchoring_benchmark
# uses for its constraint-count sweep.
#
# Pipeline:
#   1. engramd query-recovery-headers   -- pull the CURRENT SOVEREIGN/
#      RECOVERING interval's real tracked headers (keeper.HeaderHistory).
#   2. circuit/reanchoring_witness      -- link the raw headers into a real
#      prev_hash chain (Poseidon2), without a Go-side hash implementation
#      (see that crate's doc).
#   3. circuit/reanchoring              -- the real N_MAX=256 circuit: nargo
#      execute + bb prove against the SAME verification key the Go node
#      embeds (x/sovereignty/keeper/zk_assets/vk), so the proof verifies
#      against exactly what the chain will check.
#   4. engramd tx-submit-recovery-proof -- broadcast the real proof.
#
# Requires: nargo, bb on PATH; a running engramd node; the engramd binary
# built from THIS checkout (query-recovery-headers/tx-submit-recovery-proof
# are new, see cmd/engramd/reanchor_cli.go).
#
# Rolling checkpoint, oldest-COUNT slice, COUNT dynamic up to N_MAX=256 (see
# x/sovereignty/keeper/msg_server.go's SubmitRecoveryProof doc): proves the
# OLDEST min(TOTAL_N, N_MAX) tracked headers after rt_last, padding the
# remaining N_MAX-COUNT circuit slots with zero-valued (unconstrained, see
# main.nr's `active` gate) headers. Any COUNT from 1 to N_MAX is provable in
# one proof -- unlike the old fixed-N=4 design, which could never prove a
# trailing remainder shorter than N once the interval stopped growing
# (liveness gap), and raced behind a backlog it couldn't cover (E2 S7:
# 29/52 real proof attempts rejected).
#
# HeaderHistory keeps every header from the checkpoint until the proven
# segment is pruned, so the oldest COUNT entries stay a valid, provable
# segment however many more accumulate -- up to N_MAX, beyond which this
# script must be re-run (watch_and_prove.sh does this automatically).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
WITNESS_CIRCUIT_DIR="$REPO_ROOT/circuit/reanchoring_witness"
MAIN_CIRCUIT_DIR="$REPO_ROOT/circuit/reanchoring"
EMBEDDED_VK="$REPO_ROOT/x/sovereignty/keeper/zk_assets/vk"

ENGRAMD="${ENGRAMD_BIN:-$REPO_ROOT/engramd}"
NODE_URL="${NODE_URL:-http://127.0.0.1:26657}"
# Must match circuit/reanchoring/src/main.nr's CHAIN_ID constant exactly --
# build-boundary-witness reconstructs real CometBFT vote sign-bytes with
# this chain ID, and a mismatch makes every signature fail to verify.
CHAIN_ID="${ENGRAM_CHAIN_ID:-engram-dev-1}"
# Must match circuit/reanchoring/src/main.nr's `global N_MAX` AND
# circuit/reanchoring_witness/src/main.nr's `global N_MAX` (kept in sync by
# hand across both files, see their headers).
N_MAX=256

if [ ! -x "$ENGRAMD" ]; then
  echo "[reanchoring_prover] building engramd from $REPO_ROOT..." >&2
  (cd "$REPO_ROOT" && go build -o "$ENGRAMD" ./cmd/engramd)
fi
if [ ! -f "$EMBEDDED_VK" ]; then
  echo "[reanchoring_prover] ERROR: $EMBEDDED_VK not found -- run 'make proto-gen' or check out the repo fully" >&2
  exit 1
fi

echo "[reanchoring_prover] 1/4: querying real header history from $NODE_URL..."
HEADERS_OUT=$("$ENGRAMD" query-recovery-headers --node "$NODE_URL")

RT_LAST=$(echo "$HEADERS_OUT" | grep '^rt_last=' | cut -d= -f2)
ALL_HEADER_LINES=$(echo "$HEADERS_OUT" | grep '^height=')
TOTAL_N=$(echo "$ALL_HEADER_LINES" | grep -c '^height=' || true)

if [ "$TOTAL_N" -lt 1 ]; then
  echo "[reanchoring_prover] ERROR: currently tracked interval has 0 headers -- no proof attempted." >&2
  exit 1
fi

# COUNT = min(TOTAL_N, N_MAX) -- any COUNT from 1 to N_MAX is provable in
# one proof, so no waiting for an exact/minimum count; a longer interval is
# capped to the oldest N_MAX headers per proof.
COUNT="$TOTAL_N"
if [ "$COUNT" -gt "$N_MAX" ]; then
  COUNT="$N_MAX"
fi

# Here-string, NOT `echo ... | head -n "$COUNT"`: once TOTAL_N exceeds the
# OS pipe buffer (~64KB), `head` exits before `echo` finishes writing, so
# `echo` gets SIGPIPE and (under pipefail) the script aborts with 141 before
# submitting anything. A here-string has no live pipe to race.
HEADER_LINES=$(head -n "$COUNT" <<< "$ALL_HEADER_LINES")

field() {
  # field <line-number 1-indexed> <field-name>
  echo "$HEADER_LINES" | sed -n "${1}p" | grep -o "${2}=[^ ]*" | cut -d= -f2
}

echo "[reanchoring_prover] 2/4: linking real prev_hash chain via circuit/reanchoring_witness (count=$COUNT, N_MAX=$N_MAX)..."
{
  echo "rt_last = \"$RT_LAST\""
  echo "count = \"$COUNT\""
  printf 'raw_fsm_state = ['
  for ((i = 1; i <= COUNT; i++)); do printf '"%s",' "$(field "$i" fsm_state)"; done
  for ((i = COUNT; i < N_MAX; i++)); do printf '"0",'; done
  printf ']\n'
  printf 'raw_withdrawal_locked = ['
  for ((i = 1; i <= COUNT; i++)); do printf '"%s",' "$(field "$i" withdrawal_locked)"; done
  for ((i = COUNT; i < N_MAX; i++)); do printf '"0",'; done
  printf ']\n'
  printf 'raw_state_root = ['
  for ((i = 1; i <= COUNT; i++)); do printf '"%s",' "$(field "$i" state_root)"; done
  for ((i = COUNT; i < N_MAX; i++)); do printf '"0",'; done
  printf ']\n'
} > "$WITNESS_CIRCUIT_DIR/Prover.toml"

WITNESS_OUT=$(cd "$WITNESS_CIRCUIT_DIR" && nargo execute witness 2>&1)
LINKED_CHAIN=$(echo "$WITNESS_OUT" | grep '^Header ')
linked_field() {
  # linked_field <line-number 1-indexed> <field-name>
  echo "$LINKED_CHAIN" | sed -n "${1}p" | grep -o "${2}: [^,}]*" | awk '{print $2}'
}

echo "[reanchoring_prover] 3/4: building real Prover.toml (count=$COUNT real + $((N_MAX - COUNT)) padding) + running nargo execute + bb prove..."
# rt_new is hash_header(headers[count-1]), NOT that header's raw state_root
# field (circuit/reanchoring/src/main.nr's rt_new-binding doc) --
# reanchoring_witness prints this as a distinct rt_new_hash= line.
RT_NEW=$(echo "$WITNESS_OUT" | grep '^rt_new_hash=' | cut -d= -f2)
if [ -z "$RT_NEW" ]; then
  echo "[reanchoring_prover] ERROR: reanchoring_witness did not print rt_new_hash= -- check nargo execute output above" >&2
  exit 1
fi
echo "[reanchoring_prover] 3b/5: fetching real boundary-block signature witness (circuit/README.md's Scope section)..."
BOUNDARY_WITNESS_TOML=$(mktemp)
trap 'rm -f "$BOUNDARY_WITNESS_TOML"' EXIT
# --last-height MUST be headers[COUNT-1]'s own real height, not
# build-boundary-witness's own default (the highest height BlockHashByHeight
# currently has data for, i.e. the live tip) -- when COUNT is capped below
# TOTAL_N (a tracked interval longer than N_MAX), the tip keeps climbing
# past headers[COUNT-1] while this proof is being built, so the default
# fetches boundary-signature data for a LATER block than the one this proof
# actually claims as its last header. verify_boundary_block then checks a
# real commit's quorum/hash against public inputs computed from a different
# block, which can fail non-deterministically (real proposers rotating
# in/out between the two heights change which validators' signatures land
# in each commit) -- reproduced live as a "Failed constraint:
# signed_power*3 >= total_power*2" nargo execute failure, misleadingly
# similar-looking to (but NOT) the SubmitRecoveryProof-side staleness race
# documented below.
LAST_REAL_HEIGHT=$(field "$COUNT" height)
"$ENGRAMD" build-boundary-witness --node "$NODE_URL" --chain-id "$CHAIN_ID" --last-height "$LAST_REAL_HEIGHT" --out "$BOUNDARY_WITNESS_TOML"

# TOML requires every loose (non-table) key to precede any [table]/[[table]]
# section -- rt_last/rt_new/count and the boundary witness's own loose keys
# (first_height, first_block_hash, ...) must ALL come before headers'
# [[headers]] array-of-tables and the boundary witness's own
# [validators]/[first_block]/[last_block] tables, or nargo silently
# misattributes later keys to the wrong (or no) top-level scope. Split the
# boundary witness at its first `[` line so its loose keys land in the same
# leading block as rt_last/rt_new/count.
BOUNDARY_LOOSE=$(awk '/^\[/{exit} {print}' "$BOUNDARY_WITNESS_TOML")
BOUNDARY_TABLES=$(awk 'BEGIN{p=0} /^\[/{p=1} p{print}' "$BOUNDARY_WITNESS_TOML")

{
  echo "rt_last = \"$RT_LAST\""
  echo "rt_new = \"$RT_NEW\""
  echo "count = \"$COUNT\""
  echo "$BOUNDARY_LOOSE"
  echo ""
  echo "$BOUNDARY_TABLES"
  for ((i = 1; i <= COUNT; i++)); do
    echo ""
    echo "[[headers]]"
    echo "prev_hash = \"$(linked_field "$i" prev_hash)\""
    echo "fsm_state = \"$(linked_field "$i" fsm_state)\""
    echo "withdrawal_locked = \"$(linked_field "$i" withdrawal_locked)\""
    echo "state_root = \"$(linked_field "$i" state_root)\""
  done
  # Padding slots -- unconstrained by the circuit's `active` gate once
  # i >= count, so their values are irrelevant; zero-filled for simplicity.
  for ((i = COUNT; i < N_MAX; i++)); do
    echo ""
    echo "[[headers]]"
    echo 'prev_hash = "0"'
    echo 'fsm_state = "0"'
    echo 'withdrawal_locked = "0"'
    echo 'state_root = "0"'
  done
} > "$MAIN_CIRCUIT_DIR/Prover.toml"

(
  cd "$MAIN_CIRCUIT_DIR"
  nargo execute witness
  rm -rf target/proof
  mkdir -p target/proof
  cp "$EMBEDDED_VK" target/proof/vk
  bb prove -b target/reanchoring.json -w target/witness.gz -o target/proof -k target/proof/vk
)

# Publish the real (non-padding) header chain to Celestia DA before
# submitting the proof -- pure audit trail, never verified on-chain (see
# publish-recovery-witness's doc): HeaderHistory is pruned once the proof is
# accepted, so without this no late-joining node or external auditor could
# retrieve the header chain it was built from. A publish failure degrades to
# skipping --da-height, not aborting the submission.
#
# Throttled, not on every proof: celestia-bridge-prover signs every blob
# submission (this AND the validators' regular block-data publishing) from
# one shared account -- publishing per proof sustained 3-4 account-sequence-
# mismatch races/sec, degrading DA health for all 7 validators. Skipped
# attempts still submit the proof itself (never throttled) with no
# --da-height.
WITNESS_PUBLISH_MIN_INTERVAL_S="${WITNESS_PUBLISH_MIN_INTERVAL_S:-120}"
WITNESS_PUBLISH_MARKER="$MAIN_CIRCUIT_DIR/target/.last_witness_publish"
NOW_S=$(date +%s)
LAST_PUBLISH_S=0
[ -f "$WITNESS_PUBLISH_MARKER" ] && LAST_PUBLISH_S=$(cat "$WITNESS_PUBLISH_MARKER")

DA_HEIGHT_FLAG=()
if [ $((NOW_S - LAST_PUBLISH_S)) -ge "$WITNESS_PUBLISH_MIN_INTERVAL_S" ]; then
  echo "[reanchoring_prover] 4/5: publishing real header chain to Celestia DA..."
  {
    echo "rt_last = \"$RT_LAST\""
    echo "rt_new = \"$RT_NEW\""
    echo "count = \"$COUNT\""
    for ((i = 1; i <= COUNT; i++)); do
      echo ""
      echo "[[headers]]"
      echo "prev_hash = \"$(linked_field "$i" prev_hash)\""
      echo "fsm_state = \"$(linked_field "$i" fsm_state)\""
      echo "withdrawal_locked = \"$(linked_field "$i" withdrawal_locked)\""
      echo "state_root = \"$(linked_field "$i" state_root)\""
    done
  } > "$MAIN_CIRCUIT_DIR/target/proof/witness_headers.toml"

  if DA_OUT=$("$ENGRAMD" publish-recovery-witness --headers "$MAIN_CIRCUIT_DIR/target/proof/witness_headers.toml" 2>&1); then
    DA_HEIGHT=$(echo "$DA_OUT" | grep '^da_celestia_height=' | cut -d= -f2)
    if [ -n "$DA_HEIGHT" ]; then
      echo "[reanchoring_prover] witness published at Celestia height $DA_HEIGHT"
      DA_HEIGHT_FLAG=(--da-height "$DA_HEIGHT")
      echo "$NOW_S" > "$WITNESS_PUBLISH_MARKER"
    fi
  else
    echo "[reanchoring_prover] WARNING: witness DA publish failed, proceeding without --da-height: $DA_OUT" >&2
  fi
else
  echo "[reanchoring_prover] 4/5: skipping witness publish (throttled, last one ${WITNESS_PUBLISH_MIN_INTERVAL_S}s ago not yet elapsed)"
fi

echo "[reanchoring_prover] 5/5: submitting real proof to $NODE_URL..."
"$ENGRAMD" tx-submit-recovery-proof \
  "${DA_HEIGHT_FLAG[@]}" \
  --node "$NODE_URL" \
  --proof "$MAIN_CIRCUIT_DIR/target/proof/proof" \
  --public-inputs "$MAIN_CIRCUIT_DIR/target/proof/public_inputs"

echo "[reanchoring_prover] done."
