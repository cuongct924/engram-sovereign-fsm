#!/usr/bin/env python3
"""Why does Engram's FSM leave ANCHORED under transaction load?

Raising MaxPeerLatency to 5000 ms did not keep the chain in ANCHORED at 100-500 tx/s, so the peer-latency bound is
not the only cause. This reads every validator's own sensor_snapshot lines (what each node measured, logged but never
committed) for an idle window and for a load window, and counts, per predicate of IsHealthyCondition
(x/sovereignty/types/predicates.go), the share of snapshots that violate it, using the thresholds of the running
genesis. The committed aggregate is a weighted median of these readings, so a predicate that fails on most nodes
under load and on none while idle is the cause of the demotion. subnet_diversity is not in the snapshot line and is
not checked.

    python3 scripts/e7_consensus_overhead/diagnose_load_demotion.py [--rate 200] [--idle-s 60]
"""

import argparse
import json
import os
import re
import subprocess
import sys
import time
from datetime import datetime, timezone

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.abspath(os.path.join(HERE, "..", ".."))
sys.path.insert(0, os.path.join(HERE, "..", "framework"))
from cluster import CLUSTER  # noqa: E402

RESULTS = os.path.join(HERE, "results_live")
SNAP = re.compile(
    r"^(?P<ts>\S+) .*sensor_snapshot height=(?P<height>\d+) btc_gap=(?P<btc_gap>\d+) "
    r"anchor_reverification_failed=(?P<arf>\w+) da_gap=(?P<da_gap>\d+) da_failed=(?P<da_failed>\w+) "
    r"attestation_failed=(?P<att>\w+) active_anchors=(?P<anchors>\d+) clean_peers=(?P<peers>\d+) "
    r"churn_rate=(?P<churn>\d+) avg_tenure=(?P<tenure>\d+) peer_latency=(?P<lat>\d+)")


def genesis_params():
    with open(os.path.join(REPO, "testnet-data", CLUSTER.nodes[0], "config", "genesis.json")) as f:
        return json.load(f)["app_state"]["sovereignty"]["params"]


def snapshots(node, since, until):
    out = subprocess.run(["docker", "logs", "-t", "--since", since, "--until", until, node],
                         capture_output=True, text=True, timeout=300)
    rows = []
    for line in (out.stdout + out.stderr).splitlines():
        m = SNAP.match(line)
        if m:
            rows.append({k: (v == "true" if v in ("true", "false") else v) for k, v in m.groupdict().items()})
    return rows


def violations(r, p):
    gap, da = int(r["btc_gap"]), int(r["da_gap"])
    return {
        "btc_gap_suspicious": p["suspicious_threshold"] <= gap < p["sovereign_threshold"],
        "btc_gap_sovereign": gap >= p["sovereign_threshold"],
        "anchor_reverification_failed": r["arf"],
        "da_gap": da >= p["da_threshold"],
        "da_failed": r["da_failed"],
        "attestation_failed": r["att"],
        "active_anchors": int(r["anchors"]) < p["min_anchor_peers"],
        "clean_peers": int(r["peers"]) < p["min_peers"],
        "churn_rate": int(r["churn"]) > p["max_churn_rate"],
        "avg_tenure": int(r["tenure"]) < p["min_avg_tenure"],
        "peer_latency": int(r["lat"]) > p["max_peer_latency"],
    }


def summarize(window, p):
    table = {}
    for node, rows in window.items():
        for r in rows:
            for name, bad in violations(r, p).items():
                c = table.setdefault(name, [0, 0])
                c[0] += int(bool(bad))
                c[1] += 1
    return {k: (v[0] / v[1] if v[1] else None, v[1]) for k, v in table.items()}


def stamp(t):
    return datetime.fromtimestamp(t, tz=timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--rate", type=float, default=200)
    ap.add_argument("--idle-s", type=float, default=60)
    args = ap.parse_args()

    p = genesis_params()
    t_idle = time.time()
    time.sleep(args.idle_s)
    t_load = time.time()
    sweep = subprocess.run([sys.executable, "-u", os.path.join(HERE, "live_throughput_sweep.py"), "--arm", "engram",
                            "--tag", "diag", "--rates", str(args.rate), "--repeats", "1"], capture_output=True, text=True)
    t_end = time.time()
    print(sweep.stdout[-600:])
    if sweep.returncode != 0:
        print(sweep.stderr[-600:])

    idle = {n: snapshots(n, stamp(t_idle), stamp(t_load)) for n in CLUSTER.nodes}
    load = {n: snapshots(n, stamp(t_load), stamp(t_end + 5)) for n in CLUSTER.nodes}
    res = {"rate": args.rate, "genesis_params": p, "idle": summarize(idle, p), "load": summarize(load, p),
           "snapshots_idle": sum(len(v) for v in idle.values()), "snapshots_load": sum(len(v) for v in load.values())}
    os.makedirs(RESULTS, exist_ok=True)
    path = os.path.join(RESULTS, f"diag_load_demotion_{args.rate:g}_{time.strftime('%Y%m%dT%H%M%S')}.json")
    with open(path, "w") as f:
        json.dump(res, f, indent=1)

    print(f"{'predicate violated':32s} {'idle':>8s} {'load':>8s}   (share of node snapshots)")
    for name in res["load"]:
        i, l = res["idle"].get(name, (None, 0))[0], res["load"][name][0]
        print(f"{name:32s} {('-' if i is None else f'{i:.2f}'):>8s} {('-' if l is None else f'{l:.2f}'):>8s}")
    print(f"snapshots: idle {res['snapshots_idle']}, load {res['snapshots_load']}; wrote {path}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
