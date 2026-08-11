#!/usr/bin/env bash
# Verifies rule 6 of the github-actions-ci skill: go-test.yml and go-lint.yml must pin the same
# engram-consensus-core commit, and (if a local sibling checkout exists) flags drift against it.
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
workflows=("$repo_root/.github/workflows/go-test.yml" "$repo_root/.github/workflows/go-lint.yml")

extract_pin() {
  # matches: git -C ../engram-consensus-core checkout --quiet <sha>
  # (portable grep -oE, not -P, so this also works with macOS/BSD grep)
  grep -oE 'engram-consensus-core checkout --quiet [0-9a-f]{7,40}' "$1" \
    | sed -E 's/.* //' || true
}

declare -A pins
for wf in "${workflows[@]}"; do
  if [[ ! -f "$wf" ]]; then
    echo "MISSING: $wf"
    exit 1
  fi
  pin="$(extract_pin "$wf")"
  if [[ -z "$pin" ]]; then
    echo "NO PIN FOUND in $wf (expected 'engram-consensus-core checkout --quiet <sha>')"
    exit 1
  fi
  pins["$wf"]="$pin"
  echo "$(basename "$wf"): $pin"
done

first_wf="${workflows[0]}"
first_pin="${pins[$first_wf]}"
for wf in "${workflows[@]:1}"; do
  if [[ "${pins[$wf]}" != "$first_pin" ]]; then
    echo
    echo "LOCKSTEP VIOLATION: $(basename "$first_wf") pins $first_pin but $(basename "$wf") pins ${pins[$wf]}"
    exit 1
  fi
done

sibling="$repo_root/../engram-consensus-core"
if [[ -d "$sibling/.git" ]]; then
  local_head="$(git -C "$sibling" rev-parse HEAD)"
  echo "local sibling HEAD: $local_head"
  if [[ "$local_head" != "$first_pin"* ]]; then
    echo
    echo "DRIFT: local ../engram-consensus-core is at $local_head, CI is pinned to $first_pin"
    echo "Not necessarily an error (local may be ahead intentionally) -- but confirm before pushing"
    echo "a workflow change, per the skill's rule 6."
    exit 2
  fi
else
  echo "(no local ../engram-consensus-core checkout -- skipping drift check)"
fi

echo
echo "OK: both workflows pinned in lockstep${local_head:+, matches local sibling checkout}."
