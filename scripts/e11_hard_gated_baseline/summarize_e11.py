#!/usr/bin/env python3
"""E11 aggregation: results_live/e11_summary.csv -> results_live/e11_summary.md (per fault x mode
table) and results/e11_height_timeline.{pdf,png} (block height vs time, fault window shaded, one
panel per fault, first trial of each mode).

Rotate or delete e11_summary.csv before a fresh measurement: every row in it is aggregated.

Usage:
    python3 scripts/e11_hard_gated_baseline/summarize_e11.py
"""

import csv
import os
import sys
from collections import defaultdict

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(HERE, ".."))
from utils import figsize_multi_panel, savefig_academic, setup_academic_plot_style  # noqa: E402

import matplotlib.pyplot as plt  # noqa: E402

RESULTS_LIVE = os.path.join(HERE, "results_live")
OUT_DIR = os.path.join(HERE, "results")
MODES = ["hard-gated", "engram", "soft"]
FAULTS = ["btc", "da", "both"]
FAULT_LABEL = {"btc": "Checkpoint-submission pause", "da": "DA outage", "both": "Both"}
MODE_COLOR = {"hard-gated": "#d62728", "engram": "#1f77b4", "soft": "#ff7f0e"}


def num(v):
    return None if v in ("", None) else float(v)


def load_rows():
    with open(os.path.join(RESULTS_LIVE, "e11_summary.csv"), newline="") as f:
        return list(csv.DictReader(f))


def rng(values):
    vals = [v for v in values if v is not None]
    if not vals:
        return "n/a"
    mean = sum(vals) / len(vals)
    return f"{mean:.1f} [{min(vals):.0f}, {max(vals):.0f}]"


def write_table(rows):
    groups = defaultdict(list)
    for r in rows:
        groups[(r["fault"], r["mode"])].append(r)
    lines = [
        "# E11 -- liveness under peripheral degradation (hard-gated vs. Engram vs. soft)",
        "",
        "Cells are mean [min, max] over trials. `Stalled` counts trials whose longest block gap in",
        "the fault window reached 30 s or whose chain never resumed. `Unsafe wd.` is withdrawals",
        "committed while the FSM state at that height was SOVEREIGN/RECOVERING (committed in total).",
        "",
        "| Fault | System | Trials | Blocks in fault | Longest gap (s) | Max round | Resume after heal (s) | Stalled | Unsafe wd. (of committed) |",
        "|---|---|---|---|---|---|---|---|---|",
    ]
    for fault in FAULTS:
        for mode in MODES:
            g = groups.get((fault, mode))
            if not g:
                continue
            stalled = sum(1 for r in g if r["stalled"] == "True")
            unsafe = sum(int(r["withdrawals_unsafe"]) for r in g)
            committed = sum(int(r["withdrawals_committed"]) for r in g)
            lines.append(
                f"| {FAULT_LABEL[fault]} | {mode} | {len(g)} | {rng([num(r['fault_blocks']) for r in g])} "
                f"| {rng([num(r['longest_gap_s']) for r in g])} | {rng([num(r['max_round_in_fault']) for r in g])} "
                f"| {rng([num(r['resume_after_heal_s']) for r in g])} | {stalled}/{len(g)} | {unsafe} ({committed}) |"
            )
    path = os.path.join(RESULTS_LIVE, "e11_summary.md")
    with open(path, "w") as f:
        f.write("\n".join(lines) + "\n")
    print(f"wrote {path}")


def write_figure(rows):
    setup_academic_plot_style()
    fig, axes = plt.subplots(len(FAULTS), 1, figsize=figsize_multi_panel(len(FAULTS), panel_height=1.6), sharex=False)
    for ax, fault in zip(axes, FAULTS):
        for mode in MODES:
            first = next((r for r in rows if r["fault"] == fault and r["mode"] == mode and r["trial"] == "1"), None)
            if first is None:
                continue
            with open(os.path.join(RESULTS_LIVE, first["timeline_csv"]), newline="") as f:
                tl = [r for r in csv.DictReader(f) if int(r["height"]) >= 0]
            h0 = int(tl[0]["height"])
            ax.plot([float(r["t"]) for r in tl], [int(r["height"]) - h0 for r in tl],
                    color=MODE_COLOR[mode], label=mode, linewidth=1.0)
            ax.axvspan(float(first["t_fault_start"]), float(first["t_heal"]), color="grey", alpha=0.12, linewidth=0)
        ax.set_ylabel("blocks")
        ax.set_title(FAULT_LABEL[fault], fontsize=7, loc="left")
    axes[-1].set_xlabel("time since trial start (s); shaded = fault window")
    axes[0].legend(frameon=False, ncol=3, fontsize=6, loc="upper left")
    fig.tight_layout()
    savefig_academic(fig, OUT_DIR, "e11_height_timeline")
    print(f"wrote {OUT_DIR}/e11_height_timeline.pdf/.png")


def main() -> int:
    rows = load_rows()
    write_table(rows)
    write_figure(rows)
    return 0


if __name__ == "__main__":
    sys.exit(main())
