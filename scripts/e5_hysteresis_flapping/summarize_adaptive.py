#!/usr/bin/env python3
"""Aggregate run_adaptive_sweep.py's *_metrics.json files: per configuration and environment, the flap count,
ANCHORED uptime and demotions per validator-trial (mean across the 7 validators of each trial, then mean +- 95% CI
across trials).

    python3 scripts/e5_hysteresis_flapping/summarize_adaptive.py
"""

import glob
import json
import os
import re
import statistics as st

RESULTS_DIR = os.path.join(os.path.dirname(__file__), "results_live")
FIELDS = ("flapping_count", "anchored_uptime", "demotion_count", "total_transitions")


def ci95(vals):
    if len(vals) < 2:
        return 0.0
    return 1.96 * st.stdev(vals) / len(vals) ** 0.5


def main():
    cells = {}
    for path in sorted(glob.glob(os.path.join(RESULTS_DIR, "live_spot_check_adaptive_*_metrics.json"))):
        d = json.load(open(path))
        m = re.match(r"adaptive_(\w+?)_t(\d+)_(\w+)$", d["label"])
        if not m:
            continue
        cfg, trial, env = m.group(1), int(m.group(2)), m.group(3)
        nodes = list(d["nodes"].values())
        cells.setdefault((cfg, env), {})[trial] = {f: st.mean(n[f] for n in nodes) for f in FIELDS}
    print("| config | env | trials | flaps / validator | ANCHORED uptime | demotions / validator |")
    print("|---|---|---:|---|---|---|")
    for (cfg, env), by_trial in sorted(cells.items()):
        row = [f"{st.mean(t[f] for t in by_trial.values()):.2f} ± {ci95([t[f] for t in by_trial.values()]):.2f}"
               for f in ("flapping_count", "anchored_uptime", "demotion_count")]
        print(f"| {cfg} | {env} | {len(by_trial)} | " + " | ".join(row) + " |")


if __name__ == "__main__":
    main()
