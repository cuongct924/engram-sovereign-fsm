#!/usr/bin/env bash
# Proving worker: runs inside the prover image on a worker host. Reads circuit/reanchoring's Prover.toml on stdin, runs nargo execute
# and bb prove, and writes a tar of `proof` and `public_inputs` on stdout. Progress goes to stderr. Started by prove_segment.sh over SSH.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
EMBEDDED_VK="$REPO_ROOT/x/sovereignty/keeper/zk_assets/vk"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

cp -R "$REPO_ROOT/circuit/reanchoring/." "$WORK/"
cat > "$WORK/Prover.toml"
cd "$WORK"
rm -rf target/proof target/witness.gz
mkdir -p target/proof
cp "$EMBEDDED_VK" target/proof/vk
nargo execute witness >&2
bb prove -b target/reanchoring.json -w target/witness.gz -o target/proof -k target/proof/vk >&2
tar -c -C target/proof proof public_inputs
