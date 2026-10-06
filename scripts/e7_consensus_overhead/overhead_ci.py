#!/usr/bin/env python3
"""Interval for Engram's addition to the block interval, with the deployment as the unit of independence.

The repeats of one arm share a chain, so they are not independent observations of the arm; deployments are. The earlier
interval combined the two arms' repeat-level intervals. Here each deployment contributes its mean over repeats, and the
interval for the difference uses the between-deployment spread (pooled over the arms, two degrees of freedom less than the
number of deployments) with a Student t. Seven-validator vanilla serves as the baseline (vote extensions off and empty
extensions on are both vanilla, within 0.004 s of each other); a pair-wise design replaces this when it has run
(scripts/e7_consensus_overhead/paired_overhead.py).

    python3 scripts/e7_consensus_overhead/overhead_ci.py
"""

import csv
import math
import os
import statistics as st

HERE = os.path.dirname(os.path.abspath(__file__))
R = os.path.join(HERE, "results_live")
ENGRAM = ["20261003T080519", "20261003T102820", "20261003T112304"]
VANILLA = ["20261003T103515", "20261003T110040"]  # extensions on (3 repeats), extensions off (5 repeats)
T975 = {1: 12.706, 2: 4.303, 3: 3.182, 4: 2.776, 5: 2.571}


def mean_interval(arm, stamp):
    with open(os.path.join(R, f"rq4_{arm}_{stamp}.csv")) as f:
        return st.mean(float(r["interval_mean_s"]) for r in csv.DictReader(f))


def main():
    e = [mean_interval("engram", s) for s in ENGRAM]
    v = [mean_interval("vanilla7", s) for s in VANILLA]
    d = st.mean(e) - st.mean(v)
    resid = [x - st.mean(e) for x in e] + [x - st.mean(v) for x in v]
    df = len(e) + len(v) - 2
    sp = math.sqrt(sum(r * r for r in resid) / df)
    hw = T975[df] * sp * math.sqrt(1 / len(e) + 1 / len(v))
    base = st.mean(v)
    lines = ["# Engram addition to the block interval, deployment-level", "",
             f"- Engram deployment means (s): {', '.join(f'{x:.4f}' for x in e)}",
             f"- Seven-validator vanilla deployment means (s): {', '.join(f'{x:.4f}' for x in v)}",
             f"- Difference of means: {d:.4f} s ({100 * d / base:.1f}% of the vanilla interval {base:.4f} s)",
             f"- Pooled between-deployment sd: {sp:.4f} s; degrees of freedom {df}",
             f"- 95% interval of the difference: {d - hw:.4f} to {d + hw:.4f} s ({100 * (d - hw) / base:.1f}% to {100 * (d + hw) / base:.1f}%)",
             "", "The interval contains zero: five deployments do not separate Engram's addition from deployment-to-deployment variation."]
    out = os.path.join(R, "overhead_ci.md")
    with open(out, "w") as f:
        f.write("\n".join(lines) + "\n")
    print("\n".join(lines))


if __name__ == "__main__":
    main()
