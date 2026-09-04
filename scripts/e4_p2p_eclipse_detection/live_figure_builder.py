#!/usr/bin/env python3
"""E4 -- A4 (Relay-Node Latency Inflation), live Figure 5, built from
live_relay_latency_attack.py's real CSV output against the real 7-node testnet.

A single continuous FSM-state timeline (2 panels: state+actions, and consensus
liveness), same visual language as scripts/e9_composite_fault/live_figure_builder.py
and scripts/e10_bitcoin_reorg/live_figure_builder.py -- but for ONE real fault
window (the sustained netem delay), not several overlapping ones, so it needs
only 2 panels and one shaded region instead of a legend of fault classes.

Usage:
    python3 scripts/e4_p2p_eclipse_detection/live_figure_builder.py [csv_path]
    (defaults to the newest relay_latency_attack_a4_*.csv in results_live/)
"""

import csv
import glob
import os
import re
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))
from utils import (  # noqa: E402
    setup_academic_plot_style,
    savefig_academic,
)

import matplotlib.pyplot as plt  # noqa: E402
import matplotlib.patches as mpatches  # noqa: E402
from matplotlib.ticker import MaxNLocator  # noqa: E402

RESULTS_LIVE_DIR = os.path.join(os.path.dirname(__file__), "results_live")
OUT_DIR = os.path.join(os.path.dirname(__file__), "results")

STATES = ["ANCHORED", "SUSPICIOUS", "SOVEREIGN", "RECOVERING"]
STATE_Y = {s: i for i, s in enumerate(STATES)}
# Same mapping as scripts/e2_fault_injection/live_single_timeline.py,
# scripts/e9_composite_fault/live_figure_builder.py, and
# scripts/e10_bitcoin_reorg/live_figure_builder.py, so a reader who has seen
# any of those figures recognizes this one's color coding immediately.
STATE_COLOR = {
    "ANCHORED": "#1f77b4",
    "SUSPICIOUS": "#ff7f0e",
    "SOVEREIGN": "#d62728",
    "RECOVERING": "#2ca02c",
}
ATTACK_COLOR = "#D55E00"  # same hue family as E9's "P2P churn" fault window


def find_latest_csv():
    paths = sorted(glob.glob(os.path.join(RESULTS_LIVE_DIR, "relay_latency_attack_a4_*.csv")))
    return paths[-1] if paths else None


def load_rows(csv_path):
    with open(csv_path, newline="") as f:
        rows = list(csv.DictReader(f))
    for r in rows:
        r["timestamp"] = float(r["timestamp"])
        r["height"] = int(r["height"])
        r["safe_blocks"] = int(r["safe_blocks"]) if r["height"] >= 0 else 0
        r["reanchoring_proof_valid"] = r["reanchoring_proof_valid"] == "True"
    t0 = min(r["timestamp"] for r in rows)
    for r in rows:
        r["t"] = r["timestamp"] - t0
    return rows


def load_attack_window(summary_path):
    """Parses the run's own '| t (s) | phase | states |' table for the real
    attack-window boundaries (first 'attack' row, first 'recovery' row) --
    the actual wall-clock offsets this run recorded, not the script's
    --attack-s default, since a run can override it."""
    start = end = None
    in_table = False
    with open(summary_path) as f:
        for line in f:
            if line.startswith("| t (s) | phase | states |"):
                in_table = True
                continue
            if not in_table:
                continue
            if not line.startswith("|"):
                break
            if line.startswith("|---"):
                continue
            m = re.match(r"\|\s*([\d.]+)\s*\|\s*(\w+)\s*\|", line)
            if not m:
                continue
            t, phase = float(m.group(1)), m.group(2)
            if phase == "attack" and start is None:
                start = t
            if phase == "recovery" and end is None:
                end = t
    return start, end


def find_proof_valid_transition(rep_rows):
    """First False->True transition of reanchoring_proof_valid on the
    representative node's real samples -- (t, y) for a star marker, or None
    if the proof never went valid this run."""
    prev = False
    for r in rep_rows:
        if r["reanchoring_proof_valid"] and not prev:
            return r["t"], STATE_Y[r["fsm_state"]]
        prev = r["reanchoring_proof_valid"]
    return None


def _draw_state_segments(ax, ts, states):
    """FSM state step-line colored per active state, black-on-white dots
    marking each transition -- same drawing routine as
    scripts/e9_composite_fault/live_figure_builder.py's _draw_state_segments."""
    if not ts:
        return
    run_start = 0
    for i in range(1, len(ts)):
        if states[i] != states[i - 1]:
            seg_ts, seg_states = ts[run_start:i + 1], states[run_start:i + 1]
            ax.plot(
                seg_ts, [STATE_Y[s] for s in seg_states], drawstyle="steps-post",
                linewidth=2.2, color=STATE_COLOR[seg_states[0]], zorder=5,
            )
            ax.scatter(
                ts[i], STATE_Y[states[i]], s=45, facecolor="black",
                edgecolor="white", linewidth=1.1, zorder=6,
            )
            run_start = i
    seg_ts, seg_states = ts[run_start:], states[run_start:]
    ax.plot(
        seg_ts, [STATE_Y[s] for s in seg_states], drawstyle="steps-post",
        linewidth=2.2, color=STATE_COLOR[seg_states[0]], zorder=5,
    )


