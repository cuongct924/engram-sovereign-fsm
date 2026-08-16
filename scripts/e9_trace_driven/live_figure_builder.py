#!/usr/bin/env python3
"""E9 -- Trace-Driven Stress Test, LIVE-docker Figure 2 (docs/EXPERIMENT.md's
E9), built from live_combined_trace.py's real output against the real 4-node
testnet (not tests/e2e's in-process mock).

live_combined_trace.py's doc explains why BTC finality gap, DA availability,
and P2P health score aren't committed state (x/sovereignty/preblock.go's
NewPreBlocker only ever commits the already-agreed fsm_state/receipts, never
a fresh local sensor read -- see that function's doc for the real safety bug
this restriction fixes), so a live RPC/ABCI-query poll structurally cannot
observe them AS COMMITTED DATA.

This script builds 4 panels (2 if the sensor scrape is missing) from what IS
committed/agreed and observable via CometBFT RPC + Query.State, grouped by
signal kind rather than one row per raw field:
    1. System State & Actions -- FSM state step-line (real), red shading over
       derived withdrawal-locked windows (fsm_state in
       {SOVEREIGN, RECOVERING}), a star at the real re-anchoring
       proof-valid transition (if any this run), SafeBlocks (real hysteresis
       counter), and cross-node AppHash agreement as a caption fact when
       constant this run (plotted as a line only if it ever diverges)
    2. Sensor Health -- BTC finality gap + DA availability gap, twin axes
       (DIAGNOSTIC/LOCAL sensor reads, not committed state -- see
       x/sovereignty/preblock.go's NewPreBlocker doc)
    3. P2P Network -- active anchors + peer latency, twin axes (also
       DIAGNOSTIC/LOCAL)
    4. Consensus Liveness -- block height alone, proving the chain never
       stalled

The 3 real fault windows (BTC congestion / DA outage / P2P churn, from the
run's own marker timestamps) are shaded translucently across every panel
(not a separate categorical panel), with one shared legend, so a reader can
draw a single vertical line through all panels and see the storm converge.

Panels 2-3 (both sensor-CSV-dependent) drop out together if no sibling
`<csv>_sensors.csv` scrape exists, leaving a 2-panel figure.

Usage:
    python3 scripts/e9_trace_driven/live_figure_builder.py [csv_path]
    (defaults to the newest e9_combined_trace_*.csv in results_live/)
"""

import csv
import glob
import os
import re
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))
from utils import (  # noqa: E402
    setup_academic_plot_style,
    figsize_multi_panel,
    savefig_academic,
)

import matplotlib.pyplot as plt  # noqa: E402
import matplotlib.patches as mpatches  # noqa: E402
from matplotlib.ticker import MaxNLocator  # noqa: E402

RESULTS_LIVE_DIR = os.path.join(os.path.dirname(__file__), "results_live")
OUT_DIR = os.path.join(os.path.dirname(__file__), "results")

STATES = ["ANCHORED", "SUSPICIOUS", "SOVEREIGN", "RECOVERING"]
STATE_Y = {s: i for i, s in enumerate(STATES)}
# Same mapping as scripts/e2_fault_injection/live_single_timeline.py and
# scripts/e10_bitcoin_reorg/live_figure_builder.py, so a reader who has seen
# either figure recognizes this one's color coding immediately.
STATE_COLOR = {
    "ANCHORED": "#1f77b4",
    "SUSPICIOUS": "#ff7f0e",
    "SOVEREIGN": "#d62728",
    "RECOVERING": "#2ca02c",
}


def find_latest_csv():
    paths = sorted(
        glob.glob(os.path.join(RESULTS_LIVE_DIR, "e9_combined_trace_*.csv"))
    )
    # Exclude the *_sensors.csv diagnostic scrape -- it has no timestamp column
    # and would fail load_rows if picked as the main source.
    paths = [p for p in paths if not p.endswith("_sensors.csv")]
    if not paths:
        return None
    return paths[-1]


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


