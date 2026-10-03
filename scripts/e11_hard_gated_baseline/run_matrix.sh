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
FIRST_TRIAL="${FIRST_TRIAL:-1}"
N="${N:-7}"
HERE="$(cd "$(dirname "$0")" && pwd)"
cd "$HERE/../.."

export ENGRAM_VALIDATOR_COUNT="$N"

deploy_mode() { # $1 = mode: fresh cluster, engram first, then switch the validators' mode (state kept)
	local mode="$1"
	echo "=== E11: deploying a fresh $N-validator cluster in mode $mode ==="
	# Clean redeploy (wipes the Bitcoin and Celestia data too): a genesis against a Bitcoin regtest that has kept
	# mining starts with a btc_gap of hundreds and never reaches ANCHORED within the readiness timeout.
	# A hard-gated chain cannot start from genesis: with the FSM pinned to ANCHORED, block 1 needs a DA
	# receipt that does not exist yet, so no proposal ever validates. Every mode therefore starts as
	# engram, reaches a healthy ANCHORED, and only then is the validators' mode switched (state kept).
	"$HERE/../testnet_redeploy_clean.sh" engram || echo "--> redeploy failed"
	python3 - <<'PY'
import sys
sys.path.insert(0, "scripts/framework")
from logger import sample_all_nodes
import time
deadline = time.time() + 1800
while time.time() < deadline:
    s = sample_all_nodes()
    if s and all(x.fsm_state == "ANCHORED" for x in s):
        sys.exit(0)
    time.sleep(5)
sys.exit("cluster never reached ANCHORED in engram mode")
PY
	if [ "$mode" != "engram" ]; then
		echo "--> switching validators to $mode (state kept)"
		svcs=$(for i in $(seq 1 "$N"); do printf 'engram-node%02d ' "$i"; done)
		ENGRAM_MODE="$mode" docker compose --env-file .env -f docker-compose.yml --profile seven-node up -d --no-deps --force-recreate $svcs
		sleep 45
	fi
}

for mode in $MODES; do
	first=1
	for fault in $FAULTS; do
		# A hard-gated chain that stalls under a fault does not resume by itself after the fault ends (the
		# fresh checkpoint it needs is derived from blocks it can no longer commit), so every fault, and
		# every trial, starts from a fresh deployment. The other modes recover and share one deployment.
		if [ "$first" = "1" ] || [ "$mode" = "hard-gated" ]; then
			deploy_mode "$mode" || true
			first=0
		fi
		if [ "$mode" = "hard-gated" ]; then
			for t in $(seq "$FIRST_TRIAL" "$TRIALS"); do
				python3 -u "$HERE/live_hard_gated_vs_engram.py" --mode "$mode" --fault "$fault" --first-trial "$t" --trials "$t" || true
				[ "$t" -lt "$TRIALS" ] && { deploy_mode "$mode" || true; }
			done
		else
			python3 -u "$HERE/live_hard_gated_vs_engram.py" --mode "$mode" --fault "$fault" --first-trial "$FIRST_TRIAL" --trials "$TRIALS" --ready-timeout-s 1800
		fi
	done
done

python3 "$HERE/summarize_e11.py"
