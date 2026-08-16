#!/usr/bin/env python3
"""
E5 -- Hysteresis and Flapping Sensitivity (docs/EXPERIMENT.md's E5, Figure 4).

Consumes REAL data from tests/e2e/results/e5_hysteresis_sweep.csv, produced
by `go test ./tests/e2e/... -run TestE5_HysteresisSweep`, which sweeps
HysteresisWait in {0,1,3,5,10,20} through the real Harness/BeginBlocker path
under 5 environments: stable (control) and 4 noisy (noisy_btc, noisy_da,
noisy_p2p, combined_adversarial), each with a per-block 20% chance of a
1-block disturbance, using a fixed RNG seed shared across all values so the
"weather" is identical and only HysteresisWait's filtering differs.

Real finding (differs from docs/EXPERIMENT.md's aspirational
"HYSTERESIS_WAIT=3-5 sweet spot" -- reported as measured): under sustained
noise, anchored_uptime decreases MONOTONICALLY as HYSTERESIS_WAIT increases
for noisy_btc (0.60 at HW=0 down to 0.00 at HW=20) -- no interior sweet spot.
Architectural reason, not a tuning artifact: HysteresisSafety's gate only
applies to RECOVERING->ANCHORED; once in ANCHORED a single bad reading drops
the FSM out (ANCHORED has zero hysteresis of its own), and SUSPICIOUS->
ANCHORED has no gate at all (CalculateNextState's SUSPICIOUS case returns
ANCHORED unconditionally when healthy). So a larger HYSTERESIS_WAIT only
delays reaching ANCHORED without making it stickier -- a real design
property worth stating explicitly.

Usage:
    go test ./tests/e2e/... -run TestE5_HysteresisSweep
    python3 scripts/e5_hysteresis_flapping/data_preprocessor.py
"""

import csv
import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))
from utils import setup_academic_plot_style, figsize_row, savefig_academic  # noqa: E402

import matplotlib.pyplot as plt  # noqa: E402

E2E_RESULTS_DIR = os.path.join(
    os.path.dirname(__file__), "..", "..", "tests", "e2e", "results"
)
CSV_IN = os.path.join(E2E_RESULTS_DIR, "e5_hysteresis_sweep.csv")
CSV_IN_5B = os.path.join(E2E_RESULTS_DIR, "e5b_down_hysteresis_sweep.csv")
CSV_IN_5C = os.path.join(E2E_RESULTS_DIR, "e5c_suspicious_exit_sweep.csv")
OUT_DIR = os.path.join(os.path.dirname(__file__), "results")

ENVIRONMENTS = ["stable", "noisy_btc", "noisy_da", "noisy_p2p", "combined_adversarial"]


def load_rows():
    with open(CSV_IN, newline="") as f:
        rows = list(csv.DictReader(f))
    for r in rows:
        r["hysteresis_wait"] = int(r["hysteresis_wait"])
        r["reached_anchored"] = r["reached_anchored"].lower() == "true"
        r["first_anchored_at"] = int(r["first_anchored_at"])
        r["flapping_count"] = int(r["flapping_count"])
        r["total_transitions"] = int(r["total_transitions"])
        r["anchored_uptime"] = float(r["anchored_uptime"])
    return rows


def plot_figure4(rows):
    setup_academic_plot_style()
    by_env = {
        env: sorted(
            [r for r in rows if r["environment"] == env],
            key=lambda r: r["hysteresis_wait"],
        )
        for env in ENVIRONMENTS
    }

    fig, axes = plt.subplots(1, 3, figsize=figsize_row(3))

    for env in ENVIRONMENTS:
        rs = by_env[env]
        axes[0].plot(
            [r["hysteresis_wait"] for r in rs],
            [r["anchored_uptime"] for r in rs],
            marker="o",
            label=env,
        )
    axes[0].set_xlabel("HYSTERESIS_WAIT")
    axes[0].set_ylabel("ANCHORED uptime (fraction of window)")
    axes[0].set_title("(A) Stability vs. HYSTERESIS_WAIT")
    axes[0].legend(fontsize=8)
    axes[0].set_ylim(-0.05, 1.05)

    for env in ENVIRONMENTS:
        rs = by_env[env]
        axes[1].plot(
            [r["hysteresis_wait"] for r in rs],
            [r["flapping_count"] for r in rs],
            marker="s",
            label=env,
        )
    axes[1].set_xlabel("HYSTERESIS_WAIT")
    axes[1].set_ylabel("Flapping count (window of 100 blocks)")
    axes[1].set_title("(B) Flapping vs. HYSTERESIS_WAIT")
    axes[1].legend(fontsize=8)

    for env in ENVIRONMENTS:
        rs = by_env[env]
        y = [
            r["first_anchored_at"] if r["first_anchored_at"] >= 0 else None for r in rs
        ]
        axes[2].plot([r["hysteresis_wait"] for r in rs], y, marker="^", label=env)
    axes[2].set_xlabel("HYSTERESIS_WAIT")
    axes[2].set_ylabel("Blocks to first reach ANCHORED (gap = never)")
    axes[2].set_title("(C) Time-to-First-Recovery vs. HYSTERESIS_WAIT")
    axes[2].legend(fontsize=8)

    fig.suptitle(
        "Figure 4 -- Recovery Stability vs. HYSTERESIS_WAIT (real tests/e2e data, 5 environments)"
    )
    fig.tight_layout()
    savefig_academic(fig, OUT_DIR, "figure4_hysteresis")


