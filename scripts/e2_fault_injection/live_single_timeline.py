#!/usr/bin/env python3
"""E2 -- single continuous FSM-state timeline across S1..S7 (live Docker data).

Unlike live_figure_builder.py (one panel per scenario, per-scenario t0), this
plots ONE figure whose horizontal axis is BLOCK HEIGHT through the whole
S1->S7 run. Height is the natural consensus unit: the FSM transitions on
blocks (safe_blocks/suspicious_duration are block counts), and elapsed time
would be distorted by the cooldowns and polling dead-time live_scenario_matrix.py
spends between phases.

The x-axis is phase-normalized: each scenario occupies the SAME visual width
(phase units 0..7), with the true block range printed under its label. This
keeps the run on one continuous row -- no broken axis, no inset zoom -- while
the short S1/S7 phases (~10/14 blocks vs ~546 for S2..S6) are not compressed
into slivers. Within a phase the step line is drawn proportional to block
height, so the FSM shape stays truthful. State-colored step line, soft area
fill under it, and black transition markers (including the phase-boundary
transitions, which cross sampling gaps like block 38646 between S5/S6) are
preserved.

Reads the same results_live/s*.csv files as live_figure_builder.py, plots the
best-covered node per scenario (same representative_rows heuristic -- node01 is
deliberately isolated in S5).

Usage:
    python3 scripts/e2_fault_injection/live_single_timeline.py
"""

import csv
import glob
import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))
from utils import (  # noqa: E402
    setup_academic_plot_style,
    savefig_academic,
)

import matplotlib.pyplot as plt  # noqa: E402

RESULTS_LIVE_DIR = os.path.join(os.path.dirname(__file__), "results_live")
OUT_DIR = os.path.join(os.path.dirname(__file__), "results")

STATES = ["ANCHORED", "SUSPICIOUS", "SOVEREIGN", "RECOVERING"]
STATE_Y = {s: i for i, s in enumerate(STATES)}
# Distinct color per state so a region flat in one state is instantly readable
# and a transition reads as a clear color jump even where it is short.
STATE_COLOR = {
    "ANCHORED": "#1f77b4",     # blue -- healthy baseline
    "SUSPICIOUS": "#ff7f0e",   # orange -- warning
    "SOVEREIGN": "#d62728",    # red -- degraded
    "RECOVERING": "#2ca02c",   # green -- recovering
}

# Legend labels in FSM order with a one-word meaning, so the reader does not
# need to recall the color mapping from memory.
STATE_LEGEND = [
    plt.Line2D([0], [0], color=STATE_COLOR[s], lw=3, label=f"{s} ({m})")
    for s, m in [
        ("ANCHORED", "healthy"),
        ("SUSPICIOUS", "warning"),
        ("SOVEREIGN", "degraded"),
        ("RECOVERING", "re-anchoring"),
    ]
]

# One-word cause printed at each transition marker (thresholds from utils.py's
# FSM_PARAMETERS). Keeps the annotation compact while explaining WHY the FSM
# moved, not just where it moved.
TRANSITION_CAUSE = {
    "SUSPICIOUS": "gap>100",
    "SOVEREIGN": "gap>500",
    "RECOVERING": "re-anchor",
    "ANCHORED": "re-anchored",
}

# Ordered S1..S7 in run order (matches live_scenario_matrix.py's phases).
SCENARIO_ORDER = [
    "s1_normal",
    "s2_btc_congestion",
    "s3_da_unavailable",
    "s4_p2p_eclipse_partial",
    "s5_anchor_isolation",
    "s6_combined_btc_da_failure",
    "s7_recovery",
]
SCENARIO_TITLES = {
    "s1_normal": "S1 Normal",
    "s2_btc_congestion": "S2 BTC Congestion",
    "s3_da_unavailable": "S3 DA Unavailable",
    "s4_p2p_eclipse_partial": "S4 P2P Eclipse (partial)",
    "s5_anchor_isolation": "S5 Anchor Isolation",
    "s6_combined_btc_da_failure": "S6 Combined BTC+DA Failure",
    "s7_recovery": "S7 Recovery",
}


