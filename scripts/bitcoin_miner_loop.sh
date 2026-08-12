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
# Talks to bitcoind over RPC directly (-rpcconnect), not `docker exec` --
# lets this run as a container itself (docker/bitcoin-miner-loop.yml)
# without a docker socket mount.
#
# POSIX sh, not bash: the image it also runs inside (lncm/bitcoind, Alpine)
# has no bash, only busybox ash -- no arrays, no `set -o pipefail`.
#
# OVERRIDE_FILE lets a fault-injection script (e.g. E2's S2 BTC-congestion
# phase) genuinely slow real BTC confirmation mid-run without restarting
# this container -- read fresh every loop iteration, mirroring this
# session's other "fresh-every-call" reachability checks (x/anchor/x/da's
# Reachable). A plain --duration'd sleep-longer command wouldn't do this:
# netem/RPC-level delay (chaos-btc-delay) only slows queries against an
# otherwise-unaffected bitcoind, not the actual confirmation cadence
# btc_gap is computed from (confirmed live: zero FSM effect from 500ms
# netem jitter against a 20s natural mining cadence).
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
