#!/usr/bin/env bash
# Proves one segment on a worker host: takes a free worker (one proof per worker at a time), sends it Prover.toml over SSH, and
# receives the proof and the public inputs. Used by prove_segment.sh when WORKERS is set; the worker side is worker_prove.sh.
#
#   remote_prove.sh <Prover.toml> <output dir>      writes <output dir>/proof, public_inputs and worker (the host that proved it)
#
# Environment: WORKERS (space-separated user@host), WORKER_KEY (SSH key), WORKER_IMAGE (prover image on the workers), STATE (a directory
# shared with the other segments, for the per-worker locks and known_hosts), JOB (a name for this segment, default the process id).
# Stopped by TERM or INT, it removes the container it started on the worker, so a segment that is cancelled does not keep the worker busy.
set -uo pipefail

TOML="${1:?Prover.toml}"
OUT="${2:?output dir}"
: "${WORKERS:?}" "${WORKER_IMAGE:?}" "${STATE:?}"
WORKER_KEY="${WORKER_KEY:-/run/worker_key}"
JOB="${JOB:-$$}"
CONTAINER="engram-prove-$JOB"
mkdir -p "$OUT"

ssh_to() {
  local host="$1"; shift
  ssh -i "$WORKER_KEY" -o BatchMode=yes -o StrictHostKeyChecking=accept-new -o UserKnownHostsFile="$STATE/known_hosts" \
      -o ServerAliveInterval=15 -o ServerAliveCountMax=4 "$host" "$@"
}

LOCK=""
HOST=""
BG=""
release() { [ -n "$LOCK" ] && rmdir "$LOCK" 2>/dev/null; LOCK=""; }
cancel() {
  if [ -n "$BG" ]; then pkill -TERM -P "$BG" 2>/dev/null; kill -TERM "$BG" 2>/dev/null; fi
  [ -n "$HOST" ] && ssh_to "$HOST" "docker rm -f $CONTAINER" >/dev/null 2>&1
  release
  exit 143
}
trap cancel TERM INT
trap release EXIT

while [ -z "$LOCK" ]; do
  for w in $WORKERS; do
    if mkdir "$STATE/lock.${w//[@.]/_}" 2>/dev/null; then LOCK="$STATE/lock.${w//[@.]/_}"; HOST="$w"; break; fi
  done
  [ -n "$LOCK" ] || sleep 0.5
done

# The transfer runs in the background and this shell waits for it: bash runs a trap only after a foreground command ends, so a cancel
# would otherwise wait for the whole proof.
{
  ssh_to "$HOST" "docker run --rm -i --name $CONTAINER --entrypoint /app/scripts/reanchoring_prover/worker_prove.sh $WORKER_IMAGE" \
      < "$TOML" | tar -x -C "$OUT"
  echo "${PIPESTATUS[0]} ${PIPESTATUS[1]}" > "$OUT/.status"
} &
BG=$!
wait "$BG"
BG=""
read -r ssh_status tar_status < "$OUT/.status" 2>/dev/null || { ssh_status=1; tar_status=1; }
if [ "$ssh_status" -ne 0 ] || [ "$tar_status" -ne 0 ] || [ ! -s "$OUT/proof" ] || [ ! -s "$OUT/public_inputs" ]; then
  echo "[remote_prove] worker $HOST failed (ssh exit $ssh_status, tar exit $tar_status)" >&2
  exit 1
fi
echo "$HOST" > "$OUT/worker"