def load_scenario_csv(path):
    with open(path, newline="") as f:
        rows = list(csv.DictReader(f))
    for r in rows:
        r["height"] = int(r["height"])
        r["timestamp"] = float(r["timestamp"])
    return rows


def representative_rows(rows):
    """Pick the best-covered node (see live_figure_builder.py) so node01's
    deliberate S5 isolation doesn't blank that region."""
    if not rows:
        return []
    by_node = {}
    for r in rows:
        if r["height"] >= 0:
            by_node.setdefault(r["node"], []).append(r)
    if not by_node:
        return []
    best_node = max(by_node, key=lambda n: len(by_node[n]))
    return sorted(by_node[best_node], key=lambda r: r["height"])


def _phase_x(idx, height, band_start, band_end):
    """Map a block height into phase units: phase idx occupies [idx, idx+1],
    proportional to the true block span within that phase."""
    span = band_end - band_start
    frac = 0.0 if span <= 0 else (height - band_start) / span
    return idx + min(max(frac, 0.0), 1.0)


def _plot_step_line(ax, name, scenarios, band_of):
    """Draw one scenario's state-colored step line + transition markers on the
    phase-normalized x-axis. Shared by the main axis only (no inset/panel)."""
    rep = representative_rows(scenarios[name])
    if not rep:
        return
    hs = [r["height"] for r in rep if r["fsm_state"]]
    states = [r["fsm_state"] for r in rep if r["fsm_state"]]
    if not hs:
        return

    idx = SCENARIO_ORDER.index(name)
    band_start, band_end = band_of[name]
    xs = [_phase_x(idx, h, band_start, band_end) for h in hs]

    # For each state-colored run (piecewise-constant steps), draw the soft area
    # fill under the step line -- turns the row into an area chart so the
    # degradation depth (how far below ANCHORED the state sits) reads at a
    # glance -- plus the step line itself, recolored at every state change.
    run_start = 0
    for i in range(1, len(xs)):
        if states[i] != states[i - 1]:
            ax.fill_between(
                xs[run_start:i + 1],
                [STATE_Y[s] for s in states[run_start:i + 1]],
                0,
                step="post",
                color=STATE_COLOR[states[run_start]],
                alpha=0.18,
                zorder=2,
            )
            ax.plot(
                xs[run_start:i + 1],
                [STATE_Y[s] for s in states[run_start:i + 1]],
                drawstyle="steps-post",
                linewidth=2.4,
                color=STATE_COLOR[states[run_start]],
                zorder=3,
            )
            run_start = i
    ax.fill_between(
        xs[run_start:],
        [STATE_Y[s] for s in states[run_start:]],
        0,
        step="post",
        color=STATE_COLOR[states[run_start]],
        alpha=0.18,
        zorder=2,
    )
    ax.plot(
        xs[run_start:],
        [STATE_Y[s] for s in states[run_start:]],
        drawstyle="steps-post",
        linewidth=2.4,
        color=STATE_COLOR[states[run_start]],
        zorder=3,
    )

    # Marker at each actual transition so a short state blip is not lost
    # inside a long flat region. White rim keeps the dot visible on any state
    # color (and on the area fill), so the marker reads as an explicit
    # "state changed here" glyph rather than blending into the line.
    for i in range(1, len(xs)):
        if states[i] != states[i - 1]:
            ax.scatter(
                xs[i],
                STATE_Y[states[i]],
                s=90,
                facecolor="black",
                edgecolor="white",
                linewidth=1.5,
                zorder=4,
            )
            # Block height + cause BELOW every marker (halo so it stays readable over
            # the state-colored area fill). Placing all annotations at the same
            # vertical side (rather than alternating above/below) keeps every
            # label exactly one state-level apart, so adjacent transitions --
            # e.g. a phase-end "re-anchored" and the next phase's "gap>100" --
            # can never overlap. y=0 still has room below (ylim bottom -0.5).
            ax.annotate(
                f"{hs[i]} · {TRANSITION_CAUSE[states[i]]}",
                (xs[i], STATE_Y[states[i]]),
                xytext=(0, -16),
                textcoords="offset points",
                ha="center",
                va="top",
                fontsize=7.5,
                color="0.15",
                zorder=6,
                bbox=dict(
                    boxstyle="round,pad=0.15",
                    facecolor="white",
                    edgecolor="0.6",
                    alpha=0.75,
                ),
            )