def load_sensor_rows(csv_path):
    """Loads the sibling `<csv>_sensors.csv` if it exists -- diagnostic-only
    per-validator BTC/DA/P2P sensor reads (sensor_log_scraper.py's output).
    Returns [] (not an error) if the file doesn't exist, matching the
    "gracefully fall back to the state+liveness-only panels" behavior the
    module doc promises.
    """
    sensors_path = csv_path.replace(".csv", "_sensors.csv")
    if not os.path.exists(sensors_path):
        return []
    with open(sensors_path, newline="") as f:
        rows = list(csv.DictReader(f))
    for r in rows:
        r["height"] = int(r["height"])
        r["btc_gap"] = int(r["btc_gap"])
        r["da_gap"] = int(r["da_gap"])
        r["active_anchors"] = int(r["active_anchors"])
        r["peer_latency"] = int(r["peer_latency"])
        r["approx_timestamp"] = float(r["approx_timestamp"]) if r["approx_timestamp"] not in ("", "None") else None
    return rows


def load_markers(summary_path):
    """Parses the real '| t (s) | Event |' table from the run's own
    _summary.md -- these are the actual wall-clock offsets
    live_combined_trace.py recorded when it started/stopped each fault,
    not estimated after the fact.
    """
    markers = []
    if not os.path.exists(summary_path):
        return markers
    with open(summary_path) as f:
        in_table = False
        for line in f:
            if line.startswith("| t (s) | Event |"):
                in_table = True
                continue
            if in_table:
                if not line.startswith("|"):
                    break
                if line.startswith("|---"):
                    continue
                m = re.match(r"\|\s*(\d+)\s*\|\s*(.+?)\s*\|", line)
                if m:
                    markers.append((float(m.group(1)), m.group(2)))
    return markers


def fault_windows(markers):
    """Derives [start, end] windows for each of the 3 overlapping fault
    classes from the real marker text -- matching live_combined_trace.py's
    own fixed phrasing for each start/stop event.
    """
    windows = {
        "BTC congestion": [None, None],
        "DA outage": [None, None],
        "P2P churn": [None, None],
    }
    for t, event in markers:
        if "chaos-btc-delay started" in event:
            windows["BTC congestion"][0] = t
        elif "chaos-btc-delay cleaned up" in event:
            windows["BTC congestion"][1] = t
        elif "celestia-bridge stopped" in event:
            windows["DA outage"][0] = t
        elif "celestia-bridge restarted" in event:
            windows["DA outage"][1] = t
        elif "churn burst starting" in event:
            windows["P2P churn"][0] = t
        elif "churn burst complete" in event:
            windows["P2P churn"][1] = t
    return windows


def withdrawal_locked(fsm_state):
    """Deterministic function of fsm_state -- no withdrawal_locked column
    exists in this live CSV schema, but x/sovereignty/preblock.go only ever
    sets WithdrawalLocked=true for SOVEREIGN/RECOVERING headers (see
    x/sovereignty/types/recovery_header.go), so it's derived here instead of
    added as a new data dependency."""
    return fsm_state in ("SOVEREIGN", "RECOVERING")


def locked_windows(rep_rows):
    """Contiguous [start, end] elapsed-time spans where the representative
    node's real fsm_state was SOVEREIGN or RECOVERING."""
    windows, start, prev_t = [], None, None
    for r in rep_rows:
        locked = withdrawal_locked(r["fsm_state"])
        if locked and start is None:
            start = r["t"]
        elif not locked and start is not None:
            windows.append((start, prev_t))
            start = None
        prev_t = r["t"]
    if start is not None:
        windows.append((start, prev_t))
    return windows


def shade_fault_windows(ax, windows, colors, t_max, alpha=0.13):
    """Translucent axvspan per real fault window, replicated on every panel
    (not a separate categorical panel) so a vertical line through all panels
    shows the real storm convergence directly. Same 3 colors everywhere;
    captioned once via a single figure-level legend, not repeated per panel."""
    for name, (start, end) in windows.items():
        if start is None:
            continue
        end = end if end is not None else t_max
        ax.axvspan(start, end, color=colors[name], alpha=alpha, zorder=0, linewidth=0)


def find_proof_valid_transition(rep_rows):
    """First False->True transition of reanchoring_proof_valid on the
    representative node's real samples -- (t, y) for a star marker, or None
    if the proof never went valid this run. Conditional on the data so a
    real event is never silently skipped."""
    prev = False
    for r in rep_rows:
        if r["reanchoring_proof_valid"] and not prev:
            return r["t"], STATE_Y[r["fsm_state"]]
        prev = r["reanchoring_proof_valid"]
    return None


