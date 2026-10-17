#!/usr/bin/env python3
"""ExtendVote time against Bitcoin RPC latency on the multi-host cluster (Testbed B): netem adds a fixed delay to every Bitcoin RPC
response leaving the infra host (all seven validators see it), one window per delay, no transaction load. The advisor's claim to
show: ExtendVote time no longer grows with RPC latency (snapshot arm), while the inline-RPC arm (sync) grows with it.

    ENGRAM_CLUSTER=ssh python3 -u scripts/e7_consensus_overhead/live_rpc_latency.py --arm sync --deploy
    ENGRAM_CLUSTER=ssh python3 -u scripts/e7_consensus_overhead/live_rpc_latency.py --arm snapshot --deploy

Per window: blocks and block interval, ExtendVote ms (mean / p95 / max) from the `extend_vote height=.. ms=..` lines, the FSM states seen
and, on the snapshot arm, the number of `snapshot_degraded=true` lines. The first SETTLE_S seconds of a window are not counted (the
delay takes effect, the cache refills). The delay is always removed on exit. Delays stay below the 800 ms RPC timeout minus the
validator-to-infra round trip, so they test "slow", not "timed out" (that is live_rpc_hang.py).
"""

import argparse
import csv
import json
import os
import re
import statistics
import sys
import time

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
sys.path.insert(0, os.path.join(HERE, "..", "framework"))
from cluster import CLUSTER  # noqa: E402
from logger import query_node  # noqa: E402
import run_cluster_ab as ab  # noqa: E402

RESULTS = os.path.join(HERE, "results_live")
EXTEND_RE = re.compile(r"extend_vote height=(\d+) ms=([0-9.]+)")
SNAP_RE = re.compile(r"sensor_snapshot height=(\d+) .*?snapshot_degraded=(true|false)")
SETTLE_S = 15.0


def pct(xs, p):
    if not xs:
        return None
    xs = sorted(xs)
    return xs[min(len(xs) - 1, int(round(p * (len(xs) - 1))))]


def sample_all():
    out = []
    for n in CLUSTER.nodes:
        try:
            s = query_node(n)
            out.append((n, s.height, s.fsm_state, ""))
        except Exception as exc:  # noqa: BLE001 -- recorded, not fatal
            out.append((n, 0, "", str(exc)[:80]))
    return out


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--arm", choices=["sync", "snapshot"], required=True)
    ap.add_argument("--deploy", action="store_true")
    ap.add_argument("--delays-ms", default="0,100,300,500")
    ap.add_argument("--window-s", type=float, default=90)
    ap.add_argument("--tag", default="b4")
    args = ap.parse_args()
    if CLUSTER.name != "ssh":
        sys.exit("run with ENGRAM_CLUSTER=ssh")
    if args.deploy:
        for var in ("DOCKERHUB_USER", "ENGRAM_IMAGE_TAG", "BITCOIN_RPC_PASSWORD"):
            if not os.environ.get(var):
                sys.exit(f"{var} is not set")
        ab.deploy(args.arm)
    ab.check_config(args.arm)
    if not ab.wait_anchored(1800):
        sys.exit("the chain is not ANCHORED on all validators")

    rows, windows, t0 = [], [], time.time()
    try:
        for d in [int(x) for x in args.delays_ms.split(",")]:
            CLUSTER.btc_delay(False)
            if d > 0:
                CLUSTER.btc_delay(True, d, 0)
            print(f"== delay {d} ms", flush=True)
            start = time.time()
            first_h = None
            end = start + args.window_s
            while time.time() < end:
                t = time.time()
                for n, h, st, err in sample_all():
                    rows.append(dict(t=round(t - t0, 2), delay_ms=d, node=n, height=h, fsm_state=st, error=err))
                    if first_h is None and t - start >= SETTLE_S and h:
                        first_h = h
                time.sleep(max(0.0, 2.0 - (time.time() - t)))
            last = [r["height"] for r in rows if r["delay_ms"] == d and r["height"]]
            windows.append(dict(delay_ms=d, from_height=first_h or min(last), to_height=max(last), t_from=start + SETTLE_S, t_to=end))
    finally:
        CLUSTER.btc_delay(False)
        print("== delay removed", flush=True)

    print("== reading validator logs", flush=True)
    ev, snap = {}, []
    for n in CLUSTER.nodes:
        for line in CLUSTER.logs(n).splitlines():
            m = EXTEND_RE.search(line)
            if m:
                ev.setdefault(int(m.group(1)), []).append(float(m.group(2)))
                continue
            m = SNAP_RE.search(line)
            if m:
                snap.append((int(m.group(1)), m.group(2) == "true"))

    summary = dict(arm=args.arm, image_tag=os.environ.get("ENGRAM_IMAGE_TAG"), window_s=args.window_s, windows=[])
    for w in windows:
        lo, hi = w["from_height"], w["to_height"]
        ms = [x for h in range(lo, hi + 1) for x in ev.get(h, [])]
        ps = [r for r in rows if r["delay_ms"] == w["delay_ms"] and r["t"] + t0 >= w["t_from"] and r["height"]]
        span = (max(r["t"] for r in ps) - min(r["t"] for r in ps)) if ps else 0
        blocks = max(0, hi - lo)
        states = {}
        for r in ps:
            states[r["fsm_state"]] = states.get(r["fsm_state"], 0) + 1
        sl = [s for s in snap if lo <= s[0] <= hi]
        summary["windows"].append(dict(
            delay_ms=w["delay_ms"], heights=[lo, hi], blocks=blocks, block_interval_s=round(span / blocks, 3) if blocks else None,
            extend_vote_ms_mean=round(statistics.mean(ms), 2) if ms else None, extend_vote_ms_p95=pct(ms, 0.95),
            extend_vote_ms_max=max(ms) if ms else None, extend_vote_samples=len(ms), fsm_state_samples=states,
            snapshot_lines=len(sl), snapshot_degraded_lines=sum(1 for s in sl if s[1])))

    stamp = time.strftime("%Y%m%dT%H%M%S")
    os.makedirs(RESULTS, exist_ok=True)
    base = os.path.join(RESULTS, f"rpc_latency_{args.arm}_{args.tag}_{stamp}")
    with open(base + ".csv", "w", newline="") as f:
        w = csv.DictWriter(f, fieldnames=["t", "delay_ms", "node", "height", "fsm_state", "error"])
        w.writeheader()
        w.writerows(rows)
    with open(base + "_summary.json", "w") as f:
        json.dump(summary, f, indent=2)
    print(json.dumps(summary, indent=2))
    print("wrote", base + "_summary.json")


if __name__ == "__main__":
    main()
