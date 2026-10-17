#!/usr/bin/env python3
"""Table of ExtendVote time against injected Bitcoin RPC delay (live_rpc_latency.py output) for the paper:
docs/paper/results/testbedb_latency_table.tex and a JSON with the numbers.

The block interval is recomputed from the sample CSV: per tick the highest height of the seven validators, ignoring failed samples
(height <= 0), from SETTLE_S after the delay was applied to the end of the window. (The summary's own interval used the lowest
sampled height, which a failed sample can set to -1.) ExtendVote statistics and FSM state counts come from the summary.

    python3 scripts/e7_consensus_overhead/make_latency_table.py
"""
import collections
import csv
import glob
import json
import os

HERE = os.path.dirname(os.path.abspath(__file__))
RES = os.path.join(HERE, "results_live")
OUT = os.path.join(HERE, "..", "..", "docs", "paper", "results")
SETTLE_S = 15


def one(arm):
    f = sorted(glob.glob(os.path.join(RES, f"rpc_latency_{arm}_b4_*_summary.json")))[-1]
    s = json.load(open(f))
    rows = list(csv.DictReader(open(f.replace("_summary.json", ".csv"))))
    out = {}
    for w in s["windows"]:
        d = w["delay_ms"]
        ticks = collections.defaultdict(list)
        err = 0
        for r in rows:
            if int(r["delay_ms"]) != d:
                continue
            if int(r["height"]) <= 0:
                err += 1
                continue
            ticks[round(float(r["t"]), 0)].append(int(r["height"]))
        ts = sorted(ticks)
        pts = [(t, max(ticks[t])) for t in ts if t >= ts[0] + SETTLE_S]
        (ta, ha), (tb, hb) = pts[0], pts[-1]
        states = {k or "failed sample": v for k, v in w["fsm_state_samples"].items()}
        out[d] = dict(blocks=hb - ha, interval_s=round((tb - ta) / (hb - ha), 2), ev_mean=w["extend_vote_ms_mean"], ev_p95=w["extend_vote_ms_p95"],
                      ev_max=w["extend_vote_ms_max"], ev_n=w["extend_vote_samples"], states=states, failed_samples=err,
                      degraded=w["snapshot_degraded_lines"], snapshot_lines=w["snapshot_lines"])
    return dict(source=os.path.basename(f), windows=out)


def share(states):
    tot = sum(v for k, v in states.items() if k != "failed sample")
    return ", ".join(f"{k.capitalize()} {100 * v / tot:.0f}\\%" for k, v in sorted(states.items(), key=lambda kv: -kv[1]) if k != "failed sample")


def main():
    d = {a: one(a) for a in ("sync", "snapshot")}
    L = ["\\begin{tabular}{@{}llrrrl@{}}", "\\toprule",
         "\\textbf{Build} & \\textbf{Added delay (ms)} & \\textbf{Blocks} & \\textbf{Interval (s)} & \\texttt{ExtendVote} mean / p95 / max (ms) & \\textbf{States (share of samples)} \\\\", "\\midrule"]
    for arm, name in (("sync", "Inline reads"), ("snapshot", "Background reads")):
        for i, (dl, x) in enumerate(sorted(d[arm]["windows"].items())):
            L.append(f"{name if i == 0 else ''} & {dl} & {x['blocks']} & {x['interval_s']:.2f} & {x['ev_mean']:.2f} / {x['ev_p95']:.1f} / {x['ev_max']:.1f} & {share(x['states'])} \\\\")
        if arm == "sync":
            L.append("\\midrule")
    L += ["\\bottomrule", "\\end{tabular}"]
    os.makedirs(OUT, exist_ok=True)
    open(os.path.join(OUT, "testbedb_latency_table.tex"), "w").write("\n".join(L) + "\n")
    json.dump(d, open(os.path.join(RES, "testbedb_latency_numbers.json"), "w"), indent=2)
    print("\n".join(L))


if __name__ == "__main__":
    main()
