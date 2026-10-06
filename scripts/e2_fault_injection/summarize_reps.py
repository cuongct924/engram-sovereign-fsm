#!/usr/bin/env python3
"""Per-scenario summary of the repeated S1-S7 runs (results_live/rep_N/s*.csv, one fresh chain per rep).

For each scenario and rep: states seen on any validator, whether SOVEREIGN was reached, seconds from the scenario's first
sample to the first SOVEREIGN, state at the last sample, and block rate. Over the reps: counts and mean +/- 95% Student-t.

    python3 scripts/e2_fault_injection/summarize_reps.py
"""

import csv
import glob
import math
import os
import statistics as st

HERE = os.path.dirname(os.path.abspath(__file__))
LIVE = os.path.join(HERE, "results_live")
SCENARIOS = ["s1_normal", "s2_btc_congestion", "s3_da_unavailable", "s4_p2p_eclipse_partial", "s5_anchor_isolation",
             "s6_combined_btc_da_failure", "s7_recovery"]
T = {1: 12.706, 2: 4.303, 3: 3.182, 4: 2.776, 5: 2.571}


def load(path):
    rows = [r for r in csv.DictReader(open(path)) if not r["error"]]
    for r in rows:
        r["t"] = float(r["timestamp"])
        r["h"] = int(r["height"])
    return sorted(rows, key=lambda r: r["t"])


def summarize(rows):
    t0 = rows[0]["t"]
    states = sorted({r["fsm_state"] for r in rows})
    sov = [r["t"] - t0 for r in rows if r["fsm_state"] == "SOVEREIGN"]
    last = rows[-1]
    # a validator can lag, so the last sample's majority state is used
    tail = [r["fsm_state"] for r in rows if r["t"] >= last["t"] - 6]
    final = max(set(tail), key=tail.count)
    dur = last["t"] - t0
    rate = (max(r["h"] for r in rows) - min(r["h"] for r in rows)) / dur if dur else float("nan")
    return {"states": states, "sov": bool(sov), "t_sov": min(sov) if sov else None, "final": final, "dur": dur, "rate": rate}


def ci(v):
    if len(v) < 2:
        return f"{v[0]:.1f}" if v else "--"
    return f"{st.mean(v):.1f} ± {T.get(len(v), 2.0) * st.stdev(v) / math.sqrt(len(v)):.1f}"


def main():
    reps = sorted(glob.glob(os.path.join(LIVE, "rep_*")))
    print(f"{len(reps)} reps: {[os.path.basename(r) for r in reps]}")
    print(f"{'scenario':28s} {'reps':>4s} {'reached SOV':>11s} {'t to SOV (s)':>16s} {'final states':>26s} {'blocks/s':>14s}")
    for sc in SCENARIOS:
        out = []
        for rep in reps:
            p = os.path.join(rep, sc + ".csv")
            if os.path.exists(p):
                out.append(summarize(load(p)))
        if not out:
            continue
        tsov = [o["t_sov"] for o in out if o["t_sov"] is not None]
        finals = {}
        for o in out:
            finals[o["final"]] = finals.get(o["final"], 0) + 1
        print(f"{sc:28s} {len(out):4d} {sum(o['sov'] for o in out):>6d}/{len(out):<4d} {ci(tsov):>16s} "
              f"{str(finals):>26s} {ci([o['rate'] for o in out]):>14s}")
        for i, o in enumerate(out, 1):
            print(f"    rep {i}: states {o['states']}, SOV at {o['t_sov'] if o['t_sov'] is None else round(o['t_sov'])}s, final {o['final']}")


if __name__ == "__main__":
    main()
