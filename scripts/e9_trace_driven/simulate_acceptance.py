#!/usr/bin/env python3
"""
E9 -- Trace-Driven Stress Test (docs/EXPERIMENT.md's E9, Figure 2).

Consumes REAL data from tests/e2e/results/e9_trace_driven.csv, produced by
`go test ./tests/e2e/... -run TestE9_TraceDrivenCombinedFailure`, which
replays one continuous combined-failure trace (BTC congestion -> +DA outage
-> +P2P churn spike -> phased healing -> recovery) through the real
Harness/BeginBlocker path, and renders the 6-panel timeline docs/EXPERIMENT.md
asks for: BTC finality gap, DA availability, P2P health, block commit rate,
withdrawal-lock status, and proof-submission status.

Usage:
    go test ./tests/e2e/... -run TestE9_TraceDrivenCombinedFailure
    python3 scripts/e9_trace_driven/simulate_acceptance.py
"""

import csv
import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))
from utils import (  # noqa: E402
    setup_academic_plot_style,
    figsize_multi_panel,
    savefig_academic,
)

import matplotlib.pyplot as plt  # noqa: E402

E2E_RESULTS_DIR = os.path.join(
    os.path.dirname(__file__), "..", "..", "tests", "e2e", "results"
)
CSV_IN = os.path.join(E2E_RESULTS_DIR, "e9_trace_driven.csv")
OUT_DIR = os.path.join(os.path.dirname(__file__), "results")

STATES = ["ANCHORED", "SUSPICIOUS", "SOVEREIGN", "RECOVERING"]
STATE_Y = {s: i for i, s in enumerate(STATES)}


def load_rows():
    with open(CSV_IN, newline="") as f:
        rows = list(csv.DictReader(f))
    for r in rows:
        r["height"] = int(r["height"])
        r["btc_gap"] = int(r["btc_gap"])
        for k in (
            "da_healthy",
            "p2p_healthy",
            "withdraw_locked",
            "proof_submitted",
            "block_committed",
        ):
            r[k] = r[k].lower() == "true"
    return rows


def plot_figure2(rows):
    setup_academic_plot_style()
    heights = [r["height"] for r in rows]

    fig, axes = plt.subplots(6, 1, figsize=figsize_multi_panel(6), sharex=True)

    axes[0].step(
        heights,
        [STATE_Y[r["state"]] for r in rows],
        where="post",
        linewidth=2,
        color="black",
    )
    axes[0].set_yticks(range(len(STATES)))
    axes[0].set_yticklabels(STATES, fontsize=8)
    axes[0].set_title("FSM State")

    axes[1].step(heights, [r["btc_gap"] for r in rows], where="post", color="C0")
    axes[1].set_title("BTC Finality Gap")

    axes[2].step(
        heights, [0 if r["da_healthy"] else 1 for r in rows], where="post", color="C1"
    )
    axes[2].set_yticks([0, 1])
    axes[2].set_yticklabels(["healthy", "outage"], fontsize=8)
    axes[2].set_title("DA Gap (availability)")

    axes[3].step(
        heights, [1 if r["p2p_healthy"] else 0 for r in rows], where="post", color="C2"
    )
    axes[3].set_yticks([0, 1])
    axes[3].set_yticklabels(["unhealthy", "healthy"], fontsize=8)
    axes[3].set_title("P2P Health Score")

    axes[4].step(
        heights,
        [1 if r["block_committed"] else 0 for r in rows],
        where="post",
        color="C3",
    )
    axes[4].set_yticks([0, 1])
    axes[4].set_ylim(-0.2, 1.2)
    axes[4].set_title("Block Commit Rate (1 = committed this height)")

    for h in (r["height"] for r in rows if r["withdraw_locked"]):
        axes[5].axvspan(h - 0.5, h + 0.5, color="red", alpha=0.15)
    axes[5].step(
        heights,
        [1 if r["proof_submitted"] else 0 for r in rows],
        where="post",
        color="C4",
        label="proof_submitted",
    )
    axes[5].set_yticks([0, 1])
    axes[5].set_title("Withdrawal Lock (red shading) / Proof Submission Status (line)")
    axes[5].set_xlabel("Block height")
    axes[5].legend(loc="upper left", fontsize=8)

    fig.suptitle(
        "Figure 2 -- FSM Timeline Under a Combined-Failure Trace (real tests/e2e data)"
    )
    fig.tight_layout()
    savefig_academic(fig, OUT_DIR, "figure2_trace_timeline")


def main():
    if not os.path.exists(CSV_IN):
        print(
            f"{CSV_IN} not found -- run 'go test ./tests/e2e/... -run TestE9_TraceDrivenCombinedFailure' first.",
            file=sys.stderr,
        )
        sys.exit(1)
    rows = load_rows()
    plot_figure2(rows)
    print(f"Loaded {len(rows)} real trace rows from {CSV_IN}")
    print(f"Figure 2 written to {OUT_DIR}/figure2_trace_timeline.{{png,pdf}}")


if __name__ == "__main__":
    main()
