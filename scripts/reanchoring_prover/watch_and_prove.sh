#!/usr/bin/env bash
# Polls the real, live node's currently-tracked SOVEREIGN/RECOVERING header
# count (via `engramd query-recovery-headers`) and fires prove_and_submit.sh
# the INSTANT it equals EXPECTED_N (the circuit's fixed N=4) -- that window
# lasts exactly one block (the header count grows by 1 every WithdrawLocked
# block, and prove_and_submit.sh refuses at any other count), so this must
# react within one block interval, not be run by hand.
#
# Exits successfully (0) once a real proof has been submitted, or once
# --max-checks is exhausted without ever seeing exactly N headers.
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
ENGRAMD="${ENGRAMD_BIN:-$REPO_ROOT/engramd}"
NODE_URL="${NODE_URL:-http://127.0.0.1:26657}"
EXPECTED_N=4
POLL_INTERVAL_S="${POLL_INTERVAL_S:-2}"
MAX_CHECKS="${MAX_CHECKS:-600}"  # ~20 min at 2s/check

if [ ! -x "$ENGRAMD" ]; then
  echo "[watch_and_prove] building engramd..." >&2
  (cd "$REPO_ROOT" && go build -o "$ENGRAMD" ./cmd/engramd)
fi

echo "[watch_and_prove] watching $NODE_URL for exactly $EXPECTED_N tracked headers..."
i=0
while [ "$i" -lt "$MAX_CHECKS" ]; do
  i=$((i + 1))
  OUT=$("$ENGRAMD" query-recovery-headers --node "$NODE_URL" 2>&1)
  N=$(echo "$OUT" | grep -c '^height=' || true)
  TS=$(date -u +%H:%M:%S)
  echo "[$TS] check $i: N=$N"
  if [ "$N" -eq "$EXPECTED_N" ]; then
    echo "[watch_and_prove] N=$EXPECTED_N caught -- firing prove_and_submit.sh NOW"
    "$SCRIPT_DIR/prove_and_submit.sh"
    exit $?
  fi
  sleep "$POLL_INTERVAL_S"
done

echo "[watch_and_prove] gave up after $MAX_CHECKS checks without ever seeing exactly $EXPECTED_N headers." >&2
exit 1
