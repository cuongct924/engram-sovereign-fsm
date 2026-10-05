#!/usr/bin/env python3
"""Is the Celestia side CPU-limited while the chain carries load?

The DA publisher's blob.Submit / blob.GetAll calls time out under load (diagnose_load_demotion.py). The containers have
CPU limits (celestia-app 1.0 CPU for all eight bridges, each bridge 0.5, each validator 1.0, on a 10-core host), and the
cgroup counts how often a container hit its limit. This reads those counters before and after a load run and samples
the CPU share of every container during it:

  throttled share = nr_throttled / nr_periods over the run (how often the container was stopped by its limit)
  throttled_ms    = time the container's threads were stopped by the limit

A container that is throttled in a large share of the periods is a bottleneck; one that is never throttled is not.
Changing the limits changes the testbed, so decide from this table, not from a guess.

    python3 scripts/e7_consensus_overhead/probe_da_cpu_under_load.py [--rate 200]
"""

import argparse
import json
import os
import subprocess
import sys
import threading
import time

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(HERE, "..", "framework"))
from cluster import CLUSTER  # noqa: E402

RESULTS = os.path.join(HERE, "results_live")


def containers():
    names = ["celestia-app"] + [CLUSTER.celestia_bridge(n) if hasattr(CLUSTER, "celestia_bridge") else None for n in CLUSTER.nodes]
    return ["celestia-app", "celestia-bridge-prover"] + [f"celestia-bridge-node{i:02d}" for i in range(1, len(CLUSTER.nodes) + 1)] + list(CLUSTER.nodes)


def cpu_stat(name):
    out = subprocess.run(["docker", "exec", name, "sh", "-c", "cat /sys/fs/cgroup/cpu.stat"], capture_output=True, text=True, timeout=30).stdout
    return {k: int(v) for k, v in (line.split() for line in out.splitlines() if line.strip())}


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--rate", type=float, default=200)
    args = ap.parse_args()
    names = containers()
    before = {n: cpu_stat(n) for n in names}

    samples, stop = {n: [] for n in names}, threading.Event()

    def sampler():
        while not stop.is_set():
            out = subprocess.run(["docker", "stats", "--no-stream", "--format", "{{.Name}} {{.CPUPerc}}"], capture_output=True, text=True, timeout=60).stdout
            for line in out.splitlines():
                name, pct = line.split()
                if name in samples:
                    samples[name].append(float(pct.rstrip("%")))

    th = threading.Thread(target=sampler, daemon=True)
    th.start()
    run = subprocess.run([sys.executable, "-u", os.path.join(HERE, "live_throughput_sweep.py"), "--arm", "engram", "--tag", "cpuprobe",
                          "--rates", str(args.rate), "--repeats", "1"], capture_output=True, text=True)
    stop.set()
    th.join(timeout=70)
    print(run.stdout[-400:])
    after = {n: cpu_stat(n) for n in names}

    rows = {}
    for n in names:
        periods = after[n]["nr_periods"] - before[n]["nr_periods"]
        thr = after[n]["nr_throttled"] - before[n]["nr_throttled"]
        rows[n] = {"periods": periods, "throttled": thr, "throttled_share": round(thr / periods, 4) if periods else None,
                   "throttled_ms": round((after[n]["throttled_usec"] - before[n]["throttled_usec"]) / 1000, 1),
                   "cpu_pct_max": max(samples[n], default=None), "cpu_pct_mean": round(sum(samples[n]) / len(samples[n]), 1) if samples[n] else None}
    path = os.path.join(RESULTS, f"da_cpu_probe_{args.rate:g}_{time.strftime('%Y%m%dT%H%M%S')}.json")
    json.dump(rows, open(path, "w"), indent=1)
    print(f"{'container':26s} {'throttled share':>16s} {'throttled ms':>13s} {'cpu% mean':>10s} {'cpu% max':>9s}")
    for n, r in rows.items():
        print(f"{n:26s} {str(r['throttled_share']):>16s} {r['throttled_ms']:>13} {str(r['cpu_pct_mean']):>10s} {str(r['cpu_pct_max']):>9s}")
    print("wrote", path)


if __name__ == "__main__":
    main()
