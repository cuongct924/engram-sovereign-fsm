#!/usr/bin/env bash
# E6 -- chunk aggregation vs. chaining (docs/EXPERIMENT.md's Table 6E).
#
# For each interval length L: split into ceil(L/256) chunks, prove every chunk with
# circuit/reanchoring (noir-recursive target), then fold them into ONE proof with
# circuit/reanchoring_aggregator. Records leaf and aggregate prove/verify cost and sizes.
# "Chaining" cost is the sum over the leaf proofs a chain would submit one by one.
#
# Chunks come from generate_chain.go's chunk mode (one deterministic chain, shared validator set,
# real ed25519 precommits), so consecutive chunks genuinely chain through rt_last/rt_new.
#
# Usage: benchmark_aggregate.sh [L ...]   (default: 256 512 1024; at most K*256)
# Env:   TRIALS=3 (aggregate prove/verify repetitions per L), KEEP=1 (reuse cached leaf proofs)
# Requires: nargo, bb, go, python3 on PATH.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
LEAF_DIR="$REPO_ROOT/circuit/reanchoring"
AGG_DIR="$REPO_ROOT/circuit/reanchoring_aggregator"
RESULTS_DIR="$SCRIPT_DIR/results"
CSV_OUT="$RESULTS_DIR/table6e_aggregate.csv"
GENERATOR="$SCRIPT_DIR/generate_chain.go"
WORK="$LEAF_DIR/target/chunks"
N_MAX=256
K=$(sed -nE 's/^global K: u32 = ([0-9]+);.*/\1/p' "$AGG_DIR/src/main.nr")
TRIALS="${TRIALS:-3}"
KEEP="${KEEP:-1}"

if [ "$#" -gt 0 ]; then LENGTHS=("$@"); else LENGTHS=(256 512 1024); fi

mkdir -p "$RESULTS_DIR" "$WORK"

now_s() { date +%s.%N; }
elapsed_s() { awk -v a="$1" -v b="$2" 'BEGIN { printf "%.3f", b - a }'; }
# timed <cmd...>: runs it once, prints "<wall_s> <peak_resident_MiB>"
timed() {
  local log t0 t1 mb
  log=$(mktemp)
  t0=$(now_s)
  if [ "$(uname)" = "Darwin" ]; then /usr/bin/time -l "$@" >/dev/null 2>"$log"; else /usr/bin/time -v "$@" >/dev/null 2>"$log"; fi
  t1=$(now_s)
  if [ "$(uname)" = "Darwin" ]; then mb=$(awk '/maximum resident/ {printf "%.0f", $1/1048576}' "$log")
  else mb=$(awk '/Maximum resident/ {printf "%.0f", $NF/1024}' "$log"); fi
  rm -f "$log"
  echo "$(elapsed_s "$t0" "$t1") ${mb:-0}"
}

backup() { cp -f "$1" "$2" 2>/dev/null || true; }
LEAF_TOML_BAK=$(mktemp); AGG_TOML_BAK=$(mktemp)
backup "$LEAF_DIR/Prover.toml" "$LEAF_TOML_BAK"; backup "$AGG_DIR/Prover.toml" "$AGG_TOML_BAK"
restore() {
  cp -f "$LEAF_TOML_BAK" "$LEAF_DIR/Prover.toml" 2>/dev/null || true
  cp -f "$AGG_TOML_BAK" "$AGG_DIR/Prover.toml" 2>/dev/null || true
  rm -f "$LEAF_TOML_BAK" "$AGG_TOML_BAK"
}
trap restore EXIT

echo "[e6-agg] K=$K, lengths: ${LENGTHS[*]}, trials=$TRIALS"

# Leaf VK under the recursive target; the aggregator hardcodes it, so regenerate and fail on drift.
(cd "$LEAF_DIR" && nargo compile >/dev/null 2>&1)
mkdir -p "$LEAF_DIR/target/leaf_rec"
(cd "$LEAF_DIR" && bb write_vk -t noir-recursive -b target/reanchoring.json -o target/leaf_rec >/dev/null 2>&1)
python3 "$AGG_DIR/gen_leaf_vk.py" "$LEAF_DIR/target/leaf_rec" > "$WORK/leaf_vk.nr.new"
if ! cmp -s "$WORK/leaf_vk.nr.new" "$AGG_DIR/src/leaf_vk.nr"; then
  echo "[e6-agg] leaf VK changed -- refreshing aggregator's src/leaf_vk.nr"
  cp "$WORK/leaf_vk.nr.new" "$AGG_DIR/src/leaf_vk.nr"
fi
(cd "$AGG_DIR" && nargo compile >/dev/null 2>&1)
mkdir -p "$AGG_DIR/target/agg"
(cd "$AGG_DIR" && bb write_vk -b target/reanchoring_aggregator.json -o target/agg >/dev/null 2>&1)

gates_json=$(cd "$AGG_DIR" && bb gates -b target/reanchoring_aggregator.json 2>/dev/null)
agg_size=$(echo "$gates_json" | grep -o '"circuit_size": [0-9]*' | head -1 | awk '{print $2}')
gates_json=$(cd "$LEAF_DIR" && bb gates -b target/reanchoring.json 2>/dev/null)
leaf_size=$(echo "$gates_json" | grep -o '"circuit_size": [0-9]*' | head -1 | awk '{print $2}')

