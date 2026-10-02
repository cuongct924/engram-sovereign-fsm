#!/usr/bin/env bash
# E11 driver: for each comparison system, redeploy a fresh cluster in that mode, then run every
# fault for TRIALS trials. Run from the repo root with the prover available (testnet-up starts it).
#
#   scripts/e11_hard_gated_baseline/run_matrix.sh
#   MODES="hard-gated" TRIALS=1 N=4 scripts/e11_hard_gated_baseline/run_matrix.sh
#   WIPE_DATA=1 scripts/e11_hard_gated_baseline/run_matrix.sh   # also wipe bitcoin/celestia data
#
# Redeploys wipe testnet-data/ (testnet-up does). WIPE_DATA=1 additionally wipes data/bitcoin-node0*
# and data/celestia-* for a clean baseline; CLAUDE.md warns a bridge can lose its one-time
# TrustedHash race when wiped together with celestia-app, so it is opt-in.
set -euo pipefail

MODES="${MODES:-hard-gated engram soft}"
FAULTS="${FAULTS:-btc da both}"
TRIALS="${TRIALS:-3}"
N="${N:-7}"
HERE="$(cd "$(dirname "$0")" && pwd)"
cd "$HERE/../.."

export ENGRAM_VALIDATOR_COUNT="$N"

for mode in $MODES; do
	echo "=== E11: deploying a fresh $N-validator cluster in mode $mode ==="
	make testnet-down || true
	if [ "${WIPE_DATA:-0}" = "1" ]; then
		echo "--> WIPE_DATA=1: wiping data/bitcoin-node0* and data/celestia-*"
		rm -rf data/bitcoin-node0* data/celestia-*
	fi
	make testnet-up MODE="$mode" N="$N"
	for fault in $FAULTS; do
		python3 -u "$HERE/live_hard_gated_vs_engram.py" --mode "$mode" --fault "$fault" --trials "$TRIALS"
	done
done

python3 "$HERE/summarize_e11.py"
