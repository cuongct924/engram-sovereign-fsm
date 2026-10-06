#!/usr/bin/env bash
# E6 -- parallel chunk proving (docs/paper Discussion, "The unproven tail"; teacher feedback item 5).
#
# The aggregate proof of Section "Chunk Aggregation" is built from K independent leaf proofs. Proved one after another, the latency
# grows with K; proved concurrently, it is that of one leaf plus one aggregation. This script measures both on ONE host and
# models separate machines by giving each concurrent prover a share of the cores (bb reads HARDWARE_CONCURRENCY).
#
# Per interval length L (chunks = L/256): (1) witness preparation per chunk, (2) sequential proving with all cores (the
# baseline of table6e), (3) P concurrent leaf proofs, each with floor(NCPU/P) threads, (4) one aggregation with all cores,
# (5) the unproven tail the total latency implies at BLOCK_RATE blocks per second. Also: one leaf proof at several thread counts,
# the latency of a leaf on a machine with that many cores.
#
# Needs the host to itself: P concurrent provers hold about 6 GB each. It refuses to start when engram containers are running
# (set ALLOW_BUSY=1 to override) or when free memory is below the need. MODE=prep stops after step (1) as a dry run.
#
# Usage: benchmark_parallel.sh [L ...]   (default 512 1024)      Env: BLOCK_RATE=0.8, MAXP=4 (most concurrent provers; waves beyond it), MODE=full|prep, ALLOW_BUSY=0
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
LEAF_DIR="$REPO_ROOT/circuit/reanchoring"
AGG_DIR="$REPO_ROOT/circuit/reanchoring_aggregator"
RESULTS_DIR="$SCRIPT_DIR/results"
CSV_OUT="$RESULTS_DIR/table6f_parallel.csv"
GENERATOR="$SCRIPT_DIR/generate_chain.go"
WORK="$LEAF_DIR/target/par"
N_MAX=256
K=$(sed -nE 's/^global K: u32 = ([0-9]+);.*/\1/p' "$AGG_DIR/src/main.nr")
NCPU=$(sysctl -n hw.ncpu 2>/dev/null || nproc)
BLOCK_RATE="${BLOCK_RATE:-0.8}"
MAXP="${MAXP:-4}"
MODE="${MODE:-full}"
NEED_GB_PER_PROVER=6.5
if [ "$#" -gt 0 ]; then LENGTHS=("$@"); else LENGTHS=(512 1024); fi

now_s() { date +%s.%N; }
elapsed_s() { awk -v a="$1" -v b="$2" 'BEGIN { printf "%.3f", b - a }'; }
mem_free_gb() { # free + inactive + speculative, in GiB (macOS)
  vm_stat | awk '/page size of/ {ps=$8} /Pages free/ {f=$3} /Pages inactive/ {i=$3} /Pages speculative/ {s=$3}
                 END {gsub(/\./,"",f); gsub(/\./,"",i); gsub(/\./,"",s); printf "%.1f", (f+i+s)*ps/1073741824}'
}
guard() { # guard <provers>
  if [ "${ALLOW_BUSY:-0}" != "1" ] && [ "$(docker ps -q --filter name=engram- 2>/dev/null | wc -l | tr -d ' ')" != "0" ]; then
    echo "[par] engram containers are running; stop them or set ALLOW_BUSY=1" >&2; exit 2; fi
  local need free; need=$(awk -v p="$1" -v g="$NEED_GB_PER_PROVER" 'BEGIN{printf "%.1f", p*g}'); free=$(mem_free_gb)
  if awk -v n="$need" -v f="$free" 'BEGIN{exit !(f < n)}'; then echo "[par] free memory ${free} GiB < ${need} GiB needed for $1 provers" >&2; exit 3; fi
}

mkdir -p "$RESULTS_DIR" "$WORK"
LEAF_TOML_BAK=$(mktemp); AGG_TOML_BAK=$(mktemp)
cp -f "$LEAF_DIR/Prover.toml" "$LEAF_TOML_BAK" 2>/dev/null || true; cp -f "$AGG_DIR/Prover.toml" "$AGG_TOML_BAK" 2>/dev/null || true
restore() { cp -f "$LEAF_TOML_BAK" "$LEAF_DIR/Prover.toml" 2>/dev/null || true; cp -f "$AGG_TOML_BAK" "$AGG_DIR/Prover.toml" 2>/dev/null || true; rm -f "$LEAF_TOML_BAK" "$AGG_TOML_BAK"; }
trap restore EXIT

