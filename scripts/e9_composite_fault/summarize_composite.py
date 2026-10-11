#!/usr/bin/env python3
"""Per-run metrics of the composite-fault runs, from the sample CSVs written by live_composite_fault.py.

    .venv/bin/python scripts/e9_composite_fault/summarize_composite.py <csv> [<csv> ...]

For each run: duration, heights, divergent heights (a height at which two validators report different AppHash values), RPC errors,
the time of the first SUSPICIOUS and first SOVEREIGN observation, the time RECOVERING began and how long it lasted (until every
validator reports ANCHORED), the fall-backs from RECOVERING to SOVEREIGN, and the final state. Times are seconds after the first sample.
With several runs, the mean and the 95% Student-t half-width of the SOVEREIGN time and of the RECOVERING duration follow.
"""
import csv
import statistics
import sys
from collections import defaultdict

T95 = {1: 12.706, 2: 4.303, 3: 3.182, 4: 2.776, 5: 2.571, 6: 2.447, 7: 2.365, 8: 2.306}  # two-sided, df = n - 1


def summarize(path):
    rows = list(csv.DictReader(open(path)))
    t0 = min(float(r["timestamp"]) for r in rows)
    by_height = defaultdict(set)
    by_node = defaultdict(list)
    errors = 0
    for r in rows:
        if r["error"]:
            errors += 1
            continue
        by_height[int(r["height"])].add(r["app_hash"])
        by_node[r["node"]].append((float(r["timestamp"]) - t0, r["fsm_state"], int(r["height"])))
    divergent = sum(1 for h in by_height.values() if len(h) > 1)
    first = {}
    fallbacks = 0
    for node, seq in by_node.items():
        seq.sort()
        prev = None
        for t, st, _ in seq:
            if st and st != prev:
                first.setdefault(st, t)
                if prev == "RECOVERING" and st == "SOVEREIGN":
                    fallbacks += 1
                prev = st
    # RECOVERING lasts until the first instant every validator's latest sample is ANCHORED after RECOVERING began
    times = sorted({t for seq in by_node.values() for t, _, _ in seq})
    latest = {n: None for n in by_node}
    cursor = {n: 0 for n in by_node}
    end = None
    for t in times:
        for n, seq in by_node.items():
            while cursor[n] < len(seq) and seq[cursor[n]][0] <= t:
                latest[n] = seq[cursor[n]][1]
                cursor[n] += 1
        if "RECOVERING" in first and t >= first["RECOVERING"] and all(v == "ANCHORED" for v in latest.values()):
            end = t
            break
    heights = [h for h in by_height]
    return {
        "run": path.split("/")[-1], "duration_s": max(times), "height_from": min(heights), "height_to": max(heights),
        "divergent_heights": divergent, "rpc_errors": errors,
        "suspicious_s": first.get("SUSPICIOUS"), "sovereign_s": first.get("SOVEREIGN"), "recovering_s": first.get("RECOVERING"),
        "recovering_duration_s": (end - first["RECOVERING"]) if end is not None and "RECOVERING" in first else None,
        "fallbacks": fallbacks, "final": sorted({seq[-1][1] for seq in by_node.values()}),
    }


def main():
    out = [summarize(p) for p in sys.argv[1:]]
    for r in out:
        print({k: (round(v, 1) if isinstance(v, float) else v) for k, v in r.items()})
    for key in ("sovereign_s", "recovering_s", "recovering_duration_s"):
        v = [r[key] for r in out if r[key] is not None]
        if len(v) >= 2:
            half = T95[len(v) - 1] * statistics.stdev(v) / len(v) ** 0.5
            print(f"{key}: n={len(v)} min {min(v):.0f} max {max(v):.0f} mean {statistics.mean(v):.1f} 95% CI +-{half:.1f}")


if __name__ == "__main__":
    main()
