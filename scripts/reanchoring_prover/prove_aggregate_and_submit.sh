#!/usr/bin/env bash
# Chunked re-anchoring: proves an interval of up to K*N_MAX headers as ONE aggregate proof.
# Each chunk of <= N_MAX headers is proven by circuit/reanchoring (noir-recursive target), then
# circuit/reanchoring_aggregator folds them into a single proof with the same 224-byte public
# inputs as a plain leaf proof, so SubmitRecoveryProof's bindings are unchanged.
#
# prove_and_submit.sh hands over to this script when the tracked interval exceeds N_MAX and
# ENGRAM_AGGREGATE=1. Requires: nargo, bb, python3 on PATH; a running engramd node.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
WITNESS_CIRCUIT_DIR="$REPO_ROOT/circuit/reanchoring_witness"
MAIN_CIRCUIT_DIR="$REPO_ROOT/circuit/reanchoring"
AGG_CIRCUIT_DIR="$REPO_ROOT/circuit/reanchoring_aggregator"
EMBEDDED_AGG_VK="$REPO_ROOT/x/sovereignty/keeper/zk_assets/vk_aggregate"

ENGRAMD="${ENGRAMD_BIN:-$REPO_ROOT/engramd}"
NODE_URL="${NODE_URL:-http://127.0.0.1:26657}"
CHAIN_ID="${ENGRAM_CHAIN_ID:-engram-dev-1}"
N_MAX=256
# Must match circuit/reanchoring_aggregator/src/main.nr's `global K`.
K=$(sed -nE 's/^global K: u32 = ([0-9]+);.*/\1/p' "$AGG_CIRCUIT_DIR/src/main.nr")

if [ ! -x "$ENGRAMD" ]; then
  echo "[reanchoring_prover] building engramd from $REPO_ROOT..." >&2
  (cd "$REPO_ROOT" && go build -o "$ENGRAMD" ./cmd/engramd)
fi
[ -f "$EMBEDDED_AGG_VK" ] || { echo "[reanchoring_prover] ERROR: $EMBEDDED_AGG_VK not found" >&2; exit 1; }

echo "[reanchoring_prover] agg 1/5: querying real header history from $NODE_URL..."
HEADERS_OUT=$("$ENGRAMD" query-recovery-headers --node "$NODE_URL")
RT_LAST=$(echo "$HEADERS_OUT" | grep '^rt_last=' | cut -d= -f2)
ALL_HEADER_LINES=$(echo "$HEADERS_OUT" | grep '^height=')
TOTAL_N=$(echo "$ALL_HEADER_LINES" | grep -c '^height=' || true)
[ "$TOTAL_N" -ge 1 ] || { echo "[reanchoring_prover] ERROR: tracked interval has 0 headers" >&2; exit 1; }

# Oldest min(TOTAL_N, K*N_MAX) headers, like prove_and_submit.sh's rolling checkpoint.
COUNT="$TOTAL_N"
[ "$COUNT" -le $((K * N_MAX)) ] || COUNT=$((K * N_MAX))
CHUNKS=$(( (COUNT + N_MAX - 1) / N_MAX ))
# An aggregate with count <= N_MAX would be verified against the leaf VK and rejected.
if [ "$CHUNKS" -lt 2 ]; then
  echo "[reanchoring_prover] ERROR: $COUNT headers fit one chunk -- use prove_and_submit.sh" >&2
  exit 1
fi
echo "[reanchoring_prover] aggregating $COUNT headers as $CHUNKS chunk(s) (K=$K, N_MAX=$N_MAX)"

# Leaf VK under the recursive target; the aggregator hardcodes it (src/leaf_vk.nr), so refuse to
# run against a stale aggregator rather than produce a proof the chain's VK would reject.
LEAF_REC_DIR="$MAIN_CIRCUIT_DIR/target/leaf_rec"
mkdir -p "$LEAF_REC_DIR"
if [ ! -f "$LEAF_REC_DIR/vk" ] || [ "$MAIN_CIRCUIT_DIR/target/reanchoring.json" -nt "$LEAF_REC_DIR/vk" ]; then
  (cd "$MAIN_CIRCUIT_DIR" && bb write_vk -t noir-recursive -b target/reanchoring.json -o target/leaf_rec >/dev/null)
