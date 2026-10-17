#!/usr/bin/env python3
"""Table of the 100 tx/s comparison on Testbed B (four arms, three fresh deployments each) for the paper:
docs/paper/results/testbedb_load_table.tex and a JSON with every number and its source file.

    python3 scripts/e7_consensus_overhead/make_testbedb_load_table.py
"""
import glob
import json
import os
import statistics as st

HERE = os.path.dirname(os.path.abspath(__file__))
RES = os.path.join(HERE, "results_live")
OUT = os.path.join(HERE, "..", "..", "docs", "paper", "results")
T95 = {1: 12.706, 2: 4.303, 3: 3.182}
ARMS = [("vanilla", "vanilla7_b2_vanilla", "throughput_vanilla7_b2_vanilla_t{}_*_summary.json"),
        ("empty", "engram_b3_empty", "throughput_engram_b3_empty_t{}_*_summary.json"),
        ("sync", "engram_b3_sync", "throughput_engram_b3_sync_t{}_*_summary.json"),
        ("snapshot", "engram_b3_snapshot", "throughput_engram_b3_snapshot_t{}_*_summary.json")]
KEYS = [("block_interval_s", 3), ("latency_p50_s", 2), ("latency_p95_s", 2), ("latency_p99_s", 2), ("extend_vote_ms_mean", 2),
        ("rounds_per_height", 3), ("share_anchored", 3), ("validator_cpu_pct_max", 1), ("committed_over_offered", 3), ("mempool_max", 0)]


def cell(arm_glob, trial):
    f = sorted(glob.glob(os.path.join(RES, arm_glob.format(trial))))
    assert len(f) == 1, (arm_glob, trial, f)
    d = json.load(open(f[0]))
    c = d["cells"]["100.0"]
    return os.path.basename(f[0]), {k: (c.get(k) or {}).get("mean") for k, _ in KEYS}


def main():
    data, src = {}, {}
    for arm, _, g in ARMS:
        vals = [cell(g, t) for t in (1, 2, 3)]
        src[arm] = [v[0] for v in vals]
        data[arm] = {k: [v[1][k] for v in vals] for k, _ in KEYS}

    def fmt(arm, k, nd):
        xs = data[arm][k]
        if any(x is None for x in xs):
            return "--"
        m = st.mean(xs)
        h = T95[len(xs) - 1] * st.stdev(xs) / len(xs) ** 0.5
        return f"{m:.{nd}f} $\\pm$ {h:.{nd}f}"

    rows = [("Block interval (s)", "block_interval_s", 3), ("Latency p50 (s)", "latency_p50_s", 2), ("Latency p95 (s)", "latency_p95_s", 2),
            ("Latency p99 (s)", "latency_p99_s", 2), ("\\texttt{ExtendVote}, mean (ms)", "extend_vote_ms_mean", 2),
            ("Rounds per height", "rounds_per_height", 3), ("Share of samples in \\textsc{Anchored}", "share_anchored", 3),
            ("Busiest validator CPU (\\%)", "validator_cpu_pct_max", 1), ("Committed / offered", "committed_over_offered", 3)]
    lines = ["\\begin{tabular}{@{}lrrrr@{}}", "\\toprule", " & \\textbf{Vanilla} & \\textbf{Empty ext.} & \\textbf{Sync} & \\textbf{Snapshot} \\\\", "\\midrule"]
    for label, k, nd in rows:
        cells = []
        for arm, _, _ in ARMS:
            cells.append("n/a" if (arm == "vanilla" and k in ("extend_vote_ms_mean", "share_anchored")) else fmt(arm, k, nd))
        lines.append(f"{label} & " + " & ".join(cells) + " \\\\")
    p95 = {arm: [round(x, 2) for x in data[arm]["latency_p95_s"]] for arm, _, _ in ARMS}
    lines += ["\\bottomrule", "\\end{tabular}"]
    os.makedirs(OUT, exist_ok=True)
    open(os.path.join(OUT, "testbedb_load_table.tex"), "w").write("\n".join(lines) + "\n")
    json.dump(dict(data=data, p95_per_deployment=p95, sources=src), open(os.path.join(RES, "testbedb_load_numbers.json"), "w"), indent=2)
    print("\n".join(lines))
    print(json.dumps(p95))


if __name__ == "__main__":
    main()
