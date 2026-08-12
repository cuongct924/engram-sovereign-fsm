#!/usr/bin/env bash
# Steady regtest miner for local docker testnet development.
#
# bitcoind regtest never mines on its own -- AnchorTracker's OP_RETURN
# checkpoints (x/anchor/anchor.go) need real confirmations to reach
# kDeepFinality (default 2), and anchor.VerifyReceipt's tolerance window
# at that setting is only kDeepFinality blocks wide (see verify.go's
# Tolerance doc). Mining in irregular manual bursts (e.g. 5 blocks at once
# to fund a wallet) advances h_btc_current far ahead of whatever checkpoint
# height a proposal is carrying by the time it's cross-validated a few
# consensus rounds later, permanently pushing it outside that tolerance --
# confirmed empirically: this is what stalled the 4-node testnet at height 1
# for good after a burst-mined wallet funding step, not a code bug.
#
# One block roughly every ROUND_INTERVAL_S keeps h_btc_current advancing
# close to consensus's own round cadence, so a just-confirmed checkpoint
# stays inside the tolerance window by the time it's used.
#
# Usage: scripts/bitcoin_miner_loop.sh [interval_seconds]
set -euo pipefail

INTERVAL="${1:-20}"
RPC_USER="${BITCOIN_RPC_USER:-cuongct}"
RPC_PASSWORD="${BITCOIN_RPC_PASSWORD:-cuongct123}"
WALLET="${BITCOIN_WALLET:-engramwallet}"
CONTAINER="${BITCOIN_CONTAINER:-bitcoin-node01}"

ADDR=$(docker exec "$CONTAINER" bitcoin-cli -regtest -rpcuser="$RPC_USER" -rpcpassword="$RPC_PASSWORD" -rpcwallet="$WALLET" getnewaddress)
echo "[bitcoin_miner_loop] mining 1 block every ${INTERVAL}s to $ADDR on $CONTAINER"

while true; do
  docker exec "$CONTAINER" bitcoin-cli -regtest -rpcuser="$RPC_USER" -rpcpassword="$RPC_PASSWORD" -rpcwallet="$WALLET" \
    generatetoaddress 1 "$ADDR" > /dev/null
  sleep "$INTERVAL"
done
