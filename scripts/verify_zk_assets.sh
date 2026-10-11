#!/usr/bin/env bash
# Checks the binary ZK assets against x/sovereignty/keeper/zk_assets/ASSETS.lock.
#
#   scripts/verify_zk_assets.sh             fast: SHA-256 of every asset against the lock
#   scripts/verify_zk_assets.sh --rebuild   also recompiles the circuits with the pinned nargo and regenerates the verification
#                                           keys with the pinned bb, and compares them with the committed ones
#
# The wasm verifier is not rebuilt here (it comes from github.com/cuongct924/engram-barretenberg-wasm; see the lock). Its behaviour
# is covered by `go test ./x/sovereignty/keeper -run TestVerifyZKProof_RealProof`.
set -euo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LOCK="$REPO/x/sovereignty/keeper/zk_assets/ASSETS.lock"
sha() { if command -v sha256sum >/dev/null; then sha256sum "$1" | cut -d' ' -f1; else shasum -a 256 "$1" | cut -d' ' -f1; fi; }

fail=0
while read -r tag want file; do
  [ "$tag" = sha256 ] || continue
  got="$(sha "$REPO/$file")"
  if [ "$got" = "$want" ]; then echo "ok       $file"; else echo "MISMATCH $file (lock $want, file $got)"; fail=1; fi
done < <(grep '^sha256 ' "$LOCK")

if [ "${1:-}" = "--rebuild" ]; then
  [ "$(bb --version)" = "5.0.0-nightly.20260522" ] || { echo "bb 5.0.0-nightly.20260522 required, found $(bb --version)" >&2; exit 2; }
  [ "$(nargo --version | head -1)" = "nargo version = 1.0.0-beta.22" ] || { echo "nargo 1.0.0-beta.22 required" >&2; exit 2; }
  tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
  for c in "reanchoring:vk" "reanchoring_aggregator:vk_aggregate"; do
    circuit="${c%%:*}"; asset="${c##*:}"
    mkdir -p "$tmp/$circuit"
    cp -R "$REPO/circuit/$circuit/." "$tmp/$circuit/"
    rm -rf "$tmp/$circuit/target"
    if ! (cd "$tmp/$circuit" && nargo compile && bb write_vk -b "target/$circuit.json" -o "$tmp/out_$circuit") >"$tmp/$circuit.log" 2>&1; then
      echo "REBUILD FAILED $circuit (log: $tmp/$circuit.log)"; tail -5 "$tmp/$circuit.log"; fail=1; continue
    fi
    got="$(sha "$tmp/out_$circuit/vk")"; want="$(sha "$REPO/x/sovereignty/keeper/zk_assets/$asset")"
    if [ "$got" = "$want" ]; then echo "rebuilt  $asset"; else echo "REBUILD MISMATCH $asset (committed $want, rebuilt $got)"; fail=1; fi
  done
fi
exit "$fail"
