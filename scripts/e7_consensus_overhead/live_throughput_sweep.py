#!/usr/bin/env python3
"""Committed-throughput sweep (RQ4, docs/NEXT_SESSION_PLAN.md item 11): open-loop load at increasing
offered rates against the 7-validator cluster, to find where committed tx/s stops following offered tx/s.

Definitions (per repeat):
  offered      transactions the generator actually sent / window  (nominal rate and achieved rate are both
               reported: if achieved < nominal the generator, not the chain, is the limit)
  accepted     transactions the mempool took (broadcast_tx_sync code 0)
  committed    of the accepted ones, how many appear in a block first seen inside the window
  committed/s  committed / window length
  saturated    committed/s < 0.95 * achieved offered rate
Latency is send time -> first 0.25 s poll that sees the transaction in a block (host clock).
Each validator's CPU is sampled once mid-window (`docker stats`) so a CPU-bound knee can be told from a
configuration or generator bound. The load is MsgSubmitForcedTx envelopes: unsigned, no application logic, and
the Engram-specific forced-tx path (queue write, censorship check), so the numbers are a bound on the
consensus layer on this host, not a chain benchmark.

  --arm engram    cluster in ENGRAM_MODE=engram          --arm vanilla7  cluster in ENGRAM_MODE=vanilla
Usage:
    python3 -u scripts/e7_consensus_overhead/live_throughput_sweep.py --arm engram --rates 20 50 100 200
"""

import argparse
import csv
import http.client
import json
import os
import subprocess
import sys
import threading
import time

HERE = os.path.dirname(__file__)
sys.path.insert(0, HERE)
sys.path.insert(0, os.path.join(HERE, "..", "framework"))
import consensus_metrics as cm  # noqa: E402
import live_rq4_metrics as rq4  # noqa: E402
from cluster import CLUSTER  # noqa: E402
from logger import NODE_RPC_PORTS, query_node  # noqa: E402

REPO_ROOT = os.path.abspath(os.path.join(HERE, "..", ".."))
RESULTS_DIR = os.path.join(HERE, "results_live")
BIN = os.path.join(REPO_ROOT, "build", "engramd")


def generate_txs(count: int, prefix: str, payload_bytes: int) -> list:
    out = subprocess.run([BIN, "tx-gen-forced-batch", "--count", str(count), "--prefix", prefix,
                          "--payload-bytes", str(payload_bytes)], capture_output=True, text=True, check=True, timeout=600)
    return out.stdout.split()


def cpu_sample(containers: list) -> dict:
    p = subprocess.run(["docker", "stats", "--no-stream", "--format", "{{.Name}} {{.CPUPerc}} {{.MemUsage}}", *containers],
                       capture_output=True, text=True, timeout=60)
    res = {}
    for line in p.stdout.splitlines():
        parts = line.split()
        if len(parts) >= 2:
            res[parts[0]] = float(parts[1].rstrip("%"))
    return res


