#!/usr/bin/env python3
"""E2 / item-15 -- N independent repeats of the real 7-phase scenario matrix
(`live_scenario_matrix.py`), reporting S7 recovery time as mean +- 95% confidence interval
across repeats -- the advisor's confidence-interval/effect-size ask, applied
to E2's headline recovery-time metric.

Recovery time = seconds from S7's first sample to its first ANCHORED sample
in `results_live/s7_recovery.csv`, per node (all validators stay in lockstep
in every measured run so far -- reported per-node values should agree).

Each repeat re-runs the FULL matrix (S1-S7 chained, ~20-30 min/run: `live_scenario_matrix.py`'s
own module doc explains why phases can't be run in isolation -- S7's "before" is
S1-S6's healed "after"), so N=5 costs ~2-2.5 hours of wall-clock. Pick N
accordingly for the time you actually have; this script reports whatever N
you ran, not a fixed 5.

Usage:
    python3 scripts/e2_fault_injection/ci_recovery_time.py --repeats 5
    (runs live_scenario_matrix.py --repeats times, archiving each run's
    s7_recovery.csv before the next repeat overwrites it)
"""

import os as _os, sys as _sys
_sys.path.insert(0, _os.path.join(_os.path.dirname(_os.path.abspath(__file__)), "..", "framework"))
from stats import ci95_halfwidth  # noqa: E402
import argparse
import csv
import os
import shutil
import subprocess
import sys
from datetime import datetime

SCRIPT_DIR = os.path.dirname(__file__)
RESULTS_DIR = os.path.join(SCRIPT_DIR, "results_live")
MATRIX_SCRIPT = os.path.join(SCRIPT_DIR, "live_scenario_matrix.py")
S7_CSV = os.path.join(RESULTS_DIR, "s7_recovery.csv")


def mean_stdev_ci95(values):
    n = len(values)
    if n == 0:
        return None, None, None
    m = sum(values) / n
    if n < 2:
        return m, 0.0, 0.0
    var = sum((v - m) ** 2 for v in values) / (n - 1)
    stdev = var ** 0.5
    return m, stdev, ci95_halfwidth(stdev, n)


def recovery_time_from_csv(path: str) -> float:
    """Seconds from the first sample to the first ANCHORED sample (max across
    nodes, since the run isn't "done" until every validator has recovered)."""
    rows = list(csv.DictReader(open(path)))
    if not rows:
        raise ValueError(f"{path} is empty")
    t0 = min(float(r["timestamp"]) for r in rows)
    by_node_anchored = {}
    for r in rows:
        if r["fsm_state"] == "ANCHORED":
            t = float(r["timestamp"])
            if r["node"] not in by_node_anchored or t < by_node_anchored[r["node"]]:
                by_node_anchored[r["node"]] = t
    if not by_node_anchored:
        raise ValueError(f"{path}: no node reached ANCHORED")
    return max(by_node_anchored.values()) - t0


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--repeats", type=int, default=5)
    args = parser.parse_args()

    os.makedirs(RESULTS_DIR, exist_ok=True)
    recovery_times = []
    for rep in range(1, args.repeats + 1):
        print(f"=== repeat {rep}/{args.repeats}: running full S1-S7 matrix ===", flush=True)
        subprocess.run([sys.executable, "-u", MATRIX_SCRIPT], check=True)
        rt = recovery_time_from_csv(S7_CSV)
        recovery_times.append(rt)
        print(f"=== repeat {rep}/{args.repeats}: recovery_time={rt:.1f}s ===", flush=True)
        ts_label = datetime.now().strftime("%Y%m%dT%H%M%S")
        shutil.copy(S7_CSV, os.path.join(RESULTS_DIR, f"s7_recovery_rep{rep}_{ts_label}.csv"))

    m, sd, ci = mean_stdev_ci95(recovery_times)
    print(f"\nrecovery time across {len(recovery_times)} repeat(s): mean={m:.1f}s +- {ci:.1f}s (stdev={sd:.1f}s)")
    print(f"values (s): {[f'{v:.1f}' for v in recovery_times]}")

    ts_label = datetime.now().strftime("%Y%m%dT%H%M%S")
    out_path = os.path.join(RESULTS_DIR, f"recovery_time_confidence_interval_{ts_label}.md")
    with open(out_path, "w") as f:
        f.write("# E2 / item-15: S7 recovery time (mean +- 95% confidence interval across repeats)\n\n")
        f.write(f"* Repeats: {len(recovery_times)}\n")
        f.write(f"* Mean +- 95% confidence interval: {m:.1f}s +- {ci:.1f}s (stdev={sd:.1f}s)\n")
        f.write(f"* Values (s): {', '.join(f'{v:.1f}' for v in recovery_times)}\n")
    print(f"wrote {out_path}")


if __name__ == "__main__":
    main()