def load_rows_5b():
    with open(CSV_IN_5B, newline="") as f:
        rows = list(csv.DictReader(f))
    for r in rows:
        r["down_hysteresis_threshold"] = int(r["down_hysteresis_threshold"])
        r["anchored_uptime"] = float(r["anchored_uptime"])
    return rows


def load_rows_5c():
    with open(CSV_IN_5C, newline="") as f:
        rows = list(csv.DictReader(f))
    for r in rows:
        r["suspicious_hysteresis_wait"] = int(r["suspicious_hysteresis_wait"])
        r["anchored_uptime"] = float(r["anchored_uptime"])
        r["absorption_rate"] = float(r["absorption_rate"])
    return rows


def build_figure4b(rows_5a, rows_5b, rows_5c):
    """Figure 4b -- one figure, 3 subplots, one per hysteresis-gated edge,
    all real in-process tests/e2e data (fixed-seed noise -- the same data
    Figure 4/e5_table.md already report, reshaped for a side-by-side
    edge-to-edge comparison). Deliberately in-process, not live: this repo's
    own E5 finding is that the live curves are noisy/non-monotonic (real
    per-run network jitter can't cancel out the way a fixed seed does), so a
    live version of this trade-off figure wouldn't show the clean trends
    below -- see docs/EXPERIMENT.md's E5 "Live vs. in-process" note.
    """
    setup_academic_plot_style()
    fig, axes = plt.subplots(1, 3, figsize=(11, 3.6))

    # (a) 5a Up-hysteresis: only 3 visually distinct curves exist among the 5
    # environments (noisy_da==noisy_p2p, noisy_btc==combined_adversarial per
    # the real measured data -- see e5_table.md) -- one legend entry each,
    # not 5, so the 3-way comparison in (a)/(b)/(c) reads at a glance.
    rep_envs = [
        ("stable", "stable"),
        ("noisy_da", "noisy_da / noisy_p2p"),
        ("noisy_btc", "noisy_btc / combined_adversarial"),
    ]
    for env, label in rep_envs:
        rs = sorted([r for r in rows_5a if r["environment"] == env], key=lambda r: r["hysteresis_wait"])
        axes[0].plot(
            [r["hysteresis_wait"] for r in rs], [r["anchored_uptime"] for r in rs],
            marker="o", markersize=4, linewidth=1.6, label=label,
        )
    axes[0].set_xlabel("HYSTERESIS_WAIT", fontsize=9)
    axes[0].set_ylabel("ANCHORED uptime", fontsize=9)
    axes[0].set_title("(a) Up-Hysteresis\nRECOVERING -> ANCHORED", fontsize=9.5, fontweight="bold")
    axes[0].set_ylim(-0.05, 1.05)
    axes[0].legend(fontsize=6.5, loc="upper right")
    axes[0].tick_params(labelsize=7.5)

    # (b) 5b Down-hysteresis: all 4 environments give identical numbers at
    # every threshold (see e5b's Results table) -- a single real curve, no
    # legend needed.
    rs = sorted(
        [r for r in rows_5b if r["environment"] == rows_5b[0]["environment"]],
        key=lambda r: r["down_hysteresis_threshold"],
    )
    axes[1].plot(
        [r["down_hysteresis_threshold"] for r in rs], [r["anchored_uptime"] for r in rs],
        marker="o", markersize=4, linewidth=1.6, color="C2",
    )
    axes[1].set_xlabel("DownHysteresisThreshold", fontsize=9)
    axes[1].set_ylabel("ANCHORED uptime", fontsize=9)
    axes[1].set_title("(b) Down-Hysteresis\nANCHORED -> SUSPICIOUS", fontsize=9.5, fontweight="bold")
    axes[1].set_ylim(-0.05, 1.05)
    axes[1].tick_params(labelsize=7.5)

    # (c) 5c Suspicious-exit: AnchoredUptime (falling) vs. AbsorptionRate
    # (rising) on twin axes -- the real crossing exposes the trade-off
    # (absorbing more noise on this edge speeds up escalation to SOVEREIGN).
    rs = sorted(rows_5c, key=lambda r: r["suspicious_hysteresis_wait"])
    shw = [r["suspicious_hysteresis_wait"] for r in rs]
    l1, = axes[2].plot(shw, [r["anchored_uptime"] for r in rs], marker="o", markersize=4,
                        linewidth=1.6, color="C3", label="ANCHORED uptime")
    axes[2].set_xlabel("SuspiciousHysteresisWait", fontsize=9)
    axes[2].set_ylabel("ANCHORED uptime", color="C3", fontsize=9)
    axes[2].tick_params(axis="y", labelcolor="C3", labelsize=7.5)
    axes[2].tick_params(axis="x", labelsize=7.5)
    axes[2].set_ylim(-0.05, 1.05)
    ax2 = axes[2].twinx()
    l2, = ax2.plot(shw, [r["absorption_rate"] for r in rs], marker="s", markersize=4,
                    linewidth=1.6, color="C4", label="AbsorptionRate")
    ax2.set_ylabel("AbsorptionRate", color="C4", fontsize=9)
    ax2.tick_params(axis="y", labelcolor="C4", labelsize=7.5)
    ax2.set_ylim(-0.05, 1.05)
    axes[2].set_title("(c) Suspicious-Exit\nSUSPICIOUS -> ANCHORED", fontsize=9.5, fontweight="bold")
    axes[2].legend(handles=[l1, l2], fontsize=6.5, loc="center right")

    fig.suptitle(
        "Figure 4b -- FSM Hysteresis Parameter Sweeps Under Natural Noise",
        fontsize=12,
        fontweight="bold",
        y=0.99,
    )
    fig.text(
        0.5, 0.92,
        "(real tests/e2e in-process data, fixed-seed noise, one hysteresis-gated edge per panel)",
        ha="center", va="top", fontsize=8.5,
    )
    fig.subplots_adjust(top=0.68, bottom=0.16, wspace=0.55)
    savefig_academic(fig, OUT_DIR, "figure4b_hysteresis_tradeoffs")


