#!/usr/bin/env bash
# Funds one Celestia account from celestia-app's genesis account. Runs on the infra host, where
# the celestia-app container lives. Idempotent: skips an account that already has a balance.
# Same logic as scripts/testnet_fund_celestia_bridge.sh, minus the bridge-container lookups.
#
# Usage: fund_address.sh <celestia1...address> [amount_utia]
set -euo pipefail

ADDR="${1:?usage: fund_address.sh <address> [amount_utia]}"
AMOUNT_UTIA="${2:-1000000000}"
APP="${CELESTIA_APP_CONTAINER:-celestia-app}"

balance() {
  docker exec "$APP" celestia-appd query bank balances "$1" -o json 2>/dev/null \
    | jq -r '[.balances[]? | select(.denom=="utia") | .amount][0] // "0"'
}

if [ "$(balance "$ADDR")" != "0" ]; then
  echo "$ADDR already funded, skipping"
  exit 0
fi

GENESIS_ADDR=$(docker exec "$APP" celestia-appd keys list --keyring-backend test --output json | jq -r '.[0].address')

for attempt in 1 2 3 4 5; do
  CHAIN_ID=$(docker exec "$APP" celestia-appd status 2>&1 | jq -r '.NodeInfo.network' 2>/dev/null || true)
  if [ -z "$CHAIN_ID" ] || [ "$CHAIN_ID" = "null" ]; then
    sleep 2
    continue
  fi
  # --broadcast-mode block: the genesis account's sequence number is already incremented when the
  # next account's funding starts (sync mode raced it, see testnet_fund_celestia_bridge.sh).
  CODE=$(docker exec "$APP" celestia-appd tx bank send "$GENESIS_ADDR" "$ADDR" "${AMOUNT_UTIA}utia" \
    --keyring-backend test --chain-id "$CHAIN_ID" --fees 5000utia --broadcast-mode block --yes --output json 2>/dev/null \
    | jq -r '.code // 1' 2>/dev/null || echo 1)
  if [ "$CODE" = "0" ]; then
    sleep 2
    if [ "$(balance "$ADDR")" != "0" ]; then
      echo "funded $ADDR"
      exit 0
    fi
  fi
  echo "attempt $attempt/5 failed (code=$CODE), retrying" >&2
  sleep 2
done

echo "FAILED to fund $ADDR: the FSM would stay in SOVEREIGN. Check celestia-app's chain-id and keyring." >&2
exit 1
