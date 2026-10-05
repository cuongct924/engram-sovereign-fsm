#!/usr/bin/env python3
"""Summarise tests/e2e/results/e5c_adaptive_nonstationary.csv (TestE5c_AdaptiveNonStationary).

Per configuration, over the seeds (the SAME noise schedule for every configuration, so differences are paired):
  noisy phase   flaps, demotions, share of blocks in ANCHORED;
  responsiveness  blocks until a sustained DA outage is detected, cold and after the noise has ended and the chain
                  has been healthy in ANCHORED for 0 / 16 / 48 / 96 / 200 blocks.
Means with 95% intervals (normal approximation: hundreds of seeds). -1 = not detected within the cap,
-2 = ANCHORED not regained; both are counted and left out of the delay means.

    python3 scripts/e5_hysteresis_flapping/summarize_nonstationary.py
"""

import csv
import os
import statistics as st
import sys
from collections import defaultdict

HERE = os.path.dirname(os.path.abspath(__file__))
SRC = os.path.join(HERE, "..", "..", "tests", "e2e", "results", "e5c_adaptive_nonstationary.csv")
OFFSETS = (0, 16, 48, 96, 200)


def ms(vals, nd=2):
    if not vals:
        return "--"
    m = st.mean(vals)
    ci = 1.96 * st.stdev(vals) / len(vals) ** 0.5 if len(vals) > 1 else 0.0
    return f"{m:.{nd}f} ± {ci:.{nd}f}"


def main() -> int:
    rows = defaultdict(list)
    with open(SRC) as f:
        for r in csv.DictReader(f):
            rows[r["config"]].append(r)
    out = ["| config | seeds | flaps (noisy) | demotions | ANCHORED share | delay cold | " +
           " | ".join(f"delay +{o}" for o in OFFSETS) + " | missed |", "|" + "---|" * (7 + len(OFFSETS))]
    for cfg, rs in rows.items():
        cells = []
        missed = 0
        for key in ["delay_cold"] + [f"delay_after_{o}" for o in OFFSETS]:
            vals = [float(r[key]) for r in rs]
            ok = [v for v in vals if v >= 0]
            missed += len(vals) - len(ok)
            cells.append(ms(ok, 1))
        out.append(f"| {cfg} | {len(rs)} | {ms([float(r['noisy_flaps']) for r in rs], 1)} | "
                   f"{ms([float(r['noisy_demotions']) for r in rs], 1)} | "
                   f"{ms([float(r['noisy_anchored_share']) for r in rs], 3)} | " + " | ".join(cells) + f" | {missed} |")
    print("\n".join(out))
    return 0


if __name__ == "__main__":
    sys.exit(main())