def build_table(rows):
    lines = [
        "**E5 -- Hysteresis Sweep (measured, real tests/e2e data, 5 environments, 100-block window):**",
        "",
        "| HYSTERESIS_WAIT | Environment | Reached ANCHORED | First at (blocks) | Final State | Flapping | ANCHORED uptime |",
        "| ---: | --- | :---: | ---: | --- | ---: | ---: |",
    ]
    for r in sorted(
        rows, key=lambda r: (r["hysteresis_wait"], ENVIRONMENTS.index(r["environment"]))
    ):
        first_at = r["first_anchored_at"] if r["first_anchored_at"] >= 0 else "never"
        lines.append(
            f"| {r['hysteresis_wait']} | {r['environment']} | {'yes' if r['reached_anchored'] else 'NO'} "
            f"| {first_at} | {r['final_state']} | {r['flapping_count']} | {r['anchored_uptime']*100:.1f}% |"
        )
    return "\n".join(lines)


def main():
    if not os.path.exists(CSV_IN):
        print(
            f"{CSV_IN} not found -- run 'go test ./tests/e2e/... -run TestE5_HysteresisSweep' first.",
            file=sys.stderr,
        )
        sys.exit(1)

    rows = load_rows()
    plot_figure4(rows)
    table = build_table(rows)

    os.makedirs(OUT_DIR, exist_ok=True)
    with open(os.path.join(OUT_DIR, "e5_table.md"), "w") as f:
        f.write(table + "\n")

    print(table)
    print(f"\nFigure 4 written to {OUT_DIR}/figure4_hysteresis.{{png,pdf}}")

    if os.path.exists(CSV_IN_5B) and os.path.exists(CSV_IN_5C):
        build_figure4b(rows, load_rows_5b(), load_rows_5c())
        print(f"Figure 4b written to {OUT_DIR}/figure4b_hysteresis_tradeoffs.{{png,pdf}}")
    else:
        print(
            f"Skipping Figure 4b -- {CSV_IN_5B} and/or {CSV_IN_5C} not found "
            "(run TestE5b_DownHysteresisSweep / TestE5c_SuspiciousExitHysteresisSweep first).",
            file=sys.stderr,
        )


if __name__ == "__main__":
    main()
