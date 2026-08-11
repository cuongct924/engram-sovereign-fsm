#!/usr/bin/env bash
# Manual on/off toggle for the duplicate-key double-signing harness
# (docker/engram-node04-double-sign.yml) -- stages node04's real
# priv_validator_key.json + the shared genesis.json into a FRESH identity
# directory (own priv_validator_state.json, deliberately not shared with
# the real node04 -- see that compose file's own doc for why), resolves
# persistent_peers to the 3 honest validators' real node IDs, then starts
# or stops the profile-gated engram-node04-duplicate service.
#
# For the fully automated E8 test (staging + start + poll for
# "SLASHABLE EVIDENCE DETECTED" + stop), use
# scripts/e8_attack_resilience/live_double_signing_test.py instead -- this
# script is the manual/interactive equivalent behind `make double-sign-on/off`.
#
# Usage: scripts/testnet_double_sign_toggle.sh on|off
set -euo pipefail

ACTION="${1:?Usage: $0 on|off}"
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DUP_HOME="$REPO_ROOT/testnet-data/engram-node04-duplicate"
REAL_KEY="$REPO_ROOT/testnet-data/engram-node04/config/priv_validator_key.json"
REAL_GENESIS="$REPO_ROOT/testnet-data/engram-node01/config/genesis.json"

if [ "$ACTION" = "off" ]; then
  echo "[testnet_double_sign_toggle] stopping engram-node04-duplicate"
  docker compose --profile double-sign-harness stop engram-node04-duplicate
  docker compose --profile double-sign-harness rm -f engram-node04-duplicate
  exit 0
fi

if [ "$ACTION" != "on" ]; then
  echo "Usage: $0 on|off" >&2
  exit 1
fi

[ -f "$REAL_KEY" ] || { echo "missing $REAL_KEY -- is the real cluster up? (make testnet-up)" >&2; exit 1; }
[ -f "$REAL_GENESIS" ] || { echo "missing $REAL_GENESIS" >&2; exit 1; }

echo "[testnet_double_sign_toggle] staging fresh identity at $DUP_HOME"
rm -rf "$DUP_HOME/config" "$DUP_HOME/data"
mkdir -p "$DUP_HOME/config" "$DUP_HOME/data"
cp "$REAL_KEY" "$DUP_HOME/config/priv_validator_key.json"
cp "$REAL_GENESIS" "$DUP_HOME/config/genesis.json"
# Fresh FilePV state -- deliberately NOT copied from the real node04's own,
# already-advanced state file (the entire point of this harness).
printf '{"height":"0","round":0,"step":0}' > "$DUP_HOME/data/priv_validator_state.json"

echo "[testnet_double_sign_toggle] resolving honest validators' real node IDs"
declare -A RPC_PORTS=([engram-node01]=26657 [engram-node02]=26757 [engram-node03]=26857)
peers=""
for host in engram-node01 engram-node02 engram-node03; do
  port="${RPC_PORTS[$host]}"
  id=$(curl -s "http://localhost:${port}/status" | jq -r .result.node_info.id)
  if [ -z "$id" ] || [ "$id" = "null" ]; then
    echo "could not resolve node id for $host (localhost:$port) -- is it up?" >&2
    exit 1
  fi
  peers="${peers:+$peers,}${id}@${host}:26656"
done
echo "[testnet_double_sign_toggle] DUPLICATE_PERSISTENT_PEERS=$peers"

DUPLICATE_PERSISTENT_PEERS="$peers" docker compose --profile double-sign-harness up -d engram-node04-duplicate
echo "[testnet_double_sign_toggle] engram-node04-duplicate is up -- watch for real"
echo "  \"SLASHABLE EVIDENCE DETECTED\" via: docker logs -f engram-node01"
