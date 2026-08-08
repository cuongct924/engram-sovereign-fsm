#!/usr/bin/env bash
# Polls the real, live node's currently-tracked SOVEREIGN/RECOVERING header
# count (via `engramd query-recovery-headers`) and fires prove_and_submit.sh
# the INSTANT it equals EXPECTED_N (the circuit's fixed N=4) -- that window
# lasts exactly one block (the header count grows by 1 every WithdrawLocked
# block, and prove_and_submit.sh refuses at any other count), so this must
# react within one block interval, not be run by hand.
#
# Keeps running after a successful submission rather than exiting -- see
# x/sovereignty/keeper/msg_server.go's SubmitRecoveryProof doc on rolling
# checkpoints: LastAnchoredRoot now advances (and HeaderHistory prunes only
# the covered prefix) on EVERY accepted proof, not just once at the true end
# of the interval, so the tracked count legitimately drops back near 0 and
# starts climbing again for the NEXT segment. A long unhealthy interval
# therefore needs this watcher to catch N=4 repeatedly, not once -- exiting
# after the first hit (the previous behavior) meant no further segments of
# a still-open interval could ever be proven, exactly the gap that made the
# fixed-N circuit unusable against a real, unbounded-length interval.
#
# Exits successfully (0) once --max-checks is exhausted (the natural way to
# stop watching, e.g. once the interval has genuinely ended), or non-zero if
# it never saw exactly N headers even once in that budget. A rejected
# submission (prove_and_submit.sh's documented staleness race, see its own
# header comment) logs a warning and keeps watching rather than aborting.
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

echo "[watch_and_prove] watching $NODE_URL for exactly $EXPECTED_N tracked headers (repeating, rolling-checkpoint mode)..."
i=0
proofs_submitted=0
proofs_rejected=0
while [ "$i" -lt "$MAX_CHECKS" ]; do
  i=$((i + 1))
  OUT=$("$ENGRAMD" query-recovery-headers --node "$NODE_URL" 2>&1)
  N=$(echo "$OUT" | grep -c '^height=' || true)
  TS=$(date -u +%H:%M:%S)
  echo "[$TS] check $i: N=$N (proofs so far: $proofs_submitted submitted, $proofs_rejected rejected)"
  if [ "$N" -eq "$EXPECTED_N" ]; then
    echo "[watch_and_prove] N=$EXPECTED_N caught -- firing prove_and_submit.sh NOW"
    if "$SCRIPT_DIR/prove_and_submit.sh"; then
      proofs_submitted=$((proofs_submitted + 1))
      echo "[watch_and_prove] proof #$proofs_submitted accepted -- checkpoint advanced, continuing to watch for the next segment."
    else
      proofs_rejected=$((proofs_rejected + 1))
      echo "[watch_and_prove] proof rejected (likely the documented staleness race -- interval grew before submission landed, see prove_and_submit.sh's doc) -- continuing to watch." >&2
    fi
  fi
  sleep "$POLL_INTERVAL_S"
done

echo "[watch_and_prove] stopped after $MAX_CHECKS checks: $proofs_submitted proof(s) submitted, $proofs_rejected rejected."
if [ "$proofs_submitted" -eq 0 ]; then
  exit 1
fi
exit 0
