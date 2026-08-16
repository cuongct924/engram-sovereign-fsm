"""
E6 -- Reanchoring Feasibility Evaluation: Table 6C / Figure 7 (docs/EXPERIMENT.md,
optional backend comparison: Noir+Barretenberg/UltraHonk vs Plonky3).

Combines the real Noir measurements already in table6b_scaling.csv with
benchmark_plonky3.sh's real table6c_plonky3.csv (both scripts must have been
run first) into results/table6c_backend_comparison.md + figure7_backend_tradeoff.{png,pdf}.

Run after benchmark_plonky3.sh:
    python3 scripts/e6_zk_reanchoring_benchmark/table6c_collector.py
"""

import csv
import math
import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))
from utils import setup_academic_plot_style, figsize_row, savefig_academic  # noqa: E402

import matplotlib.pyplot as plt  # noqa: E402
import matplotlib.ticker as mticker  # noqa: E402

RESULTS_DIR = os.path.dirname(__file__) + "/results"
NOIR_CSV = os.path.join(RESULTS_DIR, "table6b_scaling.csv")
PLONKY3_CSV = os.path.join(RESULTS_DIR, "table6c_plonky3.csv")
TABLE_OUT = os.path.join(RESULTS_DIR, "table6c_backend_comparison.md")

# Representative N for the headline comparison numbers -- the largest N both
# sides actually measured (256), matching docs/EXPERIMENT.md's Table 6C style
# of quoting single representative numbers per backend rather than a curve.
REPRESENTATIVE_N = 256


def _compact_num(v, pos=None):
    """K/M-suffixed for values >=1000 (never scientific "e+04"/"e+06"
    notation), plain below that -- shared by Figure 7's bar-top value labels
    and its log-scale axis tick labels so both use one consistent number
    style. ".3g" (3 significant figures), not bare "g" (6 sig figs) --
    "1278939/1e6" otherwise prints as "1.27894M", not the "1.28M" a reader
    actually wants. `pos` is accepted (and ignored) so this doubles as a
    matplotlib FuncFormatter callback."""
    if v >= 1e6:
        return f"{v / 1e6:.3g}M"
    if v >= 1e3:
        return f"{v / 1e3:.3g}K"
    return f"{v:.3g}"


def load_csv(path):
    with open(path, newline="") as f:
        return list(csv.DictReader(f))


def row_at_n(rows, n, n_key="n"):
    for r in rows:
        if int(r[n_key]) == n:
            return r
    raise ValueError(f"no row with n={n} in the given rows")


def build_table6c(noir_rows, plonky3_rows):
    noir = row_at_n(noir_rows, REPRESENTATIVE_N)
    p3 = row_at_n(plonky3_rows, REPRESENTATIVE_N)

    noir_proof_size = int(noir["proof_size_bytes"])
    noir_verify_ms = float(noir["verify_s"]) * 1000
    noir_prove_s = float(noir["prove_s"])

    p3_proof_size = int(p3["proof_size_bytes"])
    p3_verify_ms = float(p3["verify_s"]) * 1000
    p3_prove_s = float(p3["prove_s"])

    lines = [
        "**Table 6C -- Backend Comparison (measured, N=256 sovereign blocks):**",
        "",
        "Noir+Honk measures circuit/reanchoring/src/main.nr (Poseidon2 header-chain "
        "continuity, real nargo+bb pipeline, table6b_scaling.csv). Plonky3 measures "
        "the same dominant cost driver -- N chained Poseidon2 permutations -- via "
        "Plonky3's own first-party benchmark example (prove_prime_field_31, pinned "
        "commit a31a1443a114c58735850daa5b5fc5c43c138d9d), NOT a hand-rolled "
        "reimplementation of main.nr's exact header struct; see "
        "benchmark_plonky3.sh's header comment for why. The two circuits are "
        "therefore not bit-identical, but both genuinely isolate the same "
        "primitive (Poseidon2, two permutations per header on the Noir side) "
        "that dominates constraint count there (table6a_6b.md's regression). "
        "Trusted setup / PQ secure are well-established properties of the "
        "underlying commitment scheme (KZG pairing-based for UltraHonk, FRI "
        "hash-based for Plonky3), not per-run measurements -- qualitative "
        "facts, not fabricated numbers. Recursion is a qualitative note, not "
        "scored: Barretenberg ships documented recursive-UltraHonk support; "
        "this Plonky3 checkout (0.6.0-era) has no ready-made recursive-verifier "
        "example to measure against, so no claim is made about its recursion "
        "maturity.",
        "",
        "| Metric | Noir + Honk | Plonky3 (Poseidon2/FRI) |",
        "| --- | ---: | ---: |",
        f"| Proof size | {noir_proof_size:,} B | {p3_proof_size:,} B |",
        f"| Verify time | {noir_verify_ms:.1f} ms | {p3_verify_ms:.1f} ms |",
        f"| Prove time | {noir_prove_s:.3f} s | {p3_prove_s:.3f} s |",
        "| Trusted setup | Yes (KZG/Aztec Ignition SRS) | No (FRI, transparent) |",
        "| PQ secure | No (elliptic-curve pairings) | Yes (hash-based FRI) |",
        "| Recursion support | Documented (Barretenberg recursive-UltraHonk) | Not evaluated at this checkout (see note above) |",
        "",
        "**Full N-sweep (both backends, real measurements):**",
        "",
        "| N | Noir prove (s) | Noir verify (ms) | Noir proof (B) | Plonky3 prove (s) | Plonky3 verify (ms) | Plonky3 proof (B) |",
        "| ---: | ---: | ---: | ---: | ---: | ---: | ---: |",
    ]
    p3_by_n = {int(r["n"]): r for r in plonky3_rows}
    for r in noir_rows:
        n = int(r["n"])
        if n not in p3_by_n:
            continue  # N=4: below Plonky3's vectorized-AIR floor of 8, see benchmark_plonky3.sh
        p3r = p3_by_n[n]
        lines.append(
            f"| {n} | {float(r['prove_s']):.3f} | {float(r['verify_s']) * 1000:.1f} | {int(r['proof_size_bytes']):,} "
            f"| {float(p3r['prove_s']):.3f} | {float(p3r['verify_s']) * 1000:.1f} | {int(p3r['proof_size_bytes']):,} |"
        )
    return "\n".join(lines)


