#!/usr/bin/env bash
# Polls the real, live node's currently-tracked SOVEREIGN/RECOVERING header
# count (via `engramd query-recovery-headers`) and fires prove_and_submit.sh
# once at least EXPECTED_N are tracked -- prove_and_submit.sh proves the
# OLDEST EXPECTED_N of them (see its own header comment), so this no longer
# needs to catch an exact count on an exact block: unlike an earlier version
# of this script (which required the count to equal EXPECTED_N exactly, a
# window exactly one block wide that a real testnet's block cadence
# consistently blew past before any poll could react), "at least" stays
# true continuously once the interval starts, so a normal poll interval
# reliably catches it.
#
# Keeps running after a successful submission rather than exiting -- see
# x/sovereignty/keeper/msg_server.go's SubmitRecoveryProof doc on rolling
# checkpoints: LastAnchoredRoot now advances (and HeaderHistory prunes only
# the covered prefix) on EVERY accepted proof, not just once at the true end
# of the interval, so the tracked count legitimately drops back below
# EXPECTED_N and starts climbing again for the NEXT segment. A long
# unhealthy interval therefore needs this watcher to fire repeatedly, not
# once -- exiting after the first hit (the original behavior) meant no
# further segments of a still-open interval could ever be proven, exactly
# the gap that made the fixed-N circuit unusable against a real,
# unbounded-length interval.
#
# Exits successfully (0) once --max-checks is exhausted (the natural way to
# stop watching, e.g. once the interval has genuinely ended), or non-zero if
# it never submitted a single proof in that budget. A rejected submission
# logs a warning and keeps watching rather than aborting -- with the
# oldest-slice fix above, a rejection here now means something more
# meaningful than a stale race (e.g. a genuine mismatch), since the normal
# staleness race no longer applies to the oldest-EXPECTED_N slice.
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

echo "[watch_and_prove] watching $NODE_URL for at least $EXPECTED_N tracked headers (repeating, rolling-checkpoint mode)..."
i=0
proofs_submitted=0
proofs_rejected=0
while [ "$i" -lt "$MAX_CHECKS" ]; do
  i=$((i + 1))
  OUT=$("$ENGRAMD" query-recovery-headers --node "$NODE_URL" 2>&1)
  N=$(echo "$OUT" | grep -c '^height=' || true)
  TS=$(date -u +%H:%M:%S)
  echo "[$TS] check $i: N=$N (proofs so far: $proofs_submitted submitted, $proofs_rejected rejected)"
  if [ "$N" -ge "$EXPECTED_N" ]; then
    echo "[watch_and_prove] N=$N >= $EXPECTED_N -- firing prove_and_submit.sh NOW"
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
