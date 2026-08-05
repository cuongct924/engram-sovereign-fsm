#!/usr/bin/env bash
# E2/E3/E7 -- Vanilla CometBFT baseline (docs/EXPERIMENT.md).
#
# Runs two REAL engramd nodes locally, side by side: one normal (real
# PrepareProposal/ProcessProposal/PreBlocker, carries ExtendedProposal), one
# --vanilla (BaseApp's default handlers, no ExtendedProposal at all -- see
# app/app.go's NewEngramApp doc). Measures real per-block proposal-size
# overhead and block interval via RPC polling -- no synthetic numbers.
#
# Usage: scripts/e7_consensus_overhead/vanilla_comparison.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
RESULTS_DIR="$SCRIPT_DIR/results"
BIN="/tmp/engramd-vanilla-comparison"
HOME_NORMAL="/tmp/engramd-vc-normal"
HOME_VANILLA="/tmp/engramd-vc-vanilla"
SAMPLE_BLOCKS=15   # how many blocks to sample from each node once it's warmed up
WARMUP_BLOCKS=5

mkdir -p "$RESULTS_DIR"
rm -rf "$HOME_NORMAL" "$HOME_VANILLA"

echo "[vanilla-cmp] Building engramd..."
(cd "$REPO_ROOT" && go build -o "$BIN" ./cmd/engramd)

echo "[vanilla-cmp] Initializing two node homes..."
"$BIN" init normal-node --home "$HOME_NORMAL" > /dev/null
"$BIN" init vanilla-node --home "$HOME_VANILLA" > /dev/null

# Shift the vanilla node's ports so both can run simultaneously on one machine.
sed -i.bak \
  -e 's|tcp://127.0.0.1:26657|tcp://127.0.0.1:36657|' \
  -e 's|tcp://0.0.0.0:26656|tcp://0.0.0.0:36656|' \
  -e 's|tcp://127.0.0.1:26658|tcp://127.0.0.1:36658|' \
  -e 's|:26660|:36660|' \
  "$HOME_VANILLA/config/config.toml"
rm -f "$HOME_VANILLA/config/config.toml.bak"

cleanup() {
  pkill -f "$BIN start" 2>/dev/null || true
}
trap cleanup EXIT

echo "[vanilla-cmp] Starting normal node..."
"$BIN" start --home "$HOME_NORMAL" > /tmp/engramd-vc-normal.log 2>&1 &
sleep 1
echo "[vanilla-cmp] Starting vanilla node..."
"$BIN" start --vanilla --home "$HOME_VANILLA" > /tmp/engramd-vc-vanilla.log 2>&1 &

echo "[vanilla-cmp] Waiting for both nodes to warm up..."
sleep $((WARMUP_BLOCKS + 3))

sample_node() {
  local port=$1 label=$2 out_csv=$3
  echo "height,proposal_tx0_bytes,num_txs,block_time" > "$out_csv"
  local height
  height=$(curl -s "http://localhost:$port/status" | python3 -c "import json,sys;print(json.load(sys.stdin)['result']['sync_info']['latest_block_height'])")
  local start=$((height - SAMPLE_BLOCKS))
  [ "$start" -lt 1 ] && start=1
  for h in $(seq "$start" "$height"); do
    curl -s "http://localhost:$port/block?height=$h" | python3 -c "
import json, sys, base64
d = json.load(sys.stdin)['result']['block']
txs = d['data']['txs'] or []
tx0_len = len(base64.b64decode(txs[0])) if txs else 0
print(f'$h,{tx0_len},{len(txs)},{d[\"header\"][\"time\"]}')
" >> "$out_csv"
  done
  echo "[vanilla-cmp] $label: sampled heights $start..$height -> $out_csv"
}

sample_node 26657 normal "$RESULTS_DIR/normal_samples.csv"
sample_node 36657 vanilla "$RESULTS_DIR/vanilla_samples.csv"

echo "[vanilla-cmp] Done. Run 'python3 $SCRIPT_DIR/measure_overhead.py --with-vanilla' to build the comparison table."
