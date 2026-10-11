#!/usr/bin/env bash
# One segment of the pipelined re-anchoring prover (see watch_and_prove_pipeline.sh). Same stages and the same proof as
# prove_and_submit.sh, split so that several segments are proven at once:
#   A (serial, cheap)  headers, boundary witness and header linking. Needs the end height and rt_new of the previous segment,
#                      so segment k+1 starts A only after segment k finished A. Writes $STATE/seg<k>.stageA.
#   B (parallel)       nargo execute + bb prove in a private copy of the circuits (the shared Prover.toml and target/ of
#                      prove_and_submit.sh would collide).
#   C (serial, cheap)  submit after the previous segment was accepted. SubmitRecoveryProof checks rt_last and first_height
#                      against the chain at submission time, so only the order of submissions matters, not that of proving.
# Marks $STATE/seg<k>.done on success and $STATE/seg<k>.failed otherwise; a failed segment invalidates the ones after it.
#
#   prove_segment.sh <seg> [<previous seg>]
set -uo pipefail

SEG="${1:?segment id}"
PREV="${2:-}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
STATE="${PIPELINE_STATE:?PIPELINE_STATE not set}"
ENGRAMD="${ENGRAMD_BIN:-$REPO_ROOT/engramd}"
NODE_URL="${NODE_URL:-http://127.0.0.1:26657}"
CHAIN_ID="${ENGRAM_CHAIN_ID:-engram-dev-1}"
EMBEDDED_VK="$REPO_ROOT/x/sovereignty/keeper/zk_assets/vk"
N_MAX=256
MIN_BATCH="${MIN_BATCH:-1}"
BB_THREADS="${BB_THREADS:-0}"
BOUNDARY_RETRIES="${BOUNDARY_RETRIES:-16}"
WITNESS_PUBLISH_MIN_INTERVAL_S="${WITNESS_PUBLISH_MIN_INTERVAL_S:-120}"

WD="$STATE/w$SEG"
MAIN="$WD/reanchoring"
WIT="$WD/reanchoring_witness"

fail() {
  echo "[segment $SEG] $*" >&2
  : > "$STATE/seg$SEG.failed"
  exit 1
}

tip_height() {
  { curl -s --max-time 3 "$NODE_URL/status" 2>/dev/null || wget -q -T 3 -O - "$NODE_URL/status" 2>/dev/null; } \
    | grep -o '"latest_block_height":"[0-9]*"' | grep -o '[0-9][0-9]*' | head -1 || true
}

# wait_for <file> -> 0 when it exists, 1 when seg<PREV>.failed appears first.
wait_prev() {
  local want="$1"
  while [ ! -e "$want" ]; do
    [ -e "$STATE/seg$PREV.failed" ] && return 1
    sleep 0.5
  done
  return 0
}

T0=$(date +%s)
TIP_QUERY=$(tip_height)

# Private copies of the circuits; target/ keeps only the compiled circuit. Dependencies come from nargo's own cache.
rm -rf "$WD"
mkdir -p "$MAIN/target" "$WIT"
cp -R "$REPO_ROOT/circuit/reanchoring/Nargo.toml" "$REPO_ROOT/circuit/reanchoring/src" "$MAIN/"
cp "$REPO_ROOT/circuit/reanchoring/target/reanchoring.json" "$MAIN/target/"
cp -R "$REPO_ROOT/circuit/reanchoring_witness/." "$WIT/"

# ---- Stage A ----
FIRST_FLAG=()
MIN_HEIGHT=0
if [ -n "$PREV" ]; then
  wait_prev "$STATE/seg$PREV.stageA" || fail "previous segment $PREV failed before stage A"
  read -r PREV_END PREV_RT_NEW _ < "$STATE/seg$PREV.stageA"
  MIN_HEIGHT=$((PREV_END + 1))
  FIRST_FLAG=(--first-height "$MIN_HEIGHT")
fi

HEADERS_OUT=$("$ENGRAMD" query-recovery-headers --node "$NODE_URL") || fail "query-recovery-headers failed"
if [ -n "$PREV" ]; then
  RT_LAST="$PREV_RT_NEW"
