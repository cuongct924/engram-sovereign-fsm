#!/usr/bin/env bash
# E6 -- Reanchoring Feasibility Evaluation (docs/EXPERIMENT.md's Table 6A/6B).
#
# Drives circuit/reanchoring/src/main.nr's recovery-proof circuit through the
# real Noir (nargo) + Barretenberg (bb, UltraHonk) toolchain at increasing
# N (number of sovereign blocks in the recovery interval), recording real
# constraint counts and prove/verify timings -- no placeholder numbers.
#
# Usage: scripts/e6_zk_reanchoring_benchmark/benchmark_prover.sh
# Requires: nargo, bb on PATH (see circuit/reanchoring's toolchain notes).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CIRCUIT_DIR="$(cd "$SCRIPT_DIR/../../circuit/reanchoring" && pwd)"
RESULTS_DIR="$SCRIPT_DIR/results"
CHAIN_FILE="$RESULTS_DIR/.chain_dump.txt"
CSV_OUT="$RESULTS_DIR/table6b_scaling.csv"

N_VALUES=(4 8 16 32 64 128 256)
CHAIN_LEN=256 # must match main.nr's `global CHAIN_LEN` for dump_chain

mkdir -p "$RESULTS_DIR"
cd "$CIRCUIT_DIR"

MAIN_NR="src/main.nr"
MAIN_NR_BACKUP=$(mktemp)
PROVER_TOML_BACKUP=$(mktemp)
cp "$MAIN_NR" "$MAIN_NR_BACKUP"
cp Prover.toml "$PROVER_TOML_BACKUP"

restore() {
  cp "$MAIN_NR_BACKUP" "$MAIN_NR"
  cp "$PROVER_TOML_BACKUP" Prover.toml
  rm -f "$MAIN_NR_BACKUP" "$PROVER_TOML_BACKUP"
}
trap restore EXIT

now_s() { date +%s.%N; }
elapsed_s() { awk -v a="$1" -v b="$2" 'BEGIN { printf "%.3f", b - a }'; }

echo "[e6] Dumping a $CHAIN_LEN-header valid chain via 'nargo test dump_chain' (values reused across all N)..."
nargo test dump_chain --show-output 2>&1 | grep '^Header ' > "$CHAIN_FILE"
actual_lines=$(wc -l < "$CHAIN_FILE" | tr -d ' ')
if [ "$actual_lines" -ne "$CHAIN_LEN" ]; then
  echo "[e6] ERROR: expected $CHAIN_LEN chain entries, got $actual_lines (did CHAIN_LEN drift from main.nr?)" >&2
  exit 1
fi

# Parse one field (prev_hash|fsm_state|withdrawal_locked|state_root) from
# chain line $1 (1-indexed) of $CHAIN_FILE.
field() {
  local line_no=$1 field_name=$2
  sed -n "${line_no}p" "$CHAIN_FILE" | grep -o "${field_name}: [^,}]*" | awk '{print $2}'
}

write_prover_toml() {
  local n=$1
  local rt_last rt_new
  rt_last=$(field 1 "prev_hash")
  rt_new=$(field "$n" "state_root")
  {
    echo "rt_last = \"$rt_last\""
    echo "rt_new = \"$rt_new\""
    for ((i = 1; i <= n; i++)); do
      echo ""
      echo "[[headers]]"
      echo "prev_hash = \"$(field "$i" "prev_hash")\""
      echo "fsm_state = \"$(field "$i" "fsm_state")\""
      echo "withdrawal_locked = \"$(field "$i" "withdrawal_locked")\""
      echo "state_root = \"$(field "$i" "state_root")\""
    done
  } > Prover.toml
}

echo "n,acir_opcodes,circuit_size,compile_s,execute_s,prove_s,verify_s,proof_size_bytes" > "$CSV_OUT"

for n in "${N_VALUES[@]}"; do
  echo "[e6] === N=$n ==="
  # The #[test] functions after `fn main`'s closing brace hardcode a
  # 4-element header array, which only type-checks when N=4 -- strip them
  # for the benchmark compile (they aren't needed for compile/execute/prove/
  # verify) rather than making them artificially N-generic.
  main_end_line=$(grep -n '^fn main' "$MAIN_NR_BACKUP" | head -1 | cut -d: -f1)
  main_end_line=$(awk -v start="$main_end_line" 'NR >= start && /^}/ { print NR; exit }' "$MAIN_NR_BACKUP")
  head -n "$main_end_line" "$MAIN_NR_BACKUP" | sed "s/^global N: u32 = [0-9]*;/global N: u32 = $n;/" > "$MAIN_NR"
  write_prover_toml "$n"

  rm -rf target/proof
  mkdir -p target/proof

  t0=$(now_s); nargo compile > /tmp/e6_compile.log 2>&1; t1=$(now_s)
  compile_s=$(elapsed_s "$t0" "$t1")

  gates_json=$(bb gates -b target/reanchoring.json 2>/dev/null)
  acir_opcodes=$(echo "$gates_json" | grep -o '"acir_opcodes": [0-9]*' | awk '{print $2}')
  circuit_size=$(echo "$gates_json" | grep -o '"circuit_size": [0-9]*' | awk '{print $2}')

  bb write_vk -b target/reanchoring.json -o target/proof > /tmp/e6_writevk.log 2>&1

  t0=$(now_s); nargo execute witness > /tmp/e6_execute.log 2>&1; t1=$(now_s)
  execute_s=$(elapsed_s "$t0" "$t1")

  t0=$(now_s)
  bb prove -b target/reanchoring.json -w target/witness.gz -o target/proof -k target/proof/vk > /tmp/e6_prove.log 2>&1
  t1=$(now_s)
  prove_s=$(elapsed_s "$t0" "$t1")

  t0=$(now_s)
  bb verify -k target/proof/vk -p target/proof/proof -i target/proof/public_inputs > /tmp/e6_verify.log 2>&1
  t1=$(now_s)
  verify_s=$(elapsed_s "$t0" "$t1")

  proof_size_bytes=$(stat -f%z target/proof/proof 2>/dev/null || stat -c%s target/proof/proof)

  echo "$n,$acir_opcodes,$circuit_size,$compile_s,$execute_s,$prove_s,$verify_s,$proof_size_bytes" >> "$CSV_OUT"
  echo "[e6] N=$n: acir_opcodes=$acir_opcodes circuit_size=$circuit_size compile=${compile_s}s execute=${execute_s}s prove=${prove_s}s verify=${verify_s}s proof_size=${proof_size_bytes}B"
done

echo "[e6] Done. Raw results: $CSV_OUT"
echo "[e6] Run 'python3 $SCRIPT_DIR/stats_collector.py' to build Table 6A/6B markdown and Figure 6."
