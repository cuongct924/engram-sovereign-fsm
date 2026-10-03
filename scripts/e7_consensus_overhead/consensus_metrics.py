#!/usr/bin/env python3
"""Post-hoc per-height consensus metrics for RQ4 -- docs/NEXT_SESSION_PLAN.md item 11.

Scans EVERY height in a range (no polling, so no blind spots) and attributes the block-rate
overhead to extra consensus rounds vs. proposal size vs. validation rejects:

  * rounds per height   -- /commit signed_header.commit.round (+1 = rounds used)
  * nil precommits      -- last-commit signatures with block_id_flag == 3 (nil), per height
  * absent precommits   -- block_id_flag == 1
  * tx per block / block bytes -- /block data.txs (Txs[0] is the HealthCertificate in
                           extended mode, so --skip-cert subtracts 1 and its bytes)
  * block interval      -- header.time deltas between consecutive heights
  * REJECT histogram    -- `ProcessProposal REJECT ... reason=` lines from `docker logs`

Nil prevotes are not stored on chain, so they are not measured here; the REJECT histogram is
the proxy for why a validator prevoted nil. Stdlib only.

Usage:
    python3 consensus_metrics.py --from-height 100 --to-height 400 --skip-cert \
        --docker-logs engram-node01,engram-node02 --out results_live/deep_<tag>
    python3 consensus_metrics.py --last 300 --skip-cert        # most recent N heights
"""

import argparse
import base64
import csv
import json
import os
import re
import subprocess
import sys
import time
import urllib.request
from collections import Counter
from datetime import datetime

RESULTS_DIR = os.path.join(os.path.dirname(__file__), "results_live")
REJECT_RE = re.compile(r"ProcessProposal REJECT height=(\d+) round=(\d+) reason=(\w+)")
BLOCK_ID_FLAG_ABSENT, BLOCK_ID_FLAG_COMMIT, BLOCK_ID_FLAG_NIL = 1, 2, 3


def rpc(port: int, path: str, timeout: float = 5.0) -> dict:
    with urllib.request.urlopen(f"http://127.0.0.1:{port}{path}", timeout=timeout) as r:
        return json.load(r)["result"]


def parse_time(s: str) -> float:
    # CometBFT emits RFC3339 with nanoseconds; datetime only takes microseconds.
    s = s.rstrip("Z")
    head, _, frac = s.partition(".")
    dt = datetime.strptime(head, "%Y-%m-%dT%H:%M:%S")
    return dt.timestamp() + (float("0." + frac) if frac else 0.0)  # naive local-time epoch; only deltas are used


def percentile(sorted_vals: list, q: float):
    if not sorted_vals:
        return None
    return sorted_vals[min(len(sorted_vals) - 1, int(len(sorted_vals) * q))]


def scan_heights(port: int, lo: int, hi: int, skip_cert: bool) -> list:
    rows, prev_time = [], None
    for h in range(lo, hi + 1):
        blk = rpc(port, f"/block?height={h}")["block"]
        commit = rpc(port, f"/commit?height={h}")["signed_header"]["commit"]
        txs = blk["data"].get("txs") or []
        sizes = [len(base64.b64decode(t)) for t in txs]
        if skip_cert and sizes:
            sizes = sizes[1:]
        sigs = commit.get("signatures", [])
        flags = Counter(s["block_id_flag"] for s in sigs)
        t = parse_time(blk["header"]["time"])
        rows.append({
            "height": h,
            "round": int(commit["round"]),
            "num_txs": len(sizes),
            "tx_bytes": sum(sizes),
            "nil_precommits": flags.get(BLOCK_ID_FLAG_NIL, 0),
            "absent_precommits": flags.get(BLOCK_ID_FLAG_ABSENT, 0),
            "commit_precommits": flags.get(BLOCK_ID_FLAG_COMMIT, 0),
            "interval_s": (t - prev_time) if prev_time is not None else None,
        })
        prev_time = t
    return rows


