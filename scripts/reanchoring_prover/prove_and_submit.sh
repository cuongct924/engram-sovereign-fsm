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
#      prev_hash chain (Poseidon/pedersen_hash), WITHOUT a Go-side hash
#      implementation -- see that crate's own doc for why.
#   3. circuit/reanchoring             -- the real N=4 circuit: nargo
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
# Known limitations (see circuit/reanchoring/src/main.nr's `global N`, and
# CLAUDE.md):
#   - N is fixed at compile time. This script only works when EXACTLY N
#     headers are currently tracked -- if the SOVEREIGN/RECOVERING interval
#     is a different length, it exits with an error rather than attempting
#     a mismatched proof. Recompiling the circuit for a different N (or
#     building variable-length/recursive proof support) is out of scope
#     here.
#   - A real, inherent race: this script queries the interval, then spends
#     real wall-clock time (query + nargo execute + bb prove, ~50-200ms of
#     actual proving at N=4 per the E6 benchmark, plus RPC/CLI overhead) off
#     the hot path before submitting. If the chain is STILL producing new
#     blocks in a still-unhealthy SOVEREIGN/RECOVERING run during that
#     window, the interval keeps growing underneath the proof -- by
#     submission time, the tip's real state_root may already differ from
#     the rt_new this proof was built against, and x/sovereignty/keeper/
#     msg_server.go's SubmitRecoveryProof correctly rejects it (confirmed
#     empirically: multiple real end-to-end runs against a continuously-
#     producing test node were rejected this way, exactly as designed --
#     this is the same anti-staleness protection RealProofSubmittedHeight
#     enforces for the "proof already accepted, then interval grew before
#     RECOVERING was reached" case, just observed here at submission time
#     instead). This is not a bug: a real operator needs to submit while the
#     interval is genuinely stable (e.g. once BTC/DA/P2P conditions clearly
#     aren't recovering further within the current window), not race a
#     continuously-growing one.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
WITNESS_CIRCUIT_DIR="$REPO_ROOT/circuit/reanchoring_witness"
MAIN_CIRCUIT_DIR="$REPO_ROOT/circuit/reanchoring"
EMBEDDED_VK="$REPO_ROOT/x/sovereignty/keeper/zk_assets/vk"

ENGRAMD="${ENGRAMD_BIN:-$REPO_ROOT/engramd}"
NODE_URL="${NODE_URL:-http://127.0.0.1:26657}"
# Must match circuit/reanchoring/src/main.nr's `global N` AND
# circuit/reanchoring_witness/src/main.nr's `global N` (kept in sync by
# hand across both files, see their headers).
EXPECTED_N=4

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
HEADER_LINES=$(echo "$HEADERS_OUT" | grep '^height=')
N=$(echo "$HEADER_LINES" | grep -c '^height=' || true)

if [ "$N" -ne "$EXPECTED_N" ]; then
  echo "[reanchoring_prover] ERROR: currently tracked interval has $N header(s), circuit requires exactly $EXPECTED_N -- no proof attempted (see this script's doc on the fixed-N limitation)." >&2
  exit 1
fi

field() {
  # field <line-number 1-indexed> <field-name>
  echo "$HEADER_LINES" | sed -n "${1}p" | grep -o "${2}=[^ ]*" | cut -d= -f2
}

echo "[reanchoring_prover] 2/4: linking real prev_hash chain via circuit/reanchoring_witness..."
{
  echo "rt_last = \"$RT_LAST\""
  printf 'raw_fsm_state = ['
  for ((i = 1; i <= N; i++)); do printf '"%s",' "$(field "$i" fsm_state)"; done
  printf ']\n'
  printf 'raw_withdrawal_locked = ['
  for ((i = 1; i <= N; i++)); do printf '"%s",' "$(field "$i" withdrawal_locked)"; done
  printf ']\n'
  printf 'raw_state_root = ['
  for ((i = 1; i <= N; i++)); do printf '"%s",' "$(field "$i" state_root)"; done
  printf ']\n'
} > "$WITNESS_CIRCUIT_DIR/Prover.toml"

LINKED_CHAIN=$(cd "$WITNESS_CIRCUIT_DIR" && nargo execute witness 2>&1 | grep '^Header ')
linked_field() {
  # linked_field <line-number 1-indexed> <field-name>
  echo "$LINKED_CHAIN" | sed -n "${1}p" | grep -o "${2}: [^,}]*" | awk '{print $2}'
}

echo "[reanchoring_prover] 3/4: building real Prover.toml + running nargo execute + bb prove..."
RT_NEW=$(linked_field "$N" "state_root")
{
  echo "rt_last = \"$RT_LAST\""
  echo "rt_new = \"$RT_NEW\""
  for ((i = 1; i <= N; i++)); do
    echo ""
    echo "[[headers]]"
    echo "prev_hash = \"$(linked_field "$i" prev_hash)\""
    echo "fsm_state = \"$(linked_field "$i" fsm_state)\""
    echo "withdrawal_locked = \"$(linked_field "$i" withdrawal_locked)\""
    echo "state_root = \"$(linked_field "$i" state_root)\""
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
