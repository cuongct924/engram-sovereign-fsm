#!/usr/bin/env bash
# Pulls celestia-bridge's admin/write JWT (generated on first `bridge init`,
# written to /home/celestia/bridge_auth.txt inside the container) and writes
# it into the given .env file's CELESTIA_BRIDGE_AUTH_TOKEN= line -- all 4
# validators need this in their environment before they start (see
# docker/engram-validator-cluster.yml's CELESTIA_BRIDGE_AUTH_TOKEN doc).
#
# Usage: scripts/testnet_fetch_celestia_token.sh [env_file]
set -euo pipefail

ENV_FILE="${1:-.env}"
CONTAINER="${CELESTIA_BRIDGE_CONTAINER:-celestia-bridge}"

echo "[testnet_fetch_celestia_token] waiting for $CONTAINER to write bridge_auth.txt..."
TOKEN=""
for _ in $(seq 1 60); do
  TOKEN=$(docker exec "$CONTAINER" cat /home/celestia/bridge_auth.txt 2>/dev/null || true)
  [ -n "$TOKEN" ] && break
  sleep 1
done
if [ -z "$TOKEN" ]; then
  echo "[testnet_fetch_celestia_token] $CONTAINER never wrote bridge_auth.txt -- is it healthy?" >&2
  exit 1
fi

if grep -q '^CELESTIA_BRIDGE_AUTH_TOKEN=' "$ENV_FILE"; then
  # -i.bak suffix form works identically on both BSD (macOS) and GNU sed.
  sed -i.bak "s#^CELESTIA_BRIDGE_AUTH_TOKEN=.*#CELESTIA_BRIDGE_AUTH_TOKEN=${TOKEN}#" "$ENV_FILE"
  rm -f "$ENV_FILE.bak"
else
  echo "CELESTIA_BRIDGE_AUTH_TOKEN=${TOKEN}" >> "$ENV_FILE"
fi
echo "[testnet_fetch_celestia_token] wrote token into $ENV_FILE"