def run_one(arm: str, rate: float, repeat: int, args) -> dict:
    ports = [list(NODE_RPC_PORTS.values())[i % len(NODE_RPC_PORTS)] for i in range(args.senders)]
    count = int(rate * args.duration_s * 1.15) + 50
    txs = generate_txs(count, f"sw-{arm}-{int(rate)}-{repeat}-{int(time.time())}", args.payload_bytes)
    per = [txs[i::args.senders] for i in range(args.senders)]
    sample_port = ports[0]
    sent_at, lock = {}, threading.Lock()
    counters = {"sent": 0, "accepted": 0, "rejected": 0, "errors": 0, "lag": 0.0}

    w = rq4.Watcher(sample_port)
    w.start()
    time.sleep(1.0)
    start_height = int(rq4.get(sample_port, "/status")["sync_info"]["latest_block_height"])
    start = time.time()
    deadline = start + args.duration_s
    interval = args.senders / rate
    cpu = {}

    def sender(port: int, mine: list, offset: float):
        conn = http.client.HTTPConnection("127.0.0.1", port, timeout=10)
        nxt = start + offset
        i = 0
        while nxt < deadline and i < len(mine):
            now = time.time()
            if now < nxt:
                time.sleep(min(nxt - now, 0.01))
                continue
            t = time.time()
            ok = False
            try:
                conn.request("GET", f"/broadcast_tx_sync?tx=0x{mine[i]}")
                r = json.loads(conn.getresponse().read())
                ok = r.get("result", {}).get("code", -1) == 0
                with lock:
                    counters["sent"] += 1
                    counters["accepted" if ok else "rejected"] += 1
                    counters["lag"] += max(0.0, t - nxt)
                    if ok:
                        sent_at[__import__("base64").b64encode(bytes.fromhex(mine[i])).decode()] = t
            except Exception:  # noqa: BLE001 -- a failed send is a data point
                with lock:
                    counters["sent"] += 1
                    counters["errors"] += 1
                conn = http.client.HTTPConnection("127.0.0.1", port, timeout=10)
            i += 1
            nxt += interval

    def cpu_probe():
        time.sleep(args.duration_s / 2)
        cpu.update(cpu_sample(list(NODE_RPC_PORTS)))

    # FSM state seen on node01 every 2 s: under load the P2P-latency predicate can demote Engram without any
    # peripheral fault (peer RTT above MaxPeerLatency), and a throughput point measured in SOVEREIGN is not an
    # ANCHORED steady state. vanilla has no FSM query; its samples stay empty.
    states = []

    def state_probe():
        while time.time() < deadline + args.drain_s:
            try:
                states.append(query_node("engram-node01").fsm_state)
            except Exception:  # noqa: BLE001
                pass
            time.sleep(2.0)

    threads = [threading.Thread(target=sender, args=(p, m, k * interval / args.senders), daemon=True)
               for k, (p, m) in enumerate(zip(ports, per))]
    probe = threading.Thread(target=cpu_probe, daemon=True)
    probe.start()
    threading.Thread(target=state_probe, daemon=True).start()
    for t in threads:
        t.start()
    for t in threads:
        t.join()
    time.sleep(args.drain_s)
    w.stop.set()
    w.join(timeout=5)
    end_height = int(rq4.get(sample_port, "/status")["sync_info"]["latest_block_height"])

    in_window = [k for k, t in sent_at.items() if k in w.seen and w.seen[k] <= deadline]
    committed_all = [k for k in sent_at if k in w.seen]
    lat = [w.seen[k] - t for k, t in sent_at.items() if k in w.seen]
    rows = cm.scan_heights(sample_port, start_height + 1, end_height, skip_cert=(arm == "engram"))
    s = cm.summarize(rows, None)
    sent = max(counters["sent"], 1)
    achieved = counters["sent"] / args.duration_s
    # A transaction accepted near the end of the window is committed ~1 block later, so counting only what is
    # committed inside the window under-reads a system that keeps up. If the backlog is gone after the drain,
    # everything accepted was committed at the offered pace; otherwise only the in-window count is trusted.
    drained = counters["accepted"] > 0 and len(committed_all) >= 0.99 * counters["accepted"]
    committed_rate = (len(committed_all) if drained else len(in_window)) / args.duration_s
    return {
        "arm": arm, "nominal_tx_s": rate, "repeat": repeat,
        "achieved_offered_tx_s": round(achieved, 2),
        "generator_mean_lag_s": round(counters["lag"] / sent, 4),
        "accepted": counters["accepted"], "rejected": counters["rejected"], "errors": counters["errors"],
        "committed_in_window": len(in_window), "committed_after_drain": len(committed_all),
        "committed_tx_s": round(committed_rate, 2),
        "committed_over_offered": round(committed_rate / achieved, 3) if achieved else None,
        "backlog_after_drain": counters["accepted"] - len(committed_all),
        "saturated": (not drained) or committed_rate < 0.95 * achieved,
        "latency_p50_s": rq4.pct(lat, 0.5), "latency_p95_s": rq4.pct(lat, 0.95), "latency_p99_s": rq4.pct(lat, 0.99),
        "mempool_mean": sum(w.mempool) / len(w.mempool) if w.mempool else None,
        "mempool_max": max(w.mempool) if w.mempool else None,
        "tx_per_block": s["tx_per_block_mean"], "block_interval_s": s["interval_mean_s"],
        "rounds_per_height": s["rounds_per_height_mean"], "nil_precommits": s["nil_precommits_total"],
        "share_anchored": round(sum(1 for x in states if x == "ANCHORED") / len(states), 3) if any(states) else None,
        "fsm_states_seen": "|".join(sorted(set(x for x in states if x))),
        "validator_cpu_pct_mean": round(sum(cpu.values()) / len(cpu), 1) if cpu else None,
        "validator_cpu_pct_max": max(cpu.values()) if cpu else None,
    }


