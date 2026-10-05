#!/usr/bin/env python3
"""Adds `safety_replay_ok` to collusion_suite_*.csv files written before the column existed.

The raw CSVs stay untouched; each gets a sibling `<name>_annotated.csv`. `safety_replay_ok` is true when the checks
safety_held, liveness and replay_clean hold (a check a case does not have counts as met). `pass` is kept as written:
it also requires the transition check, which fails here even in the no-adversary control c0 because of sensor noise.

    python3 scripts/e8_attack_resilience/annotate_suite_csv.py
"""

import csv
import glob
import json
import os

LIVE = os.path.join(os.path.dirname(os.path.abspath(__file__)), "results_live")


def main() -> None:
    for path in sorted(glob.glob(os.path.join(LIVE, "collusion_suite_*.csv"))):
        if path.endswith("_annotated.csv"):
            continue
        with open(path) as f:
            rows = list(csv.DictReader(f))
        if not rows or "safety_replay_ok" in rows[0]:
            continue
        for r in rows:
            checks = json.loads(r.get("checks") or "{}")
            r["safety_replay_ok"] = all(checks.get(k, True) for k in ("safety_held", "liveness", "replay_clean"))
        out = path[:-4] + "_annotated.csv"
        with open(out, "w", newline="") as f:
            w = csv.DictWriter(f, fieldnames=list(rows[0].keys()))
            w.writeheader()
            w.writerows(rows)
        bad = sum(1 for r in rows if not r["safety_replay_ok"])
        print(f"{os.path.basename(out)}: {len(rows)} rows, safety_replay_ok false in {bad}")


if __name__ == "__main__":
    main()
