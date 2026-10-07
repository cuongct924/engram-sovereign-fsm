#!/usr/bin/env python3
"""Flaps against detection delay for the live three-phase runs (results_live/nonstationary_*.json).

One point per configuration: x is the detection delay after the noise at offset 0 (mean, bars span the three runs), y the flaps of
the noisy phase (mean over the six runs of both offsets). Lower and to the left is better. The static thresholds sit on the
frontier; the adaptive configurations lie to the right of the static k=4 point at every flap count.

    .venv/bin/python scripts/e5_hysteresis_flapping/plot_live_tradeoff.py
"""

import glob
import json
import os
import re
import shutil
import statistics as st
import sys
from collections import defaultdict

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(HERE, ".."))
from utils import savefig_academic, setup_academic_plot_style  # noqa: E402

import matplotlib.pyplot as plt  # noqa: E402

SRC = os.path.join(HERE, "results_live")
OUT = os.path.join(HERE, "results")
PAPER = os.path.join(HERE, "..", "..", "docs", "paper", "figures")
LABELS = {"static2": "static $k{=}2$", "static4": "static $k{=}4$", "adaptiveW16": "adaptive, window 16",
          "adaptiveW64": "adaptive, window 64", "adaptiveW128": "adaptive, window 128", "adaptiveK4W64": "adaptive, cap 4, window 64"}
STATIC = {"static2", "static4"}


def main():
    runs = defaultdict(list)
    for f in glob.glob(os.path.join(SRC, "nonstationary_*_o*_t*_*.json")):
        m = re.match(r"nonstationary_(.+)_o(\d+)_t\d+_", os.path.basename(f))
        runs[(m.group(1), int(m.group(2)))].append(json.load(open(f)))
    setup_academic_plot_style()
    plt.rcParams.update({"font.size": 9, "axes.labelsize": 10, "legend.fontsize": 8, "xtick.labelsize": 9, "ytick.labelsize": 9})
    fig, ax = plt.subplots(figsize=(5.2, 3.2))
    for cfg, label in LABELS.items():
        r0 = runs.get((cfg, 0), [])
        allr = r0 + runs.get((cfg, 60), [])
        if not r0:
            continue
        d = [r["detection_delay_s"] for r in r0]
        flaps = st.mean(r["noisy_flaps"] for r in allr)
        mean = st.mean(d)
        ax.errorbar(mean, flaps, xerr=[[mean - min(d)], [max(d) - mean]], fmt="s" if cfg in STATIC else "o", ms=6,
                    color="black" if cfg in STATIC else "tab:orange", ecolor="0.5", capsize=2, lw=1.2)
        dy = -13 if cfg == "adaptiveK4W64" else 4
        ax.annotate(label, (mean, flaps), textcoords="offset points", xytext=(6, dy), fontsize=8)
    ax.set_xlabel("Detection delay after the noise, offset 0 (s)")
    ax.set_ylabel("Flaps in the noisy phase")
    ax.set_xlim(10, 21)
    ax.set_ylim(-0.5, 8.5)
    fig.tight_layout()
    savefig_academic(fig, OUT, "figure_live_tradeoff")
    os.makedirs(PAPER, exist_ok=True)
    for ext in ("pdf", "png"):
        shutil.copy(os.path.join(OUT, f"figure_live_tradeoff.{ext}"), os.path.join(PAPER, f"figure_live_tradeoff.{ext}"))
    print("wrote figure_live_tradeoff")


if __name__ == "__main__":
    main()
