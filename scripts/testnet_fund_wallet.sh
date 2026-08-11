#!/usr/bin/env bash
# Idempotent Bitcoin regtest wallet setup for the local docker testnet --
# creates + funds "engramwallet" (bitcoin_miner_loop.sh's hardcoded default
# wallet name) with 101 blocks (100 confirmations needed before a coinbase
# output is spendable), but only mines on first creation. Re-running against
# an already-funded wallet must not burst-mine more blocks while engramd may
# already be running -- see bitcoin_miner_loop.sh's own doc for why that
# desyncs h_btc_current and stalls consensus.
#
# Usage: scripts/testnet_fund_wallet.sh [wallet_name]
set -euo pipefail

WALLET="${1:-engramwallet}"
CONTAINER="${BITCOIN_CONTAINER:-bitcoin-node01}"
RPC_USER="${BITCOIN_RPC_USER:?BITCOIN_RPC_USER must be set (see .env)}"
RPC_PASSWORD="${BITCOIN_RPC_PASSWORD:?BITCOIN_RPC_PASSWORD must be set (see .env)}"

btc() {
  docker exec "$CONTAINER" bitcoin-cli -regtest -rpcuser="$RPC_USER" -rpcpassword="$RPC_PASSWORD" "$@"
}

echo "[testnet_fund_wallet] waiting for $CONTAINER RPC..."
for _ in $(seq 1 60); do
  btc getblockcount >/dev/null 2>&1 && break
  sleep 1
done
btc getblockcount >/dev/null

if btc -rpcwallet="$WALLET" getwalletinfo >/dev/null 2>&1; then
  echo "[testnet_fund_wallet] $WALLET already loaded, skipping create+fund"
  exit 0
fi

if btc loadwallet "$WALLET" >/dev/null 2>&1; then
  echo "[testnet_fund_wallet] $WALLET already existed on disk, loaded (not re-mining)"
  exit 0
fi

echo "[testnet_fund_wallet] creating $WALLET and mining 101 blocks (100 confirmations for coinbase spendability)"
btc createwallet "$WALLET" >/dev/null
ADDR=$(btc -rpcwallet="$WALLET" getnewaddress)
btc -rpcwallet="$WALLET" generatetoaddress 101 "$ADDR" >/dev/null
echo "[testnet_fund_wallet] $WALLET funded at $ADDR"
