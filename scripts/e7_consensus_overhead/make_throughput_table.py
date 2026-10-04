#!/usr/bin/env python3
"""docs/paper/results/throughput_table.tex from the open-loop throughput CSVs: p95 latency and peak validator CPU per
arm and offered rate (mean and 95% Student-t interval over the repeats, recomputed from the per-repeat rows because
the *_summary.json intervals use the normal 1.96), plus the share of samples in ANCHORED where it was recorded.

    python3 scripts/e7_consensus_overhead/make_throughput_table.py
"""

import csv
import glob
import os
import statistics as st

HERE = os.path.dirname(os.path.abspath(__file__))
LIVE = os.path.join(HERE, "results_live")
OUT = os.path.join(HERE, "..", "..", "docs", "paper", "results", "throughput_table.tex")
T975 = {1: 12.706, 2: 4.303, 3: 3.182, 4: 2.776, 5: 2.571}
ARMS = [("engram", "throughput_engram_2026*.csv", "Engram"),
        ("latency", "throughput_engram_latency5000_2026*.csv", "Engram, peer-latency bound 5000\\,ms"),
        ("vanilla", "throughput_vanilla7_2026*.csv", "Vanilla")]


def load(pattern):
    # the plain Engram pattern also matches the tagged file; keep untagged ones only
    files = [f for f in sorted(glob.glob(os.path.join(LIVE, pattern))) if "latency5000" in pattern or "latency5000" not in f]
    rows = []
    for f in files[-1:]:
        rows += list(csv.DictReader(open(f)))
    return rows


def cell(vals, nd):
    m = st.mean(vals)
    ci = T975.get(len(vals) - 1, 1.96) * st.stdev(vals) / len(vals) ** 0.5 if len(vals) > 1 else 0
    return f"{m:.{nd}f} $\\pm$ {ci:.{nd}f}"


def main():
    data = {k: load(p) for k, p, _ in ARMS}
    rates = sorted({float(r["nominal_tx_s"]) for rows in data.values() for r in rows})
    lines = ["\\begin{tabular}{@{}rlrrrr@{}}", "\\toprule",
             "\\textbf{Offered (tx/s)} & \\textbf{Arm} & \\textbf{Committed (tx/s)} & \\textbf{p95 latency (s)} & "
             "\\textbf{Peak CPU (\\%)} & \\textbf{\\textsc{Anchored} share} \\\\", "\\midrule"]
    for rate in rates:
        for k, _, name in ARMS:
            rs = [r for r in data[k] if float(r["nominal_tx_s"]) == rate]
            if not rs:
                continue
            # Vanilla has no FSM (its queried state stays at the genesis value) and the first Engram arm predates the
            # recording, so the share is reported for the tagged Engram arm only.
            share = [float(r["share_anchored"]) for r in rs
                     if k == "latency" and r.get("share_anchored") not in ("", None, "None")]
            lines.append(f"{rate:g} & {name} & {cell([float(r['committed_tx_s']) for r in rs], 0)} & "
                         f"{cell([float(r['latency_p95_s']) for r in rs], 1)} & "
                         f"{cell([float(r['validator_cpu_pct_max']) for r in rs], 0)} & "
                         f"{cell(share, 2) if share else '--'} \\\\")
        lines.append("\\addlinespace")
    lines += ["\\bottomrule", "\\end{tabular}"]
    with open(OUT, "w") as f:
        f.write("\n".join(lines) + "\n")
    print("\n".join(lines))


if __name__ == "__main__":
    main()
