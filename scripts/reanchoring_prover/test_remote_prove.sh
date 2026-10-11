#!/usr/bin/env bash
# Tests remote_prove.sh against a stand-in for ssh that plays the worker (no network, no proving): one proof per worker at a time,
# every worker used, a failing worker reported and its lock released, and a cancelled job that removes its container and its lock.
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
mkdir -p "$T/bin" "$T/state" "$T/log"
cat > "$T/bin/ssh" <<'S'
#!/usr/bin/env bash
# Fake ssh: the host is the second to last argument, the command the last one.
args=("$@"); cmd="${args[$((${#args[@]}-1))]}"; host="${args[$((${#args[@]}-2))]}"
echo "$host $cmd" >> "$FAKE_LOG/calls"
case "$cmd" in
  *"docker rm -f"*) echo "$cmd" >> "$FAKE_LOG/removed"; exit 0 ;;
  *"docker run"*)
    cat > "$FAKE_LOG/toml.$$"
    if mkdir "$FAKE_LOG/busy.$host" 2>/dev/null; then :; else echo "$host" >> "$FAKE_LOG/overlap"; fi
    echo "$host" >> "$FAKE_LOG/used"
    sleep "${FAKE_SLEEP:-1}"
    rmdir "$FAKE_LOG/busy.$host" 2>/dev/null
    [ "${FAKE_FAIL:-}" = "$host" ] && { echo "boom" >&2; exit 1; }
    d="$(mktemp -d)"; head -c 14656 /dev/zero | tr '\0' 'p' > "$d/proof"; head -c 224 /dev/zero | tr '\0' 'i' > "$d/public_inputs"
    tar -c -C "$d" proof public_inputs ;;
esac
S
chmod +x "$T/bin/ssh"; echo '[x]' > "$T/Prover.toml"
export PATH="$T/bin:$PATH" FAKE_LOG="$T/log" STATE="$T/state" WORKERS="ubuntu@w1 ubuntu@w2" WORKER_IMAGE=img WORKER_KEY=/dev/null
fail=0; check() { if eval "$2"; then echo "ok   $1"; else echo "FAIL $1"; fail=1; fi; }

# A: four jobs on two workers
pids=(); for j in 1 2 3 4; do JOB=a$j "$HERE/remote_prove.sh" "$T/Prover.toml" "$T/out$j" 2>/dev/null & pids+=($!); done
rc=0; for p in "${pids[@]}"; do wait "$p" || rc=1; done
check "four jobs succeed" '[ $rc -eq 0 ]'
check "proof and public inputs received" '[ -s "$T/out1/proof" ] && [ "$(wc -c < "$T/out1/public_inputs" | tr -d " ")" = 224 ]'
check "no worker ran two proofs at once" '[ ! -e "$T/log/overlap" ]'
check "both workers were used" '[ "$(sort -u "$T/log/used" | wc -l | tr -d " ")" = 2 ]'
check "no lock left" '[ -z "$(ls "$T/state" | grep ^lock)" ]'

# B: a failing worker
rm -f "$T/log/used"; export WORKERS="ubuntu@w1" FAKE_FAIL=ubuntu@w1
JOB=b1 "$HERE/remote_prove.sh" "$T/Prover.toml" "$T/outb" 2>/dev/null; rc=$?
check "a failing worker gives a non-zero exit" '[ $rc -ne 0 ]'
check "its lock is released" '[ -z "$(ls "$T/state" | grep ^lock)" ]'
unset FAKE_FAIL

# C: cancelled while proving
export FAKE_SLEEP=20
JOB=c1 "$HERE/remote_prove.sh" "$T/Prover.toml" "$T/outc" 2>/dev/null & p=$!
sleep 2; t0=$(date +%s); kill -TERM $p; wait $p; rc=$?; took=$(( $(date +%s) - t0 ))
check "a cancelled job exits with 143" '[ $rc -eq 143 ]'
check "the cancel does not wait for the proof (took ${took} s of a 20 s proof)" '[ $took -lt 5 ]'
check "it removed its container on the worker" 'grep -q "docker rm -f engram-prove-c1" "$T/log/removed"'
check "its lock is released" '[ -z "$(ls "$T/state" | grep ^lock)" ]'
exit $fail