def _draw_state_segments(ax, ts, states):
    """FSM state step-line colored per active state (STATE_COLOR), matching
    scripts/e10_bitcoin_reorg/live_figure_builder.py's visual language --
    small black-on-white dots mark each real transition. No per-segment
    legend label; state identity reads off the color-matched y-tick labels
    instead (avoids a 4-entry line legend competing with panel 1's other
    handles)."""
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


def _finish_figure(fig, windows, colors, csv_path):
    """Shared suptitle + single fault-window legend + save, used by both the
    2-panel and 4-panel branches."""
    fault_handles = [
        mpatches.Patch(facecolor=colors[name], alpha=0.35, label=name)
        for name, (start, _end) in windows.items()
        if start is not None
    ]
    # Header band (suptitle + fault-window legend, both outside the panel
    # axes) laid out with golden-ratio-proportioned spacing: the gap above
    # the suptitle and the gap between suptitle and legend follow the same
    # ~1.618 rhythm as the figure's own width:height, instead of arbitrary
    # y-values.
    fig.suptitle(
        "Figure 2 -- FSM Timeline Under a Real Combined-Failure Trace",
        fontsize=13,
        fontweight="bold",
        y=0.97,
    )
    fig.text(
        0.5, 0.945,
        f"(live 4-node Docker testnet, {os.path.basename(csv_path)})",
        ha="center", va="top", fontsize=10,
    )
    if fault_handles:
        fig.legend(
            handles=fault_handles,
            loc="upper center",
            bbox_to_anchor=(0.5, 0.885),
            ncol=len(fault_handles),
            frameon=False,
            fontsize=8,
            title="Fault-injection windows (real markers, shaded on every panel)",
            title_fontsize=8,
        )
    fig.tight_layout()
    fig.subplots_adjust(top=0.78, hspace=0.55)
    savefig_academic(fig, OUT_DIR, "figure2_trace_timeline_live")


