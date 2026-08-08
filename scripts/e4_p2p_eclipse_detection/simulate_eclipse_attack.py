#!/usr/bin/env python3
"""
E4 -- P2P Eclipse/Sybil Detection (docs/EXPERIMENT.md's E4, Table 6).

IMPORTANT METHODOLOGY NOTE: live Pumba/Docker chaos injection against a real
multi-node network is NOT available in this environment (no Docker daemon
running -- see CLAUDE.md's M7 status). This consumes REAL data from
tests/e2e/results/e4_p2p_detector_comparison.csv, produced by
`go test ./tests/e2e/... -run TestE4_P2PDetectorComparison`, which runs the
real production detector function (types.IsP2PQualityHealthy) and a naive
peer-count-only baseline against SYNTHETIC, hand-constructed randomized peer
snapshots modeling each attack's known signature (2000 Monte Carlo trials per
cell, fixed RNG seed for reproducibility). The detector code under test is
real; the traffic it's tested against is simulated, not observed on a live
network. Do not present this as a live-network measurement.

Usage:
    go test ./tests/e2e/... -run TestE4_P2PDetectorComparison
    python3 scripts/e4_p2p_eclipse_detection/simulate_eclipse_attack.py
"""

import csv
import os
import sys

E2E_RESULTS_DIR = os.path.join(
    os.path.dirname(__file__), "..", "..", "tests", "e2e", "results"
)
CSV_IN = os.path.join(E2E_RESULTS_DIR, "e4_p2p_detector_comparison.csv")
OUT_DIR = os.path.join(os.path.dirname(__file__), "results")
TABLE_OUT = os.path.join(OUT_DIR, "table6_p2p_detector_accuracy.md")

ATTACK_LABELS = {
    "A1_PeerSlotExhaustion": "Peer Slot Exhaustion",
    "A2_BGPHijackSybil": "BGP Hijacking / Sybil",
    "A3_ChurnBasedRotation": "Churn-based Rotation",
    "A4_RelayNodeAttack": "Relay Node Attack",
}


def load_rows():
    with open(CSV_IN, newline="") as f:
        return list(csv.DictReader(f))


def build_table(rows, param_set):
    lines = [
        f"**Table 6 -- Detection Accuracy of P2P Profiler vs. Peer-Count Baseline "
        f"(SYNTHETIC Monte Carlo, param_set={param_set}, 2000 trials/cell -- see script docstring):**",
        "",
        "| Attack Scenario | Detector | FPR | FNR | Avg. Detection Delay (snapshots) |",
        "| --- | --- | ---: | ---: | ---: |",
    ]
    by_attack = {}
    for r in rows:
        if r["param_set"] != param_set:
            continue
        by_attack.setdefault(r["attack"], {})[r["detector"]] = r

    for attack, label in ATTACK_LABELS.items():
        entry = by_attack.get(attack, {})
        first = True
        for detector, dname in (
            ("peer_count_only", "Peer-count"),
            ("tri_interface", "**Tri-interface**"),
        ):
            r = entry.get(detector)
            if r is None:
                continue
            delay = r["avg_detection_delay_snapshots"]
            delay_str = "N/A" if float(delay) < 0 else delay
            scenario_col = label if first else ""
            lines.append(
                f"| {scenario_col} | {dname} | {float(r['fpr'])*100:.1f}% | {float(r['fnr'])*100:.1f}% | {delay_str} |"
            )
            first = False
    return "\n".join(lines)


def main():
    if not os.path.exists(CSV_IN):
        print(
            f"{CSV_IN} not found -- run 'go test ./tests/e2e/... -run TestE4_P2PDetectorComparison' first.",
            file=sys.stderr,
        )
        sys.exit(1)

    rows = load_rows()
    param_sets = sorted({r["param_set"] for r in rows})

    os.makedirs(OUT_DIR, exist_ok=True)
    full_output = []
    for param_set in param_sets:
        table = build_table(rows, param_set)
        full_output.append(table)
        print(table)
        print()

    with open(TABLE_OUT, "w") as f:
        f.write("\n\n".join(full_output) + "\n")
    print(f"Table written to {TABLE_OUT}")


if __name__ == "__main__":
    main()
