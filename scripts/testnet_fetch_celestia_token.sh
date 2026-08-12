#!/usr/bin/env bash
# Pulls a celestia-bridge container's admin/write JWT (generated on first
# `bridge init`, written to /home/celestia/bridge_auth.txt inside the
# container) and writes it into the given .env file's VAR_NAME= line -- all
# 4 validators need CELESTIA_BRIDGE_AUTH_TOKEN in their environment before
# they start (see docker/engram-validator-cluster.yml's doc);
# reanchoring-prover needs CELESTIA_BRIDGE2_AUTH_TOKEN for its own separate
# celestia-bridge-2 (docker/celestia-local-cluster.yml's doc).
#
# Usage: scripts/testnet_fetch_celestia_token.sh [env_file] [container] [var_name]
set -euo pipefail

ENV_FILE="${1:-.env}"
CONTAINER="${2:-${CELESTIA_BRIDGE_CONTAINER:-celestia-bridge}}"
VAR_NAME="${3:-CELESTIA_BRIDGE_AUTH_TOKEN}"

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

if grep -q "^${VAR_NAME}=" "$ENV_FILE"; then
  # -i.bak suffix form works identically on both BSD (macOS) and GNU sed.
  sed -i.bak "s#^${VAR_NAME}=.*#${VAR_NAME}=${TOKEN}#" "$ENV_FILE"
  rm -f "$ENV_FILE.bak"
else
  # A missing trailing newline on the file's last existing line would
  # otherwise merge onto it (confirmed live: silently produced
  # "...=8CELESTIA_BRIDGE2_AUTH_TOKEN=..." as ONE commented-out line,
  # dropping the new var entirely).
  [ -s "$ENV_FILE" ] && [ "$(tail -c1 "$ENV_FILE")" != "" ] && echo >> "$ENV_FILE"
  echo "${VAR_NAME}=${TOKEN}" >> "$ENV_FILE"
fi
echo "[testnet_fetch_celestia_token] wrote $VAR_NAME into $ENV_FILE"
