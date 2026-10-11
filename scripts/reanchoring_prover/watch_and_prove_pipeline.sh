#!/usr/bin/env bash
# Pipelined re-anchoring prover: keeps up to PROVERS proofs in flight, one new segment every STAGGER_S seconds.
#
# A proof costs the same for any batch (the circuit is padded to N_MAX) and takes L seconds, so after a proof lands the
# unproven tail is about L times the block rate, and with one prover in flight it then grows for another L seconds. Starting
# a segment every L/PROVERS seconds shortens the interval between landings, so the tail stays within roughly
# rate * L * (1 + 1/PROVERS). SubmitRecoveryProof checks rt_last and first_height at submission, so segments are proven
# concurrently and submitted in order (prove_segment.sh). A failed segment invalidates the ones after it: the workers still
# running are stopped and the next segment starts again from the chain's checkpoint.
#
# Environment: PROVERS (4, or the number of WORKERS), L_EST_S (75), STAGGER_S (L_EST_S / PROVERS), BB_THREADS (cores / PROVERS, local proving only), MIN_BATCH (1),
# NODE_URL, POLL_INTERVAL_S (2), MAX_CHECKS (600). Memory: a proof peaks at about 7 GB, so PROVERS proofs need that much each.
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
ENGRAMD="${ENGRAMD_BIN:-$REPO_ROOT/engramd}"
export NODE_URL="${NODE_URL:-http://127.0.0.1:26657}"
# With WORKERS (space-separated user@host, see prove_segment.sh) one proof is in flight per worker. L_EST_S is the expected seconds from
# a proof's start to its submission; segments start L_EST_S / PROVERS seconds apart.
WORKERS="${WORKERS:-}"
if [ -n "$WORKERS" ] && [ -z "${PROVERS:-}" ]; then PROVERS=$(wc -w <<< "$WORKERS" | tr -d ' '); fi
PROVERS="${PROVERS:-4}"
L_EST_S="${L_EST_S:-75}"
STAGGER_S="${STAGGER_S:-$(( (L_EST_S + PROVERS - 1) / PROVERS ))}"
export WORKERS
POLL_INTERVAL_S="${POLL_INTERVAL_S:-2}"
MAX_CHECKS="${MAX_CHECKS:-600}"
export MIN_BATCH="${MIN_BATCH:-1}"
NCPU=$( (nproc 2>/dev/null || sysctl -n hw.ncpu) )
export BB_THREADS="${BB_THREADS:-$((NCPU / PROVERS))}"
export PIPELINE_STATE
PIPELINE_STATE="$(mktemp -d "${TMPDIR:-/tmp}/reanchor_pipeline.XXXXXX")"
export ENGRAMD_BIN="$ENGRAMD"

if [ ! -x "$ENGRAMD" ]; then
  (cd "$REPO_ROOT" && go build -o "$ENGRAMD" ./cmd/engramd)
fi

echo "[pipeline] $PROVERS provers, one segment per ${STAGGER_S}s, $BB_THREADS bb threads each, state in $PIPELINE_STATE"

declare -A PID
next=1
prev=""
last_launch=0
submitted=0
failed=0

running() { local n=0 s; for s in "${!PID[@]}"; do kill -0 "${PID[$s]}" 2>/dev/null && n=$((n + 1)); done; echo "$n"; }

reset_epoch() {
  local s
  for s in "${!PID[@]}"; do kill "${PID[$s]}" 2>/dev/null; done
  wait 2>/dev/null
  PID=()
  prev=""
  rm -f "$PIPELINE_STATE"/seg*.stageA "$PIPELINE_STATE"/seg*.done "$PIPELINE_STATE"/seg*.failed
}

trap 'reset_epoch; rm -rf "$PIPELINE_STATE"' EXIT

i=0
while [ "$i" -lt "$MAX_CHECKS" ]; do
  i=$((i + 1))
  # Reap finished workers and react to a failure.
  for s in "${!PID[@]}"; do
    if ! kill -0 "${PID[$s]}" 2>/dev/null; then
      wait "${PID[$s]}" 2>/dev/null
      if [ -e "$PIPELINE_STATE/seg$s.done" ]; then
        submitted=$((submitted + 1))
        unset "PID[$s]"
      elif [ -e "$PIPELINE_STATE/seg$s.failed" ]; then
        failed=$((failed + 1))
        echo "[pipeline] segment $s failed -- stopping the later ones and restarting from the chain's checkpoint" >&2
        reset_epoch
        break
      else
        unset "PID[$s]"
      fi
    fi
  done

  now=$(date +%s)
  if [ "$(running)" -lt "$PROVERS" ] && [ $((now - last_launch)) -ge "$STAGGER_S" ]; then
    # With nothing in flight the next segment starts from the chain; otherwise it follows the previous segment, once that one
    # has fixed its end (stageA), and only if headers beyond it exist.
    if [ -z "${PID[*]:-}" ]; then prev=""; fi
    if [ -z "$prev" ] || [ -e "$PIPELINE_STATE/seg$prev.stageA" ]; then
      OUT=$("$ENGRAMD" query-recovery-headers --node "$NODE_URL" 2>&1)
      min=0
      [ -n "$prev" ] && min=$(( $(cut -d' ' -f1 "$PIPELINE_STATE/seg$prev.stageA") + 1 ))
      avail=$(echo "$OUT" | grep '^height=' | awk -F'[= ]' -v m="$min" '$2 >= m' | wc -l | tr -d ' ')
      echo "[$(date -u +%H:%M:%S)] check $i: $avail headers beyond $((min - 1)), in flight $(running), submitted $submitted, failed $failed"
      if [ "$avail" -ge "$MIN_BATCH" ]; then
        "$SCRIPT_DIR/prove_segment.sh" "$next" "$prev" &
        PID[$next]=$!
        echo "[pipeline] segment $next started (follows ${prev:-chain checkpoint})"
        prev=$next
        next=$((next + 1))
        last_launch=$now
      fi
    fi
  fi
  sleep "$POLL_INTERVAL_S"
done

wait
echo "[pipeline] stopped after $MAX_CHECKS checks: $submitted segment(s) submitted, $failed failed."
[ "$submitted" -gt 0 ]
