#!/usr/bin/env python3
"""RQ4 consensus-overhead rows that live_throughput_latency.py does not produce
(docs/NEXT_SESSION_PLAN.md item 11; paper Table `tab:overhead`, lower block).

One timed load phase per repeat (default 60 s at 20 tx/s, 5 repeats), against either the running
7-validator cluster (--arm engram) or a local single-process vanilla node (--arm vanilla; the same
asymmetric arm live_throughput_latency.py uses). Per repeat it records, on the host clock:
  * committed tx/s            our txs found in blocks / load window
  * commit latency p50 / p95  send time -> first poll (0.25 s) that sees the tx in a block
  * mempool backlog mean/max  /num_unconfirmed_txs once per second on the sampled node
and, over the repeat's heights (consensus_metrics.py): rounds per height, nil precommits per
height, proposals rejected per height (distinct height/round/reason, from docker logs), tx per
block (Txs[0], the health certificate, excluded in engram mode).

Nil prevotes are not stored on chain: rejected proposals are the proxy for why a validator
prevoted nil. Usage:
    python3 -u scripts/e7_consensus_overhead/live_rq4_metrics.py --arm engram
    python3 -u scripts/e7_consensus_overhead/live_rq4_metrics.py --arm vanilla
"""

import argparse
import base64
import csv
import json
import os
import subprocess
import sys
import threading
import time
import urllib.request

HERE = os.path.dirname(__file__)
sys.path.insert(0, HERE)
sys.path.insert(0, os.path.join(HERE, "..", "framework"))
import consensus_metrics as cm  # noqa: E402
import live_throughput_latency as ltl  # noqa: E402
from logger import NODE_RPC_PORTS  # noqa: E402

RESULTS_DIR = os.path.join(HERE, "results_live")
POLL_S = 0.25


def get(port: int, path: str, timeout: float = 5.0):
    with urllib.request.urlopen(f"http://127.0.0.1:{port}{path}", timeout=timeout) as r:
        return json.load(r)["result"]


class Watcher(threading.Thread):
    """Host-clock observation of block inclusion and mempool size on one node."""

    def __init__(self, port: int):
        super().__init__(daemon=True)
        self.port = port
        self.stop = threading.Event()
        self.seen = {}  # tx base64 -> first observation time
        self.mempool = []
        self.last_height = None
        self._next_mem = 0.0

    def run(self):
        while not self.stop.is_set():
            try:
                h = int(get(self.port, "/status")["sync_info"]["latest_block_height"])
                if self.last_height is None:
                    self.last_height = h
                for height in range(self.last_height + 1, h + 1):
                    t = time.time()
                    for tx in get(self.port, f"/block?height={height}")["block"]["data"].get("txs") or []:
                        self.seen.setdefault(tx, t)
                self.last_height = max(self.last_height, h)
                if time.time() >= self._next_mem:
                    self.mempool.append(int(get(self.port, "/num_unconfirmed_txs")["n_txs"]))
                    self._next_mem = time.time() + 1.0
            except Exception:  # noqa: BLE001 -- a missed poll is a gap, not a crash
                pass
            time.sleep(POLL_S)


def pct(vals, q):
    vals = sorted(vals)
    return vals[min(len(vals) - 1, int(len(vals) * q))] if vals else None


