#!/usr/bin/env bash
# RQ4 pure-vanilla CometBFT arm: a FRESH 7-validator deploy whose genesis has vote extensions DISABLED
# (ENGRAM_DISABLE_VOTE_EXTENSIONS=1) and whose validators run --vanilla (ENGRAM_MODE=vanilla), so there is
# no vote-extension protocol and no Engram handler. Comparing it with run_vanilla7_ab.sh's vanilla arm
# (extensions on, empty) separates the protocol's cost from Engram's handlers'.
#
# Wipes testnet-data/ like any redeploy. The engram arm must be re-deployed afterwards (`make testnet-up`).
# testnet-up's final sensor-health wait cannot succeed here (vanilla never logs sensor_snapshot), so its
# failure is tolerated and blocks advancing is checked directly.
#
#   scripts/e7_consensus_overhead/run_vanilla_pure.sh
set -uo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
cd "$HERE/../.."
PY="${PY:-.venv/bin/python}"

export ENGRAM_DISABLE_VOTE_EXTENSIONS=1
make testnet-up MODE=vanilla N=7 || echo "--> testnet-up's sensor wait failed (expected in vanilla); checking blocks directly"

"$PY" - <<'PY' || exit 1
import sys, time
sys.path.insert(0, "scripts/framework")
from logger import sample_all_nodes
prev, deadline = None, time.time() + 600
while time.time() < deadline:
    s = sample_all_nodes()
    cur = [x.height for x in s]
    if all(h > 0 for h in cur) and prev and all(c > p for c, p in zip(cur, prev)):
        sys.exit(0)
    prev = cur if all(h > 0 for h in cur) else None
    time.sleep(10)
sys.exit("pure-vanilla cluster is not producing blocks")
PY
sleep 30
"$PY" -u scripts/e7_consensus_overhead/live_rq4_metrics.py --arm vanilla7 --repeats "${REPEATS:-5}"
