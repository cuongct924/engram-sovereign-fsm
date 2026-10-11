#!/usr/bin/env python3
"""Live measurement of the unproven tail while the chain stays in SOVEREIGN (Testbed A, seven validators).

Holds the chain in SOVEREIGN for --hold-min minutes by pausing checkpoint submission on every validator, releases it, waits for
ANCHORED, and keeps what is needed to compute the tail: a sample of every validator every --interval-s seconds (height, FSM
state, whether the recovery proof is valid) and the prover container's log, whose PROOFLOG lines give one row per submitted proof
(see scripts/reanchoring_prover/prove_and_submit.sh). The prover mode (CONTINUOUS, MIN_BATCH) is the container's environment, set
when the cluster is deployed. Needs the cluster up and ANCHORED in Engram mode with proof-only recovery. Runs on the local Docker testnet or, with ENGRAM_CLUSTER=ssh, on the
hosts of deploy/ (see deploy/README.md).

    python3 -u scripts/e6_zk_reanchoring_benchmark/live_prover_tail.py --arm continuous --label cont_t1 --hold-min 25

Outputs in scripts/e6_zk_reanchoring_benchmark/results_live/: <label>_samples.csv, <label>_prover.log, <label>_meta.json.
"""
import argparse
import csv
import json
import os
import sys
import time

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(HERE, "..", "framework"))
from cluster import CLUSTER  # noqa: E402
from logger import sample_all_nodes  # noqa: E402

OUT = os.path.join(HERE, "results_live")


def poll(duration_s, interval_s, rows, phase, until=None):
    end = time.time() + duration_s
    while time.time() < end:
        s = sample_all_nodes()
        for x in s:
            rows.append((x.timestamp, phase, x.node, x.height, x.fsm_state, x.safe_blocks, x.reanchoring_proof_valid))
        states = {x.fsm_state for x in s}
        if until and states == {until}:
            return True
        time.sleep(interval_s)
    return False


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--arm", required=True, choices=["trigger", "continuous"])
    ap.add_argument("--label", required=True)
    ap.add_argument("--hold-min", type=float, default=25.0)
    ap.add_argument("--interval-s", type=float, default=5.0)
    ap.add_argument("--sovereign-timeout-s", type=float, default=600.0)
    ap.add_argument("--recovery-s", type=float, default=900.0)
    a = ap.parse_args()
    os.makedirs(OUT, exist_ok=True)
    rows = []
    t_start = time.time()
    CLUSTER.anchor_pause(True)
    t_pause = time.time()
    reached = poll(a.sovereign_timeout_s, a.interval_s, rows, "to_sovereign", until="SOVEREIGN")
    t_sov = time.time()
    poll(a.hold_min * 60, a.interval_s, rows, "hold")
    CLUSTER.anchor_pause(False)
    t_release = time.time()
    recovered = poll(a.recovery_s, a.interval_s, rows, "recovery", until="ANCHORED")
    t_end = time.time()
    with open(os.path.join(OUT, f"{a.label}_prover.log"), "w") as f:
        f.write(CLUSTER.prover_logs(int(t_start) - 5))
    with open(os.path.join(OUT, f"{a.label}_samples.csv"), "w", newline="") as f:
        w = csv.writer(f)
        w.writerow(["timestamp", "phase", "node", "height", "fsm_state", "safe_blocks", "proof_valid"])
        w.writerows(rows)
    meta = {"label": a.label, "arm": a.arm, "hold_min": a.hold_min, "t_start": t_start, "t_pause": t_pause, "t_sovereign": t_sov,
            "reached_sovereign": reached, "t_release": t_release, "recovered_to_anchored": recovered, "t_end": t_end,
            "recovery_s": t_end - t_release if recovered else None}
    with open(os.path.join(OUT, f"{a.label}_meta.json"), "w") as f:
        json.dump(meta, f, indent=1)
    print(json.dumps(meta))


if __name__ == "__main__":
    main()
