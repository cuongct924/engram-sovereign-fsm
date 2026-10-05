#!/usr/bin/env python3
"""Gate for the throughput ceiling sweep: did relaxing the DA time limits keep the FSM in ANCHORED under load?

Reads the per-point CSVs of run_throughput_fresh.py written with --tag containing "da_relaxed" and exits 0 when the mean
share of samples in ANCHORED is >= 0.95 at every offered rate, 1 otherwise. A failure means the bottleneck is elsewhere
(see probe_da_cpu_under_load.py), so the sweep to the ceiling is not worth its hours and RQ4 stays at the cost of the
extension under light load.

    python3 scripts/e7_consensus_overhead/decide_ceiling.py
"""

import csv
import glob
import os
import statistics as st
import sys
from collections import defaultdict

LIVE = os.path.join(os.path.dirname(os.path.abspath(__file__)), "results_live")
THRESHOLD = 0.95


def main() -> int:
    by_rate = defaultdict(list)
    for f in glob.glob(os.path.join(LIVE, "throughput_engram_*da_relaxed*_2026*.csv")):
        for r in csv.DictReader(open(f)):
            if r.get("share_anchored") not in ("", None, "None"):
                by_rate[float(r["nominal_tx_s"])].append(float(r["share_anchored"]))
    if not by_rate:
        print("no da_relaxed runs found")
        return 1
    ok = True
    for rate, v in sorted(by_rate.items()):
        m = st.mean(v)
        print(f"rate {rate:g}: share ANCHORED mean {m:.3f} over {len(v)} runs (min {min(v):.3f})")
        ok = ok and m >= THRESHOLD
    print("PASS" if ok else "FAIL")
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())
