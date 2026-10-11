#!/usr/bin/env python3
"""E10 -- Bitcoin Reorg Fork-Choice Reaction, LIVE-docker figure
(docs/EXPERIMENT.md's E10), built from live_reorg_test.py's real output
against the real 7-node testnet.

One figure, 5 stacked panels -- FSM state timeline (engram-node01, real,
one representative node; all 7 validators agree at every sample in every
trial) for one consolidated fresh run covering every depth that matters:
    1. Shallow (depth=1, < K_DEEP_FINALITY=2)
    2. Boundary (depth=2, = K_DEEP_FINALITY)
    3. Boundary (depth=3, = K_DEEP_FINALITY+1)
    4. Deep (depth=5, = SuspiciousThreshold)
    5. Deep (depth=15), well past the anchored checkpoint

Each panel's own CSV only covers that script run's own settle window; the
depth=15 trial ends still SOVEREIGN at the CSV's last sample -- the cluster
recovered to ANCHORED afterward on its own, outside this CSV's sampled
window (see docs/EXPERIMENT.md's E10 section).

Panels are 5 independent trials at different depths, not one continuous run
(unlike E2's live_single_timeline.py, which plots a single sequential
S1..S7 run on one shared axis) -- so each trial keeps its own axis and
elapsed-time scale, but the state-colored step line, area fill, and
white-rimmed transition markers follow live_single_timeline.py's visual
language, so a reader who has seen that figure recognizes this one's
color coding immediately.

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
# Same mapping as scripts/e2_fault_injection/live_single_timeline.py, so a
# reader who has seen that figure recognizes the color coding immediately.
STATE_COLOR = {
    "ANCHORED": "#1f77b4",
    "SUSPICIOUS": "#ff7f0e",
    "SOVEREIGN": "#d62728",
    "RECOVERING": "#2ca02c",
}
STATE_LEGEND = [
    plt.Line2D([0], [0], color=STATE_COLOR[s], lw=3, label=f"{s} ({m})")
    for s, m in [
        ("ANCHORED", "healthy"),
        ("SUSPICIOUS", "warning"),
        ("SOVEREIGN", "degraded"),
        ("RECOVERING", "re-anchoring"),
    ]
]

TRIALS = [
    ("Shallow (depth=1, < K_DEEP_FINALITY)", "reorg_shallow_20260904T005405.csv"),
    ("Boundary (depth=2, = K_DEEP_FINALITY)", "reorg_deep_20260904T005602.csv"),
    ("Boundary (depth=3, = K_DEEP_FINALITY+1)", "reorg_deep_20260904T005801.csv"),
    ("Deep (depth=5, = SuspiciousThreshold)", "reorg_deep_20260904T005958.csv"),
    ("Deep (depth=15), well past the anchored checkpoint", "reorg_deep_20260904T010155.csv"),
]


def load_node_samples(csv_path: str, node: str = NODE):
    rows = list(csv.DictReader(open(csv_path)))
    node_rows = [r for r in rows if r["node"] == node and r["fsm_state"]]
    t0 = min(float(r["timestamp"]) for r in node_rows)
    times = [float(r["timestamp"]) - t0 for r in node_rows]
    states = [r["fsm_state"] for r in node_rows]
    return times, states


def _draw_run(ax, xs, states):
    """One piecewise-constant run: soft area fill under the step line (reads
    as an area chart, so degradation depth is visible at a glance) plus the
    step line itself, both in that run's state color."""
    if not xs:
        return
    ys = [STATE_Y[s] for s in states]
    color = STATE_COLOR[states[0]]
    ax.fill_between(xs, ys, 0, step="post", color=color, alpha=0.18, zorder=2)
    ax.plot(xs, ys, drawstyle="steps-post", linewidth=2.2, color=color, zorder=3)


def plot_trial(ax, times, states):
    """State-colored step line + white-rimmed transition markers, matching
    live_single_timeline.py's per-run visual language."""
    run_start = 0
    for i in range(1, len(times)):
        if states[i] != states[i - 1]:
            _draw_run(ax, times[run_start:i + 1], states[run_start:i + 1])
            run_start = i
    _draw_run(ax, times[run_start:], states[run_start:])

    for i in range(1, len(times)):
        if states[i] != states[i - 1]:
            ax.scatter(
                times[i], STATE_Y[states[i]],
                s=70, facecolor="black", edgecolor="white", linewidth=1.4, zorder=4,
            )
            ax.annotate(
                f"t={times[i]:.0f}s",
                (times[i], STATE_Y[states[i]]),
                xytext=(0, -14),
                textcoords="offset points",
                ha="center", va="top", fontsize=7,
                color="0.15", zorder=6,
                bbox=dict(boxstyle="round,pad=0.15", facecolor="white", edgecolor="0.6", alpha=0.75),
            )


def main():
    setup_academic_plot_style()

    available = [(label, fname) for label, fname in TRIALS if os.path.exists(os.path.join(RESULTS_LIVE_DIR, fname))]
    if not available:
        print("no E10 results_live CSVs found")
        return

    fig, axes = plt.subplots(len(available), 1, figsize=(10, 1.9 * len(available)), sharex=False)
    if len(available) == 1:
        axes = [axes]

    for ax, (label, fname) in zip(axes, available):
        path = os.path.join(RESULTS_LIVE_DIR, fname)
        times, states = load_node_samples(path)
        plot_trial(ax, times, states)

        ax.set_yticks(range(len(STATES)))
        ytick_labels = ax.set_yticklabels(STATES, fontsize=8)
        # Color each y-tick label with its state color so the row/color
        # mapping needs no lookup against the shared legend.
        for lbl, s in zip(ytick_labels, STATES):
            lbl.set_color(STATE_COLOR[s])
            lbl.set_fontweight("bold")
        ax.tick_params(axis="y", which="major", left=True, length=5, width=1.0, color="0.35")
        ax.set_ylim(-0.5, len(STATES) - 0.5)
        ax.set_title(label, fontsize=9, fontweight="bold", color="0.15", loc="left")
        ax.grid(axis="y", linestyle=":", alpha=0.6, zorder=1)

    axes[-1].set_xlabel("Elapsed time (s)", fontsize=11)

    # No figure title or subtitle: the paper's caption carries both.
    fig.legend(
        handles=STATE_LEGEND,
        loc="upper center",
        bbox_to_anchor=(0.5, 0.995),
        ncol=len(STATES),
        frameon=False,
        fontsize=9,
    )
    fig.tight_layout(rect=(0, 0, 1, 0.94))

    savefig_academic(fig, OUT_DIR, "reorg_depth_reaction")
    print(f"Figure written to {OUT_DIR}/reorg_depth_reaction.{{png,pdf}}")


if __name__ == "__main__":
    main()
