#!/usr/bin/env bash
# Polls the real, live node's currently-tracked SOVEREIGN/RECOVERING header
# count (via `engramd query-recovery-headers`) and fires prove_and_submit.sh
# once ready -- prove_and_submit.sh proves up to min(TOTAL_N, N_MAX=256) of
# them (see its own header comment). Unlike the old fixed-N=4 design (which
# required at least EXPECTED_N=4 tracked before attempting anything, and
# could never prove a trailing remainder shorter than 4 at all -- a real
# liveness gap), ANY count from 1 to 256 is immediately provable in one
# proof, so this never blocks waiting for a batch that might not arrive.
#
# BATCH_THRESHOLD (default 256 = N_MAX, the circuit's own hard cap):
# "ready" means N >= BATCH_THRESHOLD, OR N has been unchanged for
# STABLE_POLLS_REQUIRED consecutive polls (stopped growing -- caught up to
# the current tip, no benefit to waiting longer). Firing on the very first
# tracked header (N>=1, the original design) was live-verified wasting most
# of each ~10s proof-generation cycle's fixed cost on padding: nargo
# execute/bb prove's wall-clock cost is dominated by the circuit's FIXED
# N_MAX=256 size regardless of how many of those slots are real headers vs.
# padding, so a 5-7-header proof costs essentially the same as a
# 200-header one -- batching bigger, not proving faster, is what actually
# speeds up catching up a real SOVEREIGN backlog.
#
# A single unchanged poll is NOT enough to call it stalled: live-observed
# with POLL_INTERVAL_S=2, N climbs on almost every 2s poll while a real
# backlog exists (e.g. 29,31,32,32 -- new-block timing just doesn't align
# perfectly with the poll cadence), so requiring only 1 unchanged reading
# fired repeatedly on batches of 7-20 instead of riding the real growth up
# toward 256. STABLE_POLLS_REQUIRED consecutive unchanged polls (default 3,
# ~6s of genuinely no new headers) distinguishes "just unlucky poll timing"
# from "actually caught up" while still keeping the liveness fix intact for
# a real trailing remainder.
#
# Keeps running after a successful submission rather than exiting -- see
# x/sovereignty/keeper/msg_server.go's SubmitRecoveryProof doc on rolling
# checkpoints: LastAnchoredRoot now advances (and HeaderHistory prunes only
# the covered prefix) on EVERY accepted proof, not just once at the true end
# of the interval, so the tracked count legitimately drops back to 0 and
# starts climbing again for the NEXT segment. A long unhealthy interval
# therefore needs this watcher to fire repeatedly, not once -- exiting after
# the first hit (the original behavior) meant no further segments of a
# still-open interval could ever be proven, exactly the gap that made the
# fixed-N circuit unusable against a real, unbounded-length interval.
#
# Exits successfully (0) once --max-checks is exhausted (the natural way to
# stop watching, e.g. once the interval has genuinely ended), or non-zero if
# it never submitted a single proof in that budget. A rejected submission
# logs a warning and keeps watching rather than aborting.
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
  # Fire once N reaches BATCH_THRESHOLD (make each fixed-cost proof cycle
  # carry as much real work as possible), OR once N has held steady for
  # STABLE_POLLS_REQUIRED consecutive polls (genuinely caught up to the
  # current tip, not just an unlucky single poll -- see header doc). N>=1
  # alone (the original trigger) fired on every single new header,
  # live-confirmed submitting proofs carrying only 5-7 of 256 real headers
  # each -- most of every ~10s proving cycle spent on padding instead of
  # real backlog.
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