def _connect_phases(ax, scenarios, band_of):
    """Bridge the sampling gap at each phase boundary so the run reads as ONE
    continuous step line + area fill. The last sample of phase i and the first
    sample of phase i+1 map to different phase-unit x (e.g. S5 ends at x=5.0,
    S6 starts at x=5.028), with unsampled blocks in between (38646, 38717).
    Under steps-post semantics the previous state simply HOLDS across the gap,
    then transitions at the first sample of the next phase -- so draw the hold
    area fill + horizontal hold segment in the old color, then a vertical jump
    in the new color with the usual black transition marker."""
    for i in range(len(SCENARIO_ORDER) - 1):
        n1, n2 = SCENARIO_ORDER[i], SCENARIO_ORDER[i + 1]
        rep1 = representative_rows(scenarios[n1])
        rep2 = representative_rows(scenarios[n2])
        if not rep1 or not rep2:
            continue
        s1 = [r["fsm_state"] for r in rep1 if r["fsm_state"]]
        s2 = [r["fsm_state"] for r in rep2 if r["fsm_state"]]
        if not s1 or not s2:
            continue
        x1 = _phase_x(i, rep1[-1]["height"], *band_of[n1])
        x2 = _phase_x(i + 1, rep2[0]["height"], *band_of[n2])
        y1, y2 = STATE_Y[s1[-1]], STATE_Y[s2[0]]
        # Hold the previous state across the unsampled gap: area fill + line.
        ax.fill_between(
            [x1, x2],
            [y1, y1],
            0,
            step="post",
            color=STATE_COLOR[s1[-1]],
            alpha=0.18,
            zorder=2,
        )
        ax.plot(
            [x1, x2],
            [y1, y1],
            linewidth=2.4,
            color=STATE_COLOR[s1[-1]],
            zorder=3,
        )
        # Transition at the first sample of the next phase.
        if y1 != y2:
            ax.plot(
                [x2, x2],
                [y1, y2],
                linewidth=2.4,
                color=STATE_COLOR[s2[0]],
                zorder=3,
            )
            # Same white-rimmed marker as internal transitions, so a state
            # change at a phase boundary is as visible as one inside a phase.
            ax.scatter(
                x2,
                y2,
                s=90,
                facecolor="black",
                edgecolor="white",
                linewidth=1.5,
                zorder=4,
            )
            ax.annotate(
                f"{rep2[0]['height']} · {TRANSITION_CAUSE[s2[0]]}",
                (x2, y2),
                xytext=(0, -16),
                textcoords="offset points",
                ha="center",
                va="top",
                fontsize=7.5,
                color="0.15",
                zorder=6,
                bbox=dict(
                    boxstyle="round,pad=0.15",
                    facecolor="white",
                    edgecolor="0.6",
                    alpha=0.75,
                ),
            )


