#!/usr/bin/env bash
# Steady regtest miner for local docker testnet development.
#
# bitcoind regtest never mines on its own -- AnchorTracker's OP_RETURN
# checkpoints (x/anchor/anchor.go) need real confirmations to reach
# kDeepFinality (default 2), and VerifyReceipt's tolerance window at that
# setting is only kDeepFinality blocks wide (verify.go's Tolerance doc).
# Burst-mining (e.g. 5 blocks to fund a wallet) advances h_btc_current past
# whatever checkpoint a proposal carries when cross-validated, permanently
# pushing it outside that tolerance. One block every ROUND_INTERVAL_S keeps
# h_btc_current close to consensus's cadence so a just-confirmed checkpoint
# stays inside the window.
#
# Talks to bitcoind over RPC directly (-rpcconnect), not `docker exec`, so
# it can run as a container (docker/bitcoin-miner-loop.yml) with no socket
# mount. POSIX sh only: the lncm/bitcoind image (Alpine) has busybox ash,
# no arrays or `set -o pipefail`.
#
# OVERRIDE_FILE lets a fault-injection script (e.g. E2's S2 BTC-congestion)
# slow real BTC confirmation mid-run -- read fresh every loop iteration,
# mirroring the fresh-every-call Reachable checks in x/anchor/x/da.
#
# Usage: scripts/bitcoin_miner_loop.sh [interval_seconds]
set -eu

INTERVAL="${1:-20}"
OVERRIDE_FILE="${MINER_INTERVAL_OVERRIDE_FILE:-/tmp/miner_interval_override}"
RPC_USER="${BITCOIN_RPC_USER:-cuongct}"
RPC_PASSWORD="${BITCOIN_RPC_PASSWORD:-cuongct123}"
WALLET="${BITCOIN_WALLET:-engramwallet}"
RPC_HOST="${BITCOIN_RPC_HOST:-bitcoin-node01}"
RPC_PORT="${BITCOIN_RPC_PORT:-18443}"

CLI_ARGS="-regtest -rpcconnect=$RPC_HOST -rpcport=$RPC_PORT -rpcuser=$RPC_USER -rpcpassword=$RPC_PASSWORD -rpcwallet=$WALLET"

echo "[bitcoin_miner_loop] waiting for $RPC_HOST:$RPC_PORT RPC and wallet $WALLET..."
until bitcoin-cli $CLI_ARGS getwalletinfo >/dev/null 2>&1; do
  sleep 2
done

ADDR=$(bitcoin-cli $CLI_ARGS getnewaddress)
echo "[bitcoin_miner_loop] mining 1 block every ${INTERVAL}s to $ADDR on $RPC_HOST"

while true; do
  bitcoin-cli $CLI_ARGS generatetoaddress 1 "$ADDR" > /dev/null

  # Sleeps in 1s steps, re-reading OVERRIDE_FILE on every tick, rather than
  # one single `sleep "$CUR_INTERVAL"` -- a single long sleep can't react to
  # the override file being REMOVED mid-sleep (a large override, e.g. 99999s
  # to fully pause mining for a reorg test, commits the process to a ~27hr
  # sleep that a later `rm -f` can't interrupt). Confirmed live: E10's
  # reorg-test pause/resume left bitcoin-miner-loop stuck not mining for the
  # remainder of that stale sleep, well after resume had already run.
  elapsed=0
  while true; do
    target="$INTERVAL"
    if [ -f "$OVERRIDE_FILE" ]; then
      OVERRIDE="$(cat "$OVERRIDE_FILE" 2>/dev/null || true)"
      case "$OVERRIDE" in
        ''|*[!0-9]*) : ;; # not a positive integer -- ignore, keep target
        *) target="$OVERRIDE" ;;
      esac
    fi
    if [ "$elapsed" -ge "$target" ]; then
      break
    fi
    sleep 1
    elapsed=$((elapsed + 1))
  done
done