(cd "$LEAF_DIR" && nargo compile >/dev/null 2>&1)
mkdir -p "$LEAF_DIR/target/leaf_rec"
(cd "$LEAF_DIR" && bb write_vk -t noir-recursive -b target/reanchoring.json -o target/leaf_rec >/dev/null 2>&1)
python3 "$AGG_DIR/gen_leaf_vk.py" "$LEAF_DIR/target/leaf_rec" > "$WORK/leaf_vk.nr.new"
cmp -s "$WORK/leaf_vk.nr.new" "$AGG_DIR/src/leaf_vk.nr" || { echo "[par] leaf VK differs from the aggregator's: run benchmark_aggregate.sh first" >&2; exit 4; }
(cd "$AGG_DIR" && nargo compile >/dev/null 2>&1)
mkdir -p "$AGG_DIR/target/agg"
(cd "$AGG_DIR" && bb write_vk -b target/reanchoring_aggregator.json -o target/agg >/dev/null 2>&1)

[ -f "$CSV_OUT" ] || echo "date,host_ncpu,L,chunks,phase,provers,threads_each,wall_s,peak_mb_max,detail" > "$CSV_OUT"
log() { echo "$(date +%F),$NCPU,$1,$2,$3,$4,$5,$6,$7,$8" >> "$CSV_OUT"; echo "[par] L=$1 chunks=$2 $3 provers=$4 threads=$5 wall=${6}s peak=${7}MB $8"; }

prove_one() { # prove_one <dir> <threads> -> writes <dir>/wall and <dir>/mb
  local dir=$1 th=$2 log; log=$(mktemp); local t0 t1
  t0=$(now_s)
  (cd "$LEAF_DIR" && HARDWARE_CONCURRENCY="$th" /usr/bin/time -l bb prove -t noir-recursive -b target/reanchoring.json -w "$dir/witness.gz" -k target/leaf_rec/vk -o "$dir" >/dev/null 2>"$log")
  t1=$(now_s)
  elapsed_s "$t0" "$t1" > "$dir/wall"
  awk '/maximum resident/ {printf "%.0f", $1/1048576}' "$log" > "$dir/mb"; rm -f "$log"
}

