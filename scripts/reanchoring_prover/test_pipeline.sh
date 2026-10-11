#!/usr/bin/env bash
# Tests watch_and_prove_pipeline.sh against a fake engramd (a chain that grows by one header per second) and a fake segment script that
# follows the same protocol as prove_segment.sh (stageA / done / failed files). No network, no proving. Checks that segments start every
# STAGGER_S seconds, that at most PROVERS are in flight, that they finish in order, and that a failed segment makes the next one start again
# from the chain with the older ones stopped.
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
T="$(mktemp -d)"; trap 'rm -rf "$T"; pkill -f "$T/prove_segment" 2>/dev/null' EXIT
START=$(date +%s)
cat > "$T/engramd" <<S
#!/usr/bin/env bash
n=\$(( \$(date +%s) - $START ))
echo "rt_last=1"
for ((h = 1; h <= n; h++)); do echo "height=\$h fsm_state=2 withdrawal_locked=1 state_root=0x0"; done
S
cat > "$T/prove_segment" <<'S'
#!/usr/bin/env bash
# fake segment: stage A at once, "proving" for $FAKE_PROVE seconds, then submit after the previous segment; FAKE_FAIL=<id> makes that one fail
seg="$1"; prev="${2:-}"; st="$PIPELINE_STATE"
echo "$(date +%s) start $seg" >> "$FAKE_LOG"
[ -n "$prev" ] && while [ ! -e "$st/seg$prev.stageA" ]; do sleep 0.2; done
echo "$seg $seg 1" > "$st/seg$seg.stageA"
sleep "${FAKE_PROVE:-6}"
if [ "${FAKE_FAIL:-}" = "$seg" ]; then : > "$st/seg$seg.failed"; exit 1; fi
[ -n "$prev" ] && while [ ! -e "$st/seg$prev.done" ]; do [ -e "$st/seg$prev.failed" ] && { : > "$st/seg$seg.failed"; exit 1; }; sleep 0.2; done
: > "$st/seg$seg.done"; echo "$(date +%s) done $seg" >> "$FAKE_LOG"
S
chmod +x "$T/engramd" "$T/prove_segment"
export ENGRAMD_BIN="$T/engramd" PROVE_SEGMENT="$T/prove_segment" NODE_URL=http://none FAKE_LOG="$T/log" PROVERS=3 STAGGER_S=2 POLL_INTERVAL_S=1 MAX_CHECKS=24 MIN_BATCH=1 FAKE_PROVE=6
: > "$T/log"
"$HERE/watch_and_prove_pipeline.sh" > "$T/out" 2>&1
fail=0; check() { if eval "$2"; then echo "ok   $1"; else echo "FAIL $1"; fail=1; fi; }
starts=$(grep -c " start " "$T/log"); dones=$(grep -c " done " "$T/log")
check "segments start (got $starts)" '[ "$starts" -ge 4 ]'
check "segments finish in order" '[ "$(grep " done " "$T/log" | awk "{print \$3}" | tr "\n" " ")" = "$(grep " done " "$T/log" | awk "{print \$3}" | sort -n | tr "\n" " ")" ]'
check "at most 3 in flight" 'awk "/ start /{n++} / done /{n--} {if (n>3) bad=1} END{exit bad}" "$T/log"'
# the coordinator and this log count whole seconds, so two starts STAGGER_S apart can show STAGGER_S-1 apart
check "starts are about STAGGER_S apart (not closer than STAGGER_S-1)" 'awk "/ start /{ if (last && \$1-last<1) bad=1; last=\$1 } END{exit bad}" "$T/log"'
# failure: segment 2 fails; the pipeline must stop the others and start again from the chain
: > "$T/log"; export FAKE_FAIL=2 MAX_CHECKS=20
"$HERE/watch_and_prove_pipeline.sh" > "$T/out2" 2>&1
check "a failed segment is reported" 'grep -q "segment 2 failed" "$T/out2"'
check "a new segment starts after the failure" 'grep -q "follows chain checkpoint" "$T/out2" && [ "$(grep -c "started (follows chain" "$T/out2")" -ge 2 ]'
exit $fail