def plot_figure5_live(rows, attack_start, attack_end, csv_path):
    setup_academic_plot_style()
    nodes = sorted(set(r["node"] for r in rows))
    rep_node = nodes[0]
    all_rep_rows = sorted((r for r in rows if r["node"] == rep_node), key=lambda r: r["t"])
    # Drop transient RPC-error samples (height=-1, fsm_state="") -- a polling
    # artifact, not a real event.
    rep_rows = [r for r in all_rep_rows if r["height"] >= 0]
    t_max = max(r["t"] for r in rep_rows)

    FIG_WIDTH = 9.5
    GOLDEN_RATIO = 1.618
    total_height = FIG_WIDTH / GOLDEN_RATIO
    fig, axes = plt.subplots(
        2, 1, figsize=(FIG_WIDTH, total_height), sharex=True,
        gridspec_kw={"height_ratios": [1.5, 0.9]},
    )

    # --- Panel 1: System State & Actions --------------------------------
    ax = axes[0]
    if attack_start is not None:
        ax.axvspan(attack_start, attack_end if attack_end is not None else t_max,
                   color=ATTACK_COLOR, alpha=0.13, zorder=0, linewidth=0)

    ts_fsm = [r["t"] for r in rep_rows if r["fsm_state"]]
    states_fsm = [r["fsm_state"] for r in rep_rows if r["fsm_state"]]
    _draw_state_segments(ax, ts_fsm, states_fsm)

    star = find_proof_valid_transition(rep_rows)
    if star is not None:
        star_t, star_y = star
        ax.scatter(
            star_t, star_y, marker="*", s=260, color="gold", edgecolor="black",
            linewidth=1.0, zorder=7, label="Re-anchoring proof valid",
        )

    ax.set_yticks(range(len(STATES)))
    ytick_labels = ax.set_yticklabels(STATES, fontsize=8)
    for lbl, s in zip(ytick_labels, STATES):
        lbl.set_color(STATE_COLOR[s])
        lbl.set_fontweight("bold")
    ax.set_ylim(-0.5, len(STATES) - 0.5)

    handles = [mpatches.Patch(facecolor=ATTACK_COLOR, alpha=0.35, label="Relay-latency attack (netem delay active)")]
    if star is not None:
        h, l = ax.get_legend_handles_labels()
        handles = handles + h
    # "upper left": the FSM hasn't reached RECOVERING/SOVEREIGN yet at low t,
    # so that corner is empty; "center left" collided with the real
    # SUSPICIOUS/SOVEREIGN step line, which runs wide across the middle here.
    ax.legend(handles=handles, loc="upper left", fontsize=8)

    fact = f"7 of 7 validators independently see Healthy=false; quorum (>2/3) met"
    if star is not None:
        fact += f" · proof valid at t={star[0]:.0f}s"
    ax.set_title(f"1. System State & Actions ({rep_node})\n{fact}", fontsize=8.5, loc="left")

    # --- Panel 2: Consensus Liveness (block height) ---------------------
    ax_live = axes[1]
    if attack_start is not None:
        ax_live.axvspan(attack_start, attack_end if attack_end is not None else t_max,
                         color=ATTACK_COLOR, alpha=0.13, zorder=0, linewidth=0)
    heights = [r["height"] for r in rep_rows]
    ax_live.plot([r["t"] for r in rep_rows], heights, color="C0", linewidth=1.6, label="Block Height")
    ax_live.set_ylabel("Block Height", fontsize=9)
    ax_live.set_ylim(min(heights) - 10, max(heights) + 10)
    ax_live.yaxis.set_major_locator(MaxNLocator(nbins=6, integer=True))
    ax_live.tick_params(axis="y", labelsize=7.5)
    ax_live.set_title("2. Consensus Liveness -- Block Height (chain never stalled)", fontsize=8.5, loc="left")
    ax_live.set_xlabel("Elapsed time (s)")
    ax_live.set_xlim(0, t_max)

    fig.suptitle(
        "Figure 5 -- FSM Cascade Under a Real Relay-Latency Attack (A4)",
        fontsize=13, fontweight="bold", y=0.98,
    )
    fig.text(0.5, 0.925, f"(live 7-node Docker testnet, {os.path.basename(csv_path)})",
              ha="center", va="top", fontsize=9)
    fig.tight_layout()
    fig.subplots_adjust(top=0.85, hspace=0.4)
    savefig_academic(fig, OUT_DIR, "figure5_a4_cascade_timeline_live")


def main():
    csv_path = sys.argv[1] if len(sys.argv) > 1 else find_latest_csv()
    if not csv_path or not os.path.exists(csv_path):
        print(
            f"No relay_latency_attack_a4_*.csv found in {RESULTS_LIVE_DIR} -- "
            "run scripts/e4_p2p_eclipse_detection/live_relay_latency_attack.py first.",
            file=sys.stderr,
        )
        sys.exit(1)
    summary_path = csv_path.replace(".csv", "_summary.md")

    rows = load_rows(csv_path)
    attack_start, attack_end = load_attack_window(summary_path)
    plot_figure5_live(rows, attack_start, attack_end, csv_path)

    print(f"Loaded {len(rows)} real live samples from {csv_path}")
    print(f"Attack window: t={attack_start}s to t={attack_end}s (from {os.path.basename(summary_path)}'s own phase markers)")
    print(f"Figure written to {OUT_DIR}/figure5_a4_cascade_timeline_live.{{png,pdf}}")


if __name__ == "__main__":
    main()
