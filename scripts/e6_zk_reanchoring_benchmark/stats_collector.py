"""
E6 -- Reanchoring Feasibility Evaluation: turns benchmark_prover.sh's raw
table6b_scaling.csv (real Noir + Barretenberg/UltraHonk measurements) into
docs/EXPERIMENT.md's Table 6A, Table 6B, and Figure 6.

Run after benchmark_prover.sh:
    python3 scripts/e6_zk_reanchoring_benchmark/stats_collector.py
"""

import csv
import os
import sys

import numpy as np

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))
from utils import (
    setup_academic_plot_style,
    figsize_grid,
    savefig_academic,
)  # noqa: E402

import matplotlib.pyplot as plt  # noqa: E402

RESULTS_DIR = os.path.dirname(__file__) + "/results"
CSV_IN = os.path.join(RESULTS_DIR, "table6b_scaling.csv")
TABLE_OUT = os.path.join(RESULTS_DIR, "table6a_6b.md")


def load_rows():
    with open(CSV_IN, newline="") as f:
        rows = list(csv.DictReader(f))
    for r in rows:
        for k in ("n", "acir_opcodes", "circuit_size", "proof_size_bytes"):
            r[k] = int(r[k])
        for k in ("compile_s", "execute_s", "prove_s", "verify_s"):
            r[k] = float(r[k])
    return rows


def build_table6a(rows):
    """
    Table 6A -- Circuit Composition.

    The circuit (circuit/reanchoring/src/main.nr) has no separate SMT
    inclusion/update subcircuit like docs/EXPERIMENT.md's original sketch
    (documented simplification: header-chain continuity via Poseidon2 stands
    in for a real SMT proof). Instead of an assumed static %-breakdown, this
    derives a real two-term cost model -- fixed overhead (Honk's lookup-table
    setup, independent of N) and marginal per-header cost -- via linear
    regression across the N points actually measured.
    """
    n = np.array([r["n"] for r in rows], dtype=float)
    opcodes = np.array([r["acir_opcodes"] for r in rows], dtype=float)
    slope, intercept = np.polyfit(n, opcodes, 1)
    r2 = np.corrcoef(n, opcodes)[0, 1] ** 2

    lines = [
        "**Table 6A -- Circuit Composition (measured, real nargo/bb):**",
        "",
        "| Component | Cost model | Fit |",
        "| --- | --- | --- |",
        f"| Fixed overhead (root binding + Honk setup) | {intercept:.1f} ACIR opcodes | R^2={r2:.4f} |",
        f"| Marginal cost per header (continuity + FSM legality + withdrawal lock) | {slope:.2f} ACIR opcodes/header | linear fit across N={int(n.min())}..{int(n.max())} |",
        "",
        f"| N (headers) | ACIR opcodes | Honk circuit size (padded) |",
        "| ---: | ---: | ---: |",
    ]
    for r in rows:
        lines.append(f"| {r['n']} | {r['acir_opcodes']} | {r['circuit_size']} |")
    return "\n".join(lines), slope, intercept


def build_table6b(rows):
    lines = [
        "**Table 6B -- Scaling Benchmark (measured, real nargo/bb, UltraHonk backend):**",
        "",
        "| Sovereign Blocks (N) | ACIR Opcodes | Compile (s) | Prove (s) | Verify (ms) | Proof Size | Blocks/s |",
        "| ---: | ---: | ---: | ---: | ---: | ---: | ---: |",
    ]
    for r in rows:
        blocks_per_s = r["n"] / r["prove_s"] if r["prove_s"] > 0 else float("inf")
        lines.append(
            f"| {r['n']} | {r['acir_opcodes']} | {r['compile_s']:.3f} | {r['prove_s']:.3f} "
            f"| {r['verify_s'] * 1000:.1f} | {r['proof_size_bytes']} B | {blocks_per_s:.1f} |"
        )
    return "\n".join(lines)


def build_figure6(rows):
    setup_academic_plot_style()
    n = [r["n"] for r in rows]
    fig, axes = plt.subplots(2, 2, figsize=figsize_grid(2, 2))

    axes[0, 0].plot(n, [r["acir_opcodes"] for r in rows], marker="o", markersize=5, linewidth=1.8)
    axes[0, 0].set_title("(A) Constraint Count", fontsize=10.5, fontweight="bold")
    axes[0, 0].set_xlabel("Sovereign Blocks (N)", fontsize=9)
    axes[0, 0].set_ylabel("ACIR Opcodes", fontsize=9)

    axes[0, 1].plot(n, [r["prove_s"] for r in rows], marker="o", color="C1", markersize=5, linewidth=1.8)
    axes[0, 1].set_title("(B) Proving Time", fontsize=10.5, fontweight="bold")
    axes[0, 1].set_xlabel("Sovereign Blocks (N)", fontsize=9)
    axes[0, 1].set_ylabel("Prove (s)", fontsize=9)

    axes[1, 0].plot(n, [r["verify_s"] * 1000 for r in rows], marker="o", color="C2", markersize=5, linewidth=1.8)
    axes[1, 0].set_title("(C) Verification Time", fontsize=10.5, fontweight="bold")
    axes[1, 0].set_xlabel("Sovereign Blocks (N)", fontsize=9)
    axes[1, 0].set_ylabel("Verify (ms)", fontsize=9)
    axes[1, 0].set_ylim(bottom=0)

    axes[1, 1].plot(n, [r["proof_size_bytes"] for r in rows], marker="o", color="C3", markersize=5, linewidth=1.8)
    axes[1, 1].set_title("(D) Proof Size", fontsize=10.5, fontweight="bold")
    axes[1, 1].set_xlabel("Sovereign Blocks (N)", fontsize=9)
    axes[1, 1].set_ylabel("Proof Size (bytes)", fontsize=9)
    axes[1, 1].set_ylim(0, max(r["proof_size_bytes"] for r in rows) * 1.5)

    for ax in axes.flat:
        ax.tick_params(labelsize=8)

    fig.suptitle(
        "Figure 6 -- Recovery Proof Scaling\n(Noir + UltraHonk, real measurements)",
        fontsize=12.5,
        fontweight="bold",
    )
    fig.tight_layout(rect=(0, 0, 1, 0.92))
    savefig_academic(fig, RESULTS_DIR, "figure6_scaling")


def main():
    rows = load_rows()
    table6a, slope, intercept = build_table6a(rows)
    table6b = build_table6b(rows)
    build_figure6(rows)

    with open(TABLE_OUT, "w") as f:
        f.write(table6a + "\n\n" + table6b + "\n")

    print(table6a)
    print()
    print(table6b)
    print()
    print(f"Figure 6 written to {RESULTS_DIR}/figure6_scaling.{{pdf,png}}")
    print(f"Tables written to {TABLE_OUT}")


if __name__ == "__main__":
    main()
