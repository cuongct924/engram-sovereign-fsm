#!/usr/bin/env python3
"""Share of samples in ANCHORED under load with the default and with relaxed DA time limits (the numbers of paper Section 7.4).

Reads the per-point CSVs of run_throughput_fresh.py written with --tag containing "da_default" or "da_relaxed" (one fresh
deployment per run; relaxed: --da-rpc-timeout-s 90 --da-check-timeout-ms 2500). Cells are the mean over runs with the range.
Latency and CPU are shown for orientation only: the runs are single-repeat fresh deployments and are not comparable with
the five-repeat arms of the load table.

    python3 scripts/e7_consensus_overhead/summarize_da_limits.py
"""

import csv
import glob
import os
import statistics as st
from collections import defaultdict

HERE = os.path.dirname(os.path.abspath(__file__))
LIVE = os.path.join(HERE, "results_live")
OUT = os.path.join(LIVE, "da_limits_summary.md")


def cell(v, nd):
    return f"{st.mean(v):.{nd}f} ({min(v):.{nd}f}--{max(v):.{nd}f})"


def main():
    lines = ["# ANCHORED share under load: default against relaxed DA time limits", "",
             "| DA limits | offered (tx/s) | runs | ANCHORED share | p95 latency (s) | peak validator CPU (%) | FSM states seen |", "|---|---|---|---|---|---|---|"]
    for tag, name in (("da_default", "default"), ("da_relaxed", "relaxed")):
        by = defaultdict(list)
        for f in sorted(glob.glob(os.path.join(LIVE, f"throughput_engram_engram_{tag}_r*_2026*.csv"))):
            for r in csv.DictReader(open(f)):
                by[float(r["nominal_tx_s"])].append(r)
        for rate in sorted(by):
            rs = by[rate]
            share = [float(r["share_anchored"]) for r in rs if r["share_anchored"] not in ("", "None")]
            states = sorted(set("|".join(r["fsm_states_seen"] for r in rs).split("|")))
            lines.append(f"| {name} | {rate:g} | {len(rs)} | {cell(share, 2)} | {cell([float(r['latency_p95_s']) for r in rs], 1)} | "
                         f"{cell([float(r['validator_cpu_pct_max']) for r in rs], 0)} | {', '.join(states)} |")
    with open(OUT, "w") as f:
        f.write("\n".join(lines) + "\n")
    print("\n".join(lines))


if __name__ == "__main__":
    main()
