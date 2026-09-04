"""
E6 -- Reanchoring Feasibility Evaluation: Table 6C (docs/EXPERIMENT.md,
optional backend comparison: Noir+Barretenberg/UltraHonk vs Plonky3).

Combines the real Noir measurements already in table6b_scaling.csv with
benchmark_plonky3.sh's real table6c_plonky3.csv (both scripts must have been
run first) into results/table6c_backend_comparison.md.

Historical, frozen comparison -- do NOT rerun against a fresh table6b_scaling.csv.
This table is only a fair hash-primitive-only comparison because table6b_scaling.csv,
at the time this was last generated, predated condition (e)'s ed25519 boundary-signature
verification (circuit/README.md). Since then, the deployed circuit's table6b_scaling.csv
includes condition (e), which Plonky3's table6c_plonky3.csv has no equivalent for --
rerunning this script against today's table6b_scaling.csv would silently compare a
full-statement Noir proof against a hash-primitive-only Plonky3 proof, which is not a
fair comparison (docs/EXPERIMENT.md's E6 Appendix explains this gap explicitly).

Run after benchmark_plonky3.sh:
    python3 scripts/e6_zk_reanchoring_benchmark/table6c_collector.py
"""

import csv
import os

RESULTS_DIR = os.path.dirname(__file__) + "/results"
NOIR_CSV = os.path.join(RESULTS_DIR, "table6b_scaling.csv")
PLONKY3_CSV = os.path.join(RESULTS_DIR, "table6c_plonky3.csv")
TABLE_OUT = os.path.join(RESULTS_DIR, "table6c_backend_comparison.md")

# Representative N for the headline comparison numbers -- the largest N both
# sides actually measured (256), matching docs/EXPERIMENT.md's Table 6C style
# of quoting single representative numbers per backend rather than a curve.
REPRESENTATIVE_N = 256


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
        "primitive (Poseidon2, two permutations per header). This is a "
        "hash-primitive-only comparison -- neither side proves FSM/root-binding "
        "legality or the deployed circuit's condition (e) ed25519 "
        "boundary-signature verification (circuit/README.md); see the fuller-"
        "statement comparison below for a closer (still incomplete) attempt. "
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


def main():
    noir_rows = load_csv(NOIR_CSV)
    plonky3_rows = load_csv(PLONKY3_CSV)

    table6c = build_table6c(noir_rows, plonky3_rows)

    with open(TABLE_OUT, "w") as f:
        f.write(table6c + "\n")

    print(table6c)
    print()
    print(f"Table written to {TABLE_OUT}")


if __name__ == "__main__":
    main()
