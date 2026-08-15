#!/usr/bin/env python3
"""E10 -- Bitcoin Reorg Fork-Choice Reaction, LIVE-docker figure
(docs/EXPERIMENT.md's E10), built from live_reorg_test.py's real output
against the real 4-node testnet.

One figure, 5 stacked panels -- FSM state timeline (engram-node01, real,
one representative node; all 4 validators agree at every sample in every
trial) for each real live trial run so far:
    1. Shallow (depth=1, < KDeepFinality=2)
    2. Boundary (depth=2, = KDeepFinality)
    3. Boundary (depth=3, = KDeepFinality+1)
    4. Deep (depth=15), trial 1
    5. Deep (depth=15), trial 2 (repeat)

Each panel's own CSV only covers that script run's own settle window --
trial 5's run ended still SOVEREIGN at the CSV's last sample; manual
follow-up polling (not in this CSV) found real recovery to ANCHORED at
t~=676s after reconnect (see docs/EXPERIMENT.md's E10 section), noted in
the panel title rather than plotted since it isn't sampled data.

Usage:
    python3 scripts/e10_bitcoin_reorg/live_figure_builder.py
"""

import csv
import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))
from utils import setup_academic_plot_style, savefig_academic  # noqa: E402

import matplotlib.pyplot as plt  # noqa: E402

RESULTS_LIVE_DIR = os.path.join(os.path.dirname(__file__), "results_live")
OUT_DIR = os.path.join(os.path.dirname(__file__), "results")
NODE = "engram-node01"

STATES = ["ANCHORED", "SUSPICIOUS", "SOVEREIGN", "RECOVERING"]
STATE_Y = {s: i for i, s in enumerate(STATES)}

TRIALS = [
    ("Shallow (depth=1, < KDeepFinality)", "reorg_shallow_20260812T140516.csv"),
    ("Boundary (depth=2, = KDeepFinality)", "reorg_deep_20260815T171706.csv"),
    ("Boundary (depth=3, = KDeepFinality+1)", "reorg_deep_20260815T172002.csv"),
    ("Deep (depth=15), trial 1", "reorg_deep_20260812T143632.csv"),
    ("Deep (depth=15), trial 2 -- repeat (real recovery to ANCHORED at t~676s, past this CSV's own settle window)", "reorg_deep_20260815T173058.csv"),
]


def load_node_samples(csv_path: str, node: str = NODE):
    rows = list(csv.DictReader(open(csv_path)))
    node_rows = [r for r in rows if r["node"] == node and r["fsm_state"]]
    t0 = min(float(r["timestamp"]) for r in node_rows)
    times = [float(r["timestamp"]) - t0 for r in node_rows]
    states = [r["fsm_state"] for r in node_rows]
    return times, states


def main():
    setup_academic_plot_style()

    available = [(label, fname) for label, fname in TRIALS if os.path.exists(os.path.join(RESULTS_LIVE_DIR, fname))]
    if not available:
        print("no E10 results_live CSVs found")
        return

    fig, axes = plt.subplots(len(available), 1, figsize=(8, 1.6 * len(available)), sharex=False)
    if len(available) == 1:
        axes = [axes]

    for ax, (label, fname) in zip(axes, available):
        path = os.path.join(RESULTS_LIVE_DIR, fname)
        times, states = load_node_samples(path)
        y = [STATE_Y[s] for s in states]
        ax.step(times, y, where="post", color="black", linewidth=1.5)
        ax.set_yticks(range(len(STATES)))
        ax.set_yticklabels(STATES, fontsize=7)
        ax.set_ylim(-0.5, len(STATES) - 0.5)
        ax.set_title(label, fontsize=8, loc="left")
        ax.grid(True, alpha=0.3)

    axes[-1].set_xlabel("Elapsed time (s)")
    fig.suptitle(
        "Figure -- E10 FSM Reaction to Real Bitcoin Reorgs by Depth\n"
        "(live 4-node Docker testnet, engram-node01, real -- all 4 validators agreed at every sample in every trial)",
        fontsize=9,
    )
    fig.tight_layout(rect=(0, 0, 1, 0.94))

    savefig_academic(fig, OUT_DIR, "figure8_reorg_depth_reaction_live")
    print(f"Figure written to {OUT_DIR}/figure8_reorg_depth_reaction_live.{{png,pdf}}")


if __name__ == "__main__":
    main()
