#!/usr/bin/env bash
# E6 -- Reanchoring Feasibility Evaluation (docs/EXPERIMENT.md's Table 6A/6B).
#
# Drives the recovery-proof circuit through real nargo+bb at increasing `count`
# (real headers, out of the fixed N_MAX=256 capacity), recording real
# constraint counts and prove/verify timings -- no placeholder numbers.
#
# The circuit compiles ONCE at N_MAX=256 (dynamic-padding design, VK is
# count-invariant); only Prover.toml's `count` varies per iteration.
# generate_chain.go builds a real hash-linked header chain per count value.
#
# Usage: scripts/e6_zk_reanchoring_benchmark/benchmark_prover.sh
# Requires: nargo, bb, go on PATH.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CIRCUIT_DIR="$(cd "$SCRIPT_DIR/../../circuit/reanchoring" && pwd)"
RESULTS_DIR="$SCRIPT_DIR/results"
CSV_OUT="$RESULTS_DIR/table6b_scaling.csv"
GENERATOR="$SCRIPT_DIR/generate_chain.go"
N_MAX=256

COUNT_VALUES=(1 4 16 64 256)

mkdir -p "$RESULTS_DIR"
cd "$CIRCUIT_DIR"

PROVER_TOML_BACKUP=$(mktemp)
cp -f Prover.toml "$PROVER_TOML_BACKUP" 2>/dev/null || true
restore() {
  cp -f "$PROVER_TOML_BACKUP" Prover.toml 2>/dev/null || rm -f Prover.toml
  rm -f "$PROVER_TOML_BACKUP"
}
trap restore EXIT

now_s() { date +%s.%N; }
elapsed_s() { awk -v a="$1" -v b="$2" 'BEGIN { printf "%.3f", b - a }'; }

echo "[e6] Compiling the N_MAX=$N_MAX circuit once (compile-time fixed, count varies at runtime)..."
t0=$(now_s); nargo compile > /tmp/e6_compile.log 2>&1; t1=$(now_s)
compile_s=$(elapsed_s "$t0" "$t1")

gates_json=$(bb gates -b target/reanchoring.json 2>/dev/null)
acir_opcodes=$(echo "$gates_json" | grep -o '"acir_opcodes": [0-9]*' | head -1 | awk '{print $2}')
circuit_size=$(echo "$gates_json" | grep -o '"circuit_size": [0-9]*' | head -1 | awk '{print $2}')
echo "[e6] acir_opcodes=$acir_opcodes circuit_size=$circuit_size (fixed across every count below)"

rm -rf target/proof
mkdir -p target/proof
bb write_vk -b target/reanchoring.json -o target/proof/vk > /tmp/e6_writevk.log 2>&1
echo "[e6] VK written once, reused for every count (dynamic-padding design keeps it count-invariant)."

echo "n,acir_opcodes,circuit_size,compile_s,execute_s,prove_s,verify_s,proof_size_bytes" > "$CSV_OUT"

for n in "${COUNT_VALUES[@]}"; do
  echo "[e6] === count=$n ==="
  (cd "$SCRIPT_DIR" && go run "$GENERATOR" "$n" "$N_MAX" > "$CIRCUIT_DIR/Prover.toml")

  t0=$(now_s); nargo execute witness > /tmp/e6_execute.log 2>&1; t1=$(now_s)
  execute_s=$(elapsed_s "$t0" "$t1")

  t0=$(now_s)
  bb prove -b target/reanchoring.json -w target/witness.gz -o target/proof -k target/proof/vk/vk > /tmp/e6_prove.log 2>&1
  t1=$(now_s)
  prove_s=$(elapsed_s "$t0" "$t1")

  t0=$(now_s)
  bb verify -k target/proof/vk/vk -p target/proof/proof -i target/proof/public_inputs > /tmp/e6_verify.log 2>&1
  t1=$(now_s)
  verify_s=$(elapsed_s "$t0" "$t1")

  proof_size_bytes=$(stat -f%z target/proof/proof 2>/dev/null || stat -c%s target/proof/proof)

  echo "$n,$acir_opcodes,$circuit_size,$compile_s,$execute_s,$prove_s,$verify_s,$proof_size_bytes" >> "$CSV_OUT"
  echo "[e6] count=$n: execute=${execute_s}s prove=${prove_s}s verify=${verify_s}s proof_size=${proof_size_bytes}B"
done

echo "[e6] Done. Raw results: $CSV_OUT"
echo "[e6] Run 'python3 $SCRIPT_DIR/stats_collector.py' to build Table 6A/6B markdown and Figure 6."