def main():
    csv_paths = {
        os.path.splitext(os.path.basename(p))[0]: p
        for p in glob.glob(os.path.join(RESULTS_LIVE_DIR, "s*.csv"))
    }
    missing = [k for k in SCENARIO_ORDER if k not in csv_paths]
    if missing:
        print(f"Missing scenario CSVs in {RESULTS_LIVE_DIR}: {missing}", file=sys.stderr)
        sys.exit(1)

    # Load in run order.
    scenarios = {name: load_scenario_csv(csv_paths[name]) for name in SCENARIO_ORDER}

    setup_academic_plot_style()
    # One continuous row; each phase takes equal visual width (phase units
    # 0..7). True block ranges are printed under the labels, so no information
    # is lost despite the non-linear x-scale.
    fig, ax = plt.subplots(figsize=(15, 5.4))

    # Region boundaries: each scenario's band runs from the previous phase's
    # end to this phase's end (contiguous).
    region_bounds = []
    for name in SCENARIO_ORDER:
        rep = representative_rows(scenarios[name])
        if not rep:
            continue
        region_bounds.append((min(r["height"] for r in rep), max(r["height"] for r in rep)))
    boundaries = [e for _, e in region_bounds]
    band_of = {}
    for idx, name in enumerate(SCENARIO_ORDER):
        band_start = boundaries[idx - 1] if idx > 0 else region_bounds[0][0]
        band_of[name] = (band_start, boundaries[idx])

    # Alternate light region backgrounds across the phases so each S band
    # reads as a distinct strip against the white plot floor.
    for idx, name in enumerate(SCENARIO_ORDER):
        if idx % 2 == 0:
            ax.axvspan(idx, idx + 1, color="0.93", alpha=0.6, zorder=0)

    for name in SCENARIO_ORDER:
        _plot_step_line(ax, name, scenarios, band_of)
    _connect_phases(ax, scenarios, band_of)

    # Vertical dashed lines at scenario boundaries (phase boundaries 1..6).
    for idx in range(1, len(SCENARIO_ORDER)):
        ax.axvline(idx, color="gray", linestyle="--", linewidth=1.0, alpha=0.7, zorder=2)

    # Header strip: full scenario name above each phase band, rotated 45deg.
    # Kept as low as the rotated text allows (headroom ~1.6 units above) so the
    # dead space between the top of the step line (y=3) and the header shrinks
    # and the state band uses proportionally more of the figure height.
    for idx, name in enumerate(SCENARIO_ORDER):
        ax.text(
            idx + 0.5,
            3.9,
            SCENARIO_TITLES[name],
            ha="center",
            va="center",
            rotation=45,
            fontsize=9,
            fontweight="bold",
            color="0.15",
            zorder=5,
        )

    ax.set_yticks(range(len(STATES)))
    ytick_labels = ax.set_yticklabels(STATES)
    # seaborn whitegrid turns the tick marks off; re-enable them so the 4
    # state levels read as clearly separated rows, not just floating labels.
    ax.tick_params(axis="y", which="major", left=True, length=6, width=1.0, color="0.35")
    # Color each y-tick label with its state color so the row/color mapping
    # needs no lookup.
    for label, state in zip(ytick_labels, STATES):
        label.set_color(STATE_COLOR[state])
        label.set_fontweight("bold")
    ax.set_ylim(-0.5, 4.8)
    ax.set_xlim(-0.25, len(SCENARIO_ORDER) + 0.45)
    # Phase centers labelled S1..S7, single line so the two bottom x-axes
    # never overlap; the block-height ruler below carries the true heights.
    ax.set_xticks([idx + 0.5 for idx in range(len(SCENARIO_ORDER))])
    ax.set_xticklabels(
        [f"S{idx + 1}" for idx in range(len(SCENARIO_ORDER))],
        fontsize=9,
        fontweight="bold",
    )
    ax.set_ylabel("FSM State")
    ax.grid(axis="y", linestyle=":", alpha=0.6, zorder=1)

    # True block-height ruler along the bottom: a secondary x-axis marking the
    # actual block heights at each phase boundary (0..7), so the non-linear
    # phase scale never hides the real block count the FSM ran through.
    ax2 = ax.secondary_xaxis("bottom")
    ax2.set_xticks(range(len(SCENARIO_ORDER) + 1))
    edge_heights = [region_bounds[0][0]] + boundaries
    ax2.set_xticklabels(edge_heights, fontsize=7)
    ax2.set_xlabel("Engram block height at each phase boundary", fontsize=12)

    # State legend above the plot, laid out horizontally.
    ax.legend(
        handles=STATE_LEGEND,
        loc="upper left",
        bbox_to_anchor=(0.0, 1.25),
        ncol=len(STATES),
        frameon=False,
        fontsize=9,
    )

    fig.suptitle(
        "Figure 3 -- Continuous FSM-State Timeline, S1..S7\n"
        "(live 4-node Docker testnet; equal-width phases, true block ranges)",
        y=0.99,
        fontsize=16,
        fontweight="bold",
    )
    fig.tight_layout()
    savefig_academic(fig, OUT_DIR, "figure3_single_timeline_live")

    print(f"Plotted {len(SCENARIO_ORDER)} scenarios on one equal-width phase row over {boundaries[-1]:,} blocks")
    print(f"Figure written to {OUT_DIR}/figure3_single_timeline_live.{{png,pdf}}")


if __name__ == "__main__":
    main()
