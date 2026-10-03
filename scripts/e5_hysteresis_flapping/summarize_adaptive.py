#!/usr/bin/env python3
"""Aggregate run_adaptive_sweep.py's results per configuration: flaps, ANCHORED uptime and demotions per validator
(mean over the 7 validators of a trial, then mean +- 95% CI over trials) for the stable and noisy_da environments,
and the detection / escalation delay of the detect environment.

    python3 scripts/e5_hysteresis_flapping/summarize_adaptive.py
"""

import glob
import json
import os
import re
import statistics as st

RESULTS_DIR = os.path.join(os.path.dirname(__file__), "results_live")
FIELDS = ("flapping_count", "anchored_uptime", "demotion_count")
LABEL = re.compile(r"adaptive_(\w+?)_t(\d+)_(\w+)$")


def ci95(vals):
    return 1.96 * st.stdev(vals) / len(vals) ** 0.5 if len(vals) > 1 else 0.0


def fmt(vals, nd=2):
    return f"{st.mean(vals):.{nd}f} ± {ci95(vals):.{nd}f}" if vals else "—"


def main():
    cells, detect = {}, {}
    for path in sorted(glob.glob(os.path.join(RESULTS_DIR, "live_spot_check_adaptive_*_metrics.json"))):
        d = json.load(open(path))
        m = LABEL.match(d["label"])
        if not m:
            continue
        nodes = list(d["nodes"].values())
        cells.setdefault((m.group(1), m.group(3)), {})[int(m.group(2))] = {f: st.mean(n[f] for n in nodes) for f in FIELDS}
    for path in sorted(glob.glob(os.path.join(RESULTS_DIR, "detection_adaptive_*.json"))):
        d = json.load(open(path))
        m = LABEL.match(d["label"])
        if m and d["detection_delay_s"] is not None:
            detect.setdefault(m.group(1), []).append(d)
    order = {"static2": 0, "static4": 1, "static6": 2, "static8": 3, "adaptive": 4}
    print("| config | env | trials | flaps / validator | ANCHORED uptime | demotions / validator |")
    print("|---|---|---:|---|---|---|")
    for (cfg, env), by_trial in sorted(cells.items(), key=lambda kv: (kv[0][1], order.get(kv[0][0], 9))):
        vals = list(by_trial.values())
        print(f"| {cfg} | {env} | {len(vals)} | " + " | ".join(fmt([v[f] for v in vals]) for f in FIELDS) + " |")
    print()
    print("| config | runs | detection delay (s) | blocks to detection | time to SOVEREIGN (s) |")
    print("|---|---:|---|---|---|")
    for cfg, runs in sorted(detect.items(), key=lambda kv: order.get(kv[0], 9)):
        print(f"| {cfg} | {len(runs)} | {fmt([r['detection_delay_s'] for r in runs], 1)} | "
              f"{fmt([r['blocks_to_detection'] for r in runs if r['blocks_to_detection'] is not None], 1)} | "
              f"{fmt([r['time_to_sovereign_s'] for r in runs if r['time_to_sovereign_s'] is not None], 1)} |")


if __name__ == "__main__":
    main()
