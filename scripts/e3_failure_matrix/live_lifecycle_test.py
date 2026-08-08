#!/usr/bin/env python3
"""LIVE full-lifecycle fault-injection test against the real 4-node docker
testnet -- docs/EXPERIMENT.md's E2/E3 scenario blueprint (S3 DA unavailable +
S7 Recovery), but driving the REAL celestia-bridge/celestia-app containers
(docker stop/start) instead of tests/e2e's mock-sensor Harness, per the
user's explicit requirement that this repo's scripts/ read from the live
docker cluster, not tests/e2e.

Exercises every edge in the full lifecycle:
    ANCHORED <-> SUSPICIOUS -> SOVEREIGN <-> RECOVERING -> ANCHORED

Phases (each one a real, timed fault injection against real containers):
  1. Baseline: confirm ANCHORED, healthy.
  2. Stop celestia-bridge -> expect ANCHORED -> SUSPICIOUS (DA warning).
  3. Quickly restart celestia-bridge (before MaxSuspiciousTime) -> expect
     SUSPICIOUS -> ANCHORED directly (no escalation to SOVEREIGN).
  4. Stop celestia-bridge again, this time held long enough to exceed
     Params.MaxSuspiciousTime (real blocks, not mocked) -> expect
     SUSPICIOUS -> SOVEREIGN (the "sustained suspicious = treat as attack"
     escalation the user asked to verify).
  5. Restart celestia-bridge -> expect SOVEREIGN -> RECOVERING.
  6. Immediately stop celestia-bridge again for a short window while still
     in RECOVERING -> expect RECOVERING -> SOVEREIGN (regression, the
     other user-requested bidirectional edge).
  7. Restart celestia-bridge for good, let health stabilize -> expect
     SOVEREIGN -> RECOVERING -> ANCHORED again, this time let it run to
     real completion (the real ZK re-anchoring proof pipeline,
     scripts/reanchoring_prover/watch_and_prove.sh, must already be running
     separately in the background for this final edge to close for real).

Usage:
    python3 scripts/e3_failure_matrix/live_lifecycle_test.py
"""

import csv
import os
import subprocess
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "framework"))
from logger import sample_all_nodes, write_csv  # noqa: E402

RESULTS_DIR = os.path.join(os.path.dirname(__file__), "results_live")
CELESTIA_BRIDGE = "celestia-bridge"


def docker(*args):
    subprocess.run(["docker", *args], capture_output=True, text=True, timeout=30)


def stop_celestia():
    print(f"[{now()}] >>> docker stop {CELESTIA_BRIDGE} (injecting DA outage)")
    docker("stop", CELESTIA_BRIDGE)


def start_celestia():
    print(f"[{now()}] >>> docker start {CELESTIA_BRIDGE} (restoring DA)")
    docker("start", CELESTIA_BRIDGE)


def now():
    return time.strftime("%H:%M:%S", time.gmtime())