def collect_rejects(containers: list, lo: int, hi: int, since: float = 0.0) -> tuple:
    """Count distinct (height, round, reason) rejects so one proposal logged by several
    validators is counted once per reason, plus a per-reason validator count."""
    per_validator, distinct = Counter(), set()
    for c in containers:
        # --since bounds the dump: validators log at debug level, so a whole-container
        # `docker logs` on a long-running cluster takes minutes per call.
        cmd = ["docker", "logs"] + (["--since", str(int(since))] if since else []) + [c]
        res = subprocess.run(cmd, capture_output=True, text=True, timeout=300)
        out = res.stdout + res.stderr  # engramd logs to stderr
        for m in REJECT_RE.finditer(out):
            h, r, reason = int(m.group(1)), int(m.group(2)), m.group(3)
            if lo <= h <= hi:
                per_validator[reason] += 1
                distinct.add((h, r, reason))
    by_reason = Counter(reason for _, _, reason in distinct)
    return by_reason, per_validator


def summarize(rows: list, rejects) -> dict:
    n = len(rows)
    rounds = [r["round"] + 1 for r in rows]
    intervals = sorted(r["interval_s"] for r in rows if r["interval_s"] is not None)
    span = sum(intervals)
    multi = sum(1 for x in rounds if x > 1)
    s = {
        "heights": n,
        "span_s": span,
        "blocks_per_s": (len(intervals) / span) if span else None,
        "rounds_per_height_mean": sum(rounds) / n if n else None,
        "heights_with_extra_round": multi,
        "extra_round_fraction": multi / n if n else None,
        "nil_precommits_total": sum(r["nil_precommits"] for r in rows),
        "absent_precommits_total": sum(r["absent_precommits"] for r in rows),
        "committed_tx_per_s": (sum(r["num_txs"] for r in rows) / span) if span else None,
        "tx_per_block_mean": sum(r["num_txs"] for r in rows) / n if n else None,
        "tx_bytes_per_block_mean": sum(r["tx_bytes"] for r in rows) / n if n else None,
        "interval_mean_s": span / len(intervals) if intervals else None,
        "interval_p50_s": percentile(intervals, 0.5),
        "interval_p95_s": percentile(intervals, 0.95),
        # Intervals above 1.5x the median are the extra-round candidates; if this tracks
        # extra_round_fraction the gap is round skips, if not it is something else.
        "slow_interval_fraction": (
            sum(1 for v in intervals if v > 1.5 * percentile(intervals, 0.5)) / len(intervals)
            if intervals else None
        ),
    }
    if rejects is not None:
        s["rejects_distinct_by_reason"] = dict(rejects[0])
        s["rejects_per_validator_by_reason"] = dict(rejects[1])
    return s


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--port", type=int, default=26657, help="RPC port of the node to scan")
    ap.add_argument("--from-height", type=int)
    ap.add_argument("--to-height", type=int)
    ap.add_argument("--last", type=int, help="scan the most recent N heights instead")
    ap.add_argument("--skip-cert", action="store_true", help="extended mode: drop Txs[0] (HealthCertificate)")
    ap.add_argument("--docker-logs", default="", help="comma-separated containers for the REJECT histogram")
    ap.add_argument("--out", help="output prefix (writes <out>.csv and <out>_summary.json)")
    args = ap.parse_args()

    latest = int(rpc(args.port, "/status")["sync_info"]["latest_block_height"])
    if args.last:
        hi, lo = latest, max(2, latest - args.last + 1)
    else:
        lo, hi = args.from_height, args.to_height or latest
    if lo is None:
        ap.error("give --last or --from-height")

    rows = scan_heights(args.port, lo, hi, args.skip_cert)
    containers = [c for c in args.docker_logs.split(",") if c]
    rejects = collect_rejects(containers, lo, hi) if containers else None
    summary = summarize(rows, rejects)
    summary.update({"from_height": lo, "to_height": hi, "port": args.port, "skip_cert": args.skip_cert})

    out = args.out or os.path.join(RESULTS_DIR, f"deep_{time.strftime('%Y%m%dT%H%M%S')}")
    os.makedirs(os.path.dirname(os.path.abspath(out)), exist_ok=True)
    with open(out + ".csv", "w", newline="") as f:
        w = csv.DictWriter(f, fieldnames=list(rows[0].keys()))
        w.writeheader()
        w.writerows(rows)
    with open(out + "_summary.json", "w") as f:
        json.dump(summary, f, indent=2)
    print(json.dumps(summary, indent=2))
    print(f"wrote {out}.csv, {out}_summary.json", file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main())
