#!/usr/bin/env bash
# Witnesses a sound aggregator must reject (`nargo execute` fails). Needs chunk proofs under
# circuit/reanchoring/target/chunks/ (scripts/e6_zk_reanchoring_benchmark/benchmark_aggregate.sh 512
# creates c256_s0 and c256_s256) plus an unrelated leaf proof at c256_s0 vs. itself.
# Usage: negative_tests.sh
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CH="$HERE/../reanchoring/target/chunks"
A="$CH/c256_s0"; B="$CH/c256_s256"
for d in "$A" "$B"; do [ -f "$d/proof" ] || { echo "missing $d -- run benchmark_aggregate.sh 512 first" >&2; exit 2; }; done
cd "$HERE"
cp -f Prover.toml /tmp/agg_Prover.toml.bak 2>/dev/null || true
trap 'cp -f /tmp/agg_Prover.toml.bak Prover.toml 2>/dev/null' EXIT

fail=0
expect() { # expect <accept|reject> <label> <override> <dir...>
  local want=$1 label=$2 ov=$3; shift 3
  AGG_OVERRIDE="$ov" python3 gen_prover_toml.py 4 "$@" > Prover.toml
  if nargo execute witness >/dev/null 2>&1; then got=accept; else got=reject; fi
  if [ "$got" = "$want" ]; then echo "ok   $label ($got)"; else echo "FAIL $label: wanted $want, got $got"; fail=1; fi
}

expect accept "two chained chunks"                          ""                          "$A" "$B"
expect reject "chunks in reverse order"                     ""                          "$B" "$A"
expect reject "same chunk twice (no continuation)"          ""                          "$A" "$A"
expect reject "claimed count != sum of chunk counts"        "count=999"                 "$A" "$B"
expect reject "claimed rt_new is the FIRST chunk's"         "rt_new=$(python3 -c "
b=open('$A/public_inputs','rb').read(); print(int.from_bytes(b[32:64],'big'))")" "$A" "$B"
expect reject "padding-extension: n_real=1 but claims chunk 2's rt_new/count" \
  "n_real=1,count=512,rt_new=$(python3 -c "
b=open('$B/public_inputs','rb').read(); print(int.from_bytes(b[32:64],'big'))")" "$A" "$B"
expect reject "n_real=0"                                    "n_real=0"                  "$A" "$B"
expect reject "n_real>K"                                    "n_real=5"                  "$A" "$B"
expect reject "wrong first_height"                          "first_height=7"            "$A" "$B"
expect reject "wrong validator_set_hash"                    "validator_set_hash=1"      "$A" "$B"
# Cross-chunk checks need a chunk that chains by rt but breaks height continuity / validator-set
# equality; build them from generate_chain.go's skew/key-seed env (slow: one leaf proof each).
LEAF="$HERE/../reanchoring"; GEN="$HERE/../../scripts/e6_zk_reanchoring_benchmark/generate_chain.go"
make_variant() { # make_variant <dir> <ENV=VAL>
  local d=$1; shift
  [ -f "$d/proof" ] && return
  mkdir -p "$d"
  cp -f "$LEAF/Prover.toml" /tmp/leaf_Prover.toml.bak 2>/dev/null || true
  (cd "$HERE/../../scripts/e6_zk_reanchoring_benchmark" && env "$@" go run "$GEN" 256 256 256 > "$LEAF/Prover.toml")
  (cd "$LEAF" && nargo execute witness >/dev/null 2>&1 \
    && bb prove -t noir-recursive -b target/reanchoring.json -w target/witness.gz -k target/leaf_rec/vk -o "$d" >/dev/null 2>&1)
  cp -f /tmp/leaf_Prover.toml.bak "$LEAF/Prover.toml" 2>/dev/null || true
}
make_variant "$CH/skew5" ENGRAM_FIXTURE_HEIGHT_SKEW=5
make_variant "$CH/altvals" ENGRAM_FIXTURE_KEY_SEED=alt
if [ -f "$CH/skew5/proof" ]; then
  expect reject "height gap between chunks"            "" "$A" "$CH/skew5"
else echo "FAIL could not build skew5 chunk"; fail=1; fi
if [ -f "$CH/altvals/proof" ]; then
  expect reject "different validator set in chunk 2"   "" "$A" "$CH/altvals"
else echo "FAIL could not build altvals chunk"; fail=1; fi
exit $fail
