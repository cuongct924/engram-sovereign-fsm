#!/usr/bin/env python3
"""Unproven-tail statistics of the live prover runs written by live_prover_tail.py.

The tail at time t is the tip height minus the last height covered by an accepted proof, the quantity the recovery gate bounds by
MaxUnprovenTailBlocks (x/sovereignty/preblock.go, refreshReanchoringProofValid). A proof is taken to land at its start time plus
its total duration (PROOFLOG fields 1 and 13); --shift moves every landing by that many seconds to bound that approximation.
Only samples while the chain was held in SOVEREIGN are used, and only after the first proof had landed.

    .venv/bin/python scripts/e6_zk_reanchoring_benchmark/prover_tail_analysis.py [--shift S] [--bounds 8,32,64,128,256]
"""
import argparse
import csv
import json
import os

import numpy as np

HERE = os.path.dirname(os.path.abspath(__file__))
RES = os.path.join(HERE, "results_live")
ARMS = ("continuous", "trigger")
RUNS = (1, 2, 3)


def run_tails(label, shift):
    meta = json.load(open(os.path.join(RES, f"{label}_meta.json")))
    proofs = []
    for line in open(os.path.join(RES, f"{label}_prover.log")):
        if line.startswith("PROOFLOG"):
            f = line.strip().split(",")
            proofs.append((int(f[1]) + int(f[13]) + shift, int(f[4])))
    proofs.sort()
    rows = [r for r in csv.DictReader(open(os.path.join(RES, f"{label}_samples.csv"))) if r["node"] == "engram-node01"]
    tails = []
    for r in rows:
        t, h = float(r["timestamp"]), int(r["height"])
        if not meta["t_sovereign"] <= t <= meta["t_release"]:
            continue
        covered = [last for landed, last in proofs if landed <= t]
        if covered:
            tails.append(h - max(covered))
    return np.array(tails), meta, proofs


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--shift", type=int, default=0)
    ap.add_argument("--bounds", default="8,32,64,128,256,512")
    a = ap.parse_args()
    bounds = [int(b) for b in a.bounds.split(",")]
    for arm in ARMS:
        pooled, recov = [], []
        for t in RUNS:
            tails, meta, proofs = run_tails(f"{arm}_t{t}", a.shift)
            pooled.extend(tails)
            recov.append(meta["recovery_s"])
        p = np.array(pooled)
        print(f"{arm}: runs={len(RUNS)} samples={len(p)} p50={np.percentile(p, 50):.0f} p95={np.percentile(p, 95):.0f} "
              f"max={p.max():.0f} min={p.min():.0f} recovery_s={[round(x) for x in recov]}")
        print("  bound -> share of time the gate is open")
        for b in bounds:
            print(f"  {b:>5} -> {100 * (p <= b).mean():5.1f}%")


if __name__ == "__main__":
    main()
