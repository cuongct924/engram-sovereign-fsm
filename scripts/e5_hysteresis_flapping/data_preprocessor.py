#!/usr/bin/env python3
"""
E5 -- Hysteresis and Flapping Sensitivity (docs/EXPERIMENT.md's E5, Figure 4).

Consumes REAL data from tests/e2e/results/e5_hysteresis_sweep.csv, produced
by `go test ./tests/e2e/... -run TestE5_HysteresisSweep`, which sweeps
HysteresisWait in {0,1,3,5,10,20} through the real Harness/BeginBlocker path
under 5 environments: stable (no noise, control group), and 4 noisy
environments (noisy_btc, noisy_da, noisy_p2p, combined_adversarial) each with
a per-block 20% chance of a 1-block disturbance, using a fixed RNG seed
shared across all HysteresisWait values so the "weather" is identical and
only HysteresisWait's filtering differs.

Real finding (this DOES differ from docs/EXPERIMENT.md's aspirational
"HYSTERESIS_WAIT=3-5 is the sweet spot" claim -- reported as measured, not
adjusted to match): under this sustained-noise model, anchored_uptime
decreases MONOTONICALLY as HYSTERESIS_WAIT increases for noisy_btc (0.60 at
HW=0 down to 0.00 at HW=20) -- there is no interior sweet spot. The reason is
architectural, not a tuning artifact: HysteresisSafety's HYSTERESIS_WAIT gate
only applies to the RECOVERING->ANCHORED transition; once in ANCHORED, a
single bad reading immediately drops the FSM out (ANCHORED has zero
hysteresis protection of its own), and SUSPICIOUS->ANCHORED also has no
hysteresis gate at all (CalculateNextState's SUSPICIOUS case: `if healthy {
return ANCHORED }`, unconditional). So a larger HYSTERESIS_WAIT only delays
reaching ANCHORED without making it any stickier once there -- this is a real
design property worth stating explicitly, not a bug in the experiment.

Usage:
    go test ./tests/e2e/... -run TestE5_HysteresisSweep
    python3 scripts/e5_hysteresis_flapping/data_preprocessor.py
"""

import csv
import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))
from utils import setup_academic_plot_style  # noqa: E402

import matplotlib.pyplot as plt  # noqa: E402

E2E_RESULTS_DIR = os.path.join(os.path.dirname(__file__), "..", "..", "tests", "e2e", "results")
CSV_IN = os.path.join(E2E_RESULTS_DIR, "e5_hysteresis_sweep.csv")
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
    by_env = {env: sorted([r for r in rows if r["environment"] == env], key=lambda r: r["hysteresis_wait"]) for env in ENVIRONMENTS}

    fig, axes = plt.subplots(1, 3, figsize=(15, 4.5))

    for env in ENVIRONMENTS:
        rs = by_env[env]
        axes[0].plot([r["hysteresis_wait"] for r in rs], [r["anchored_uptime"] for r in rs], marker="o", label=env)
    axes[0].set_xlabel("HYSTERESIS_WAIT")
    axes[0].set_ylabel("ANCHORED uptime (fraction of window)")
    axes[0].set_title("(A) Stability vs. HYSTERESIS_WAIT")
    axes[0].legend(fontsize=8)
    axes[0].set_ylim(-0.05, 1.05)

    for env in ENVIRONMENTS:
        rs = by_env[env]
        axes[1].plot([r["hysteresis_wait"] for r in rs], [r["flapping_count"] for r in rs], marker="s", label=env)
    axes[1].set_xlabel("HYSTERESIS_WAIT")
    axes[1].set_ylabel("Flapping count (window of 100 blocks)")
    axes[1].set_title("(B) Flapping vs. HYSTERESIS_WAIT")
    axes[1].legend(fontsize=8)

    for env in ENVIRONMENTS:
        rs = by_env[env]
        y = [r["first_anchored_at"] if r["first_anchored_at"] >= 0 else None for r in rs]
        axes[2].plot([r["hysteresis_wait"] for r in rs], y, marker="^", label=env)
    axes[2].set_xlabel("HYSTERESIS_WAIT")
    axes[2].set_ylabel("Blocks to first reach ANCHORED (gap = never)")
    axes[2].set_title("(C) Time-to-First-Recovery vs. HYSTERESIS_WAIT")
    axes[2].legend(fontsize=8)

    fig.suptitle("Figure 4 -- Recovery Stability vs. HYSTERESIS_WAIT (real tests/e2e data, 5 environments)")
    fig.tight_layout()
    os.makedirs(OUT_DIR, exist_ok=True)
    fig.savefig(os.path.join(OUT_DIR, "figure4_hysteresis.pdf"))
    fig.savefig(os.path.join(OUT_DIR, "figure4_hysteresis.png"), dpi=150)


def build_table(rows):
    lines = [
        "**E5 -- Hysteresis Sweep (measured, real tests/e2e data, 5 environments, 100-block window):**",
        "",
        "| HYSTERESIS_WAIT | Environment | Reached ANCHORED | First at (blocks) | Final State | Flapping | ANCHORED uptime |",
        "| ---: | --- | :---: | ---: | --- | ---: | ---: |",
    ]
    for r in sorted(rows, key=lambda r: (r["hysteresis_wait"], ENVIRONMENTS.index(r["environment"]))):
        first_at = r["first_anchored_at"] if r["first_anchored_at"] >= 0 else "never"
        lines.append(
            f"| {r['hysteresis_wait']} | {r['environment']} | {'yes' if r['reached_anchored'] else 'NO'} "
            f"| {first_at} | {r['final_state']} | {r['flapping_count']} | {r['anchored_uptime']*100:.1f}% |"
        )
    return "\n".join(lines)


def main():
    if not os.path.exists(CSV_IN):
        print(f"{CSV_IN} not found -- run 'go test ./tests/e2e/... -run TestE5_HysteresisSweep' first.", file=sys.stderr)
        sys.exit(1)

    rows = load_rows()
    plot_figure4(rows)
    table = build_table(rows)

    os.makedirs(OUT_DIR, exist_ok=True)
    with open(os.path.join(OUT_DIR, "e5_table.md"), "w") as f:
        f.write(table + "\n")

    print(table)
    print(f"\nFigure 4 written to {OUT_DIR}/figure4_hysteresis.{{png,pdf}}")


if __name__ == "__main__":
    main()
