#!/usr/bin/env bash
# Idempotent Celestia blob-submission account funding for one bridge
# container -- closes the gap CLAUDE.md's Troubleshooting section documents:
# "A celestia bridge's blob-submission account needs its own TIA funding --
# nothing does this automatically, and a zero balance fails DA silently
# (is_das_failed, no log line up to the FSM)." Confirmed live: an unfunded
# account makes every bridge.Submit call fail with "account ... not found",
# which feeds is_das_failed/is_attestation_failed=true into the real P2P/DA
# health sensor read every block -- keeping the FSM stuck in SOVEREIGN
# indefinitely (IsHealthyCondition requires IsDAHealthy), not just failing
# the optional witness-DA-publish audit trail.
#
# Verifies the funding tx actually landed (checks the broadcast response's
# own `code` field, not just the CLI's exit status, and re-queries the
# balance afterward) -- `celestia-appd tx ... --yes` exits 0 even when the
# tx itself fails on-chain (e.g. a wrong --chain-id gives "signature
# verification failed ... unauthorized" but a clean exit code), which
# silently left every bridge unfunded on a first pass live. Retries a few
# times before giving up loudly, since a silent failure here means the FSM
# never leaves SOVEREIGN with no obvious error pointing back to this step.
#
# Usage: scripts/testnet_fund_celestia_bridge.sh [bridge_container] [amount_utia]
#   bridge_container defaults to celestia-bridge-node01
#   amount_utia defaults to 1000000000 (1000 TIA) -- celestia-app's genesis
#   validator account holds ~995,000 TIA in this dev/regtest setup, so
#   "running out" isn't a real concern; each bridge only needs enough to
#   cover its own blob.Submit fees for the session (a few utia each).
set -euo pipefail

BRIDGE_CONTAINER="${1:-celestia-bridge-node01}"
AMOUNT_UTIA="${2:-1000000000}"
APP_CONTAINER="${CELESTIA_APP_CONTAINER:-celestia-app}"

query_balance() {
  # $1 = address. Prints the utia balance, or 0 if none/unparseable.
  docker exec "$APP_CONTAINER" celestia-appd query bank balances "$1" -o json 2>/dev/null \
    | python3 -c "import json,sys
try:
    d=json.load(sys.stdin)
    bals=d.get('balances',[])
    print(next((b['amount'] for b in bals if b['denom']=='utia'), '0'))
except Exception:
    print('0')"
}

echo "[testnet_fund_celestia_bridge] fetching $BRIDGE_CONTAINER's admin token + account address..."
TOKEN=""
for _ in $(seq 1 60); do
  TOKEN=$(docker exec "$BRIDGE_CONTAINER" celestia bridge auth admin --node.store /home/celestia 2>/dev/null || true)
  [ -n "$TOKEN" ] && break
  sleep 1
done
if [ -z "$TOKEN" ]; then
  echo "[testnet_fund_celestia_bridge] $BRIDGE_CONTAINER never produced an admin token -- is it healthy?" >&2
  exit 1
fi

ADDR=$(docker exec "$BRIDGE_CONTAINER" curl -s -X POST \
  -H "Content-Type: application/json" -H "Authorization: Bearer $TOKEN" \
  -d '{"jsonrpc":"2.0","id":1,"method":"state.AccountAddress","params":[]}' \
  http://localhost:26658 | python3 -c "import json,sys; print(json.load(sys.stdin)['result'])")
if [ -z "$ADDR" ]; then
  echo "[testnet_fund_celestia_bridge] could not resolve $BRIDGE_CONTAINER's account address" >&2
  exit 1
fi

EXISTING=$(query_balance "$ADDR")
if [ "${EXISTING:-0}" -gt 0 ] 2>/dev/null; then
  echo "[testnet_fund_celestia_bridge] $BRIDGE_CONTAINER's account $ADDR already funded ($EXISTING utia), skipping"
  exit 0
fi

GENESIS_ADDR=$(docker exec "$APP_CONTAINER" celestia-appd keys list --keyring-backend test --output json 2>/dev/null \
  | python3 -c "import json,sys; print(json.load(sys.stdin)[0]['address'])")

for attempt in $(seq 1 5); do
  CHAIN_ID=$(docker exec "$APP_CONTAINER" celestia-appd status 2>&1 | python3 -c "import json,sys; print(json.load(sys.stdin)['NodeInfo']['network'])" 2>/dev/null || true)
  if [ -z "$CHAIN_ID" ]; then
    echo "[testnet_fund_celestia_bridge] attempt $attempt/5: could not resolve $APP_CONTAINER's chain-id, retrying..." >&2
    sleep 2
    continue
  fi

  echo "[testnet_fund_celestia_bridge] attempt $attempt/5: funding $BRIDGE_CONTAINER's account $ADDR with $AMOUNT_UTIA utia from $GENESIS_ADDR (chain-id=$CHAIN_ID)..."
  # --broadcast-mode block: waits for the tx to actually land in a block
  # before returning, so the genesis account's sequence number is already
  # incremented by the time the NEXT bridge's funding call (a separate
  # script invocation) queries it -- sync mode (the default) returns once
  # mempool-accepted, which raced the next call's sequence lookup and
  # produced real "incorrect account sequence" (code 32) failures funding
  # 8 bridges back-to-back from the one genesis account live.
  TX_CODE=$(docker exec "$APP_CONTAINER" celestia-appd tx bank send "$GENESIS_ADDR" "$ADDR" "${AMOUNT_UTIA}utia" \
    --keyring-backend test --chain-id "$CHAIN_ID" --fees 5000utia --broadcast-mode block --yes --output json 2>/dev/null \
    | python3 -c "import json,sys
try:
    print(json.load(sys.stdin).get('code', 1))
except Exception:
    print(1)")

  if [ "${TX_CODE:-1}" = "0" ]; then
    sleep 2  # let the tx land in a block before re-querying the balance
    NEW_BALANCE=$(query_balance "$ADDR")
    if [ "${NEW_BALANCE:-0}" -gt 0 ] 2>/dev/null; then
      echo "[testnet_fund_celestia_bridge] $BRIDGE_CONTAINER funded at $ADDR ($NEW_BALANCE utia confirmed on-chain)"
      exit 0
    fi
    echo "[testnet_fund_celestia_bridge] attempt $attempt/5: tx broadcast with code=0 but balance still 0, retrying..." >&2
  else
    echo "[testnet_fund_celestia_bridge] attempt $attempt/5: tx failed with code=$TX_CODE, retrying..." >&2
  fi
  sleep 2
done

echo "[testnet_fund_celestia_bridge] FAILED to fund $BRIDGE_CONTAINER's account $ADDR after 5 attempts -- DA will stay broken (is_das_failed=true), FSM will stay stuck in SOVEREIGN. Check $APP_CONTAINER's chain-id/keyring manually." >&2
exit 1
