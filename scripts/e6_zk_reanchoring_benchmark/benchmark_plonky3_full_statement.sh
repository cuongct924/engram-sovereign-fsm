#!/usr/bin/env bash
# E6 -- Reanchoring Feasibility Evaluation, "full statement" Plonky3 comparison.
#
# Unlike benchmark_plonky3.sh (which drives Plonky3's own first-party
# prove_prime_field_31 example proving N chained Poseidon2 permutations at a
# different (width-16) parameterization -- a primitive, not the real
# circuit's statement), this drives a NEW, purpose-built Plonky3 AIR
# (plonky3_full_statement/) that proves the same non-signature statement
# circuit/reanchoring/src/main.nr's verify_header_chain proves: real
# header-chain continuity via a raw Poseidon2 t=4 permutation per header
# (p3-poseidon2-air's Poseidon2Air, matching main.nr's own hash_header
# construction -- see that crate's header.rs module doc for the BabyBear-
# vs-BN254 field caveat), FSM-state/withdrawal-lock legality, and root
# binding to rt_last/rt_new. (Table 6d's n=2..256 numbers before this
# comment predate this switch and used three chained p3-sha256-air
# compressions per header instead -- superseded, not comparable.)
#
# It does NOT prove ed25519 boundary-signature verification (main.nr's
# verify_boundary_block) -- see plonky3_full_statement/src/main.rs's module
# doc and this script's own header for why that's out of scope here (real
# non-native/foreign-field EC arithmetic in Plonky3's small fields has no
# off-the-shelf gadget; not attempted, not stubbed as passing).
#
# Requires: nightly Rust (p3-util needs unstable maybe_uninit_slice, same as
# benchmark_plonky3.sh). Clones its OWN pinned Plonky3 checkout into
# plonky3_full_statement/.plonky3_checkout (PLONKY3_FULL_STATEMENT_DIR env
# var overrides the location) -- separate from benchmark_plonky3.sh's
# .plonky3_checkout, which is pinned to an older commit that predates
# p3-sha256-air.
#
# Usage: scripts/e6_zk_reanchoring_benchmark/benchmark_plonky3_full_statement.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CRATE_DIR="$SCRIPT_DIR/plonky3_full_statement"
RESULTS_DIR="$SCRIPT_DIR/results"
CSV_OUT="$RESULTS_DIR/table6d_full_statement_plonky3.csv"
PINNED_COMMIT="3152b14a89067c83775a8076cc262ffc48a1fd7c"
PLONKY3_DIR="${PLONKY3_FULL_STATEMENT_DIR:-$CRATE_DIR/.plonky3_checkout}"

mkdir -p "$RESULTS_DIR"

if [ ! -d "$PLONKY3_DIR/.git" ]; then
  echo "[e6-plonky3-full] cloning Plonky3 (pinned $PINNED_COMMIT) into $PLONKY3_DIR..."
  git clone https://github.com/Plonky3/Plonky3.git "$PLONKY3_DIR"
  (cd "$PLONKY3_DIR" && git checkout "$PINNED_COMMIT")
else
  current_commit=$(cd "$PLONKY3_DIR" && git rev-parse HEAD)
  if [ "$current_commit" != "$PINNED_COMMIT" ]; then
    echo "[e6-plonky3-full] WARNING: $PLONKY3_DIR is at $current_commit, not the pinned $PINNED_COMMIT -- results may not match this script's doc comments." >&2
  fi
fi

if [ ! -L "$CRATE_DIR/.plonky3_checkout" ] && [ "$PLONKY3_DIR" != "$CRATE_DIR/.plonky3_checkout" ]; then
  # Cargo.toml's path deps are relative to plonky3_full_statement/.plonky3_checkout;
  # if the caller pointed PLONKY3_FULL_STATEMENT_DIR elsewhere, symlink it in.
  ln -sfn "$PLONKY3_DIR" "$CRATE_DIR/.plonky3_checkout"
fi

if ! rustup toolchain list 2>/dev/null | grep -q '^nightly'; then
  echo "[e6-plonky3-full] installing nightly toolchain (p3-util needs unstable maybe_uninit_slice)..."
  rustup toolchain install nightly --profile minimal
fi

echo "n,prove_s,verify_s,proof_size_bytes,conjectured_security_bits,proven_security_bits" > "$CSV_OUT"

cd "$CRATE_DIR"

for n in 2 4 8 16 32 64 128 256; do
  echo "[e6-plonky3-full] n=$n..."
  RAW=$(RUST_LOG=info cargo +nightly run --release --bin prove_full_statement -- --n "$n" 2>&1)
  OUT=$(printf '%s' "$RAW" | sed -E $'s/\x1b\\[[0-9;]*[a-zA-Z]//g')

  if ! echo "$OUT" | grep -q "Proof Verified Successfully"; then
    echo "[e6-plonky3-full] ERROR: n=$n did not verify successfully:" >&2
    echo "$OUT" >&2
    exit 1
  fi

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

  echo "$n,$prove_s,$verify_s,$proof_size,$conjectured,$proven" >> "$CSV_OUT"
  echo "[e6-plonky3-full]   prove=${prove_s}s verify=${verify_s}s proof_size=${proof_size}B conjectured=${conjectured}b proven=${proven}b"
done

echo "[e6-plonky3-full] wrote $CSV_OUT"
