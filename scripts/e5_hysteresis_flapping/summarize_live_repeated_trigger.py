#!/usr/bin/env python3
"""Summarise the live repeated-trigger runs of live_repeated_trigger.py (results_live/repeated_trigger_<config>_o<offset>_t<trial>_*.json).

The adversary stops every Celestia bridge for a nominal pulse and restarts them; stopping and restarting take time, so the
bridges are down for far longer than the nominal pulse (column "bridge down"). One row per configuration, three runs each;
cells give the range over the runs.

    python3 scripts/e5_hysteresis_flapping/summarize_live_repeated_trigger.py
"""

import glob
import json
import os
import re
import statistics as st
from collections import defaultdict

HERE = os.path.dirname(os.path.abspath(__file__))
SRC = os.path.join(HERE, "results_live")
OUT = os.path.join(SRC, "repeated_trigger_summary.md")
ORDER = ["static2", "static4", "adaptiveW16", "adaptiveW64", "adaptiveK4W64"]


def rng(vals, nd=1):
    lo, hi = min(vals), max(vals)
    return f"{lo:.{nd}f}" if lo == hi else f"{lo:.{nd}f}--{hi:.{nd}f}"


def main():
    runs = defaultdict(list)
    for f in sorted(glob.glob(os.path.join(SRC, "repeated_trigger_*_t*_*.json"))):
        d = json.load(open(f))
        m = re.match(r"(.+)_o\d+_t\d+", d["label"]) or re.match(r"(.+)_t\d+", d["label"])
        runs[m.group(1)].append(d)
    lines = ["# Live repeated-trigger runs (fresh genesis per run; range over runs)", "",
             "| config | runs | pulses sent | bridge down per pulse (s) | demotions | flaps | ANCHORED share (%) | reached SOVEREIGN |",
             "|---|---|---|---|---|---|---|---|"]
    for cfg in ORDER:
        rs = runs.get(cfg, [])
        if not rs:
            continue
        lines.append(f"| {cfg} | {len(rs)} | {rng([r['pulses'] for r in rs], 0)} | {rng([r['bridge_down_s_mean'] for r in rs])} | "
                     f"{rng([r['demotions'] for r in rs], 0)} | {rng([r['flaps'] for r in rs], 0)} | "
                     f"{rng([100 * r['anchored_share'] for r in rs])} | {sum(bool(r['reached_sovereign']) for r in rs)}/{len(rs)} |")
    allr = [r for v in runs.values() for r in v]
    lines += ["", f"All {len(allr)} runs: nominal pulse {allr[0]['pulse_s_nominal']} s; "
              f"bridge down per pulse {rng([r['bridge_down_s_mean'] for r in allr])} s on average; "
              f"SOVEREIGN reached in {sum(bool(r['reached_sovereign']) for r in allr)}/{len(allr)}; "
              f"demotions per run {rng([r['demotions'] for r in allr], 0)}."]
    with open(OUT, "w") as f:
        f.write("\n".join(lines) + "\n")
    print("\n".join(lines))


if __name__ == "__main__":
    main()
