#!/usr/bin/env python3
"""docs/paper/results/adaptive_sim_table.tex from the in-process simulations (tests/e2e/results/*.csv):

  e5c_adaptive_nonstationary.csv   200 paired noise schedules: flaps in the noisy phase, detection delay cold and right
                                   after the noise (blocks);
  e5c_repeated_trigger.csv         demotions under a naive (fixed k0 bursts) and a patient adversary, 1200 blocks;
  e8_hide_outage_placement*.csv    placements (of 35) of three Byzantine proposers that hide a real DA outage.

Prints the numbers the text quotes next to the table.

    python3 scripts/e5_hysteresis_flapping/make_adaptive_sim_table.py
"""

import os as _os, sys as _sys
_sys.path.insert(0, _os.path.join(_os.path.dirname(_os.path.abspath(__file__)), "..", "framework"))
from stats import ci95_halfwidth  # noqa: E402
import csv
import os
import statistics as st

HERE = os.path.dirname(os.path.abspath(__file__))
RES = os.path.join(HERE, "..", "..", "tests", "e2e", "results")
OUT = os.path.join(HERE, "..", "..", "docs", "paper", "results", "adaptive_sim_table.tex")

ROWS = [("static2", r"static $k{=}2$"), ("static4", r"static $k{=}4$"),
        ("adaptive2-8_w64", r"adaptive $2\to8$, $T_{\text{stable}}{=}64$"),
        ("adaptive2-4_w64", r"adaptive $2\to4$, $T_{\text{stable}}{=}64$")]


def mean_ci(vals, nd=1):
    m = st.mean(vals)
    ci = ci95_halfwidth(st.stdev(vals), len(vals))
    return f"{m:.{nd}f} $\\pm$ {ci:.{nd}f}"


def load(name):
    with open(os.path.join(RES, name), newline="") as f:
        return list(csv.DictReader(f))


def main():
    ns, rt = load("e5c_adaptive_nonstationary.csv"), load("e5c_repeated_trigger.csv")
    pl, pla = load("e8_hide_outage_placement.csv"), load("e8_hide_outage_placement_adaptive.csv")

    hidden_static = {int(r["k"]): 0 for r in pl if r["b"] == "3"}
    for r in pl:
        if r["b"] == "3" and int(r["blocks_to_detection"]) < 0:
            hidden_static[int(r["k"])] += 1
    hidden_adaptive = {}  # effective k -> placements hidden, adaptive 2->8
    for r in pla:
        k = int(r["effective_k"])
        hidden_adaptive[k] = hidden_adaptive.get(k, 0) + (1 if int(r["blocks_to_detection"]) < 0 else 0)
    # quiet / elevated hiding for each row (the adaptive 2->4 threshold equals a static 4 once the level is >= 1)
    hidden = {"static2": ("0", hidden_static[2]), "static4": (hidden_static[4], hidden_static[4]),
              "adaptive2-8_w64": (hidden_adaptive[2], hidden_adaptive[8]),
              "adaptive2-4_w64": (hidden_adaptive[2], hidden_static[4])}

    lines = [r"\begin{tabular}{@{}lrrrrrr@{}}", r"\toprule",
             r" & \multicolumn{3}{c}{\textbf{Changing noise}} & \multicolumn{2}{c}{\textbf{Repeated trigger}} & \textbf{Outage hidden} \\",
             r"\cmidrule(lr){2-4}\cmidrule(lr){5-6}",
             r"\textbf{Threshold} & \textbf{Detect, quiet} & \textbf{Flaps} & \textbf{Detect after noise} & "
             r"\textbf{Naive} & \textbf{Patient} & \textbf{quiet / raised} \\", r"\midrule"]
    quoted = []
    for key, label in ROWS:
        rs = [r for r in ns if r["config"] == key]
        cold = [float(r["delay_cold"]) for r in rs]
        flaps = [float(r["noisy_flaps"]) for r in rs]
        after = [float(r["delay_after_0"]) for r in rs]
        naive = next(int(r["demotions"]) for r in rt if r["config"] == key and r["strategy"] == "fixed_k0")
        patient = next(int(r["demotions"]) for r in rt if r["config"] == key and r["strategy"] == "patient")
        tailored = next(r for r in rt if r["config"] == key and r["strategy"] == "tailored")
        q, e = hidden[key]
        lines.append(f"{label} & {mean_ci(cold)} & {mean_ci(flaps)} & {mean_ci(after)} & {naive} & {patient} & {q}\\,/\\,{e} \\\\")
        quoted.append((key, f"flaps {st.mean(flaps):.1f}", f"after96 {st.mean([float(r['delay_after_96']) for r in rs]):.1f}",
                       f"tailored bad/demotion {tailored['bad_per_demotion']}"))
    lines += [r"\bottomrule", r"\end{tabular}"]
    with open(OUT, "w") as f:
        f.write("\n".join(lines) + "\n")
    print("\n".join(lines))
    print()
    for q in quoted:
        print(*q)
    print("static hidden (b=3):", hidden_static, "adaptive 2->8 by effective k:", hidden_adaptive)


if __name__ == "__main__":
    main()