else
  RT_LAST=$(echo "$HEADERS_OUT" | grep '^rt_last=' | cut -d= -f2)
fi
ALL_HEADER_LINES=$(echo "$HEADERS_OUT" | grep '^height=' | awk -F'[= ]' -v m="$MIN_HEIGHT" '$2 >= m')
TOTAL_N=$(echo "$ALL_HEADER_LINES" | grep -c '^height=' || true)
[ "$TOTAL_N" -ge "$MIN_BATCH" ] || fail "only $TOTAL_N headers beyond height $MIN_HEIGHT"

COUNT="$TOTAL_N"
[ "$COUNT" -le "$N_MAX" ] || COUNT="$N_MAX"
HEADER_LINES=$(head -n "$COUNT" <<< "$ALL_HEADER_LINES")
field() { echo "$HEADER_LINES" | sed -n "${1}p" | grep -o "${2}=[^ ]*" | cut -d= -f2; }

BOUNDARY_TOML="$WD/boundary.toml"
attempt=0
while :; do
  LAST_REAL_HEIGHT=$(field "$COUNT" height)
  if "$ENGRAMD" build-boundary-witness --node "$NODE_URL" --chain-id "$CHAIN_ID" "${FIRST_FLAG[@]}" \
      --last-height "$LAST_REAL_HEIGHT" --out "$BOUNDARY_TOML" >/dev/null; then
    break
  fi
  attempt=$((attempt + 1))
  if [ "$attempt" -gt "$BOUNDARY_RETRIES" ] || [ "$COUNT" -le 1 ]; then
    fail "no usable boundary block within $BOUNDARY_RETRIES headers of the end"
  fi
  COUNT=$((COUNT - 1))
  HEADER_LINES=$(head -n "$COUNT" <<< "$HEADER_LINES")
done
T_BOUNDARY=$(date +%s)

pad() { for ((i = COUNT; i < N_MAX; i++)); do printf '"0",'; done; }
{
  echo "rt_last = \"$RT_LAST\""
  echo "count = \"$COUNT\""
  for f in fsm_state withdrawal_locked state_root; do
    printf 'raw_%s = [' "$f"
    for ((i = 1; i <= COUNT; i++)); do printf '"%s",' "$(field "$i" "$f")"; done
    pad
    printf ']\n'
  done
} > "$WIT/Prover.toml"

WITNESS_OUT=$(cd "$WIT" && nargo execute witness 2>&1)
LINKED_CHAIN=$(echo "$WITNESS_OUT" | grep '^Header ')
RT_NEW=$(echo "$WITNESS_OUT" | grep '^rt_new_hash=' | cut -d= -f2)
[ -n "$RT_NEW" ] || fail "reanchoring_witness printed no rt_new_hash"
linked_field() { echo "$LINKED_CHAIN" | sed -n "${1}p" | grep -o "${2}: [^,}]*" | awk '{print $2}'; }
T_WITNESS=$(date +%s)

# Publish this segment's end for the next one only now: its first_height and rt_last are final.
echo "$LAST_REAL_HEIGHT $RT_NEW $COUNT" > "$STATE/seg$SEG.stageA.tmp"
mv "$STATE/seg$SEG.stageA.tmp" "$STATE/seg$SEG.stageA"

# ---- Stage B ----
BOUNDARY_LOOSE=$(awk '/^\[/{exit} {print}' "$BOUNDARY_TOML")
BOUNDARY_TABLES=$(awk 'BEGIN{p=0} /^\[/{p=1} p{print}' "$BOUNDARY_TOML")
{
  echo "rt_last = \"$RT_LAST\""
  echo "rt_new = \"$RT_NEW\""
  echo "count = \"$COUNT\""
  echo "$BOUNDARY_LOOSE"
  echo ""
  echo "$BOUNDARY_TABLES"
  for ((i = 1; i <= COUNT; i++)); do
    echo ""
    echo "[[headers]]"
    echo "prev_hash = \"$(linked_field "$i" prev_hash)\""
    echo "fsm_state = \"$(linked_field "$i" fsm_state)\""
    echo "withdrawal_locked = \"$(linked_field "$i" withdrawal_locked)\""
    echo "state_root = \"$(linked_field "$i" state_root)\""
  done
  for ((i = COUNT; i < N_MAX; i++)); do
    printf '\n[[headers]]\nprev_hash = "0"\nfsm_state = "0"\nwithdrawal_locked = "0"\nstate_root = "0"\n'
  done
} > "$MAIN/Prover.toml"

