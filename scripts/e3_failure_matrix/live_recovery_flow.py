#!/usr/bin/env python3
"""LIVE recovery-flow observation against the real 4-node docker testnet --
real e2e data on the FSM transitioning SOVEREIGN -> RECOVERING -> ANCHORED
on the LIVE network (not tests/e2e's in-process mock harness, which already
covers this synthetically in TestE2_S7_Recovery / TestRecoveryFlow_*).

Polls all 4 real nodes' RPC/ABCI-query state continuously, prints every real
state transition as it happens, and writes a full CSV timeline + markdown
summary with real time-to-recovering / time-to-anchored numbers (or "not
reached within the window" if the window runs out -- itself a real,
reportable result).

Usage:
    python3 scripts/e3_failure_matrix/live_recovery_flow.py --minutes 20
"""

import argparse
import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "framework"))
from logger import sample_all_nodes, write_csv, NODE_RPC_PORTS  # noqa: E402

RESULTS_DIR = os.path.join(os.path.dirname(__file__), "results_live")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--minutes", type=float, default=20.0)
    parser.add_argument("--interval-s", type=float, default=5.0)
    parser.add_argument("--stop-on-anchored", action="store_true", default=True)
    args = parser.parse_args()

    os.makedirs(RESULTS_DIR, exist_ok=True)
    deadline = time.time() + args.minutes * 60
    start = time.time()

    all_samples = []
    last_state = {}
    transitions = []  # (elapsed_s, node, from, to)

    print(f"=== LIVE recovery-flow observation, up to {args.minutes:.0f} min ===")
    anchored_reached_at = {}

    while time.time() < deadline:
        elapsed = time.time() - start
        round_samples = sample_all_nodes()
        all_samples.extend(round_samples)

        line_parts = []
        for s in round_samples:
            line_parts.append(
                f"{s.node}: h={s.height} state={s.fsm_state or '?'} "
                f"safe_blocks={s.safe_blocks} susp_dur={s.suspicious_duration} "
                f"proof_valid={s.reanchoring_proof_valid}"
            )
            prev = last_state.get(s.node)
            if s.fsm_state and prev and prev != s.fsm_state:
                transitions.append((elapsed, s.node, prev, s.fsm_state))
                print(
                    f"  *** TRANSITION @ {elapsed:6.0f}s  {s.node}: {prev} -> {s.fsm_state} ***"
                )
                if s.fsm_state == "ANCHORED" and s.node not in anchored_reached_at:
                    anchored_reached_at[s.node] = elapsed
            if s.fsm_state:
                last_state[s.node] = s.fsm_state

        print(f"[{elapsed:6.0f}s]", " | ".join(line_parts))

        if args.stop_on_anchored and len(anchored_reached_at) == len(NODE_RPC_PORTS):
            print("\nAll 4 nodes reached ANCHORED -- stopping early.")
            break

        time.sleep(args.interval_s)

    ts_label = time.strftime("%Y%m%dT%H%M%S")
    csv_path = os.path.join(RESULTS_DIR, f"recovery_flow_{ts_label}.csv")
    write_csv(all_samples, csv_path)

    summary_path = os.path.join(RESULTS_DIR, f"recovery_flow_{ts_label}_summary.md")
    with open(summary_path, "w") as f:
        f.write("# LIVE recovery-flow observation (real 4-node docker testnet)\n\n")
        f.write(
            f"Window: {args.minutes:.0f} min requested, {(time.time()-start)/60:.1f} min actually run.\n\n"
        )
        f.write(f"Samples: {len(all_samples)}, CSV: `{os.path.basename(csv_path)}`\n\n")
        f.write("## Real transitions observed\n\n")
        if transitions:
            f.write("| t (s) | Node | From | To |\n|---:|---|---|---|\n")
            for elapsed, node, frm, to in transitions:
                f.write(f"| {elapsed:.0f} | {node} | {frm} | {to} |\n")
        else:
            f.write("No state transitions observed in this window.\n")
        f.write("\n## ANCHORED reached\n\n")
        if anchored_reached_at:
            for node, t in anchored_reached_at.items():
                f.write(f"- {node}: reached ANCHORED at t={t:.0f}s\n")
        else:
            f.write(
                "Not reached by any node within this window (real result, not omitted).\n"
            )
    print(f"\nwrote {len(all_samples)} samples to {csv_path}")
    print(f"wrote summary to {summary_path}")


if __name__ == "__main__":
    main()