def build_figure7(noir_rows, plonky3_rows):
    """Grouped bar chart across the 3 real, measured, numeric axes (proof
    size, verify time, prove time) at N=256 -- a radar chart across all 6 of
    Table 6C's rows would force the 3 qualitative rows (trusted setup / PQ
    secure / recursion) into arbitrary numeric scores, which isn't a
    measurement; keeping to the 3 measured axes avoids implying false
    precision.
    """
    setup_academic_plot_style()
    noir = row_at_n(noir_rows, REPRESENTATIVE_N)
    p3 = row_at_n(plonky3_rows, REPRESENTATIVE_N)

    metrics = ["Proof size (B)", "Verify (ms)", "Prove (s)"]
    noir_vals = [
        int(noir["proof_size_bytes"]),
        float(noir["verify_s"]) * 1000,
        float(noir["prove_s"]),
    ]
    p3_vals = [
        int(p3["proof_size_bytes"]),
        float(p3["verify_s"]) * 1000,
        float(p3["prove_s"]),
    ]

    fig, axes = plt.subplots(1, 3, figsize=figsize_row(3))
    for ax, metric, nv, pv in zip(axes, metrics, noir_vals, p3_vals):
        # width<1 separates the two bars so each value label has room; padding
        # pushes the label above the bar top (log scale would otherwise clip
        # or overlap it on the neighboring bar).
        bars = ax.bar(["Noir+Honk", "Plonky3"], [nv, pv], width=0.6, color=["C0", "C1"])
        ax.set_title(metric, fontsize=10.5, fontweight="bold")
        ax.bar_label(bars, fmt=_compact_num, padding=4, fontsize=8.5)
        ax.tick_params(labelsize=8)

        if metric == "Verify (ms)":
            # Linear, not log: both real values (22ms, 32.2ms) sit inside
            # one decade, so log scale bought nothing here -- and only a
            # linear axis can include an actual "0" tick (log(0) is
            # undefined, a hard math limit, not a styling choice). Plain
            # 0/20/40/60/80 instead of the log-scale tick-generation logic
            # used by the other two panels below.
            ax.set_ylim(0, 88)
            ax.set_yticks([0, 20, 40, 60, 80])
            ax.grid(True, which="major", axis="y", alpha=0.55, linewidth=0.7)
        else:
            ax.set_yscale("log")
            ymin, ymax = ax.get_ylim()
            ax.set_ylim(ymin, ymax * 3)

            # Real labeled minor ticks: matplotlib's default LogLocator gives
            # sparse major-only ticks on a narrow view, and its automatic
            # minor locator produces no labels at all there. How many
            # subdivisions per decade depends on how many decades this
            # panel's own range spans -- "Proof size"/"Prove" (3-4 decades
            # each) get only the 2x/5x subset, so the labels don't turn into
            # a solid wall of overlapping numbers.
            final_ymin, final_ymax = ax.get_ylim()
            lo = int(math.floor(math.log10(final_ymin)))
            hi = int(math.ceil(math.log10(final_ymax)))
            subs = (2, 5)
            minor_ticks = [
                d * 10 ** p
                for p in range(lo, hi + 1)
                for d in subs
                if final_ymin <= d * 10 ** p <= final_ymax
            ]
            ax.set_yticks(minor_ticks, minor=True)
            # Plain labels, not LogFormatterSciNotation -- that formatter's
            # built-in overlap heuristic silently blanks out most minor
            # labels on a narrow view, which defeats the point of adding
            # these ticks. Same _compact_num used for the bar-top value
            # labels above, so the axis and the labels never disagree on
            # number style (K/M suffixes, never "2e+06"/"500000" mixed).
            plain_fmt = mticker.FuncFormatter(_compact_num)
            ax.yaxis.set_minor_formatter(plain_fmt)
            ax.yaxis.set_major_formatter(plain_fmt)
            ax.tick_params(axis="y", which="minor", labelsize=7)
            ax.grid(True, which="minor", axis="y", alpha=0.55, linewidth=0.7)

    fig.suptitle(
        f"Figure 7 -- Backend Trade-off\nat N={REPRESENTATIVE_N} sovereign blocks (real measurements, log scale except Verify)",
        fontsize=12.5,
        fontweight="bold",
    )
    fig.tight_layout(rect=(0, 0, 1, 0.88))
    savefig_academic(fig, RESULTS_DIR, "figure7_backend_tradeoff")


def main():
    noir_rows = load_csv(NOIR_CSV)
    plonky3_rows = load_csv(PLONKY3_CSV)

    table6c = build_table6c(noir_rows, plonky3_rows)
    build_figure7(noir_rows, plonky3_rows)

    with open(TABLE_OUT, "w") as f:
        f.write(table6c + "\n")

    print(table6c)
    print()
    print(f"Figure 7 written to {RESULTS_DIR}/figure7_backend_tradeoff.{{pdf,png}}")
    print(f"Table written to {TABLE_OUT}")


if __name__ == "__main__":
    main()
