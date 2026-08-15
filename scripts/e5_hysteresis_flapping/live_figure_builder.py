#!/usr/bin/env python3
"""E5 -- Hysteresis and Flapping Sensitivity, LIVE-docker Figure 4
(docs/EXPERIMENT.md's E5), built from live_spot_check.py's real output
against the real 4-node testnet (not tests/e2e's in-process mock).

Purpose (see live_spot_check.py's doc): confirm the in-process sweep's real
finding -- flapping_count increases and anchored_uptime doesn't improve as
HysteresisWait grows under sustained noise, no interior sweet spot -- holds
under REAL consensus timing too. A small 2x2 spot-check (HysteresisWait in
{2, 10} x {stable, noisy_da}), not a full 6x5 sweep: each combo needed its
own params.go edit + image rebuild + fresh redeploy.

Reads each run's *_summary.md (real per-node metrics written by
live_spot_check.py) rather than recomputing from raw CSVs -- avoids
duplicating compute_metrics, and every node's numbers have agreed in every
run so far.

Usage:
    python3 scripts/e5_hysteresis_flapping/live_figure_builder.py
"""

import glob
import os
import re
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))
from utils import (  # noqa: E402
    setup_academic_plot_style,
    figsize_row,
    savefig_academic,
)

import matplotlib.pyplot as plt  # noqa: E402

RESULTS_LIVE_DIR = os.path.join(os.path.dirname(__file__), "results_live")
OUT_DIR = os.path.join(os.path.dirname(__file__), "results")

ENVIRONMENTS = ["stable", "noisy_da"]


def find_summary_paths():
    """One *_summary.md per (hw, env) combo -- if a combo was run more than
    once, keep only the LATEST by filename timestamp, matching this repo's
    results_live/ curation convention (see docs/DEVELOPMENT.md's results/
    vs results_live/ note).
    """
    paths = sorted(
        glob.glob(os.path.join(RESULTS_LIVE_DIR, "live_spot_check_hw*_summary.md"))
    )
    by_combo = {}
    pattern = re.compile(r"live_spot_check_hw(\d+)_(stable|noisy_da)_(\d+T\d+)")
    for p in paths:
        m = pattern.search(os.path.basename(p))
        if not m:
            continue
        hw, env, ts = int(m.group(1)), m.group(2), m.group(3)
        key = (hw, env)
        if key not in by_combo or ts > by_combo[key][1]:
            by_combo[key] = (p, ts)
    return {k: v[0] for k, v in by_combo.items()}


def parse_summary(path):
    """Averages the per-node table in a *_summary.md across all 4 nodes
    (real numbers this script itself wrote; every run so far has had all
    4 nodes agree exactly, but averaging is the honest general case).
    """
    rows = []
    with open(path) as f:
        in_table = False
        for line in f:
            if line.startswith("| Node "):
                in_table = True
                continue
            if in_table:
                if not line.startswith("|"):
                    break
                if line.startswith("|---"):
                    continue
                cols = [c.strip() for c in line.strip().strip("|").split("|")]
                if len(cols) != 5:
                    continue
                _, samples, transitions, flapping, uptime = cols
                rows.append(
                    {
                        "samples": int(samples),
                        "transitions": int(transitions),
                        "flapping": int(flapping),
                        "anchored_uptime": float(uptime.rstrip("%")) / 100.0,
                    }
                )
    if not rows:
        return {"transitions": 0, "flapping": 0, "anchored_uptime": 0.0}
    n = len(rows)
    return {
        "transitions": sum(r["transitions"] for r in rows) / n,
        "flapping": sum(r["flapping"] for r in rows) / n,
        "anchored_uptime": sum(r["anchored_uptime"] for r in rows) / n,
    }


def main():
    combo_paths = find_summary_paths()
    if not combo_paths:
        print(
            f"No live_spot_check_hw*_summary.md found in {RESULTS_LIVE_DIR} -- run "
            "scripts/e5_hysteresis_flapping/live_spot_check.py for each combo first.",
            file=sys.stderr,
        )
        sys.exit(1)

    hw_values = sorted(set(hw for hw, _ in combo_paths))
    data = {
        env: {
            hw: parse_summary(combo_paths[(hw, env)])
            for hw in hw_values
            if (hw, env) in combo_paths
        }
        for env in ENVIRONMENTS
    }

    setup_academic_plot_style()
    # Extra height beyond figsize_row's default (tuned for 3 narrow
    # subplots) -- 2 wider subplots with a 2-line suptitle and rotated
    # y-axis labels need more vertical room, and a shorter label text below
    # avoids the left-edge clipping: a longer, full-sentence y-label
    # overlaps both the suptitle above and the canvas's own left edge.
    width, height = figsize_row(2)
    fig, axes = plt.subplots(1, 2, figsize=(width, height * 1.4))

    for env in ENVIRONMENTS:
        hws = sorted(data[env].keys())
        axes[0].plot(
            hws, [data[env][hw]["anchored_uptime"] for hw in hws], marker="o", label=env
        )
    axes[0].set_xlabel("HYSTERESIS_WAIT")
    axes[0].set_ylabel("ANCHORED uptime (fraction)")
    axes[0].set_title("(A) Stability vs. HYSTERESIS_WAIT", fontsize=11)
    axes[0].set_ylim(-0.05, 1.05)
    axes[0].legend(fontsize=8)
    axes[0].set_xticks(hw_values)

    for env in ENVIRONMENTS:
        hws = sorted(data[env].keys())
        axes[1].plot(
            hws, [data[env][hw]["flapping"] for hw in hws], marker="s", label=env
        )
    axes[1].set_xlabel("HYSTERESIS_WAIT")
    axes[1].set_ylabel("Flapping count (300s window)")
    axes[1].set_title("(B) Flapping vs. HYSTERESIS_WAIT", fontsize=11)
    axes[1].legend(fontsize=8)
    axes[1].set_xticks(hw_values)

    fig.suptitle(
        "Figure 4 -- Recovery Stability vs. HYSTERESIS_WAIT\n"
        "(live 4-node Docker testnet, real spot-check, 2 HW values x 2 environments)",
        fontsize=11,
        y=0.98,
    )
    fig.tight_layout(rect=(0.02, 0, 0.98, 0.85))
    savefig_academic(fig, OUT_DIR, "figure4_hysteresis_live")

    print(f"Loaded {len(combo_paths)} real live combos from {RESULTS_LIVE_DIR}")
    for (hw, env), path in sorted(combo_paths.items()):
        d = parse_summary(path)
        print(
            f"  HW={hw} env={env}: anchored_uptime={d['anchored_uptime']:.2%} "
            f"flapping={d['flapping']:.0f} transitions={d['transitions']:.0f}"
        )
    print(f"Figure written to {OUT_DIR}/figure4_hysteresis_live.{{png,pdf}}")


if __name__ == "__main__":
    main()