def one_repeat(arm: str, ports: list, duration_s: float, rate_hz: float, repeat: int, containers: list) -> dict:
    sample_port = ports[0]
    n = len(ports)
    count = int(duration_s * rate_hz) + 20 * n
    hexes = ltl.build_forced_tx_hexes(f"rq4-{arm}-{repeat}-{int(time.time())}", count, f"http://127.0.0.1:{sample_port}")
    per = [hexes[i::n] for i in range(n)]
    sent_at = {}
    lock = threading.Lock()

    w = Watcher(sample_port)
    w.start()
    time.sleep(1.0)
    start_height = int(get(sample_port, "/status")["sync_info"]["latest_block_height"])
    start = time.time()
    deadline = start + duration_s

    def sender(port, my):
        interval = n / rate_hz
        nxt = start
        i = 0
        while time.time() < deadline and i < len(my):
            if time.time() >= nxt:
                b64 = base64.b64encode(bytes.fromhex(my[i])).decode()
                t = time.time()
                if ltl.broadcast_sync(f"http://127.0.0.1:{port}", my[i]):
                    with lock:
                        sent_at[b64] = t
                i += 1
                nxt += interval
            time.sleep(0.005)

    threads = [threading.Thread(target=sender, args=(p, m), daemon=True) for p, m in zip(ports, per)]
    for t in threads:
        t.start()
    for t in threads:
        t.join()
    time.sleep(10.0)  # drain: let the last accepted txs commit
    w.stop.set()
    w.join(timeout=5)
    end_height = int(get(sample_port, "/status")["sync_info"]["latest_block_height"])

    lat = [w.seen[k] - t for k, t in sent_at.items() if k in w.seen]
    in_window = [k for k, t in sent_at.items() if k in w.seen and w.seen[k] <= deadline + 10.0]
    rows = cm.scan_heights(sample_port, start_height + 1, end_height, skip_cert=(arm == "engram"))
    # Per-height rows are the evidence for the round/interval claims: keep them.
    with open(os.path.join(RESULTS_DIR, f"rq4_{arm}_heights_r{repeat}_{int(start)}.csv"), "w", newline="") as hf:
        hw = csv.DictWriter(hf, fieldnames=list(rows[0]))
        hw.writeheader()
        hw.writerows(rows)
    rejects = cm.collect_rejects(containers, start_height + 1, end_height, since=start - 5) if containers else None
    s = cm.summarize(rows, rejects)
    heights = max(len(rows), 1)
    distinct_rejects = sum(s.get("rejects_distinct_by_reason", {}).values())
    return {
        "repeat": repeat,
        "accepted": len(sent_at),
        "committed": len(in_window),
        "committed_tx_per_s": len(in_window) / duration_s,
        "latency_p50_s": pct(lat, 0.5),
        "latency_p95_s": pct(lat, 0.95),
        "mempool_mean": sum(w.mempool) / len(w.mempool) if w.mempool else None,
        "mempool_max": max(w.mempool) if w.mempool else None,
        "heights": len(rows),
        "rounds_per_height": s["rounds_per_height_mean"],
        "nil_precommits_per_height": s["nil_precommits_total"] / heights,
        "rejected_per_height": distinct_rejects / heights,
        "tx_per_block": s["tx_per_block_mean"],
        "blocks_per_s": s["blocks_per_s"],
        "interval_mean_s": s["interval_mean_s"],
        "interval_p50_s": s["interval_p50_s"],
        "interval_p95_s": s["interval_p95_s"],
        "slow_interval_fraction": s["slow_interval_fraction"],
        "reject_reasons": json.dumps(s.get("rejects_distinct_by_reason", {}), sort_keys=True),
    }


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--arm", choices=["engram", "vanilla"], required=True)
    ap.add_argument("--repeats", type=int, default=5)
    ap.add_argument("--duration-s", type=float, default=60.0)
    ap.add_argument("--rate-hz", type=float, default=20.0)
    ap.add_argument("--senders", type=int, default=4)
    ap.add_argument("--warmup-s", type=float, default=15.0)
    args = ap.parse_args()
    os.makedirs(RESULTS_DIR, exist_ok=True)

    proc = None
    ltl.build_binary()  # the forced-tx CLI builds the load payloads in both arms
    if args.arm == "engram":
        ports = [list(NODE_RPC_PORTS.values())[i % len(NODE_RPC_PORTS)] for i in range(args.senders)]
        containers = list(NODE_RPC_PORTS)
    else:
        ltl.init_home(ltl.HOME_VANILLA, "rq4-vanilla", shift_ports=True)
        proc = ltl.start_node(ltl.HOME_VANILLA, True, "/tmp/engramd-rq4-vanilla.log")
        ports = [ltl.PORT_VANILLA] * args.senders
        containers = []
    try:
        time.sleep(args.warmup_s)
        rows = []
        for r in range(1, args.repeats + 1):
            print(f"[{ltl.now()}] repeat {r}/{args.repeats}")
            rows.append(one_repeat(args.arm, ports, args.duration_s, args.rate_hz, r, containers))
            print({k: v for k, v in rows[-1].items() if k != "reject_reasons"})
    finally:
        if proc:
            proc.terminate()

    stamp = time.strftime("%Y%m%dT%H%M%S")
    path = os.path.join(RESULTS_DIR, f"rq4_{args.arm}_{stamp}.csv")
    with open(path, "w", newline="") as f:
        w = csv.DictWriter(f, fieldnames=list(rows[0]))
        w.writeheader()
        w.writerows(rows)
    summary = {}
    for k in rows[0]:
        vals = [r[k] for r in rows if isinstance(r[k], (int, float)) and r[k] is not None]
        if vals and k != "repeat":
            m, sd, ci = ltl.mean_stdev_ci95(vals)
            summary[k] = {"mean": m, "stdev": sd, "ci95": ci}
    with open(path.replace(".csv", "_summary.json"), "w") as f:
        json.dump({"arm": args.arm, "repeats": args.repeats, "duration_s": args.duration_s, "rate_hz": args.rate_hz,
                   "metrics": summary}, f, indent=2)
    print(json.dumps(summary, indent=1))
    print("wrote", path)


if __name__ == "__main__":
    main()
