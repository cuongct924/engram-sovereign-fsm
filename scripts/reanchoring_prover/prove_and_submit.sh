#!/usr/bin/env bash
# Real end-to-end ZK re-anchoring pipeline (spec/README.md's §Re-anchoring
# via ZK-Proof of Recovery), driving REAL chain data through the REAL
# circuit -- not a synthetic fixture like scripts/e6_zk_reanchoring_benchmark
# uses for its constraint-count sweep.
#
# Pipeline:
#   1. engramd query-recovery-headers  -- pull the CURRENT SOVEREIGN/
#      RECOVERING interval's real tracked headers off-chain (x/sovereignty/
#      keeper.HeaderHistory, via the gRPC-over-ABCI query registered in
#      app.go).
#   2. circuit/reanchoring_witness     -- link the raw headers into a real
#      prev_hash chain (Poseidon2), WITHOUT a Go-side hash implementation --
#      see that crate's own doc for why.
#   3. circuit/reanchoring             -- the real N_MAX=256 circuit: nargo
#      execute + bb prove, against the SAME verification key
#      (x/sovereignty/keeper/zk_assets/vk) the Go node embeds and checks
#      proofs against, so a proof produced here is guaranteed to verify
#      against exactly what the chain will check it with.
#   4. engramd tx-submit-recovery-proof -- broadcast the real proof.
#
# Requires: nargo, bb on PATH; a running engramd node; the engramd binary
# built from THIS checkout (so query-recovery-headers/tx-submit-recovery-proof
# exist -- both new, see cmd/engramd/reanchor_cli.go).
#
# Rolling checkpoint, oldest-COUNT slice, COUNT dynamic up to N_MAX=256 (see
# x/sovereignty/keeper/msg_server.go's SubmitRecoveryProof doc for the full
# design): this script proves the OLDEST min(TOTAL_N, N_MAX) tracked headers
# after the current checkpoint (rt_last), padding the remaining N_MAX-COUNT
# circuit slots with zero-valued headers (unconstrained -- see
# circuit/reanchoring/src/main.nr's `active` gate). Unlike the old fixed-N=4
# design, COUNT is NOT required to equal N_MAX -- any COUNT from 1 to N_MAX
# is provable in one proof, which closes two real problems the fixed-N
# design had: (1) a trailing remainder shorter than a fixed N could never be
# proven at all once the interval stopped growing (a genuine liveness gap,
# not just a performance one); (2) the interval racing ahead of a small
# fixed N=4's coverage (docs/EXPERIMENT.md's E2 S7: 29/52 real proof
# attempts rejected).
#
# x/sovereignty/keeper.HeaderHistory keeps every header from the checkpoint
# onward until the proven segment is pruned, so the oldest COUNT entries are
# always a valid, provable segment regardless of how many MORE have
# accumulated past them since (up to N_MAX -- beyond that, a long interval
# still needs this script run repeatedly, scripts/reanchoring_prover/watch_and_prove.sh
# does this automatically).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
WITNESS_CIRCUIT_DIR="$REPO_ROOT/circuit/reanchoring_witness"
MAIN_CIRCUIT_DIR="$REPO_ROOT/circuit/reanchoring"
EMBEDDED_VK="$REPO_ROOT/x/sovereignty/keeper/zk_assets/vk"

ENGRAMD="${ENGRAMD_BIN:-$REPO_ROOT/engramd}"
NODE_URL="${NODE_URL:-http://127.0.0.1:26657}"
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

# COUNT = min(TOTAL_N, N_MAX) -- unlike the old fixed-N design, ANY COUNT
# from 1 to N_MAX is provable in one proof; no need to wait for an exact or
# minimum count. A long interval (TOTAL_N > N_MAX) is still capped to the
# oldest N_MAX headers per proof, same rolling-checkpoint mechanism as
# before, just with a much larger per-proof window.
COUNT="$TOTAL_N"
if [ "$COUNT" -gt "$N_MAX" ]; then
  COUNT="$N_MAX"
fi

# Here-string, NOT `echo ... | head -n "$COUNT"` -- found live (real bug,
# not a proof rejection) in the old fixed-N version of this script: once
# TOTAL_N grows large enough that ALL_HEADER_LINES exceeds the OS pipe
# buffer (~64KB, i.e. a few hundred tracked headers), `head` reads its N
# lines and exits BEFORE `echo` finishes writing the rest, so `echo` gets
# SIGPIPE -- under `set -o pipefail` that aborts this entire script with
# exit 141, before Step 4 ever submits anything to the chain. A here-string
# has no live pipe to race; `head` just reads from it directly.
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

LINKED_CHAIN=$(cd "$WITNESS_CIRCUIT_DIR" && nargo execute witness 2>&1 | grep '^Header ')
linked_field() {
  # linked_field <line-number 1-indexed> <field-name>
  echo "$LINKED_CHAIN" | sed -n "${1}p" | grep -o "${2}: [^,}]*" | awk '{print $2}'
}

echo "[reanchoring_prover] 3/4: building real Prover.toml (count=$COUNT real + $((N_MAX - COUNT)) padding) + running nargo execute + bb prove..."
RT_NEW=$(linked_field "$COUNT" "state_root")
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

echo "[reanchoring_prover] 4/4: submitting real proof to $NODE_URL..."
"$ENGRAMD" tx-submit-recovery-proof \
  --node "$NODE_URL" \
  --proof "$MAIN_CIRCUIT_DIR/target/proof/proof" \
  --public-inputs "$MAIN_CIRCUIT_DIR/target/proof/public_inputs"

echo "[reanchoring_prover] done."
