#!/usr/bin/env python3
"""docs/paper/results/f2_table.tex and b9_table.tex from the live f=2 collusion-suite CSVs.

Which file feeds which case (every run of a case starts from ANCHORED unless the case drives the chain to SOVEREIGN itself):
  suite rerun  collusion_suite_<stamp>.csv written by the rerun of a3 a4 b1 b2 b3a b3b b4 b6 b7a b7b  (latest file containing them)
  b5           first suite (20261004T204848): b5 starts from SOVEREIGN by design, so its run was not affected
  b8           20261004T211305
  c0           20261005T024611 (named b0 when it ran)
  b9           20261005T100618

    python3 scripts/e8_attack_resilience/make_f2_tables.py
"""

import csv
import glob
import json
import os
import statistics as st
from collections import defaultdict

HERE = os.path.dirname(os.path.abspath(__file__))
LIVE = os.path.join(HERE, "results_live")
OUT = os.path.join(HERE, "..", "..", "docs", "paper", "results")
RERUN_CASES = ["a3", "a4", "b1", "b2", "b3a", "b3b", "b4", "b6", "b7a", "b7b"]

DESCRIPTION = {
    "a3": ("1", "claims a DA attestation one height ahead"),
    "a4": ("1", "forged Bitcoin checkpoint hash"),
    "b1": ("2", "coordinated false degraded observations"),
    "b2": ("2", "false observations; proposer biases the certificate"),
    "b3a": ("1", "proposer keeps exactly a quorum of certificates"),
    "b3b": ("1", "proposer keeps fewer than a quorum"),
    "b4": ("2", "double-signing at the same height"),
    "b5": ("2", "withdrawal in a block of a locked state"),
    "b6": ("2", "stale DA receipt; forged Bitcoin hash"),
    "b7a": ("2", "false observations with two honest validators eclipsed"),
    "b7b": ("2", "false observations with three honest validators eclipsed"),
    "b8": ("0", "honest split view: 3 then 4 of 7 eclipsed"),
    "c0": ("0", "control: node04 restarted, no behaviour"),
}


def read(path):
    with open(path, newline="") as f:
        return list(csv.DictReader(f))


def pick_rows():
    rows = defaultdict(list)
    rerun_files = [f for f in sorted(glob.glob(os.path.join(LIVE, "collusion_suite_20261005T1[5-9]*.csv"))
                                     + glob.glob(os.path.join(LIVE, "collusion_suite_20261005T2*.csv")))
                   if {r["case"] for r in read(f)} & set(RERUN_CASES)]
    for f in rerun_files[-1:]:
        for r in read(f):
            if r["case"] in RERUN_CASES:
                rows[r["case"]].append(r)
    for r in read(os.path.join(LIVE, "collusion_suite_20261004T204848.csv")):
        if r["case"] == "b5":
            rows["b5"].append(r)
    for r in read(os.path.join(LIVE, "collusion_suite_20261004T211305.csv")):
        rows["b8"].append(r)
    for r in read(os.path.join(LIVE, "collusion_suite_20261005T024611.csv")):
        rows["c0"].append(r)
    return rows


def reasons(rs):
    tot = defaultdict(int)
    for r in rs:
        for k, v in json.loads(r["rejects"] or "{}").items():
            tot[k] += v
    return ", ".join(f"{k.replace('_', ' ')} {v}" for k, v in sorted(tot.items())) or "none"


def f2_table(rows):
    lines = [r"\begin{tabular}{@{}lrllrrr@{}}", r"\toprule",
             r"\textbf{Case} & \textbf{Byz.} & \textbf{Behaviour} & \textbf{Rejected by honest validators} & \textbf{Repeats} & "
             r"\textbf{Same AppHash} & \textbf{Replay clean} \\", r"\midrule"]
    for case in ["a3", "a4", "b1", "b2", "b3a", "b3b", "b4", "b5", "b6", "b7a", "b7b", "b8", "c0"]:
        rs = rows.get(case, [])
        if not rs:
            continue
        safe = sum(1 for r in rs if json.loads(r["checks"])["safety_held"])
        clean = sum(1 for r in rs if int(r["replay_violations"] or 0) == 0)
        byz, desc = DESCRIPTION[case]
        lines.append(f"{case} & {byz} & {desc} & {reasons(rs)} & {len(rs)} & {safe}/{len(rs)} & {clean}/{len(rs)} \\\\")
    lines += [r"\bottomrule", r"\end{tabular}"]
    return "\n".join(lines) + "\n"


def b9_table():
    rows = [r for r in read(os.path.join(LIVE, "collusion_suite_20261005T100618.csv")) if r["case"].startswith("b9_")]
    g = defaultdict(list)
    for r in rows:
        target, b = r["case"][3:].rsplit("_b", 1)
        g[(target, int(b))].append(r)
    lines = [r"\begin{tabular}{@{}llrrrr@{}}", r"\toprule",
             r"\textbf{Goal of the adversary} & \textbf{Byz.} & \textbf{Runs} & \textbf{Blocks outside honest range} & "
             r"\textbf{Same AppHash} & \textbf{Adversary's goal reached} \\", r"\midrule"]
    label = {"force_sovereign": r"force \textsc{Sovereign}", "hide_outage": "hide a DA outage"}
    for (target, b), rs in sorted(g.items(), key=lambda kv: (kv[0][0] != "force_sovereign", kv[0][1])):
        outs = [int(r["replay_outside_range"] or 0) for r in rs]
        safe = sum(1 for r in rs if json.loads(r["checks"])["safety_held"])
        reached = sum(1 for r in rs if "SOVEREIGN" in (r["transitions"] or ""))
        goal = reached if target == "force_sovereign" else len(rs) - reached
        rng = f"{min(outs)}" if min(outs) == max(outs) else f"{min(outs)}--{max(outs)}"
        lines.append(f"{label[target]} & {b} & {len(rs)} & {rng} & {safe}/{len(rs)} & {goal}/{len(rs)} \\\\")
    lines += [r"\bottomrule", r"\end{tabular}"]
    return "\n".join(lines) + "\n"


def main():
    rows = pick_rows()
    os.makedirs(OUT, exist_ok=True)
    with open(os.path.join(OUT, "f2_table.tex"), "w") as f:
        f.write(f2_table(rows))
    with open(os.path.join(OUT, "b9_table.tex"), "w") as f:
        f.write(b9_table())
    print(open(os.path.join(OUT, "f2_table.tex")).read())
    print(open(os.path.join(OUT, "b9_table.tex")).read())
    print({c: len(r) for c, r in rows.items()})


if __name__ == "__main__":
    main()