for L in "${LENGTHS[@]}"; do
  chunks=$(( (L + N_MAX - 1) / N_MAX ))
  echo "[par] === L=$L ($chunks chunks), host $NCPU cores ==="
  dirs=()
  prep_total=0
  for ((c = 0; c < chunks; c++)); do
    start=$((c * N_MAX)); count=$((L - start)); [ "$count" -gt "$N_MAX" ] && count=$N_MAX
    d="$WORK/L${L}_c${c}"; mkdir -p "$d"; dirs+=("$d")
    t0=$(now_s)
    (cd "$SCRIPT_DIR" && go run "$GENERATOR" "$count" "$N_MAX" "$start" > "$LEAF_DIR/Prover.toml")
    (cd "$LEAF_DIR" && nargo execute witness >/dev/null 2>&1)
    cp "$LEAF_DIR/target/witness.gz" "$d/witness.gz"
    t1=$(now_s); prep_total=$(awk -v a="$prep_total" -v b="$(elapsed_s "$t0" "$t1")" 'BEGIN{printf "%.3f", a+b}')
  done
  log "$L" "$chunks" witness_prep 1 1 "$prep_total" 0 "go run plus nargo execute, all chunks one after another"
  [ "$MODE" = "prep" ] && continue

  # (2) sequential baseline, all cores
  guard 1
  seq_sum=0; seq_mb=0
  for d in "${dirs[@]}"; do
    prove_one "$d" "$NCPU"; w=$(cat "$d/wall"); mb=$(cat "$d/mb")
    seq_sum=$(awk -v a="$seq_sum" -v b="$w" 'BEGIN{printf "%.3f", a+b}'); [ "$mb" -gt "$seq_mb" ] && seq_mb=$mb
  done
  log "$L" "$chunks" leaf_sequential 1 "$NCPU" "$seq_sum" "$seq_mb" "sum of $chunks leaf proofs"

  # (4) aggregation, all cores (proofs from the sequential run are valid inputs)
  python3 "$AGG_DIR/gen_prover_toml.py" "$K" "${dirs[@]}" > "$AGG_DIR/Prover.toml"
  (cd "$AGG_DIR" && nargo execute witness >/dev/null 2>&1)
  t0=$(now_s)
  (cd "$AGG_DIR" && HARDWARE_CONCURRENCY="$NCPU" /usr/bin/time -l bb prove -b target/reanchoring_aggregator.json -w target/witness.gz -k target/agg/vk -o target/agg >/dev/null 2>"$WORK/agg_time.txt")
  t1=$(now_s); agg_s=$(elapsed_s "$t0" "$t1"); agg_mb=$(awk '/maximum resident/ {printf "%.0f", $1/1048576}' "$WORK/agg_time.txt")
  (cd "$AGG_DIR" && bb verify -k target/agg/vk -p target/agg/proof -i target/agg/public_inputs >/dev/null 2>&1) || { echo "[par] aggregate did not verify" >&2; exit 5; }
  log "$L" "$chunks" aggregation 1 "$NCPU" "$agg_s" "$agg_mb" "verified"
  seq_total=$(awk -v a="$seq_sum" -v b="$agg_s" 'BEGIN{printf "%.2f", a+b}')
  log "$L" "$chunks" total_sequential 1 "$NCPU" "$seq_total" 0 "tail_blocks=$(awk -v t="$seq_total" -v r="$BLOCK_RATE" 'BEGIN{printf "%.0f", t*r}') at ${BLOCK_RATE} blocks/s"

  # (3) concurrent leaf proofs in waves of at most MAXP provers, cores split evenly within a wave
  P=$chunks; [ "$P" -gt "$MAXP" ] && P=$MAXP
  th=$(( NCPU / P )); [ "$th" -lt 1 ] && th=1
  par_wall=0; par_mb=0; walls=""
  for ((w = 0; w < chunks; w += P)); do
    guard "$P"
    pids=(); wave=("${dirs[@]:w:P}")
    t0=$(now_s)
    for d in "${wave[@]}"; do rm -f "$d/proof" "$d/public_inputs"; prove_one "$d" "$th" & pids+=($!); done
    for p in "${pids[@]}"; do wait "$p"; done
    t1=$(now_s); par_wall=$(awk -v a="$par_wall" -v b="$(elapsed_s "$t0" "$t1")" 'BEGIN{printf "%.3f", a+b}')
    for d in "${wave[@]}"; do mb=$(cat "$d/mb"); [ "$mb" -gt "$par_mb" ] && par_mb=$mb; walls="$walls$(cat "$d/wall") "; done
  done
  log "$L" "$chunks" leaf_parallel "$P" "$th" "$par_wall" "$par_mb" "$(( (chunks + P - 1) / P )) wave(s); per-prover walls: $walls"
  python3 "$AGG_DIR/gen_prover_toml.py" "$K" "${dirs[@]}" > "$AGG_DIR/Prover.toml"
  (cd "$AGG_DIR" && nargo execute witness >/dev/null 2>&1)
  (cd "$AGG_DIR" && HARDWARE_CONCURRENCY="$NCPU" bb prove -b target/reanchoring_aggregator.json -w target/witness.gz -k target/agg/vk -o target/agg >/dev/null 2>&1)
  (cd "$AGG_DIR" && bb verify -k target/agg/vk -p target/agg/proof -i target/agg/public_inputs >/dev/null 2>&1) || { echo "[par] aggregate of parallel proofs did not verify" >&2; exit 5; }
  par_total=$(awk -v a="$par_wall" -v b="$agg_s" 'BEGIN{printf "%.2f", a+b}')
  log "$L" "$chunks" total_parallel "$P" "$th" "$par_total" 0 "tail_blocks=$(awk -v t="$par_total" -v r="$BLOCK_RATE" 'BEGIN{printf "%.0f", t*r}') at ${BLOCK_RATE} blocks/s; aggregate verified"
done

# one leaf proof at several thread counts: the latency of a leaf on a machine with that many cores
if [ "$MODE" != "prep" ]; then
  d="${dirs[0]}"
  for th in "$NCPU" $((NCPU / 2)) $((NCPU / 4)) 2; do
    [ "$th" -lt 1 ] && continue
    guard 1; rm -f "$d/proof" "$d/public_inputs"; prove_one "$d" "$th"
    log 256 1 leaf_by_threads 1 "$th" "$(cat "$d/wall")" "$(cat "$d/mb")" "one leaf proof, alone on the host"
  done
fi
echo "[par] wrote $CSV_OUT"
