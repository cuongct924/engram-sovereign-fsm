#!/usr/bin/env python3
"""Table of the hung-Bitcoin-RPC test (docker pause bitcoin-node01, 240 s) for the paper, sync and snapshot arms:
docs/paper/results/testbedb_hang_table.tex and, next to the raw runs, a JSON with the numbers. Block intervals are recomputed from the raw sample CSV
(the first snapshot run's summary counted the baseline from height 0; the script was fixed afterwards).

    python3 scripts/e7_consensus_overhead/make_hang_table.py
"""
import csv
import glob
import json
import os

HERE = os.path.dirname(os.path.abspath(__file__))
RES = os.path.join(HERE, "results_live")
OUT = os.path.join(HERE, "..", "..", "docs", "paper", "results")


def interval(rows, phase):
    r = [x for x in rows if x["phase"] == phase and x["height"] not in ("", "0")]
    h = [int(x["height"]) for x in r]
    t = [float(x["t"]) for x in r]
    return (max(t) - min(t)) / (max(h) - min(h)), max(h) - min(h)


def one(arm):
    f = sorted(glob.glob(os.path.join(RES, f"rpc_hang_{arm}_b3_*_summary.json")))[-1]
    s = json.load(open(f))
    rows = list(csv.DictReader(open(f.replace("_summary.json", ".csv"))))
    out = dict(source=os.path.basename(f), recovered_after_s=s["recovered_after_s"], stale_check=s["stale_check"])
    for ph in ("baseline", "hang", "recovery"):
        bi, blocks = interval(rows, ph)
        p = s["phases"][ph]
        out[ph] = dict(block_interval_s=round(bi, 2), blocks=blocks, ev_mean=p["extend_vote_ms_mean"], ev_p95=p["extend_vote_ms_p95"],
                       ev_max=p["extend_vote_ms_max"], states=p["fsm_state_samples"])
    return out


def main():
    d = {arm: one(arm) for arm in ("sync", "snapshot")}

    def states(x):
        tot = sum(x.values())
        return ", ".join(f"{k.capitalize()} {100 * v / tot:.0f}\\%" for k, v in sorted(x.items(), key=lambda kv: -kv[1]))

    L = ["\\begin{tabular}{@{}llrrrl@{}}", "\\toprule",
         "\\textbf{Build} & \\textbf{Phase} & \\textbf{Blocks} & \\textbf{Interval (s)} & \\texttt{ExtendVote} mean / p95 / max (ms) & \\textbf{States (share of samples)} \\\\", "\\midrule"]
    for arm, name in (("sync", "Inline reads"), ("snapshot", "Background reads")):
        for ph, pn in (("baseline", "60\\,s healthy"), ("hang", "240\\,s hung"), ("recovery", "until \\textsc{Anchored}")):
            x = d[arm][ph]
            L.append(f"{name if ph == 'baseline' else ''} & {pn} & {x['blocks']} & {x['block_interval_s']:.2f} & "
                     f"{x['ev_mean']:.2f} / {x['ev_p95']:.1f} / {x['ev_max']:.1f} & {states(x['states'])} \\\\")
        if arm == "sync":
            L.append("\\midrule")
    L += ["\\bottomrule", "\\end{tabular}"]
    os.makedirs(OUT, exist_ok=True)
    open(os.path.join(OUT, "testbedb_hang_table.tex"), "w").write("\n".join(L) + "\n")
    json.dump(d, open(os.path.join(RES, "testbedb_hang_numbers.json"), "w"), indent=2)
    print("\n".join(L))
    print(json.dumps({a: (d[a]["recovered_after_s"], d[a]["stale_check"]) for a in d}))


if __name__ == "__main__":
    main()