fi
if ! python3 "$AGG_CIRCUIT_DIR/gen_leaf_vk.py" "$LEAF_REC_DIR" | cmp -s - "$AGG_CIRCUIT_DIR/src/leaf_vk.nr"; then
  echo "[reanchoring_prover] ERROR: circuit/reanchoring_aggregator/src/leaf_vk.nr is stale for the current leaf circuit -- regenerate it, recompile the aggregator and refresh zk_assets/vk_aggregate" >&2
  exit 1
fi

OUT_DIR="$MAIN_CIRCUIT_DIR/target/chunks_live"
rm -rf "$OUT_DIR"; mkdir -p "$OUT_DIR"
BOUNDARY_WITNESS_TOML=$(mktemp)
trap 'rm -f "$BOUNDARY_WITNESS_TOML"' EXIT
FULL_WITNESS_HEADERS="$OUT_DIR/witness_headers.toml"
: > "$FULL_WITNESS_HEADERS"

RT_PREV="$RT_LAST"
CHUNK_DIRS=()
for ((j = 0; j < CHUNKS; j++)); do
  S=$((j * N_MAX + 1))
  E=$(( (j + 1) * N_MAX )); [ "$E" -le "$COUNT" ] || E="$COUNT"
  N=$((E - S + 1))
  CHUNK_LINES=$(sed -n "${S},${E}p" <<< "$ALL_HEADER_LINES")
  cfield() { echo "$CHUNK_LINES" | sed -n "${1}p" | grep -o "${2}=[^ ]*" | cut -d= -f2; }

  echo "[reanchoring_prover] agg 2/5: chunk $((j + 1))/$CHUNKS (headers $S..$E, count=$N): linking prev_hash chain..."
  {
    echo "rt_last = \"$RT_PREV\""
    echo "count = \"$N\""
    for key in fsm_state withdrawal_locked state_root; do
      printf 'raw_%s = [' "$key"
      for ((i = 1; i <= N; i++)); do printf '"%s",' "$(cfield "$i" "$key")"; done
      for ((i = N; i < N_MAX; i++)); do printf '"0",'; done
      printf ']\n'
    done
  } > "$WITNESS_CIRCUIT_DIR/Prover.toml"
  WITNESS_OUT=$(cd "$WITNESS_CIRCUIT_DIR" && nargo execute witness 2>&1)
  LINKED_CHAIN=$(echo "$WITNESS_OUT" | grep '^Header ')
  lfield() { echo "$LINKED_CHAIN" | sed -n "${1}p" | grep -o "${2}: [^,}]*" | awk '{print $2}'; }
  RT_NEW=$(echo "$WITNESS_OUT" | grep '^rt_new_hash=' | cut -d= -f2)
  [ -n "$RT_NEW" ] || { echo "[reanchoring_prover] ERROR: reanchoring_witness printed no rt_new_hash= for chunk $j" >&2; exit 1; }

  FIRST_H=$(cfield 1 height)
  LAST_H=$(cfield "$N" height)
  "$ENGRAMD" build-boundary-witness --node "$NODE_URL" --chain-id "$CHAIN_ID" \
    --first-height "$FIRST_H" --last-height "$LAST_H" --out "$BOUNDARY_WITNESS_TOML"
  BOUNDARY_LOOSE=$(awk '/^\[/{exit} {print}' "$BOUNDARY_WITNESS_TOML")
  BOUNDARY_TABLES=$(awk 'BEGIN{p=0} /^\[/{p=1} p{print}' "$BOUNDARY_WITNESS_TOML")

  {
    echo "rt_last = \"$RT_PREV\""
    echo "rt_new = \"$RT_NEW\""
    echo "count = \"$N\""
    echo "$BOUNDARY_LOOSE"
    echo ""
    echo "$BOUNDARY_TABLES"
    for ((i = 1; i <= N; i++)); do
      printf '\n[[headers]]\nprev_hash = "%s"\nfsm_state = "%s"\nwithdrawal_locked = "%s"\nstate_root = "%s"\n' \
        "$(lfield "$i" prev_hash)" "$(lfield "$i" fsm_state)" "$(lfield "$i" withdrawal_locked)" "$(lfield "$i" state_root)"
    done
    for ((i = N; i < N_MAX; i++)); do
      printf '\n[[headers]]\nprev_hash = "0"\nfsm_state = "0"\nwithdrawal_locked = "0"\nstate_root = "0"\n'
    done
  } > "$MAIN_CIRCUIT_DIR/Prover.toml"

  # Audit-trail witness for all chunks, in the same format prove_and_submit.sh publishes.
  for ((i = 1; i <= N; i++)); do
    printf '\n[[headers]]\nprev_hash = "%s"\nfsm_state = "%s"\nwithdrawal_locked = "%s"\nstate_root = "%s"\n' \
      "$(lfield "$i" prev_hash)" "$(lfield "$i" fsm_state)" "$(lfield "$i" withdrawal_locked)" "$(lfield "$i" state_root)" \
      >> "$FULL_WITNESS_HEADERS"
  done

  echo "[reanchoring_prover] agg 3/5: proving chunk $((j + 1))/$CHUNKS (noir-recursive)..."
  CHUNK_DIR="$OUT_DIR/c$j"
  mkdir -p "$CHUNK_DIR"
  (
    cd "$MAIN_CIRCUIT_DIR"
    nargo execute witness
    bb prove -t noir-recursive -b target/reanchoring.json -w target/witness.gz -k target/leaf_rec/vk -o "$CHUNK_DIR"
  )
  CHUNK_DIRS+=("$CHUNK_DIR")
  RT_PREV="$RT_NEW"
