#!/usr/bin/env python3
"""E2 -- Fault-Injection End-to-End Prototype, LIVE-docker Figure 3
(docs/EXPERIMENT.md's E2), built from
scripts/e2_fault_injection/live_scenario_matrix.py's real S1-S7 output
against the real 4-node testnet (not tests/e2e's in-process mock).

Same real-data-only discipline as scripts/e9_trace_driven/live_figure_builder.py:
only fsm_state/height/safe_blocks/reanchoring_proof_valid/app_hash are ever
committed state a live RPC/ABCI-query poll can observe (see
x/sovereignty/preblock.go's NewPreBlocker doc) -- no withdraw_locked marker
exists in this live schema the way tests/e2e's mock CSV has one, so the
state-timeline panels here don't shade withdrawal-locked blocks the way
simulate_network_jitter.py's in-process version does.

Usage:
    python3 scripts/e2_fault_injection/live_figure_builder.py
"""

import csv
import glob
import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))
from utils import (  # noqa: E402
    setup_academic_plot_style,
    figsize_multi_panel,
    figsize_row,
    savefig_academic,
)

import matplotlib.pyplot as plt  # noqa: E402

RESULTS_LIVE_DIR = os.path.join(os.path.dirname(__file__), "results_live")
OUT_DIR = os.path.join(os.path.dirname(__file__), "results")

STATES = ["ANCHORED", "SUSPICIOUS", "SOVEREIGN", "RECOVERING"]
STATE_Y = {s: i for i, s in enumerate(STATES)}

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
        r["timestamp"] = float(r["timestamp"])
        r["height"] = int(r["height"])
    if not rows:
        return rows
    t0 = min(r["timestamp"] for r in rows)
    for r in rows:
        r["t"] = r["timestamp"] - t0
    return rows


def representative_rows(rows):
    """Picks whichever node has the MOST height-filtered (real, non-error)
    samples, not the alphabetically-first one: E2's scenarios deliberately
    isolate engram-node01 (S5's 100% loss), so always picking node01 produces
    a near-empty timeline for exactly the scenarios most worth plotting.
    Picking the best-covered node keeps every panel showing the real
    cluster-wide behavior (3/4 healthy nodes still agree during partial
    isolation)."""
    if not rows:
        return []
    by_node = {}
    for r in rows:
        if r["height"] >= 0:
            by_node.setdefault(r["node"], []).append(r)
    if not by_node:
        return []
    best_node = max(by_node, key=lambda n: len(by_node[n]))
    return sorted(by_node[best_node], key=lambda r: r["t"])


def plot_state_timelines(scenarios):
    setup_academic_plot_style()
    n = len(scenarios)
    fig, axes = plt.subplots(n, 1, figsize=figsize_multi_panel(n), sharex=False)
    if n == 1:
        axes = [axes]

    for ax, (name, rows) in zip(axes, scenarios.items()):
        rep = representative_rows(rows)
        ts = [r["t"] for r in rep if r["fsm_state"]]
        ys = [STATE_Y[r["fsm_state"]] for r in rep if r["fsm_state"]]
        ax.step(ts, ys, where="post", linewidth=2)
        ax.set_yticks(range(len(STATES)))
        ax.set_yticklabels(STATES, fontsize=9)
        ax.set_ylim(-0.5, len(STATES) - 0.5)
        ax.set_title(SCENARIO_TITLES.get(name, name), fontsize=11, loc="left")
        ax.set_xlabel("Elapsed time (s)")

    fig.suptitle(
        "Figure 3 -- FSM State Timeline under Fault Injection\n(live 4-node Docker testnet)",
        y=1.0,
    )
    fig.tight_layout()
    savefig_academic(fig, OUT_DIR, "figure3_state_timelines_live")


