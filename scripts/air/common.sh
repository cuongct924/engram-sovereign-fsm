#!/usr/bin/env bash
# Shared helpers for the runs on the second machine (see docs/AIR_CLUSTER_PLAN.md). Source from the repo root.
set -uo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../.."
export PATH="$PWD/.venv/bin:$PATH"
LOGDIR="${LOGDIR:-$HOME/engram_air_logs}"
mkdir -p "$LOGDIR"
STATUS="$LOGDIR/status.txt"
say() { echo "$(date +%H:%M) $*" | tee -a "$STATUS"; }

# Fresh genesis and a clean cluster, then wait until every validator is ANCHORED (up to 25 min).
fresh() {
  scripts/testnet_redeploy_clean.sh "${1:-engram}" > "$LOGDIR/redeploy.log" 2>&1
  python3 - <<'PY'
import sys, time
sys.path.insert(0, "scripts/framework")
from logger import sample_all_nodes
d = time.time() + 1500
while time.time() < d:
    s = sample_all_nodes()
    if s and all(x.fsm_state == "ANCHORED" for x in s):
        sys.exit(0)
    time.sleep(10)
sys.exit(1)
PY
  say "$2 anchored exit=$?"
}
