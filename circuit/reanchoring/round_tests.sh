#!/usr/bin/env bash
# Boundary-round tests for the leaf circuit: a witness whose precommits were signed in round r must
# be accepted when it claims round r, and rejected when it claims another round or carries a tampered
# boundary hash / validator-set hash. Each case is one `nargo execute` (minutes).
# Usage: round_tests.sh   (run from anywhere; writes Prover_rt_*.toml here, removed on exit)
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GEN="$HERE/../../scripts/e6_zk_reanchoring_benchmark/generate_chain.go"
cd "$HERE"
trap 'rm -f Prover_rt_*.toml' EXIT

# case <want> <label> <signed round> <claimed round> <sed expression or ''>
run_case() {
  local want=$1 label=$2 signed=$3 claimed=$4 edit=$5
  local name="rt_${label// /_}"
  (cd "$HERE/../.." && ENGRAM_FIXTURE_ROUND=$signed ENGRAM_FIXTURE_CLAIM_ROUND=$claimed go run "$GEN" 1 256) > "Prover_$name.toml" || { echo "FAIL $label: generator"; return; }
  [ -n "$edit" ] && sed -i.bak -E "$edit" "Prover_$name.toml" && rm -f "Prover_$name.toml.bak"
  if nargo execute --prover-name "Prover_$name" "w_$name" >/dev/null 2>&1; then got=accept; else got=reject; fi
  if [ "$got" = "$want" ]; then echo "ok   $label ($got)"; else echo "FAIL $label: wanted $want, got $got"; fi
}

run_case accept "signed round 0, claims 0"   0 0 "" &
run_case accept "signed round 1, claims 1"   1 1 "" &
wait
run_case accept "signed round 2, claims 2"   2 2 "" &
run_case reject "signed round 1, claims 2"   1 2 "" &
wait
run_case reject "signed round 1, claims 0"   1 0 "" &
run_case reject "signed round 0, claims 1"   0 1 "" &
wait
run_case reject "tampered last block hash"   1 1 's/^(last_block_hash_bytes = \[")0x[0-9a-f]+/\10x00/' &
run_case reject "tampered validator-set hash" 1 1 's/^(validator_set_hash_bytes = \[")0x[0-9a-f]+/\10x00/' &
wait