class Tracker:
    def __init__(self):
        self.start = time.time()
        self.samples = []
        self.transitions = []
        self.last_state = {}
        self.phase = "init"

    def elapsed(self):
        return time.time() - self.start

    def poll(self, seconds, interval=2.0, phase=None):
        if phase:
            self.phase = phase
        deadline = time.time() + seconds
        while time.time() < deadline:
            round_samples = sample_all_nodes()
            for s in round_samples:
                self.samples.append((self.phase, s))
                prev = self.last_state.get(s.node)
                if s.fsm_state and prev and prev != s.fsm_state:
                    t = self.elapsed()
                    self.transitions.append((t, self.phase, s.node, prev, s.fsm_state))
                    print(f"  *** [{self.phase}] TRANSITION @ {t:6.0f}s  {s.node}: {prev} -> {s.fsm_state} ***")
                if s.fsm_state:
                    self.last_state[s.node] = s.fsm_state
            heights = [s.height for s in round_samples]
            states = [s.fsm_state for s in round_samples]
            print(f"[{self.elapsed():6.0f}s][{self.phase}] h={heights} state={states}")
            time.sleep(interval)

    def wait_for_state(self, target_state, timeout_s, phase=None):
        if phase:
            self.phase = phase
        deadline = time.time() + timeout_s
        while time.time() < deadline:
            round_samples = sample_all_nodes()
            for s in round_samples:
                self.samples.append((self.phase, s))
                prev = self.last_state.get(s.node)
                if s.fsm_state and prev and prev != s.fsm_state:
                    t = self.elapsed()
                    self.transitions.append((t, self.phase, s.node, prev, s.fsm_state))
                    print(f"  *** [{self.phase}] TRANSITION @ {t:6.0f}s  {s.node}: {prev} -> {s.fsm_state} ***")
                if s.fsm_state:
                    self.last_state[s.node] = s.fsm_state
            states = {s.node: s.fsm_state for s in round_samples}
            print(f"[{self.elapsed():6.0f}s][{self.phase}] {states}")
            if all(v == target_state for v in states.values()):
                print(f"[{self.elapsed():6.0f}s] all 4 nodes reached {target_state}")
                return True
            time.sleep(2.0)
        print(f"[{self.elapsed():6.0f}s] TIMEOUT waiting for {target_state} after {timeout_s}s")
        return False


def main():
    os.makedirs(RESULTS_DIR, exist_ok=True)
    tr = Tracker()

    print("=== Phase 1: baseline ===")
    tr.poll(6, phase="P1_baseline")

    print("=== Phase 2: stop celestia-bridge, expect ANCHORED->SUSPICIOUS ===")
    stop_celestia()
    tr.wait_for_state("SUSPICIOUS", 60, phase="P2_da_outage")

    print("=== Phase 3: quick restart, expect SUSPICIOUS->ANCHORED (no escalation) ===")
    start_celestia()
    tr.wait_for_state("ANCHORED", 60, phase="P3_quick_recovery")

    print("=== Phase 4: stop celestia-bridge again, hold past MaxSuspiciousTime, expect SUSPICIOUS->SOVEREIGN ===")
    stop_celestia()
    tr.wait_for_state("SUSPICIOUS", 30, phase="P4_sustained_outage_enter")
    tr.wait_for_state("SOVEREIGN", 120, phase="P4_sustained_outage_escalate")

    print("=== Phase 5: restart celestia-bridge, expect SOVEREIGN->RECOVERING ===")
    start_celestia()
    tr.wait_for_state("RECOVERING", 60, phase="P5_recovery_start")

    print("=== Phase 6: brief re-outage while RECOVERING, expect RECOVERING->SOVEREIGN ===")
    stop_celestia()
    tr.wait_for_state("SOVEREIGN", 30, phase="P6_regression")

    print("=== Phase 7: restart for good, let it run to real ANCHORED (needs watch_and_prove.sh running) ===")
    start_celestia()
    reached = tr.wait_for_state("ANCHORED", 600, phase="P7_final_recovery")

    ts_label = time.strftime("%Y%m%dT%H%M%S")
    csv_path = os.path.join(RESULTS_DIR, f"lifecycle_test_{ts_label}.csv")
    all_samples = [s for _, s in tr.samples]
    write_csv(all_samples, csv_path)

    summary_path = os.path.join(RESULTS_DIR, f"lifecycle_test_{ts_label}_summary.md")
    with open(summary_path, "w") as f:
        f.write("# LIVE full-lifecycle fault-injection test (real docker celestia-bridge stop/start)\n\n")
        f.write(f"Total duration: {tr.elapsed():.0f}s. Final phase reached ANCHORED: {reached}\n\n")
        f.write("## Real transitions observed\n\n")
        f.write("| t (s) | Phase | Node | From | To |\n|---:|---|---|---|---|\n")
        for t, phase, node, frm, to in tr.transitions:
            f.write(f"| {t:.0f} | {phase} | {node} | {frm} | {to} |\n")
    print(f"\nwrote {len(all_samples)} samples to {csv_path}")
    print(f"wrote summary to {summary_path}")


if __name__ == "__main__":
    main()
