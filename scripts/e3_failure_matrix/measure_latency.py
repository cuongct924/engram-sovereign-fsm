#!/usr/bin/env python3
"""
E3 -- External-Dependency Failure Matrix (docs/EXPERIMENT.md's E3, Table 2).

Builds Table 2 (BTC/DA/P2P condition -> observed FSM state/withdrawal
policy/block production) from REAL data in tests/e2e/results/s*.csv. Each
scenario's final (steady-state) row is taken as the settled outcome for its
input condition. "Block production" is reported as continuous for every row
because tests/e2e/harness.go's Advance() never blocked or errored across any
scenario.

Usage:
    go test ./tests/e2e/...          # regenerate tests/e2e/results/*.csv
    python3 scripts/e3_failure_matrix/measure_latency.py
"""

import csv
import glob
import os
import sys

E2E_RESULTS_DIR = os.path.join(
    os.path.dirname(__file__), "..", "..", "tests", "e2e", "results"
)
OUT_DIR = os.path.join(os.path.dirname(__file__), "results")
TABLE_OUT = os.path.join(OUT_DIR, "table2_failure_matrix.md")

# DefaultParams() (x/sovereignty/types/params.go): SuspiciousThreshold=1, SovereignThreshold=2.
BTC_SUSPICIOUS_THRESHOLD = 1
BTC_SOVEREIGN_THRESHOLD = 2

SCENARIO_LABELS = {
    "s1_normal": "S1 Normal",
    "s2_btc_congestion": "S2 BTC congestion (settled)",
    "s3_da_unavailable": "S3 DA unavailable (settled)",
    "s4_p2p_eclipse_partial": "S4 P2P eclipse partial (settled)",
    "s5_anchor_isolation": "S5 Anchor isolation (settled)",
    "s6_combined_btc_da_failure": "S6 Combined BTC+DA failure (settled)",
    "s7_recovery": "S7 Recovery (settled)",
}


def btc_condition(gap):
    if gap == 0:
        return "healthy"
    if gap < BTC_SOVEREIGN_THRESHOLD:
        return "warning"
    return "critical"


def load_last_row(path):
    with open(path, newline="") as f:
        rows = list(csv.DictReader(f))
    return rows[-1] if rows else None


def build_table():
    csv_paths = sorted(glob.glob(os.path.join(E2E_RESULTS_DIR, "s*.csv")))
    if not csv_paths:
        print(
            f"No CSVs found in {E2E_RESULTS_DIR} -- run 'go test ./tests/e2e/...' first.",
            file=sys.stderr,
        )
        sys.exit(1)

    lines = [
        "**Table 2 -- Failure Matrix (measured, real tests/e2e data, DefaultParams thresholds):**",
        "",
        "| Scenario | BTC | DA | P2P | Observed FSM state | Withdrawals | Block production |",
        "| --- | --- | --- | --- | --- | --- | --- |",
    ]
    for p in csv_paths:
        name = os.path.splitext(os.path.basename(p))[0]
        row = load_last_row(p)
        if row is None:
            continue
        btc = btc_condition(int(row["btc_gap"]))
        da = "healthy" if row["da_healthy"].lower() == "true" else "failed"
        p2p = "healthy" if row["p2p_healthy"].lower() == "true" else "unhealthy"
        withdrawals = (
            "locked" if row["withdraw_locked"].lower() == "true" else "enabled"
        )
        lines.append(
            f"| {SCENARIO_LABELS.get(name, name)} | {btc} | {da} | {p2p} | {row['state']} "
            f"| {withdrawals} | continuous ({row['height']} blocks, 0 halts) |"
        )
    return "\n".join(lines)


def main():
    table = build_table()
    os.makedirs(OUT_DIR, exist_ok=True)
    with open(TABLE_OUT, "w") as f:
        f.write(table + "\n")
    print(table)
    print(f"\nTable written to {TABLE_OUT}")


if __name__ == "__main__":
    main()