def plot_figure2_live(rows, markers, csv_path, sensor_rows=None):
    setup_academic_plot_style()
    nodes = sorted(set(r["node"] for r in rows))
    rep_node = nodes[0]
    all_rep_rows = sorted(
        [r for r in rows if r["node"] == rep_node], key=lambda r: r["t"]
    )
    # Drop transient RPC-error samples (logger.py's query_node sentinel:
    # height=-1, fsm_state="") -- real blocks never go backwards; that was a
    # polling artifact, not a real event.
    rep_rows = [r for r in all_rep_rows if r["height"] >= 0]
    t0 = min(r["timestamp"] for r in rows)
    t_max = max(r["t"] for r in rep_rows)

    rep_sensor_rows = []
    if sensor_rows:
        rep_sensor_rows = sorted(
            (r for r in sensor_rows if r["node"] == rep_node and r["approx_timestamp"] is not None),
            key=lambda r: r["approx_timestamp"],
        )

    windows = fault_windows(markers)
    # A consistent "these are all faults" warm trio (Okabe-Ito colorblind-safe
    # hues) -- the previous DA-outage color (green, matplotlib C2) read as
    # "healthy" by convention and collided with RECOVERING's green in panel
    # 1's FSM line; none of these three overlaps green or blue, both of
    # which already mean "healthy" elsewhere in this figure.
    colors = {"BTC congestion": "#E69F00", "DA outage": "#CC79A7", "P2P churn": "#D55E00"}

    # Grouped-by-signal-kind layout (not one row per raw field): 4 panels
    # with sensor data (state+actions / BTC+DA / P2P / liveness), 2 without
    # (sensor-CSV-dependent panels 2-3 drop out together). All panels share
    # one x-axis (sharex=True) -- the whole point is a reader can draw one
    # vertical line through every panel and see the real storm converge.
    n_panels = 4 if rep_sensor_rows else 2
    # Uneven height_ratios instead of one flat panel_height for every row:
    # panel 1 carries 4 categorical FSM levels + a twin axis + shading + a
    # legend (the densest panel by far) and earns more vertical room; panel
    # 4 is a single plain line and needs the least. Total height matches
    # figsize_multi_panel(n_panels, panel_height=1.9)'s old flat total, just
    # redistributed.
    FIG_WIDTH = 12
    GOLDEN_RATIO = 1.618
    # Whole-figure width:height fixed to the golden ratio, not just the
    # panel stack -- the header band (suptitle + fault-window legend) is
    # carved out of this same canvas via subplots_adjust(top=...) below, not
    # added on top of it, so this height sets the entire figure's proportion.
    total_height = FIG_WIDTH / GOLDEN_RATIO
    height_ratios = [1.5, 1.0, 1.0, 0.7] if n_panels == 4 else [1.5, 0.9]
    # Wider than the strict double-column width (7.16in): at that width the
    # pre-SOVEREIGN stretch of panel 1 (the only part of the timeline clear
    # of the withdrawal-locked shading) was too narrow to hold a legend
    # without it spilling into the shaded region.
    fig, axes = plt.subplots(
        n_panels, 1, figsize=(FIG_WIDTH, total_height), sharex=True,
        gridspec_kw={"height_ratios": height_ratios},
    )

    # --- Panel 1: System State & Actions ------------------------------
    ax = axes[0]
    shade_fault_windows(ax, windows, colors, t_max)
    # Black diagonal hatch, no fill -- a solid red span here would be
    # visually indistinguishable from the "P2P churn" fault window, which is
    # also a shade of red and frequently overlaps this exact region (both
    # active at once for most of the SOVEREIGN plateau). Hatching reads as a
    # distinct annotation layer regardless of what fault color sits under it.
    for start, end in locked_windows(rep_rows):
        ax.axvspan(start, end, facecolor="none", edgecolor="black", hatch="///",
                   linewidth=0, alpha=0.35, zorder=1)

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

    # SafeBlocks (real hysteresis counter) folded in here, twin axis --
    # small count range, would be unreadable sharing the categorical FSM
    # y-axis.
    ax_sb = ax.twinx()
    ax_sb.grid(False)
    ax_sb.step(
        [r["t"] for r in rep_rows], [r["safe_blocks"] for r in rep_rows],
        where="post", color="darkviolet", linewidth=1.8, linestyle="--",
        marker=".", markersize=3.5, label="Safe blocks", zorder=4,
    )
    ax_sb.set_ylabel("Safe blocks", fontsize=9, color="darkviolet", fontweight="bold")
    ax_sb.tick_params(axis="y", labelcolor="darkviolet", labelsize=8)
    sb_vals = [r["safe_blocks"] for r in rep_rows]
    # Pin the SafeBlocks line into a band near the TOP of the panel (~75-95%
    # of the visible height) instead of near the bottom -- the FSM line
    # spends most of the run at ANCHORED (bottom, ~12%) or SOVEREIGN
    # (mid-height, ~62%), so hugging the bottom (the previous approach)
    # made SafeBlocks visually collide with the ANCHORED line, and the
    # legend box sitting near the bottom to avoid one ended up covering the
    # other. The upper band is clear for the whole run except the brief
    # instant the FSM itself reaches RECOVERING.
    sb_ref = max(max(sb_vals), 1)
    ax_sb.set_ylim(-3.75 * sb_ref, 1.25 * sb_ref)
    ax_sb.set_yticks(sorted(set([0] + sb_vals)))
    ax_sb.tick_params(axis="x", labelbottom=False)

    # Cross-node AppHash agreement -- computation unchanged from the prior
    # version. Conditional on the result: a real divergence is still
    # plotted (never silently hidden); a constant all-agree series (this
    # run) is stated as a caption fact instead of a flat, uninformative line.
    by_tick = {}
    for r in rows:
        if r["height"] < 0 or not r["app_hash"]:
            continue
        key = round(r["t"])
        by_tick.setdefault(key, {}).setdefault(r["height"], set()).add(r["app_hash"])
    agree_ts, agree_ys = [], []
    for tick in sorted(by_tick.keys()):
        heights_at_tick = by_tick[tick]
        agree = all(len(hashes) == 1 for hashes in heights_at_tick.values())
        agree_ts.append(tick)
        agree_ys.append(1 if agree else 0)
    agree_all = all(y == 1 for y in agree_ys) if agree_ys else True
    if not agree_all:
        ax_sb.step(
            agree_ts, agree_ys, where="post", color="black", linewidth=1.2,
            linestyle="--", label="AppHash agree (0/1)", zorder=3,
        )

    handles, labels = ax.get_legend_handles_labels()
    h2, l2 = ax_sb.get_legend_handles_labels()
    handles += h2
    labels += l2
    lock_patch = mpatches.Patch(
        facecolor="none", edgecolor="black", hatch="///", alpha=0.35, label="Withdrawal locked"
    )
    handles.append(lock_patch)
    labels.append(lock_patch.get_label())
    # "center left" -- for t=0..110 (before the first transition) the FSM
    # line sits flat at the bottom (ANCHORED) and SafeBlocks now sits in its
    # own band near the top, leaving the vertical middle of the panel
    # completely empty; "lower left" used to sit directly on the ANCHORED
    # line (the legend's own opaque background blanked out the real data
    # under it).
    ax.legend(handles, labels, loc="center left", fontsize=8)

    if agree_all:
        fact = f"AppHash agreement: 100% ({len(agree_ts)} ticks), zero divergence"
    else:
        fact = "AppHash agreement: DIVERGED this run (dashed line)"
    if star is not None:
        fact += f" · proof valid at t={star[0]:.0f}s"
    ax.set_title(f"1. System State & Actions ({rep_node})\n{fact}", fontsize=8.5, loc="left")

    if not rep_sensor_rows:
        axes[-1].set_xlabel("Elapsed time (s)")
        heights = [r["height"] for r in rep_rows]
        ax_live = axes[-1]
        shade_fault_windows(ax_live, windows, colors, t_max)
        ax_live.plot([r["t"] for r in rep_rows], heights, color="C0", linewidth=1.6, label="Block Height")
        ax_live.set_ylabel("Block Height", fontsize=9)
        ax_live.set_ylim(min(heights) - 10, max(heights) + 10)
        # Fewer bins and a smaller label font than the other panels -- this
        # one is the shortest (height_ratio 0.7), so the same nbins=8 used
        # elsewhere produced tick labels tall enough to overlap each other.
        ax_live.yaxis.set_major_locator(MaxNLocator(nbins=4, integer=True))
        ax_live.tick_params(axis="y", labelsize=7.5)
        ax_live.set_title("2. Consensus Liveness -- Block Height (chain never stalled)", fontsize=8.5, loc="left")
        ax_live.set_xlim(0, t_max)
        _finish_figure(fig, windows, colors, csv_path)
        return

    # --- Panel 2: Sensor Health (BTC gap + DA gap, twin axis) ---------
    sensor_ts = [r["approx_timestamp"] - t0 for r in rep_sensor_rows]
    ax = axes[1]
    shade_fault_windows(ax, windows, colors, t_max)
    # Same hues as "BTC congestion"/"DA outage" in the fault-window shading
    # above -- one color per peripheral across the whole figure, not just
    # within this panel.
    ax.step(sensor_ts, [r["btc_gap"] for r in rep_sensor_rows], where="post", color="#E69F00", linewidth=1.4, label="BTC gap", zorder=3)
    ax.set_ylabel("BTC gap", fontsize=9)
    btc_gaps = [r["btc_gap"] for r in rep_sensor_rows]
    ax.set_ylim(0, max(btc_gaps) + 2)
    ax.yaxis.set_major_locator(MaxNLocator(nbins=8, integer=True))
    ax.tick_params(axis="y", labelsize=8)
    ax_da = ax.twinx()
    ax_da.grid(False)
    ax_da.step(sensor_ts, [r["da_gap"] for r in rep_sensor_rows], where="post", color="#CC79A7", linewidth=1.4, label="DA gap", zorder=3)
    ax_da.set_ylabel("DA gap", fontsize=9)
    da_gaps = [r["da_gap"] for r in rep_sensor_rows]
    ax_da.set_ylim(0, max(da_gaps) + 10)
    ax_da.yaxis.set_major_locator(MaxNLocator(nbins=8, integer=True))
    ax_da.tick_params(axis="x", labelbottom=False)
    ax_da.tick_params(axis="y", labelsize=8)
    h1, l1 = ax.get_legend_handles_labels()
    h2, l2 = ax_da.get_legend_handles_labels()
    ax.legend(h1 + h2, l1 + l2, loc="upper left", fontsize=8)
    ax.set_title("2. Sensor Health -- BTC / DA Gap (local sensor, uncommitted)", fontsize=8.5, loc="left")

    # --- Panel 3: P2P Network (active anchors + peer latency) --------
    # clean_peers and churn_rate are constant this run (3 and 0 the whole
    # trace) -- flat, uninformative lines. peer_latency instead spikes
    # 0->537ms right around the DA-outage/P2P-churn overlap, so it's the
    # field carrying real signal and is paired with active_anchors instead.
    ax = axes[2]
    shade_fault_windows(ax, windows, colors, t_max)
    ax.step(sensor_ts, [r["active_anchors"] for r in rep_sensor_rows], where="post", color="C3", linewidth=1.4, label="Active anchors", zorder=3)
    ax.set_ylabel("Active anchors", fontsize=9)
    anchors = [r["active_anchors"] for r in rep_sensor_rows]
    ax.set_ylim(0, max(anchors) + 1)
    ax.yaxis.set_major_locator(MaxNLocator(nbins=8, integer=True))
    ax.tick_params(axis="y", labelsize=8)
    ax_lat = ax.twinx()
    ax_lat.grid(False)
    latencies = [r["peer_latency"] for r in rep_sensor_rows]
    ax_lat.step(sensor_ts, latencies, where="post", color="C5", linewidth=1.4, label="Peer latency (ms)", zorder=3)
    ax_lat.set_ylabel("Peer latency (ms)", fontsize=9)
    ax_lat.set_ylim(0, max(latencies) + 50)
    ax_lat.yaxis.set_major_locator(MaxNLocator(nbins=8, integer=True))
    ax_lat.tick_params(axis="x", labelbottom=False)
    ax_lat.tick_params(axis="y", labelsize=8)
    h1, l1 = ax.get_legend_handles_labels()
    h2, l2 = ax_lat.get_legend_handles_labels()
    # "lower left" -- active_anchors is flat at 3 for the ENTIRE run (no x
    # position avoids it at "upper left"); peer_latency is 0 until t~110, so
    # the lower-left corner is the one clear pocket.
    ax.legend(h1 + h2, l1 + l2, loc="lower left", fontsize=8)
    ax.set_title("3. P2P Network -- Active Anchors & Peer Latency (local sensor)", fontsize=8.5, loc="left")

    # --- Panel 4: Consensus Liveness (block height, alone) ------------
    ax = axes[3]
    shade_fault_windows(ax, windows, colors, t_max)
    heights = [r["height"] for r in rep_rows]
    ax.plot([r["t"] for r in rep_rows], heights, color="C0", linewidth=1.6, label="Block Height")
    ax.set_ylabel("Block Height", fontsize=9)
    ax.set_ylim(min(heights) - 10, max(heights) + 10)
    # Fewer bins and a smaller label font than the other panels -- this one
    # is the shortest (height_ratio 0.7), so the same nbins=8 used elsewhere
    # produced tick labels tall enough to overlap each other.
    ax.yaxis.set_major_locator(MaxNLocator(nbins=4, integer=True))
    ax.tick_params(axis="y", labelsize=7.5)
    ax.set_title("4. Consensus Liveness -- Block Height (chain never stalled)", fontsize=8.5, loc="left")
    ax.set_xlabel("Elapsed time (s)")
    ax.set_xlim(0, t_max)

    _finish_figure(fig, windows, colors, csv_path)


def main():
    csv_path = sys.argv[1] if len(sys.argv) > 1 else find_latest_csv()
    if not csv_path or not os.path.exists(csv_path):
        print(
            f"No e9_combined_trace_*.csv found in {RESULTS_LIVE_DIR} -- "
            "run scripts/e9_trace_driven/live_combined_trace.py first.",
            file=sys.stderr,
        )
        sys.exit(1)
    summary_path = csv_path.replace(".csv", "_summary.md")

    rows = load_rows(csv_path)
    markers = load_markers(summary_path)
    sensor_rows = load_sensor_rows(csv_path)
    plot_figure2_live(rows, markers, csv_path, sensor_rows=sensor_rows)

    print(f"Loaded {len(rows)} real live samples from {csv_path}")
    if sensor_rows:
        print(f"Loaded {len(sensor_rows)} diagnostic sensor_snapshot rows -- 4-panel figure (state+actions, sensor health, P2P, liveness)")
    else:
        print("No sensor_snapshot data found -- 2-panel figure (state+actions + liveness only; sensor-health/P2P panels unavailable)")
    print(f"Figure written to {OUT_DIR}/figure2_trace_timeline_live.{{png,pdf}}")


if __name__ == "__main__":
    main()
