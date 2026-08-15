#!/usr/bin/env python3
"""
E3 LIVE -- External-Dependency Failure Matrix from REAL docker cluster data
(docs/EXPERIMENT.md's E3, Table 2), reading scripts/e2_fault_injection/
results_live/s*.csv (live_scenario_matrix.py's output) instead of
tests/e2e/results/s*.csv's in-process mock-harness data.

Honest limitation vs. the in-process version (measure_latency.py): the live
CSVs carry NO btc_gap/da_healthy/p2p_healthy columns -- NewPreBlocker
deliberately never writes live PeripheralMetrics into committed state (it
once caused a real AppHash-divergence consensus failure across the 4-node
testnet; the in-process Harness has no such constraint since it isn't
ABCI-committed state). So this table reports what IS honestly observable
live (fsm_state, height, withdrawal-lock status computed from fsm_state
alone) and omits the condition columns rather than fabricating them.

Usage:
    python3 -u scripts/e2_fault_injection/live_scenario_matrix.py   # generate the live CSVs first
    python3 scripts/e3_failure_matrix/measure_latency_live.py
"""

import csv
import glob
import os
import sys

LIVE_RESULTS_DIR = os.path.join(
    os.path.dirname(__file__), "..", "e2_fault_injection", "results_live"
)
OUT_DIR = os.path.join(os.path.dirname(__file__), "results_live")
TABLE_OUT = os.path.join(OUT_DIR, "table2_failure_matrix_live.md")

# Mirrors x/sovereignty/types.WithdrawLocked's pure logic (SOVEREIGN/RECOVERING
# lock withdrawals) -- computable client-side from fsm_state alone, no live
# metrics needed, unlike the btc/da/p2p condition columns this table omits.
WITHDRAW_LOCKED_STATES = {"SOVEREIGN", "RECOVERING"}

SCENARIO_LABELS = {
    "s1_normal": "S1 Normal",
    "s2_btc_congestion": "S2 BTC congestion (settled)",
    "s3_da_unavailable": "S3 DA unavailable (settled)",
    "s4_p2p_eclipse_partial": "S4 P2P eclipse partial (settled)",
    "s5_anchor_isolation": "S5 Anchor isolation (settled)",
    "s6_combined_btc_da_failure": "S6 Combined BTC+DA failure (settled)",
    "s7_recovery": "S7 Recovery (settled)",
}


def load_last_row(path):
    with open(path, newline="") as f:
        rows = list(csv.DictReader(f))
    return rows[-1] if rows else None


def build_table():
    csv_paths = sorted(glob.glob(os.path.join(LIVE_RESULTS_DIR, "s*.csv")))
    if not csv_paths:
        print(
            f"No CSVs found in {LIVE_RESULTS_DIR} -- run "
            f"'python3 -u scripts/e2_fault_injection/live_scenario_matrix.py' first.",
            file=sys.stderr,
        )
        sys.exit(1)

    lines = [
        "**Table 2 LIVE -- Failure Matrix (measured, real 4-node Docker cluster data):**",
        "",
        "BTC/DA/P2P condition columns are intentionally omitted here (unlike the in-process "
        "table2_failure_matrix.md) -- PeripheralMetrics is never written into committed state on "
        "this app (a deliberate safety fix, see x/sovereignty/preblock.go's NewPreBlocker doc), so "
        "that data genuinely isn't observable live via Query.State. Only fsm_state and height are "
        "real committed data; withdrawal-lock status is computed client-side from fsm_state alone "
        "(mirrors types.WithdrawLocked's pure logic, no live metrics needed for that).",
        "",
        "| Scenario | Observed FSM state (settled) | Height reached | Withdrawals | Sample count |",
        "| --- | --- | --- | --- | --- |",
    ]
    for p in csv_paths:
        name = os.path.splitext(os.path.basename(p))[0]
        with open(p, newline="") as f:
            rows = list(csv.DictReader(f))
        if not rows:
            continue
        row = rows[-1]
        state = row.get("fsm_state", "")
        withdrawals = "locked" if state in WITHDRAW_LOCKED_STATES else "enabled"
        lines.append(
            f"| {SCENARIO_LABELS.get(name, name)} | {state} | {row.get('height', '?')} "
            f"| {withdrawals} | {len(rows)} |"
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
