#!/usr/bin/env python3
"""Paired estimate of Engram's addition to the block interval.

Each pair is two fresh deployments of the same seven-validator cluster, one in engram mode and one in vanilla mode, run
back to back in a random order; the unit of independence is the pair. The manifest (results_live/paired_manifest.csv,
columns pair,order,arm,stamp) lists the rq4_<arm>_<stamp>.csv written by live_rq4_metrics.py for every deployment.

Stopping rule, fixed before the first run: 12 pairs; if the half-width of the 95% interval of the mean paired difference
is above 0.012 s, run to 20 pairs; stop there whatever the result. `--need-more` exits 0 when the rule asks for pairs
13-20, 1 otherwise.

    python3 scripts/e7_consensus_overhead/paired_overhead.py [--need-more]
"""

import csv
import math
import os
import random
import statistics as st
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
R = os.path.join(HERE, "results_live")
T975 = {1: 12.706, 2: 4.303, 3: 3.182, 4: 2.776, 5: 2.571, 6: 2.447, 7: 2.365, 8: 2.306, 9: 2.262, 10: 2.228, 11: 2.201,
        12: 2.179, 13: 2.160, 14: 2.145, 15: 2.131, 16: 2.120, 17: 2.110, 18: 2.101, 19: 2.093}
HALF_WIDTH_LIMIT = 0.012


def deployment_mean(arm, stamp):
    with open(os.path.join(R, f"rq4_{arm}_{stamp}.csv")) as f:
        return st.mean(float(r["interval_mean_s"]) for r in csv.DictReader(f))


def pairs():
    by = {}
    with open(os.path.join(R, "paired_manifest.csv")) as f:
        for r in csv.DictReader(f):
            by.setdefault(int(r["pair"]), {})[r["arm"]] = deployment_mean(r["arm"], r["stamp"])
    return [(p, a["engram"], a["vanilla7"]) for p, a in sorted(by.items()) if "engram" in a and "vanilla7" in a]


def summarize():
    ps = pairs()
    diffs = [e - v for _, e, v in ps]
    n = len(diffs)
    if n < 3:
        return None
    m, sd = st.mean(diffs), st.stdev(diffs)
    hw = T975[n - 1] * sd / math.sqrt(n)
    rng = random.Random(1)
    bs = sorted(st.mean(rng.choice(diffs) for _ in diffs) for _ in range(20000))
    base = st.mean(v for _, _, v in ps)
    return {"n": n, "mean": m, "sd": sd, "hw": hw, "boot": (bs[500], bs[19500]), "base": base, "diffs": diffs}


def main():
    s = summarize()
    if "--need-more" in sys.argv:
        sys.exit(0 if s and s["n"] < 20 and s["hw"] > HALF_WIDTH_LIMIT else 1)
    if not s:
        raise SystemExit("fewer than three complete pairs")
    lines = ["# Paired block-interval overhead (Engram minus vanilla, seven validators)", "",
             f"- Pairs: {s['n']}; vanilla interval {s['base']:.4f} s",
             f"- Paired differences (s): {', '.join(f'{d:.4f}' for d in s['diffs'])}",
             f"- Mean difference {s['mean']:.4f} s ({100 * s['mean'] / s['base']:.1f}%), sd {s['sd']:.4f} s",
             f"- 95% interval (Student t): {s['mean'] - s['hw']:.4f} to {s['mean'] + s['hw']:.4f} s "
             f"({100 * (s['mean'] - s['hw']) / s['base']:.1f}% to {100 * (s['mean'] + s['hw']) / s['base']:.1f}%)",
             f"- 95% interval (bootstrap over pairs): {s['boot'][0]:.4f} to {s['boot'][1]:.4f} s",
             f"- Half-width {s['hw']:.4f} s against the stopping limit {HALF_WIDTH_LIMIT} s"]
    with open(os.path.join(R, "paired_overhead.md"), "w") as f:
        f.write("\n".join(lines) + "\n")
    print("\n".join(lines))


if __name__ == "__main__":
    main()
