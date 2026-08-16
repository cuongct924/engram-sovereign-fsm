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

Figure 4 combines all 3 hysteresis-gated edges (5a/5b/5c) in one figure,
in-process sweep as solid lines (fast, dense, fixed-seed -- reveals the clean
mathematical trend) with real live spot-check points overlaid as scatter dots
(sparse, real 4-node Docker runs). The two deliberately don't always agree --
see docs/EXPERIMENT.md's E5 "Live vs. in-process" note, this repo's own
finding that live noise doesn't cancel out the way a fixed seed does.

Usage:
    go test ./tests/e2e/... -run 'TestE5_HysteresisSweep|TestE5b_DownHysteresisSweep|TestE5c_SuspiciousExitHysteresisSweep'
    python3 scripts/e5_hysteresis_flapping/data_preprocessor.py
"""

import csv
import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))
from utils import setup_academic_plot_style, savefig_academic  # noqa: E402

import matplotlib.pyplot as plt  # noqa: E402
import matplotlib.lines as mlines  # noqa: E402

E2E_RESULTS_DIR = os.path.join(
    os.path.dirname(__file__), "..", "..", "tests", "e2e", "results"
)
CSV_IN = os.path.join(E2E_RESULTS_DIR, "e5_hysteresis_sweep.csv")
CSV_IN_5B = os.path.join(E2E_RESULTS_DIR, "e5b_down_hysteresis_sweep.csv")
CSV_IN_5C = os.path.join(E2E_RESULTS_DIR, "e5c_suspicious_exit_sweep.csv")
OUT_DIR = os.path.join(os.path.dirname(__file__), "results")

ENVIRONMENTS = ["stable", "noisy_btc", "noisy_da", "noisy_p2p", "combined_adversarial"]

# Real live 4-node Docker spot-check points (300s/run), already measured and
# reported in docs/EXPERIMENT.md's E5 "Live spot-check" tables -- transcribed
# here, not re-derived, so the scatter dots below match the doc verbatim.
# Sparse by construction (live is slow; in-process is the dense sweep).
LIVE_5A = {  # HYSTERESIS_WAIT -> {env: AnchoredUptime}
    0: {"stable": 1.0000, "noisy_da": 0.1170},
    2: {"stable": 1.0000, "noisy_da": 0.1529},
    5: {"stable": 1.0000, "noisy_da": 0.1875},
    10: {"stable": 1.0000, "noisy_da": 0.0106},
    20: {"stable": 1.0000, "noisy_da": 0.0104},
}
LIVE_5B = {  # DownHysteresisThreshold -> {env: AnchoredUptime}
    1: {"stable": 0.7368, "noisy_da": 0.1348},
    2: {"stable": 1.0000, "noisy_da": 0.0423},
    4: {"stable": 1.0000, "noisy_da": 0.3000},
    6: {"stable": 1.0000, "noisy_da": 0.3646},
    8: {"stable": 1.0000, "noisy_da": 0.2824},
}
LIVE_5C = {  # SuspiciousHysteresisWait -> AnchoredUptime, "20% Healthy Blips" only
    # ("Sustained Warning" is flat 0% at every SHW by construction -- zero
    # healthy blips ever, so it never exercises the exit timer this sweep is
    # about; excluded here as uninformative for this trade-off, see
    # docs/EXPERIMENT.md's E5c conclusion).
    1: 0.1875,
    2: 0.3646,
    4: 0.1250,
    6: 0.1579,
    8: 0.1711,
}

# Real DefaultParams() values (x/sovereignty/types/params.go) used by every
# live deployment outside this section's own sweeps -- the "Live Anchor" for
# each panel. All three happen to default to 2.
LIVE_HYSTERESIS_WAIT = 2
LIVE_DOWN_HYSTERESIS_THRESHOLD = 2
LIVE_SUSPICIOUS_HYSTERESIS_WAIT = 2

# Okabe-Ito colorblind-safe palette (same convention as this session's other
# E9/E10 figures) -- higher contrast than matplotlib's default C0-C4 cycle,
# easier to tell apart at a glance.
COLOR_STABLE = "#0072B2"      # blue
COLOR_NOISY_DA = "#E69F00"    # orange
COLOR_NOISY_BTC = "#D55E00"   # vermillion
COLOR_NOISY_B = "#009E73"     # green (panel b's single "noisy" curve)
COLOR_UPTIME = "#D55E00"      # vermillion (panel c, distinct panel from noisy_btc)
COLOR_ABSORPTION = "#CC79A7"  # reddish purple


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


def build_figure4(rows_5a, rows_5b, rows_5c):
    """Figure 4 -- one figure, 3 subplots, one per hysteresis-gated edge.
    Solid lines: real in-process tests/e2e data (fixed-seed noise, dense
    sweep). Scatter dots: real live 4-node Docker spot-checks (sparse,
    LIVE_5A/5B/5C above). The two deliberately don't track each other
    closely -- this repo's own E5 finding is that live noise doesn't cancel
    out the way a fixed seed does, so the divergence itself is the result,
    not a plotting artifact -- see docs/EXPERIMENT.md's E5 "Live vs.
    in-process" note.
    """
    setup_academic_plot_style()
    # Taller than a typical 3-panel row: each panel reserves real headroom
    # above y=1.0 for its in-axes legend (see below), so the figure needs
    # more vertical room for the actual data to still read clearly rather
    # than being squeezed into a small fraction of a short panel.
    fig, axes = plt.subplots(1, 3, figsize=(11, 4.3))

    # (a) 5a Up-hysteresis: only 3 visually distinct curves exist among the 5
    # environments (noisy_da==noisy_p2p, noisy_btc==combined_adversarial per
    # the real measured data -- see e5_table.md) -- one legend entry each,
    # not 5, so the 3-way comparison in (a)/(b)/(c) reads at a glance.
    rep_envs = [
        ("stable", "stable", COLOR_STABLE),
        ("noisy_da", "noisy_da / noisy_p2p", COLOR_NOISY_DA),
        ("noisy_btc", "noisy_btc / combined_adversarial", COLOR_NOISY_BTC),
    ]
    for env, label, color in rep_envs:
        rs = sorted([r for r in rows_5a if r["environment"] == env], key=lambda r: r["hysteresis_wait"])
        axes[0].plot(
            [r["hysteresis_wait"] for r in rs], [r["anchored_uptime"] for r in rs],
            marker="o", markersize=4, linewidth=1.6, label=label, color=color,
        )
        if env in ("stable", "noisy_da"):
            xs = sorted(LIVE_5A)
            axes[0].scatter(xs, [LIVE_5A[x][env] for x in xs], marker="D", s=26,
                             color=color, edgecolor="black", linewidth=1.0, zorder=5)
    axes[0].axvline(LIVE_HYSTERESIS_WAIT, color="0.35", linestyle="--", linewidth=1.1, zorder=1)
    axes[0].set_xlabel("HysteresisWait", fontsize=9)
    axes[0].set_ylabel("ANCHORED uptime", fontsize=9)
    axes[0].set_title("(a) Up-Hysteresis:\nMitigating Flapping", fontsize=9.5, fontweight="bold")
    # ylim extends well past the real data ceiling (1.0) to carve out
    # reserved headroom for the legend -- real data never enters y>1.05, so
    # anything placed at y>1.1 can never touch a line/marker, in-panel,
    # without hunting for an empty pocket among the data itself.
    axes[0].set_ylim(-0.05, 2.05)
    axes[0].set_yticks([0.0, 0.2, 0.4, 0.6, 0.8, 1.0])
    # framealpha=1 + high zorder: a legend box's default semi-transparent
    # face blends with any line passing underneath it instead of cleanly
    # occluding it -- looks like a rendering glitch (a line fading to
    # near-invisible) rather than an intentional opaque box on top.
    # One handle per live-plotted color, not a single generic gray diamond --
    # a neutral swatch next to two differently-colored on-chart diamonds
    # (blue/orange) read as a mismatch, not "shape=live, color=series".
    live_stable0 = mlines.Line2D([], [], marker="D", markersize=5, linestyle="None",
                                  markerfacecolor=COLOR_STABLE, markeredgecolor="black", label="stable (live)")
    live_noisy0 = mlines.Line2D([], [], marker="D", markersize=5, linestyle="None",
                                 markerfacecolor=COLOR_NOISY_DA, markeredgecolor="black", label="noisy_da (live)")
    live_default0 = mlines.Line2D([], [], color="0.35", linestyle="--", linewidth=1.1,
                                   label=f"Live default (HW={LIVE_HYSTERESIS_WAIT})")
    handles0, labels0 = axes[0].get_legend_handles_labels()
    leg0 = axes[0].legend(handles=handles0 + [live_stable0, live_noisy0, live_default0],
                           fontsize=6.5, loc="upper center", framealpha=1.0,
                           handlelength=1.3, labelspacing=0.3, borderpad=0.3)
    leg0.set_zorder(10)
    axes[0].tick_params(labelsize=7.5)

    # (b) 5b Down-hysteresis: all 4 warning-level environments give identical
    # numbers at every threshold (see e5b's Results table) -- one real noisy
    # curve, plus the real "stable" no-noise control (TestE5b_..._Stable,
    # added specifically so this panel has a measured baseline, not an
    # assumed one).
    rs_stable = sorted([r for r in rows_5b if r["environment"] == "stable"], key=lambda r: r["down_hysteresis_threshold"])
    # rows_5b's non-stable environments are all numerically identical; pick
    # the first non-stable environment name present for a clean single curve.
    noisy_env_name = next(r["environment"] for r in rows_5b if r["environment"] != "stable")
    rs_noisy = sorted([r for r in rows_5b if r["environment"] == noisy_env_name], key=lambda r: r["down_hysteresis_threshold"])
    axes[1].plot(
        [r["down_hysteresis_threshold"] for r in rs_stable], [r["anchored_uptime"] for r in rs_stable],
        marker="o", markersize=4, linewidth=1.6, color=COLOR_STABLE, label="stable",
    )
    axes[1].plot(
        [r["down_hysteresis_threshold"] for r in rs_noisy], [r["anchored_uptime"] for r in rs_noisy],
        marker="o", markersize=4, linewidth=1.6, color=COLOR_NOISY_B, label="noisy (all 4 warning envs)",
    )
    for env, color in (("stable", COLOR_STABLE), ("noisy_da", COLOR_NOISY_B)):
        xs = sorted(LIVE_5B)
        axes[1].scatter(xs, [LIVE_5B[x][env] for x in xs], marker="D", s=26,
                         color=color, edgecolor="black", linewidth=1.0, zorder=5)
    axes[1].axvline(LIVE_DOWN_HYSTERESIS_THRESHOLD, color="0.35", linestyle="--", linewidth=1.1, zorder=1)
    axes[1].set_xlabel("DownHysteresisThreshold", fontsize=9)
    axes[1].set_ylabel("ANCHORED uptime", fontsize=9)
    axes[1].set_title("(b) Down-Hysteresis:\nNoise Absorption", fontsize=9.5, fontweight="bold")
    # Reserved headroom above the real data ceiling (1.0), same technique as
    # panel (a) -- see its comment for why.
    axes[1].set_ylim(-0.05, 2.05)
    axes[1].set_yticks([0.0, 0.2, 0.4, 0.6, 0.8, 1.0])
    live_stable1 = mlines.Line2D([], [], marker="D", markersize=5, linestyle="None",
                                  markerfacecolor=COLOR_STABLE, markeredgecolor="black", label="stable (live)")
    live_noisy1 = mlines.Line2D([], [], marker="D", markersize=5, linestyle="None",
                                 markerfacecolor=COLOR_NOISY_B, markeredgecolor="black", label="noisy_da (live)")
    live_default1 = mlines.Line2D([], [], color="0.35", linestyle="--", linewidth=1.1,
                                   label=f"Live default (DHT={LIVE_DOWN_HYSTERESIS_THRESHOLD})")
    handles1, labels1 = axes[1].get_legend_handles_labels()
    leg1 = axes[1].legend(handles=handles1 + [live_stable1, live_noisy1, live_default1],
                           fontsize=6.5, loc="upper center", framealpha=1.0,
                           handlelength=1.3, labelspacing=0.3, borderpad=0.3)
    leg1.set_zorder(10)
    axes[1].tick_params(labelsize=7.5)

    # (c) 5c Suspicious-exit: AnchoredUptime (falling) vs. AbsorptionRate
    # (rising) on twin axes -- the real crossing exposes the trade-off
    # (absorbing more noise on this edge speeds up escalation to SOVEREIGN).
    # Live dots only on AnchoredUptime -- live spot-checks never computed an
    # AbsorptionRate-equivalent metric, so the right axis stays in-process
    # only (not fabricated to match).
    rs = sorted(rows_5c, key=lambda r: r["suspicious_hysteresis_wait"])
    shw = [r["suspicious_hysteresis_wait"] for r in rs]
    l1, = axes[2].plot(shw, [r["anchored_uptime"] for r in rs], marker="o", markersize=4,
                        linewidth=1.6, color=COLOR_UPTIME, label="ANCHORED uptime")
    live_xs = sorted(LIVE_5C)
    l1d = axes[2].scatter(live_xs, [LIVE_5C[x] for x in live_xs], marker="D", s=26,
                           color=COLOR_UPTIME, edgecolor="black", linewidth=1.0, zorder=5,
                           label="ANCHORED uptime (live)")
    axes[2].set_xlabel("SuspiciousHysteresisWait", fontsize=9)
    axes[2].set_ylabel("ANCHORED uptime", color=COLOR_UPTIME, fontsize=9)
    axes[2].tick_params(axis="y", labelcolor=COLOR_UPTIME, labelsize=7.5)
    axes[2].tick_params(axis="x", labelsize=7.5)
    # Reserved headroom above the real data ceiling (1.0), same technique as
    # panels (a)/(b) -- both twin axes extended together so their gridlines/
    # tick rows stay aligned.
    axes[2].set_ylim(-0.05, 2.05)
    axes[2].set_yticks([0.0, 0.2, 0.4, 0.6, 0.8, 1.0])
    ax2 = axes[2].twinx()
    l2, = ax2.plot(shw, [r["absorption_rate"] for r in rs], marker="s", markersize=4,
                    linewidth=1.6, color=COLOR_ABSORPTION, label="AbsorptionRate")
    ax2.set_ylabel("AbsorptionRate", color=COLOR_ABSORPTION, fontsize=9)
    ax2.tick_params(axis="y", labelcolor=COLOR_ABSORPTION, labelsize=7.5)
    ax2.set_ylim(-0.05, 2.05)
    ax2.set_yticks([0.0, 0.2, 0.4, 0.6, 0.8, 1.0])
    ax2.grid(False)

    # Live Anchor: the real SuspiciousHysteresisWait value every live
    # deployment outside this sweep actually runs with (DefaultParams()) --
    # labeled in the legend below ("Live default (SHW=2)"), not a separate
    # in-panel annotation.
    axes[2].axvline(LIVE_SUSPICIOUS_HYSTERESIS_WAIT, color="0.35", linestyle="--", linewidth=1.1, zorder=1)

    axes[2].set_title("(c) Suspicious-Exit:\nThe Tuning Trade-off", fontsize=9.5, fontweight="bold")
    live_anchor_handle = mlines.Line2D([], [], color="0.35", linestyle="--", linewidth=1.1, label="Live default (SHW=2)")
    leg2 = axes[2].legend(handles=[l1, l2, l1d, live_anchor_handle], fontsize=6.5,
                           loc="upper center", framealpha=1.0,
                           handlelength=1.3, labelspacing=0.3, borderpad=0.3)
    leg2.set_zorder(10)

    fig.suptitle(
        "Figure 4: FSM Hysteresis Parameter Sweeps and\nSystem Trade-offs Across Fault Environments",
        fontsize=12,
        fontweight="bold",
        y=0.99,
    )
    fig.text(
        0.5, 0.90,
        "lines = in-process sweep; diamonds = real live spot-checks (divergence is real, not noise); "
        "dashed line = real production default (=2)",
        ha="center", va="top", fontsize=8,
    )
    fig.subplots_adjust(top=0.78, bottom=0.13, wspace=0.55)
    savefig_academic(fig, OUT_DIR, "figure4_hysteresis_tradeoffs")


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
    if not (os.path.exists(CSV_IN_5B) and os.path.exists(CSV_IN_5C)):
        print(
            f"{CSV_IN_5B} and/or {CSV_IN_5C} not found -- run "
            "'go test ./tests/e2e/... -run TestE5b_DownHysteresisSweep|TestE5c_SuspiciousExitHysteresisSweep' first.",
            file=sys.stderr,
        )
        sys.exit(1)

    rows = load_rows()
    table = build_table(rows)

    os.makedirs(OUT_DIR, exist_ok=True)
    with open(os.path.join(OUT_DIR, "e5_table.md"), "w") as f:
        f.write(table + "\n")

    print(table)

    build_figure4(rows, load_rows_5b(), load_rows_5c())
    print(f"\nFigure 4 written to {OUT_DIR}/figure4_hysteresis_tradeoffs.{{png,pdf}}")


if __name__ == "__main__":
    main()
