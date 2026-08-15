#!/usr/bin/env python3
"""E9 -- Trace-Driven Stress Test, LIVE-docker Figure 2 (docs/EXPERIMENT.md's
E9), built from live_combined_trace.py's real output against the real 4-node
testnet (not tests/e2e's in-process mock).

live_combined_trace.py's doc explains why this can't be a drop-in
replacement for simulate_acceptance.py's 6-panel layout: BTC finality gap,
DA availability, and P2P health score are never written into COMMITTED
state (x/sovereignty/preblock.go's NewPreBlocker only ever commits the
already-agreed fsm_state/receipts, never a fresh local sensor read -- see
that function's doc for the real safety bug this restriction fixes), so a
live RPC/ABCI-query poll structurally cannot observe them.

This script does NOT fabricate those 3 panels. It builds 6 DIFFERENT panels
from what IS real and observable via CometBFT RPC + Query.State:
    1. FSM State timeline (one representative node -- all 4 matched at
       every sampled height this run, cross-checked by panel 6)
    2. Block height progression (real commit pace/rate)
    3. Fault-injection windows (BTC congestion / DA outage / P2P churn),
       shaded from the run's REAL marker timestamps -- ground truth of when
       each fault was actually active, not a synthetic gap curve
    4. SafeBlocks (real hysteresis counter from Query.State)
    5. ReanchoringProofValid (real boolean flag from Query.State)
    6. Cross-node AppHash agreement (0/1 per sampled tick) -- the real
       safety signal E8's tests use throughout

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

RESULTS_LIVE_DIR = os.path.join(os.path.dirname(__file__), "results_live")
OUT_DIR = os.path.join(os.path.dirname(__file__), "results")

STATES = ["ANCHORED", "SUSPICIOUS", "SOVEREIGN", "RECOVERING"]
STATE_Y = {s: i for i, s in enumerate(STATES)}


def find_latest_csv():
    paths = sorted(glob.glob(os.path.join(RESULTS_LIVE_DIR, "e9_combined_trace_*.csv")))
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


def plot_figure2_live(rows, markers, csv_path):
    setup_academic_plot_style()
    nodes = sorted(set(r["node"] for r in rows))
    rep_node = nodes[0]
    all_rep_rows = sorted(
        [r for r in rows if r["node"] == rep_node], key=lambda r: r["t"]
    )
    # Drop transient RPC-error samples (logger.py's query_node sentinel:
    # height=-1, fsm_state="") -- an earlier version plotted every row,
    # producing a spurious dip to height=0 at the moment node04 was briefly
    # unreachable mid-run (real blocks never go backwards; that was a polling
    # artifact, not a real event).
    rep_rows = [r for r in all_rep_rows if r["height"] >= 0]

    fig, axes = plt.subplots(6, 1, figsize=figsize_multi_panel(6), sharex=True)

    # 1. FSM state (representative node)
    ts = [r["t"] for r in rep_rows if r["fsm_state"]]
    ys = [STATE_Y[r["fsm_state"]] for r in rep_rows if r["fsm_state"]]
    axes[0].step(ts, ys, where="post", linewidth=2, color="black")
    axes[0].set_yticks(range(len(STATES)))
    axes[0].set_yticklabels(STATES, fontsize=8)
    axes[0].set_title(f"FSM State ({rep_node}, real)")

    # 2. Height progression (real commit pace)
    axes[1].plot(
        [r["t"] for r in rep_rows], [r["height"] for r in rep_rows], color="C0"
    )
    axes[1].set_title("Block Height (real commit pace)")

    # 3. Fault-injection windows, from real markers
    windows = fault_windows(markers)
    colors = {"BTC congestion": "C1", "DA outage": "C2", "P2P churn": "C3"}
    for i, (name, (start, end)) in enumerate(windows.items()):
        if start is None:
            continue
        end = end if end is not None else max(r["t"] for r in rep_rows)
        axes[2].barh(
            i, end - start, left=start, color=colors[name], alpha=0.6, height=0.6
        )
    axes[2].set_yticks(range(len(windows)))
    axes[2].set_yticklabels(list(windows.keys()), fontsize=8)
    axes[2].set_title("Fault-Injection Windows (real markers, overlapping)")

    # 4. SafeBlocks (real hysteresis counter)
    axes[3].step(
        [r["t"] for r in rep_rows],
        [r["safe_blocks"] for r in rep_rows],
        where="post",
        color="C4",
    )
    axes[3].set_title("SafeBlocks (real hysteresis counter)")

    # 5. ReanchoringProofValid (real)
    axes[4].step(
        [r["t"] for r in rep_rows],
        [1 if r["reanchoring_proof_valid"] else 0 for r in rep_rows],
        where="post",
        color="C5",
    )
    axes[4].set_yticks([0, 1])
    axes[4].set_title("ReanchoringProofValid (real)")

    # 6. Cross-node AppHash agreement at each sampled tick (real safety signal)
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
    axes[5].step(agree_ts, agree_ys, where="post", color="black")
    axes[5].set_yticks([0, 1])
    axes[5].set_yticklabels(["DIVERGED", "agree"], fontsize=8)
    axes[5].set_title("Cross-Node AppHash Agreement (real safety check)")
    axes[5].set_xlabel("Elapsed time (s)")

    fig.suptitle(
        f"Figure 2 -- FSM Timeline Under a Real Combined-Failure Trace\n"
        f"(live 4-node Docker testnet, {os.path.basename(csv_path)})",
        fontsize=13,
    )
    fig.tight_layout()
    savefig_academic(fig, OUT_DIR, "figure2_trace_timeline_live")


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
    plot_figure2_live(rows, markers, csv_path)

    print(f"Loaded {len(rows)} real live samples from {csv_path}")
    print(f"Figure written to {OUT_DIR}/figure2_trace_timeline_live.{{png,pdf}}")


if __name__ == "__main__":
    main()