echo "L,chunks,leaf_circuit_size,agg_circuit_size,leaf_prove_s_mean,leaf_prove_peak_mb,chain_prove_s,chain_verify_ms,chain_proof_bytes,agg_prove_s_mean,agg_prove_peak_mb,agg_verify_ms,agg_proof_bytes,total_agg_prove_s,trials" > "$CSV_OUT"

prove_leaf() { # prove_leaf <count> <start> -> prints "<prove_s> <peak_mb>" ; proof lands in $WORK/c<count>_s<start>
  local count=$1 start=$2 dir="$WORK/c${1}_s${2}"
  # A cached proof is only valid for the circuit it was proven against.
  if [ "$KEEP" = "1" ] && [ -f "$dir/proof" ] && [ -f "$dir/time" ] && [ "$dir/proof" -nt "$LEAF_DIR/target/reanchoring.json" ]; then cat "$dir/time"; return; fi
  mkdir -p "$dir"
  (cd "$SCRIPT_DIR" && go run "$GENERATOR" "$count" "$N_MAX" "$start" > "$LEAF_DIR/Prover.toml")
  (cd "$LEAF_DIR" && nargo execute witness >/dev/null 2>&1)
  local res
  res=$(cd "$LEAF_DIR" && timed bb prove -t noir-recursive -b target/reanchoring.json -w target/witness.gz -k target/leaf_rec/vk -o "$dir")
  echo "$res" | tee "$dir/time"

}

for L in "${LENGTHS[@]}"; do
  chunks=$(( (L + N_MAX - 1) / N_MAX ))
  if [ "$chunks" -gt "$K" ]; then echo "[e6-agg] L=$L needs $chunks chunks > K=$K, skipping" >&2; continue; fi
  echo "[e6-agg] === L=$L ($chunks chunk(s)) ==="
  dirs=(); leaf_sum=0; leaf_mb=0; chain_verify_ms=0; chain_bytes=0
  for ((c = 0; c < chunks; c++)); do
    start=$((c * N_MAX)); count=$((L - start)); [ "$count" -gt "$N_MAX" ] && count=$N_MAX
    read -r ps mb < <(prove_leaf "$count" "$start")
    dirs+=("$WORK/c${count}_s${start}")
    leaf_sum=$(awk -v a="$leaf_sum" -v b="$ps" 'BEGIN{printf "%.3f", a+b}')
    [ "$mb" -gt "$leaf_mb" ] && leaf_mb=$mb
    d="$WORK/c${count}_s${start}"
    t0=$(now_s)
    (cd "$LEAF_DIR" && bb verify -t noir-recursive -k target/leaf_rec/vk -p "$d/proof" -i "$d/public_inputs" >/dev/null 2>&1)
    t1=$(now_s)
    chain_verify_ms=$(awk -v a="$chain_verify_ms" -v b="$(elapsed_s "$t0" "$t1")" 'BEGIN{printf "%.1f", a + b*1000}')
    chain_bytes=$((chain_bytes + $(wc -c < "$d/proof")))
  done
  leaf_mean=$(awk -v a="$leaf_sum" -v n="$chunks" 'BEGIN{printf "%.2f", a/n}')

  python3 "$AGG_DIR/gen_prover_toml.py" "$K" "${dirs[@]}" > "$AGG_DIR/Prover.toml"
  (cd "$AGG_DIR" && nargo execute witness >/dev/null 2>&1)
  agg_sum=0; agg_verify=0; agg_mb=0
  for ((t = 1; t <= TRIALS; t++)); do
    read -r ps mb < <(cd "$AGG_DIR" && timed bb prove -b target/reanchoring_aggregator.json -w target/witness.gz -k target/agg/vk -o target/agg)
    agg_sum=$(awk -v a="$agg_sum" -v b="$ps" 'BEGIN{printf "%.3f", a+b}')
    [ "$mb" -gt "$agg_mb" ] && agg_mb=$mb
    t0=$(now_s)
    (cd "$AGG_DIR" && bb verify -k target/agg/vk -p target/agg/proof -i target/agg/public_inputs >/dev/null 2>&1) \
      || { echo "[e6-agg] ERROR: aggregate proof for L=$L did not verify" >&2; exit 1; }
    t1=$(now_s)
    agg_verify=$(awk -v a="$agg_verify" -v b="$(elapsed_s "$t0" "$t1")" 'BEGIN{printf "%.1f", a + b*1000}')
  done
  agg_mean=$(awk -v a="$agg_sum" -v n="$TRIALS" 'BEGIN{printf "%.2f", a/n}')
  agg_verify_mean=$(awk -v a="$agg_verify" -v n="$TRIALS" 'BEGIN{printf "%.1f", a/n}')
  agg_bytes=$(wc -c < "$AGG_DIR/target/agg/proof")
  total=$(awk -v a="$leaf_sum" -v b="$agg_mean" 'BEGIN{printf "%.2f", a+b}')

  echo "$L,$chunks,$leaf_size,$agg_size,$leaf_mean,$leaf_mb,$leaf_sum,$chain_verify_ms,$chain_bytes,$agg_mean,$agg_mb,$agg_verify_mean,$agg_bytes,$total,$TRIALS" >> "$CSV_OUT"
  echo "[e6-agg] L=$L: chain verify ${chain_verify_ms}ms/${chain_bytes}B vs aggregate verify ${agg_verify_mean}ms/${agg_bytes}B; total prove chain ${leaf_sum}s vs aggregate ${total}s"
done
echo "[e6-agg] wrote $CSV_OUT"
