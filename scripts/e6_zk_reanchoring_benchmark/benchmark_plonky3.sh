#!/usr/bin/env bash
# E6 -- Reanchoring Feasibility Evaluation, Table 6C / Figure 7 (docs/EXPERIMENT.md,
# marked "tuy chon neu con thoi gian" -- optional backend comparison).
#
# Drives Plonky3's own first-party benchmark example
# (examples/examples/prove_prime_field_31.rs, pinned commit
# a31a1443a114c58735850daa5b5fc5c43c138d9d) through a real UniStark+FRI STARK
# proof (BabyBear field, transparent setup, Poseidon2 permutation, no trusted
# setup) proving N Poseidon2 permutations, and records real prove/verify
# timings + proof size -- matching benchmark_prover.sh's convention for the
# Noir/Barretenberg side.
#
# Scope note: circuit/reanchoring/src/main.nr's per-header cost is dominated
# by one Poseidon2 hash_header per header (two Poseidon2 permutations; see
# main.nr's hash_header comment) -- the fsm_state/withdrawal_locked boolean
# asserts and root-binding asserts are O(1) per header and contribute a
# negligible constraint share (table6a_6b.md's regression). This benchmarks
# the same primitive on the Plonky3 side -- N chained Poseidon2 permutations
# -- via Plonky3's own VectorizedPoseidon2Air example, rather than
# hand-rolling an AIR reimplementing main.nr's exact header struct. Both
# sides isolate the same primitive: an earlier revision of main.nr used
# Pedersen hashing, but it has since switched to real Poseidon2, so this is a
# like-for-like primitive cost comparison, not a proxy.
#
# The vectorized AIR proves in batches of 8 (P2_VECTOR_LEN) -- num_hashes =
# 2^log_trace_length * 8, so N=4 has no representable counterpart; this
# covers N in {8,16,32,64,128,256}, reusing the N=8..256 points already
# measured on the Noir side.
#
# Requires: a Plonky3 checkout (PLONKY3_DIR env var, or this script clones
# one to a gitignored scratch dir) + a nightly Rust toolchain (p3-util uses
# the unstable `maybe_uninit_slice` feature -- the default stable rustc fails
# with E0658 on p3-util; `rustup toolchain install nightly` + `cargo +nightly`
# builds clean).
#
# Usage: scripts/e6_zk_reanchoring_benchmark/benchmark_plonky3.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RESULTS_DIR="$SCRIPT_DIR/results"
CSV_OUT="$RESULTS_DIR/table6c_plonky3.csv"
PINNED_COMMIT="a31a1443a114c58735850daa5b5fc5c43c138d9d"
PLONKY3_DIR="${PLONKY3_DIR:-$SCRIPT_DIR/.plonky3_checkout}"

mkdir -p "$RESULTS_DIR"

if [ ! -d "$PLONKY3_DIR/.git" ]; then
  echo "[e6-plonky3] cloning Plonky3 (pinned $PINNED_COMMIT) into $PLONKY3_DIR..."
  git clone https://github.com/Plonky3/Plonky3.git "$PLONKY3_DIR"
  (cd "$PLONKY3_DIR" && git checkout "$PINNED_COMMIT")
else
  current_commit=$(cd "$PLONKY3_DIR" && git rev-parse HEAD)
  if [ "$current_commit" != "$PINNED_COMMIT" ]; then
    echo "[e6-plonky3] WARNING: $PLONKY3_DIR is at $current_commit, not the pinned $PINNED_COMMIT -- results may not match this script's doc comments." >&2
  fi
fi

if ! rustup toolchain list 2>/dev/null | grep -q '^nightly'; then
  echo "[e6-plonky3] installing nightly toolchain (p3-util needs unstable maybe_uninit_slice)..."
  rustup toolchain install nightly --profile minimal
fi

# N (= num_hashes) -> log_trace_length, since num_hashes = 2^log_trace_length * 8
# (P2_VECTOR_LEN in prove_prime_field_31.rs). N=4 has no valid log_trace_length
# (floor is 8) -- intentionally excluded, see header comment.
declare -A N_TO_LOG=( [8]=0 [16]=1 [32]=2 [64]=3 [128]=4 [256]=5 )

echo "n,log_trace_length,prove_s,verify_s,proof_size_bytes,conjectured_security_bits,proven_security_bits" > "$CSV_OUT"

cd "$PLONKY3_DIR"

for n in 8 16 32 64 128 256; do
  log_len="${N_TO_LOG[$n]}"
  echo "[e6-plonky3] N=$n (log_trace_length=$log_len)..."
  # tracing_forest's ForestLayer always emits ANSI color codes regardless of
  # NO_COLOR -- an escape-reset sequence sits between "INFO" and "prove [",
  # silently breaking the grep/sed parsing below. Strip ANSI escapes from the
  # captured output before parsing.
  RAW=$(RUST_LOG=info cargo +nightly run --release --example prove_prime_field_31 -- \
    --field baby-bear --objective poseidon-2-permutations --log-trace-length "$log_len" \
    --discrete-fourier-transform radix-2-dit-parallel --merkle-hash poseidon-2 2>&1)
  OUT=$(printf '%s' "$RAW" | sed -E $'s/\x1b\\[[0-9;]*[a-zA-Z]//g')

  if ! echo "$OUT" | grep -q "Proof Verified Successfully"; then
    echo "[e6-plonky3] ERROR: N=$n did not verify successfully:" >&2
    echo "$OUT" >&2
    exit 1
  fi

  # Top-level "prove [ <dur> | ..." / "verify [ <dur> | ..." spans
  # (tracing_forest ForestLayer output), not the indented sub-spans
  # (commit/open/FRI/...).
  prove_raw=$(echo "$OUT" | grep -E "INFO +prove \[" | head -1 | sed -E 's/.*prove \[ *([0-9.]+)(µs|ms|s) .*/\1 \2/')
  verify_raw=$(echo "$OUT" | grep -E "INFO +verify \[" | head -1 | sed -E 's/.*verify \[ *([0-9.]+)(µs|ms|s) .*/\1 \2/')
  proof_size=$(echo "$OUT" | grep -oE "Proof size: [0-9]+ bytes" | grep -oE "[0-9]+")
  conjectured=$(echo "$OUT" | grep -oE "Conjectured security: [0-9]+ bits" | grep -oE "[0-9]+" || echo "0")
  proven=$(echo "$OUT" | grep -oE "Proven security: [0-9]+ bits" | grep -oE "[0-9]+" || echo "0")

  to_seconds() {
    local val unit
    val=$(echo "$1" | awk '{print $1}')
    unit=$(echo "$1" | awk '{print $2}')
    case "$unit" in
      µs) awk -v v="$val" 'BEGIN{printf "%.6f", v/1000000}' ;;
      ms) awk -v v="$val" 'BEGIN{printf "%.6f", v/1000}' ;;
      s)  awk -v v="$val" 'BEGIN{printf "%.6f", v}' ;;
      *) echo "0" ;;
    esac
  }
  prove_s=$(to_seconds "$prove_raw")
  verify_s=$(to_seconds "$verify_raw")

  echo "$n,$log_len,$prove_s,$verify_s,$proof_size,$conjectured,$proven" >> "$CSV_OUT"
  echo "[e6-plonky3]   prove=${prove_s}s verify=${verify_s}s proof_size=${proof_size}B"
done

echo "[e6-plonky3] wrote $CSV_OUT"