def wait_mempool_empty(port: int, timeout_s: float = 180.0) -> None:
    deadline = time.time() + timeout_s
    while time.time() < deadline:
        try:
            if int(rq4.get(port, "/num_unconfirmed_txs")["n_txs"]) == 0:
                return
        except Exception:  # noqa: BLE001
            pass
        time.sleep(3)


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--arm", choices=["engram", "vanilla7"], required=True)
    ap.add_argument("--rates", type=float, nargs="+", default=[20, 50, 100, 200])
    ap.add_argument("--repeats", type=int, default=5)
    ap.add_argument("--duration-s", type=float, default=60.0)
    ap.add_argument("--drain-s", type=float, default=15.0)
    ap.add_argument("--senders", type=int, default=8)
    ap.add_argument("--payload-bytes", type=int, default=32)
    ap.add_argument("--tag", default="", help="label for the output files, e.g. a changed protocol parameter")
    args = ap.parse_args()

    want = "engram" if args.arm == "engram" else "vanilla"
    for node in NODE_RPC_PORTS:
        got = CLUSTER.container_env(node, "ENGRAM_MODE") or "engram"
        if got != want:
            sys.exit(f"{node} runs ENGRAM_MODE={got}, but --arm {args.arm} needs {want}")
    if not os.path.exists(BIN):
        sys.exit(f"build {BIN} first (make build)")
    os.makedirs(RESULTS_DIR, exist_ok=True)

    rows = []
    for rate in args.rates:
        for r in range(1, args.repeats + 1):
            print(f"[{time.strftime('%H:%M:%S')}] {args.arm} rate={rate} repeat {r}/{args.repeats}", flush=True)
            wait_mempool_empty(list(NODE_RPC_PORTS.values())[0])
            rows.append(run_one(args.arm, rate, r, args))
            print({k: rows[-1][k] for k in ("achieved_offered_tx_s", "committed_tx_s", "committed_over_offered", "latency_p95_s", "validator_cpu_pct_max", "saturated")}, flush=True)
    stamp = time.strftime("%Y%m%dT%H%M%S")
    path = os.path.join(RESULTS_DIR, f"throughput_{args.arm}{'_' + args.tag if args.tag else ''}_{stamp}.csv")
    with open(path, "w", newline="") as f:
        w = csv.DictWriter(f, fieldnames=list(rows[0]))
        w.writeheader()
        w.writerows(rows)
    summary = {}
    for rate in args.rates:
        sel = [x for x in rows if x["nominal_tx_s"] == rate]
        cell = {"n": len(sel)}
        for k in ("achieved_offered_tx_s", "committed_tx_s", "committed_over_offered", "latency_p50_s", "latency_p95_s",
                  "latency_p99_s", "mempool_max", "validator_cpu_pct_max", "block_interval_s", "tx_per_block"):
            vals = [x[k] for x in sel if isinstance(x[k], (int, float))]
            if vals:
                m = sum(vals) / len(vals)
                sd = (sum((v - m) ** 2 for v in vals) / (len(vals) - 1)) ** 0.5 if len(vals) > 1 else 0.0
                cell[k] = {"mean": m, "stdev": sd, "ci95": 1.96 * sd / len(vals) ** 0.5}
        cell["saturated_runs"] = sum(1 for x in sel if x["saturated"])
        summary[str(rate)] = cell
    with open(path.replace(".csv", "_summary.json"), "w") as f:
        json.dump({"arm": args.arm, "tag": args.tag, "duration_s": args.duration_s, "payload_bytes": args.payload_bytes,
                   "senders": args.senders, "cells": summary}, f, indent=2)
    print("wrote", path)


if __name__ == "__main__":
    main()
