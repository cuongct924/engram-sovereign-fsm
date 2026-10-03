#!/usr/bin/env bash
# RQ4 like-for-like baseline: the same 7-validator Docker cluster, load and chain, alternating between
# engram and vanilla (ENGRAM_MODE=vanilla: every validator started --vanilla, no Engram ABCI++ hooks).
# Alternating rounds keeps slow drift of the shared Bitcoin/Celestia stack from landing on one arm.
#
# Needs a healthy engram cluster (`make testnet-up N=7`) in ANCHORED. Vote extensions stay enabled in the
# genesis, so this isolates the cost of Engram's handlers (ExtendVote, PrepareProposal, ProcessProposal,
# PreBlocker); it does NOT remove the vote-extension protocol itself. For that, see run_vanilla_pure.sh.
#
#   ROUNDS=3 REPEATS=3 scripts/e7_consensus_overhead/run_vanilla7_ab.sh
#
# Each arm writes results_live/rq4_{engram,vanilla7}_<ts>*.csv; `engram` also records the measured
# ExtendVote duration (extend_vote_ms_*), the direct test of the vote-extension hypothesis.
set -euo pipefail

ROUNDS="${ROUNDS:-3}"
REPEATS="${REPEATS:-3}"
N="${N:-7}"
HERE="$(cd "$(dirname "$0")" && pwd)"
cd "$HERE/../.."
PY="${PY:-.venv/bin/python}"

services=$(for i in $(seq 1 "$N"); do printf 'engram-node%02d ' "$i"; done)

switch_mode() { # $1 = engram|vanilla
	echo "=== switching all $N validators to ENGRAM_MODE=$1 ==="
	ENGRAM_MODE="$1" docker compose --env-file .env -f docker-compose.yml --profile seven-node \
		up -d --no-deps --force-recreate $services
	"$PY" - "$1" <<'PY'
import sys, time
sys.path.insert(0, "scripts/framework")
from logger import sample_all_nodes
want = sys.argv[1]
deadline = time.time() + 600
last = None
while time.time() < deadline:
    s = sample_all_nodes()
    ok = all(x.height > 0 for x in s)
    # engram must be ANCHORED again; vanilla has no FSM progress to wait for, only blocks advancing.
    if ok and last is not None and all(x.height > p for x, p in zip(s, last)):
        if want == "vanilla" or all(x.fsm_state == "ANCHORED" for x in s):
            sys.exit(0)
    last = [x.height for x in s] if ok else None
    time.sleep(10)
sys.exit("cluster did not settle after the mode switch")
PY
	sleep 30
}

for r in $(seq 1 "$ROUNDS"); do
	for mode in engram vanilla; do
		switch_mode "$mode"
		arm=$([ "$mode" = engram ] && echo engram || echo vanilla7)
		echo "=== round $r: $arm ==="
		"$PY" -u scripts/e7_consensus_overhead/live_rq4_metrics.py --arm "$arm" --repeats "$REPEATS"
	done
done
switch_mode engram
