#!/usr/bin/env python3
"""E7 LIVE (real docker testnet) -- distinct from measure_overhead.py, which
reads `go test ./tests/benchmark/...`'s synthetic V0-V5 payload benchmarks.
This script instead scans a real, already-committed range of blocks from the
live engram-node01 RPC and measures the REAL ExtendedProposal marker
overhead actually landing in Txs[0] on the running chain -- no synthetic
payload construction, no mocked ProcessProposal call.

Usage:
    python3 scripts/e7_consensus_overhead/live_overhead_scan.py [--last-n 50]
"""

import argparse
import base64
import json
import os
import sys
import urllib.request

MARKER = b"ENGRAM_EXTENDED_PROPOSAL_V1|"
RESULTS_DIR = os.path.join(os.path.dirname(__file__), "results_live")


def rpc_get(port, path):
    with urllib.request.urlopen(f"http://localhost:{port}{path}", timeout=5) as resp:
        return json.loads(resp.read())


def block_txs(port, height):
    d = rpc_get(port, f"/block?height={height}")
    return d["result"]["block"]["data"]["txs"], d["result"]["block"]["header"]["time"]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--port", type=int, default=26657)
    parser.add_argument("--last-n", type=int, default=50)
    args = parser.parse_args()

    status = rpc_get(args.port, "/status")
    tip = int(status["result"]["sync_info"]["latest_block_height"])
    start = max(1, tip - args.last_n + 1)

    rows = []
    for h in range(start, tip + 1):
        try:
            txs, block_time = block_txs(args.port, h)
        except Exception as e:
            print(f"height {h}: skip ({e})", file=sys.stderr)
            continue
        marker_bytes = 0
        marker_present = False
        other_tx_bytes = sum(len(base64.b64decode(t)) for t in txs[1:]) if len(txs) > 1 else 0
        if txs:
            raw0 = base64.b64decode(txs[0])
            if raw0.startswith(MARKER):
                marker_present = True
                marker_bytes = len(raw0)
        rows.append({
            "height": h, "block_time": block_time, "num_txs": len(txs),
            "marker_present": marker_present, "extended_proposal_bytes": marker_bytes,
            "other_tx_bytes": other_tx_bytes,
        })

    os.makedirs(RESULTS_DIR, exist_ok=True)
    csv_path = os.path.join(RESULTS_DIR, "table4_live_overhead.csv")
    import csv
    with open(csv_path, "w", newline="") as f:
        w = csv.DictWriter(f, fieldnames=list(rows[0].keys()) if rows else [])
        w.writeheader()
        for r in rows:
            w.writerow(r)

    present = [r for r in rows if r["marker_present"]]
    coverage = len(present) / len(rows) * 100 if rows else 0
    avg_bytes = sum(r["extended_proposal_bytes"] for r in present) / len(present) if present else 0

    md_path = os.path.join(RESULTS_DIR, "table4_live_overhead.md")
    with open(md_path, "w") as f:
        f.write("# E7 LIVE -- Extended Proposal Overhead (real 4-node testnet)\n\n")
        f.write(f"Scanned real heights {start}..{tip} on engram-node01 (port {args.port}).\n\n")
        f.write(f"- Blocks scanned: {len(rows)}\n")
        f.write(f"- Blocks with real `ENGRAM_EXTENDED_PROPOSAL_V1` marker in Txs[0]: {len(present)} ({coverage:.1f}%)\n")
        f.write(f"- Average real marker size: {avg_bytes:.1f} bytes\n\n")
        f.write("| Height | Txs | Marker present | ExtendedProposal bytes | Other tx bytes |\n|---:|---:|---|---:|---:|\n")
        for r in rows:
            f.write(f"| {r['height']} | {r['num_txs']} | {'yes' if r['marker_present'] else 'no'} | {r['extended_proposal_bytes']} | {r['other_tx_bytes']} |\n")

    print(f"scanned {len(rows)} real blocks ({start}..{tip})")
    print(f"marker coverage: {coverage:.1f}%, avg real overhead: {avg_bytes:.1f} bytes")
    print(f"wrote {csv_path}\nwrote {md_path}")


if __name__ == "__main__":
    main()
