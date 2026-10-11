#!/usr/bin/env python3
"""The healthy btc_gap is not flat: it follows the checkpoint cycle and touches SuspiciousThreshold (5).

Reads, from runs that already exist, (1) the validators' own sensor_snapshot readings of btc_gap in the first 18 s of each
composite run (before any fault is injected) and (2) the ANCHORED share of the quiet phase of the non-stationary runs
(no injected noise), per down threshold. If the cycle makes the healthy gap reach 5 for a few Engram blocks, k=2 should
lose a slice of the quiet phase and k=4 should not.

    python3 scripts/e5_hysteresis_flapping/baseline_gap_cycle.py
"""

import collections
import csv
import glob
import json
import os
import statistics as st

HERE = os.path.dirname(os.path.abspath(__file__))
E9 = os.path.join(HERE, "..", "e9_composite_fault", "results_live")
OUT = os.path.join(HERE, "results_live", "baseline_gap_cycle.md")
FIRST_NEW_RUN = "20261005T230700"  # the composite runs of this paper (fresh chain, healthy start)


def main():
    lines = ["# Healthy btc_gap and the quiet phase", "",
             "## btc_gap in the first 18 s of the composite runs (no fault yet), all validators", "",
             "| run | samples | share of samples at each gap | share at gap >= 5 |", "|---|---|---|---|"]
    for f in sorted(glob.glob(os.path.join(E9, "e9_composite_fault_2026100[56]T*_sensors.csv"))):
        stamp = os.path.basename(f).split("trace_")[1][:15]
        if stamp < FIRST_NEW_RUN:
            continue
        rows = list(csv.DictReader(open(f)))
        t0 = min(float(x["approx_timestamp"]) for x in rows)
        gaps = [int(x["btc_gap"]) for x in rows if float(x["approx_timestamp"]) - t0 < 18]
        c = collections.Counter(gaps)
        n = len(gaps)
        lines.append(f"| {stamp} | {n} | " + ", ".join(f"{g}: {c[g] / n:.2f}" for g in sorted(c)) + f" | {sum(v for g, v in c.items() if g >= 5) / n:.2f} |")
    lines += ["", "## ANCHORED share of the quiet phase (150 s, no injected noise) of the non-stationary live runs", "",
              "| configuration | runs | shares |", "|---|---|---|"]
    by = collections.defaultdict(list)
    for f in sorted(glob.glob(os.path.join(HERE, "results_live", "nonstationary_*_t[0-9]_2026*.json"))):
        d = json.load(open(f))
        by[d["label"].rsplit("_o", 1)[0]].append(round(d["quiet1_anchored_share"], 2))
    for k, v in sorted(by.items()):
        lines.append(f"| {k} | {len(v)} | {v} (mean {st.mean(v):.2f}) |")
    os.makedirs(os.path.dirname(OUT), exist_ok=True)
    with open(OUT, "w") as f:
        f.write("\n".join(lines) + "\n")
    print("\n".join(lines))


if __name__ == "__main__":
    main()
