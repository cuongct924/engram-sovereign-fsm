#!/usr/bin/env python3
"""Per-run and pooled metrics of the five composite runs on Testbed B (snapshot build), from the directory written by run_composite_x5.py.

    python3 scripts/e9_composite_fault/analyze_testbed_b.py [dir]

Times are seconds after the first sample of a run; a state's onset is the time at which the median (fourth of seven) validator first
reports it, as in summarize_composite.py. The final recovery is the last entry into RECOVERING (median validator) until every
validator reports ANCHORED. ExtendVote times are per region from the `extend_vote height=.. ms=..` lines of all validators.
"""
import csv
import glob
import json
import os
import statistics as st
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
T95 = {1: 12.706, 2: 4.303, 3: 3.182, 4: 2.776, 5: 2.571, 6: 2.447}
REGION = {**{f"engram-node0{i}": "us-east-2" for i in (1, 2)}, **{f"engram-node0{i}": "eu-west-1" for i in (3, 4)},
          **{f"engram-node0{i}": "ap-southeast-1" for i in (5, 6, 7)}}


def median_onset(per_node):
    ts = sorted(t for t in per_node if t is not None)
    return ts[3] if len(ts) >= 4 else None


def run_metrics(path):
    rows = list(csv.DictReader(open(path)))
    t0 = min(float(r["timestamp"]) for r in rows)
    seq, by_height, errors = {}, {}, 0
    for r in rows:
        if r["error"]:
            errors += 1
            continue
        seq.setdefault(r["node"], []).append((float(r["timestamp"]) - t0, r["fsm_state"], int(r["height"])))
        by_height.setdefault(int(r["height"]), set()).add(r["app_hash"])
    ev = {}
    for n, s in seq.items():
        s.sort()
        prev, out = None, []
        for t, state, _ in s:
            if state and state != prev:
                out.append((t, prev, state))
                prev = state
        ev[n] = out
    first = lambda state: median_onset([next((t for t, _, to in ev[n] if to == state), None) for n in ev])  # noqa: E731
    left_anchored = median_onset([next((t for t, fr, to in ev[n] if fr == "ANCHORED" and to != "ANCHORED"), None) for n in ev])
    last_rec = median_onset([max((t for t, _, to in ev[n] if to == "RECOVERING"), default=None) for n in ev])
    first_fall = median_onset([next((t for t, fr, to in ev[n] if fr == "RECOVERING" and to == "SOVEREIGN"), None) for n in ev])
    anchored_end = max((max(t for t, _, to in ev[n] if to == "ANCHORED") for n in ev))
    fallbacks = sum(1 for n in ev for _, fr, to in ev[n] if fr == "RECOVERING" and to == "SOVEREIGN")
    nodes_with_fall = sum(1 for n in ev if any(fr == "RECOVERING" and to == "SOVEREIGN" for _, fr, to in ev[n]))
    heights = [h for s in seq.values() for _, _, h in s]
    return dict(run=os.path.basename(path), samples=len(rows), duration_s=round(max(t for s in seq.values() for t, _, _ in s), 1),
                height_from=min(heights), height_to=max(heights), divergent_heights=sum(1 for v in by_height.values() if len(v) > 1),
                rpc_errors=errors, left_anchored_s=left_anchored, sovereign_first_s=first("SOVEREIGN"), recovering_first_s=first("RECOVERING"),
                fall_back_s=first_fall, recovering_last_s=last_rec, anchored_all_s=round(anchored_end, 1),
                final_recovery_s=round(anchored_end - last_rec, 1) if last_rec is not None else None,
                fallbacks=fallbacks, validators_that_fell_back=nodes_with_fall, suspicious_seen=any(to == "SUSPICIOUS" for n in ev for _, _, to in ev[n]),
                final=sorted({s[-1][1] for s in seq.values()}))


def pct(xs, p):
    xs = sorted(xs)
    return xs[min(len(xs) - 1, int(round(p * (len(xs) - 1))))]


def ci(xs):
    xs = [x for x in xs if x is not None]
    if len(xs) < 2:
        return None
    return round(T95[len(xs) - 1] * st.stdev(xs) / len(xs) ** 0.5, 1)


def main():
    d = sys.argv[1] if len(sys.argv) > 1 else os.path.join(HERE, "results_live", "testbed_b_snapshot")
    runs = [run_metrics(p) for p in sorted(glob.glob(os.path.join(d, "e9_composite_fault_*T??????.csv")))]
    out = dict(runs=runs, pooled={})
    for k in ("left_anchored_s", "sovereign_first_s", "recovering_first_s", "fall_back_s", "recovering_last_s", "final_recovery_s", "anchored_all_s"):
        xs = [r[k] for r in runs if r[k] is not None]
        if xs:
            out["pooled"][k] = dict(n=len(xs), min=round(min(xs), 1), max=round(max(xs), 1), mean=round(st.mean(xs), 1), ci95=ci(xs))
    ev = {}
    for i, _ in enumerate(runs, 1):
        p = os.path.join(d, f"run{i}_extend_vote_ms.csv")
        if not os.path.exists(p):
            continue
        for r in csv.DictReader(open(p)):
            ev.setdefault(REGION[r["node"]], []).append(float(r["ms"]))
    out["extend_vote_ms_by_region"] = {k: dict(n=len(v), median=round(st.median(v), 2), p95=round(pct(v, 0.95), 2), max=round(max(v), 2)) for k, v in ev.items()}
    allv = [x for v in ev.values() for x in v]
    if allv:
        out["extend_vote_ms_all"] = dict(n=len(allv), median=round(st.median(allv), 2), p95=round(pct(allv, 0.95), 2), max=round(max(allv), 2))
    print(json.dumps(out, indent=2))
    json.dump(out, open(os.path.join(d, "analysis.json"), "w"), indent=2)


if __name__ == "__main__":
    main()
