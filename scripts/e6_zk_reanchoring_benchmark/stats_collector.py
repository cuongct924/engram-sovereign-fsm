"""
E6 -- Reanchoring Feasibility Evaluation: turns benchmark_prover.sh's raw
table6b_scaling.csv (real Noir + Barretenberg/UltraHonk measurements) into
docs/EXPERIMENT.md's Table 6B.

Run after benchmark_prover.sh:
    python3 scripts/e6_zk_reanchoring_benchmark/stats_collector.py
"""

import csv
import os
import sys

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


def main():
    rows = load_rows()
    table6b = build_table6b(rows)

    with open(TABLE_OUT, "w") as f:
        f.write(table6b + "\n")

    print(table6b)
    print()
    print(f"Table written to {TABLE_OUT}")


if __name__ == "__main__":
    main()