def scenario_metrics(rows):
    """Real, purely CSV-derived summary metrics -- in-process-only fields
    (time_to_fallback/withdrawal_blocked_blocks) aren't observable from live
    polling, so report what IS: elapsed duration, height delta (commit
    throughput proxy), transition count (flapping proxy), and fraction of
    samples outside ANCHORED (degraded-time fraction)."""
    rep = representative_rows(rows)
    if not rep:
        return {
            "duration_s": 0,
            "height_delta": 0,
            "transitions": 0,
            "degraded_frac": 0,
        }
    duration_s = rep[-1]["t"] - rep[0]["t"]
    height_delta = rep[-1]["height"] - rep[0]["height"]
    transitions = sum(
        1
        for a, b in zip(rep, rep[1:])
        if a["fsm_state"] and b["fsm_state"] and a["fsm_state"] != b["fsm_state"]
    )
    degraded = sum(1 for r in rep if r["fsm_state"] and r["fsm_state"] != "ANCHORED")
    degraded_frac = degraded / len([r for r in rep if r["fsm_state"]])
    return {
        "duration_s": duration_s,
        "height_delta": height_delta,
        "transitions": transitions,
        "degraded_frac": degraded_frac,
    }


def plot_summary_bars(scenarios):
    setup_academic_plot_style()
    names = list(scenarios.keys())
    # Short codes ("S1".."S7"), not full names -- a 3-subplot row at
    # double-column width gives each panel ~2.4in, not enough for labels
    # like "S3 DA Unavailable"; SCENARIO_TITLES is the legend.
    short_codes = [n.split("_")[0].upper() for n in names]
    metrics = {n: scenario_metrics(rows) for n, rows in scenarios.items()}

    # Extra height beyond figsize_row's default: the 2-line suptitle plus
    # 2-line subplot titles need more vertical room than a 1-line-title row.
    width, height = figsize_row(3)
    fig, axes = plt.subplots(1, 3, figsize=(width, height * 1.55), sharey=True)
    for i, (ax, key, title) in enumerate(
        zip(
            axes,
            ("height_delta", "transitions", "degraded_frac"),
            (
                "Blocks Committed\n(real throughput)",
                "Real FSM\nTransitions Observed",
                "Fraction of Time\nOutside ANCHORED",
            ),
        )
    ):
        vals = [metrics[n][key] for n in names]
        ax.barh(short_codes, vals, color="#1f77b4")
        ax.set_title(title, fontsize=10)
        if i == 0:
            ax.invert_yaxis()

    fig.suptitle(
        "E2 Summary Metrics (live 4-node Docker testnet, real data)\n"
        "S1=Normal S2=BTC-congestion S3=DA-unavailable S4=P2P-eclipse\n"
        "S5=Anchor-isolation S6=Combined-BTC+DA S7=Recovery",
        fontsize=10,
        y=0.97,
    )
    fig.tight_layout(rect=(0, 0, 1, 0.82))
    savefig_academic(fig, OUT_DIR, "figure3_summary_bars_live")


def main():
    csv_paths = sorted(glob.glob(os.path.join(RESULTS_LIVE_DIR, "s*.csv")))
    if not csv_paths:
        print(
            f"No CSVs found in {RESULTS_LIVE_DIR} -- run "
            "scripts/e2_fault_injection/live_scenario_matrix.py first.",
            file=sys.stderr,
        )
        sys.exit(1)

    scenarios = {}
    for p in csv_paths:
        name = os.path.splitext(os.path.basename(p))[0]
        scenarios[name] = load_scenario_csv(p)

    plot_state_timelines(scenarios)
    plot_summary_bars(scenarios)

    print(f"Loaded {len(scenarios)} real live scenarios from {RESULTS_LIVE_DIR}")
    print(f"Figure written to {OUT_DIR}/figure3_state_timelines_live.{{png,pdf}}")
    print(f"Summary bars written to {OUT_DIR}/figure3_summary_bars_live.{{png,pdf}}")


if __name__ == "__main__":
    main()