(
  cd "$MAIN" || exit 1
  nargo execute witness || exit 1
  mkdir -p target/proof
  cp "$EMBEDDED_VK" target/proof/vk
  if [ "$BB_THREADS" -gt 0 ]; then export HARDWARE_CONCURRENCY="$BB_THREADS"; fi
  bb prove -b target/reanchoring.json -w target/witness.gz -o target/proof -k target/proof/vk
) >"$WD/prove.log" 2>&1 || { tail -5 "$WD/prove.log" >&2; fail "nargo/bb failed"; }
T_PROVE=$(date +%s)

# ---- Stage C ----
if [ -n "$PREV" ]; then
  wait_prev "$STATE/seg$PREV.done" || fail "previous segment $PREV failed before submission"
fi
T_TURN=$(date +%s)

DA_HEIGHT_FLAG=()
MARKER="$STATE/.last_witness_publish"
NOW_S=$(date +%s)
LAST_PUBLISH_S=0
[ -f "$MARKER" ] && LAST_PUBLISH_S=$(cat "$MARKER")
if [ $((NOW_S - LAST_PUBLISH_S)) -ge "$WITNESS_PUBLISH_MIN_INTERVAL_S" ]; then
  {
    echo "rt_last = \"$RT_LAST\""
    echo "rt_new = \"$RT_NEW\""
    echo "count = \"$COUNT\""
    for ((i = 1; i <= COUNT; i++)); do
      printf '\n[[headers]]\nprev_hash = "%s"\nfsm_state = "%s"\nwithdrawal_locked = "%s"\nstate_root = "%s"\n' \
        "$(linked_field "$i" prev_hash)" "$(linked_field "$i" fsm_state)" \
        "$(linked_field "$i" withdrawal_locked)" "$(linked_field "$i" state_root)"
    done
  } > "$MAIN/target/proof/witness_headers.toml"
  if DA_OUT=$("$ENGRAMD" publish-recovery-witness --headers "$MAIN/target/proof/witness_headers.toml" 2>&1); then
    DA_HEIGHT=$(echo "$DA_OUT" | grep '^da_celestia_height=' | cut -d= -f2)
    if [ -n "$DA_HEIGHT" ]; then
      DA_HEIGHT_FLAG=(--da-height "$DA_HEIGHT")
      echo "$NOW_S" > "$MARKER"
    fi
  else
    echo "[segment $SEG] WARNING: witness DA publish failed, submitting without --da-height: $DA_OUT" >&2
  fi
fi
T_DA=$(date +%s)

"$ENGRAMD" tx-submit-recovery-proof \
  "${DA_HEIGHT_FLAG[@]}" \
  --node "$NODE_URL" \
  --proof "$MAIN/target/proof/proof" \
  --public-inputs "$MAIN/target/proof/public_inputs" || fail "submission rejected"
T_SUBMIT=$(date +%s)
: > "$STATE/seg$SEG.done"

# Same fields as prove_and_submit.sh (stage columns: boundary, linking, prove, DA publish, submit, total), then the segment id
# and the seconds this segment waited for its predecessor's submission.
echo "PROOFLOG,$T0,$COUNT,$(field 1 height),$LAST_REAL_HEIGHT,$TOTAL_N,$TIP_QUERY,$(tip_height),$((T_BOUNDARY-T0)),$((T_WITNESS-T_BOUNDARY)),$((T_PROVE-T_WITNESS)),$((T_DA-T_TURN)),$((T_SUBMIT-T_DA)),$((T_SUBMIT-T0)),$SEG,$((T_TURN-T_PROVE))"
rm -rf "$WD"
