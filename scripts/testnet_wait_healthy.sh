#!/usr/bin/env bash
# Waits for a container's Docker healthcheck to report "healthy" -- used
# before starting the 4 validators so celestia-bridge is actually able to
# serve DA reads/submits, not just running. testnet_fetch_celestia_token.sh's
# wait (bridge_auth.txt exists) fires within seconds of container start, long
# before header.NetworkHead RPC calls succeed; validators starting on that
# weaker signal reliably see da_gap grow past MaxSuspiciousTime (24 blocks,
# ~40-50s at this cluster's cadence) before the bridge is ready, forcing an
# unwanted SOVEREIGN escalation on every fresh deploy.
#
# Usage: scripts/testnet_wait_healthy.sh <container> [timeout_s]
set -euo pipefail

CONTAINER="$1"
TIMEOUT="${2:-150}"

echo "[testnet_wait_healthy] waiting for $CONTAINER to report healthy (up to ${TIMEOUT}s)..."
elapsed=0
status="unknown"
while [ "$elapsed" -lt "$TIMEOUT" ]; do
  status=$(docker inspect --format='{{.State.Health.Status}}' "$CONTAINER" 2>/dev/null || echo "unknown")
  if [ "$status" = "healthy" ]; then
    echo "[testnet_wait_healthy] $CONTAINER is healthy after ${elapsed}s"
    exit 0
  fi
  sleep 2
  elapsed=$((elapsed + 2))
done
echo "[testnet_wait_healthy] $CONTAINER did not become healthy within ${TIMEOUT}s (last status: $status)" >&2
exit 1