done

echo "[reanchoring_prover] agg 4/5: aggregating $CHUNKS chunk proof(s)..."
python3 "$AGG_CIRCUIT_DIR/gen_prover_toml.py" "$K" "${CHUNK_DIRS[@]}" > "$AGG_CIRCUIT_DIR/Prover.toml"
AGG_PROOF_DIR="$AGG_CIRCUIT_DIR/target/proof_live"
rm -rf "$AGG_PROOF_DIR"; mkdir -p "$AGG_PROOF_DIR"
cp "$EMBEDDED_AGG_VK" "$AGG_PROOF_DIR/vk"
(
  cd "$AGG_CIRCUIT_DIR"
  nargo execute witness
  bb prove -b target/reanchoring_aggregator.json -w target/witness.gz -k "$AGG_PROOF_DIR/vk" -o "$AGG_PROOF_DIR"
  bb verify -k "$AGG_PROOF_DIR/vk" -p "$AGG_PROOF_DIR/proof" -i "$AGG_PROOF_DIR/public_inputs"
)

DA_HEIGHT_FLAG=()
if [ -n "${CELESTIA_BRIDGE_URL:-}" ]; then
  echo "[reanchoring_prover] publishing real header chain to Celestia DA..."
  if DA_OUT=$("$ENGRAMD" publish-recovery-witness --headers "$FULL_WITNESS_HEADERS" 2>&1); then
    DA_HEIGHT=$(echo "$DA_OUT" | grep '^da_celestia_height=' | cut -d= -f2)
    [ -z "$DA_HEIGHT" ] || DA_HEIGHT_FLAG=(--da-height "$DA_HEIGHT")
  else
    echo "[reanchoring_prover] WARNING: witness DA publish failed, proceeding without --da-height: $DA_OUT" >&2
  fi
fi

echo "[reanchoring_prover] agg 5/5: submitting aggregate proof to $NODE_URL..."
"$ENGRAMD" tx-submit-recovery-proof \
  ${DA_HEIGHT_FLAG[@]+"${DA_HEIGHT_FLAG[@]}"} \
  --node "$NODE_URL" \
  --proof "$AGG_PROOF_DIR/proof" \
  --public-inputs "$AGG_PROOF_DIR/public_inputs"
echo "[reanchoring_prover] done."
