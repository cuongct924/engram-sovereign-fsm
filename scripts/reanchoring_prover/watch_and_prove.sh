#!/usr/bin/env bash
# Polls the live node's tracked SOVEREIGN/RECOVERING header count (via
# `engramd query-recovery-headers`) and fires prove_and_submit.sh once
# ready. Any count from 1 to N_MAX=256 is provable in one proof (unlike the
# old fixed-N=4 design, which required N=4 and could never prove a shorter
# trailing remainder -- a real liveness gap), so this never blocks waiting
# for a batch.
#
# BATCH_THRESHOLD (default 256 = N_MAX, the circuit's hard cap): "ready"
# means N >= BATCH_THRESHOLD, OR N has been unchanged for
# STABLE_POLLS_REQUIRED consecutive polls (caught up to the tip -- no
# benefit in waiting longer). Firing on the very first header wasted most of
# each ~10s proof cycle on padding: nargo execute/bb prove's wall-clock cost
# is dominated by the circuit's FIXED N_MAX=256 size regardless of how many
# slots are real, so batching bigger is what actually catches up a backlog.
#
# A single unchanged poll is NOT stalled: with POLL_INTERVAL_S=2, N climbs
# on almost every poll while a backlog exists (e.g. 29,31,32,32), so 1
# unchanged reading fired repeatedly on batches of 7-20 instead of riding
# growth toward 256. STABLE_POLLS_REQUIRED=3 (~6s of no new headers)
# distinguishes unlucky timing from actually caught up.
#
# Keeps running after a successful submission -- see
# x/sovereignty/keeper/msg_server.go's SubmitRecoveryProof doc on rolling
# checkpoints: LastAnchoredRoot advances (and HeaderHistory prunes only the
# covered prefix) on EVERY accepted proof, so the count drops back to 0 and
# climbs again for the NEXT segment. A long unhealthy interval needs this
# watcher to fire repeatedly, not once.
#
# Exits 0 once --max-checks is exhausted (natural way to stop, e.g. interval
# genuinely ended), non-zero if no proof was ever submitted in that budget.
# A rejected submission logs a warning and keeps watching.
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
ENGRAMD="${ENGRAMD_BIN:-$REPO_ROOT/engramd}"
NODE_URL="${NODE_URL:-http://127.0.0.1:26657}"
POLL_INTERVAL_S="${POLL_INTERVAL_S:-2}"
MAX_CHECKS="${MAX_CHECKS:-600}"  # ~20 min at 2s/check
BATCH_THRESHOLD="${BATCH_THRESHOLD:-256}"  # = N_MAX: prove_and_submit.sh's own COUNT=min(TOTAL_N, N_MAX) caps it here anyway
STABLE_POLLS_REQUIRED="${STABLE_POLLS_REQUIRED:-3}"

if [ ! -x "$ENGRAMD" ]; then
  echo "[watch_and_prove] building engramd..." >&2
  (cd "$REPO_ROOT" && go build -o "$ENGRAMD" ./cmd/engramd)
fi

echo "[watch_and_prove] watching $NODE_URL, batching up to $BATCH_THRESHOLD headers per proof (repeating, rolling-checkpoint mode, N_MAX=256)..."
i=0
proofs_submitted=0
proofs_rejected=0
prev_n=0
stable_count=0
while [ "$i" -lt "$MAX_CHECKS" ]; do
  i=$((i + 1))
  OUT=$("$ENGRAMD" query-recovery-headers --node "$NODE_URL" 2>&1)
  N=$(echo "$OUT" | grep -c '^height=' || true)
  TS=$(date -u +%H:%M:%S)
  if [ "$N" -eq "$prev_n" ]; then
    stable_count=$((stable_count + 1))
  else
    stable_count=0
  fi
  prev_n=$N
  echo "[$TS] check $i: N=$N stable=$stable_count (proofs so far: $proofs_submitted submitted, $proofs_rejected rejected)"
  # Fire once N reaches BATCH_THRESHOLD (pack each fixed-cost proof cycle
  # with as much real work as possible), OR once N has held steady for
  # STABLE_POLLS_REQUIRED consecutive polls (genuinely caught up -- see
  # header doc). N>=1 alone fired on every new header, submitting proofs
  # carrying only 5-7 of 256 real headers each.
  if [ "$N" -ge 1 ] && { [ "$N" -ge "$BATCH_THRESHOLD" ] || [ "$stable_count" -ge "$STABLE_POLLS_REQUIRED" ]; }; then
    echo "[watch_and_prove] N=$N -- firing prove_and_submit.sh NOW"
    if "$SCRIPT_DIR/prove_and_submit.sh"; then
      proofs_submitted=$((proofs_submitted + 1))
      echo "[watch_and_prove] proof #$proofs_submitted accepted -- checkpoint advanced, continuing to watch for the next segment."
    else
      proofs_rejected=$((proofs_rejected + 1))
      echo "[watch_and_prove] proof rejected (likely the documented staleness race -- interval grew before submission landed, see prove_and_submit.sh's doc) -- continuing to watch." >&2
    fi
    prev_n=0
    stable_count=0
  fi
  sleep "$POLL_INTERVAL_S"
done

echo "[watch_and_prove] stopped after $MAX_CHECKS checks: $proofs_submitted proof(s) submitted, $proofs_rejected rejected."
if [ "$proofs_submitted" -eq 0 ]; then
  exit 1
fi
exit 0
