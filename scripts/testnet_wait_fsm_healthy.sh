#!/usr/bin/env bash
# Waits for a validator's real per-block sensor read (the "sensor_snapshot"
# log line -- x/sovereignty/sensors_refresh.go's RefreshMetrics, printed by
# cmd/engramd) to show BTC/DA healthy, before declaring `testnet-up` done.
#
# Deliberately does NOT use `engramd query-state`: its committed Metrics
# snapshot lags the real per-block sensor read for BTC/DA fields (confirmed
# live: query-state reported is_btc_spv_failed=false while the same
# validator's own sensor_snapshot log line, moments later, still read true)
# and never carries P2P fields at all (CleanPeers/SubnetDiversity/
# ActiveAnchors are never written into committed state -- see
# scripts/framework/logger.py's net_info doc). The log line is the only
# reliable, real-time source found this session.
#
# Catches both known deploy-time causes of a cluster stuck in SOVEREIGN
# indefinitely (IsHealthyCondition requires ALL of BTC/DA/P2P healthy):
#   - an unfunded celestia bridge blob-submission account (da_failed=true,
#     see testnet_fund_celestia_bridge.sh, now run automatically above)
#   - a real BTC checkpoint mismatch, e.g. after Bitcoin mining was disrupted
#     (CLAUDE.md: "Bitcoin regtest must mine continuously")
# A fresh deploy legitimately starts SOVEREIGN and stays there until
# safe_blocks/hysteresis is satisfied -- this script only confirms the
# SENSORS are healthy (fast, meaningful signal a deploy succeeded cleanly),
# not that the FSM has already reached ANCHORED (which can take longer and
# isn't this script's job).
#
# Usage: scripts/testnet_wait_fsm_healthy.sh [validator_container] [timeout_s]
set -euo pipefail

CONTAINER="${1:-engram-node01}"
TIMEOUT="${2:-180}"

echo "[testnet_wait_fsm_healthy] waiting for $CONTAINER's own sensor_snapshot to show BTC/DA healthy (up to ${TIMEOUT}s)..."
elapsed=0
last_line=""
while [ "$elapsed" -lt "$TIMEOUT" ]; do
  last_line=$(docker logs "$CONTAINER" --tail 500 2>&1 | grep "sensor_snapshot" | tail -1 || true)
  if [ -n "$last_line" ]; then
    if echo "$last_line" | grep -q "btc_spv_failed=false" && \
       echo "$last_line" | grep -q "da_failed=false" && \
       echo "$last_line" | grep -q "attestation_failed=false"; then
      echo "[testnet_wait_fsm_healthy] $CONTAINER's sensors are healthy after ${elapsed}s: $last_line"
      exit 0
    fi
  fi
  sleep 5
  elapsed=$((elapsed + 5))
done

echo "[testnet_wait_fsm_healthy] $CONTAINER's sensors did NOT report healthy within ${TIMEOUT}s -- last reading: ${last_line:-none}" >&2
echo "[testnet_wait_fsm_healthy] the FSM will stay stuck in SOVEREIGN until this clears. Check CLAUDE.md's Troubleshooting section for known causes (unfunded celestia bridge account, disrupted BTC mining cadence)." >&2
exit 1
