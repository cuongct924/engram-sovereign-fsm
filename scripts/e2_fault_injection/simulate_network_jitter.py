#!/usr/bin/env python3
"""
E2 -- Fault-Injection End-to-End Prototype (docs/EXPERIMENT.md's E2, Figure 3).

Consumes REAL data from tests/e2e/results/s*.csv -- produced by
`go test ./tests/e2e/...`, which drives x/sovereignty's real BeginBlocker
in-process against mock BTC/DA/P2P sensors across 7 scenarios (S1-S7). No
synthetic/placeholder data.

Scope: this in-process figure covers S1-S7 only. E2's live 4-node Docker
testnet and the vanilla-CometBFT baseline are handled separately --
live_scenario_matrix.py (scripts/e2_fault_injection) and
vanilla_comparison.sh (scripts/e7_consensus_overhead).

Usage:
    go test ./tests/e2e/...          # regenerate tests/e2e/results/*.csv
    python3 scripts/e2_fault_injection/simulate_network_jitter.py
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

E2E_RESULTS_DIR = os.path.join(
    os.path.dirname(__file__), "..", "..", "tests", "e2e", "results"
)
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
        r["height"] = int(r["height"])
        r["withdraw_locked"] = r["withdraw_locked"].lower() == "true"
    return rows


def plot_state_timelines(scenarios):
    setup_academic_plot_style()
    n = len(scenarios)
    fig, axes = plt.subplots(n, 1, figsize=figsize_multi_panel(n), sharex=False)
    if n == 1:
        axes = [axes]

    for ax, (name, rows) in zip(axes, scenarios.items()):
        heights = [r["height"] for r in rows]
        ys = [STATE_Y[r["state"]] for r in rows]
        ax.step(heights, ys, where="post", linewidth=2)

        # Shade blocks where withdrawals are locked.
        for h in (r["height"] for r in rows if r["withdraw_locked"]):
            ax.axvspan(h - 0.5, h + 0.5, color="red", alpha=0.08)

        ax.set_yticks(range(len(STATES)))
        ax.set_yticklabels(STATES, fontsize=9)
        ax.set_ylim(-0.5, len(STATES) - 0.5)
        ax.set_title(SCENARIO_TITLES.get(name, name), fontsize=11, loc="left")
        ax.set_xlabel("Block height")

    fig.suptitle(
        "Figure 3 -- FSM State Timeline under Fault Injection (real tests/e2e data)",
        y=1.0,
    )
    fig.tight_layout()
    savefig_academic(fig, OUT_DIR, "figure3_state_timelines")


def plot_summary_bars(scenarios, metrics_by_scenario):
    setup_academic_plot_style()
    names = list(scenarios.keys())
    labels = [SCENARIO_TITLES.get(n, n) for n in names]

    fig, axes = plt.subplots(1, 3, figsize=figsize_row(3))
    for ax, key, title in zip(
        axes,
        ("time_to_fallback", "recovery_time", "withdrawal_blocked_blocks"),
        (
            "Time to Fallback (blocks)",
            "Recovery Time (blocks)",
            "Withdrawal-Locked Blocks",
        ),
    ):
        vals = [metrics_by_scenario.get(n, {}).get(key) for n in names]
        xs = [v if v is not None else 0 for v in vals]
        colors = ["#999999" if v is None else "#1f77b4" for v in vals]
        ax.barh(labels, xs, color=colors)
        ax.set_title(title, fontsize=11)
        ax.invert_yaxis()

    fig.suptitle("E2 Summary Metrics (real tests/e2e data; grey = n/a)")
    fig.tight_layout()
    savefig_academic(fig, OUT_DIR, "figure3_summary_bars")


def parse_summary_md(path):
    """Parse tests/e2e/results/e2_summary.md's markdown table into a dict keyed by csv-basename."""
    name_to_key = {v: k for k, v in SCENARIO_TITLES.items()}
    out = {}
    with open(path) as f:
        for line in f:
            if not line.startswith("| S"):
                continue
            cols = [c.strip() for c in line.strip().strip("|").split("|")]
            scenario_name, ttf, rt, wb, fc, tt = cols
            key = name_to_key.get(scenario_name)
            if key is None:
                continue

            def parse(v):
                return None if v == "n/a" else int(v)

            out[key] = {
                "time_to_fallback": parse(ttf),
                "recovery_time": parse(rt),
                "withdrawal_blocked_blocks": parse(wb),
                "flapping_count": parse(fc),
                "total_transitions": parse(tt),
            }
    return out


def main():
    csv_paths = sorted(glob.glob(os.path.join(E2E_RESULTS_DIR, "s*.csv")))
    if not csv_paths:
        print(
            f"No CSVs found in {E2E_RESULTS_DIR} -- run 'go test ./tests/e2e/...' first.",
            file=sys.stderr,
        )
        sys.exit(1)

    scenarios = {}
    for p in csv_paths:
        name = os.path.splitext(os.path.basename(p))[0]
        scenarios[name] = load_scenario_csv(p)

    summary_path = os.path.join(E2E_RESULTS_DIR, "e2_summary.md")
    metrics_by_scenario = parse_summary_md(summary_path)

    plot_state_timelines(scenarios)
    plot_summary_bars(scenarios, metrics_by_scenario)

    print(f"Loaded {len(scenarios)} real scenarios from {E2E_RESULTS_DIR}")
    print(f"Figure 3 written to {OUT_DIR}/figure3_state_timelines.{{png,pdf}}")
    print(f"Summary bars written to {OUT_DIR}/figure3_summary_bars.{{png,pdf}}")


if __name__ == "__main__":
    main()
