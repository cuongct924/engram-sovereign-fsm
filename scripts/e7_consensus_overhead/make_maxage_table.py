#!/usr/bin/env python3
"""Table of the freshness-window sweep (hung Bitcoin RPC, snapshot build, one run per window) for the paper:
docs/paper/results/testbedb_maxage_table.tex and a JSON with the numbers and their source files.

    python3 scripts/e7_consensus_overhead/make_maxage_table.py
"""
import glob
import json
import os

HERE = os.path.dirname(os.path.abspath(__file__))
RES = os.path.join(HERE, "results_live")
OUT = os.path.join(HERE, "..", "..", "docs", "paper", "results")


def load(ms):
    f = sorted(glob.glob(os.path.join(RES, f"rpc_hang_snapshot_age{ms}_*_summary.json")))
    if not f:
        return None
    s = json.load(open(f[-1]))
    return dict(source=os.path.basename(f[-1]), summary=s)


def main():
    rows, data = [], {}
    for ms in (1000, 2000, 5000, 10000):
        d = load(ms)
        if d is None:
            continue
        s = d["summary"]
        b, h, st = s["baseline_check"], s["phases"]["hang"], s["stale_check"]
        data[ms] = dict(source=d["source"], baseline_lines=b["snapshot_lines"], baseline_degraded=b["degraded_lines"],
                        hang_lines=st["snapshot_lines"], hang_degraded=st["degraded_lines"], hang_btc_gap_zero=st["btc_gap_zero_lines"],
                        ev_mean=h["extend_vote_ms_mean"], ev_max=h["extend_vote_ms_max"], recovered_after_s=s["recovered_after_s"])
        x = data[ms]
        rows.append(f"{ms / 1000:g} & {x['baseline_degraded']} / {x['baseline_lines']} & {x['hang_degraded']} / {x['hang_lines']} & {x['hang_btc_gap_zero']} & "
                    f"{x['ev_mean']:.2f} / {x['ev_max']:.1f} & {x['recovered_after_s']:.1f} \\\\")
    L = ["\\begin{tabular}{@{}rrrrrr@{}}", "\\toprule",
         "\\textbf{Window (s)} & \\textbf{Degraded, healthy} & \\textbf{Degraded, frozen} & \\textbf{Healthy reads, frozen} & \\texttt{ExtendVote} mean / max (ms) & \\textbf{Back to \\textsc{Anchored} (s)} \\\\",
         "\\midrule"] + rows + ["\\bottomrule", "\\end{tabular}"]
    os.makedirs(OUT, exist_ok=True)
    open(os.path.join(OUT, "testbedb_maxage_table.tex"), "w").write("\n".join(L) + "\n")
    json.dump(data, open(os.path.join(RES, "testbedb_maxage_numbers.json"), "w"), indent=2)
    print("\n".join(L))


if __name__ == "__main__":
    main()
