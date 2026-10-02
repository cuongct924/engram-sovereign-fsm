#!/usr/bin/env python3
"""LIVE DA-backfill test (docs/EXPERIMENT.md E3 S3, backfill row): a cluster-wide DA outage of a
given length, then recovery; measures how long the chain stays in RECOVERING while the
SOVEREIGN interval's block data is republished, and re-fetches that interval from Celestia
(verify_backfill.py) to check the exit gate's claim independently.

Per outage length (seconds): stop every validator's Celestia bridge, hold, restart, then record
  - seconds to SOVEREIGN, the Engram height at which it began,
  - the height at which RECOVERING began (end of the interval) and seconds spent in RECOVERING,
  - interval length in blocks,
  - verify_backfill: share of interval heights found on Celestia and byte-identical to the block.

Usage (7-node cluster, healthy and ANCHORED):
    python3 -u scripts/e3_failure_matrix/live_backfill_test.py --outage-s 120 300 600
"""

import argparse
import csv
import json
import os
import subprocess
import sys
import time

HERE = os.path.dirname(__file__)
sys.path.insert(0, os.path.join(HERE, "..", "framework"))
sys.path.insert(0, HERE)
from logger import NODE_RPC_PORTS, sample_all_nodes, start_all_da_bridges, stop_all_da_bridges  # noqa: E402
import verify_backfill as vb  # noqa: E402

RESULTS_DIR = os.path.join(HERE, "results_live")
POLL_S = 3.0
NODE = "engram-node01"


def now() -> str:
    return time.strftime("%H:%M:%S", time.gmtime())


def snapshot():
    samples = sample_all_nodes()
    ref = next((s for s in samples if s.node == NODE and s.height > 0), None)
    states = {s.node: s.fsm_state for s in samples}
    return ref, states


def wait_state(target: str, timeout_s: float):
    """Returns (height, elapsed) at the first poll where node01 reports target, else None."""
    t0 = time.time()
    while time.time() - t0 < timeout_s:
        ref, _ = snapshot()
        if ref and ref.fsm_state == target:
            return ref.height, time.time() - t0
        time.sleep(POLL_S)
    return None


def celestia_head(token: str) -> int:
    return int(vb.rpc("http://localhost:26658", token, "header.LocalHead", [])["header"]["height"])


def run_one(outage_s: int, args) -> dict:
    row = {"outage_s": outage_s}
    print(f"[{now()}] === outage {outage_s}s: waiting for ANCHORED")
    if not wait_state("ANCHORED", 900):
        row["error"] = "cluster not ANCHORED before the outage"
        return row
    token = vb.load_token("CELESTIA_BRIDGE_NODE01_AUTH_TOKEN")
    c0 = celestia_head(token)
    t_stop = time.time()
    stop_all_da_bridges()
    print(f"[{now()}] bridges stopped; holding {outage_s}s")

    sov_height, sov_t = None, None
    while time.time() - t_stop < outage_s:
        ref, _ = snapshot()
        if ref and ref.fsm_state == "SOVEREIGN" and sov_height is None:
            sov_height, sov_t = ref.height, time.time() - t_stop
            print(f"[{now()}] SOVEREIGN at height {sov_height} after {sov_t:.0f}s")
        time.sleep(POLL_S)
    start_all_da_bridges()
    t_heal = time.time()
    print(f"[{now()}] bridges restarted")

    rec = wait_state("RECOVERING", args.recovery_timeout_s)
    row.update({"sovereign_height": sov_height, "seconds_to_sovereign": round(sov_t, 1) if sov_t else None})
    if sov_height is None or rec is None:
        row["error"] = "never reached SOVEREIGN" if sov_height is None else "never reached RECOVERING"
        return row
    rec_height, _ = rec
    t_rec = time.time()
    anchored = wait_state("ANCHORED", args.recovery_timeout_s)
    row.update({
        "recovering_height": rec_height,
        "interval_blocks": rec_height - sov_height,
        "seconds_heal_to_recovering": round(t_rec - t_heal, 1),
        "seconds_in_recovering": round(time.time() - t_rec, 1) if anchored else None,
        "anchored_height": anchored[0] if anchored else None,
    })
    if not anchored:
        row["error"] = "never returned to ANCHORED"
        return row

    time.sleep(args.settle_s)  # let the last batches confirm on Celestia
    ns = argparse.Namespace(engram_from=sov_height, engram_to=rec_height - 1, celestia_from=c0, celestia_to=0,
                            bridge_url="http://localhost:26658", engram_rpc="http://localhost:26657",
                            namespace="engramda01", token_var="CELESTIA_BRIDGE_NODE01_AUTH_TOKEN", out=os.path.join(
                                RESULTS_DIR, f"backfill_verify_{outage_s}s_{time.strftime('%Y%m%dT%H%M%S')}.csv"))
    try:
        summary = vb.verify(ns)
    except Exception as e:  # noqa: BLE001 -- keep the timing data even if the re-fetch fails
        row["error"] = f"verify_backfill failed: {e}"
        return row
    row.update({"verify_published_pct": summary["published_pct"], "verify_match_pct": summary["match_pct"],
                "verify_heights": summary["heights"]})
    print(f"[{now()}] verify_backfill: {summary}")
    return row


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--outage-s", type=int, nargs="+", default=[120, 300, 600])
    ap.add_argument("--recovery-timeout-s", type=float, default=1800.0)
    ap.add_argument("--settle-s", type=float, default=30.0)
    args = ap.parse_args()
    if len(NODE_RPC_PORTS) < 4:
        sys.exit("needs the running Docker testnet")
    os.makedirs(RESULTS_DIR, exist_ok=True)
    rows = []
    try:
        for L in args.outage_s:
            rows.append(run_one(L, args))
            print("ROW", json.dumps(rows[-1]), flush=True)  # survives a later crash
    finally:
        start_all_da_bridges()
    path = os.path.join(RESULTS_DIR, f"backfill_{time.strftime('%Y%m%dT%H%M%S')}.csv")
    fields = sorted({k for r in rows for k in r}, key=lambda k: (k != "outage_s", k))
    with open(path, "w", newline="") as f:
        w = csv.DictWriter(f, fieldnames=fields)
        w.writeheader()
        w.writerows(rows)
    print(json.dumps(rows, indent=1))
    print("wrote", path)
    sys.exit(0 if all("error" not in r and r.get("verify_match_pct") == 100.0 for r in rows) else 1)


if __name__ == "__main__":
    main()
