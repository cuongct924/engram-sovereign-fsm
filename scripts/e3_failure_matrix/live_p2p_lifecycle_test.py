#!/usr/bin/env python3
"""LIVE full-lifecycle fault-injection test against the real 4-node docker
testnet, driving a REAL P2P link failure (docker network disconnect/connect
on 2 validator-link-* networks) instead of live_lifecycle_test.py's
celestia-bridge stop/start.

A full DA outage cannot demonstrate ANCHORED -> SUSPICIOUS: da.VerifyReceipt
(x/da/verify.go, port of IsValidProposal's DA Pipeline Check,
spec/core/EngramTendermint.tla:290-294) requires every ANCHORED/RECOVERING
proposal to carry a FRESH DA attestation. With celestia-bridge fully down no
proposal can attest, so the chain can't commit ANY block while ANCHORED --
including one that would just absorb a warning. UnhealthyStreak can never
reach DownHysteresisThreshold, so SUSPICIOUS is unreachable; the only escape
is BTC's own height drifting past SOVEREIGN_THRESHOLD (BTC keeps mining
independently), forcing a direct ANCHORED -> SOVEREIGN jump. This is a
genuine liveness gap between the abstract spec (instantaneous attestation)
and the concrete pipeline's real submission latency -- not a coding bug, and
not this script's concern to fix.

P2P health has no equivalent hard gate: ProcessProposal never requires a
"P2P attestation" before committing, so a P2P warning is absorbed normally,
UnhealthyStreak accumulates and genuinely reaches SUSPICIOUS, then (via the
same sustained-SUSPICIOUS gray-failure timeout) SOVEREIGN.

Fault mechanism: disconnect 2 of the 6 validator-link-* networks (perfect
matching -- 01-02 and 03-04 -- so all 4 validators lose exactly 1 of their 3
peers simultaneously, CleanPeers 3->2 < Params.MinPeers=3, a pure P2P
warning; ActiveAnchors only drops to 2, still >= MinAnchorPeers=2, so it
never trips IsCriticalCondition's ActiveAnchors==0 branch). NOT a pumba netem
profile: a container's ethN index for a given network is unpredictable across
validators and redeploys, so `pumba netem --interface ethN` can silently
target the wrong network; `docker network disconnect`/`connect` resolves by
network NAME instead.

Phases:
  1. Baseline: confirm ANCHORED.
  2. Disconnect validator-link-01-02 (node01 side) and validator-link-03-04
     (node03 side) -> expect ANCHORED -> SUSPICIOUS within a few blocks.
  3. Keep holding -> expect SUSPICIOUS -> SOVEREIGN once suspicious_duration
     reaches Params.MaxSuspiciousTime.
  4. Reconnect both links (with their original static IPs) -> expect
     SOVEREIGN -> RECOVERING.
  5. Let health stabilize -> expect RECOVERING -> ANCHORED for real (the
     real ZK re-anchoring pipeline, watch_and_prove.sh, must already be
     running -- the reanchoring-prover container in a normal `make
     testnet-up` deployment).

Usage:
    python3 scripts/e3_failure_matrix/live_p2p_lifecycle_test.py
"""

import os
import subprocess
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "framework"))
from logger import sample_all_nodes, write_csv  # noqa: E402

RESULTS_DIR = os.path.join(os.path.dirname(__file__), "results_live")

# (network, container, static IP) -- IPs must match
# docker/engram-validator-cluster.yml exactly, or reconnecting will assign a
# different address than persistent_peers/genesis expect.
CUT_LINKS = [
    ("validator-link-01-02", "engram-node01", "172.40.0.2"),
    ("validator-link-03-04", "engram-node03", "172.40.5.2"),
]


def now():
    return time.strftime("%H:%M:%S", time.gmtime())


def docker(*args):
    subprocess.run(["docker", *args], capture_output=True, text=True, timeout=30)


def cut_links():
    for net, container, _ in CUT_LINKS:
        print(f"[{now()}] >>> docker network disconnect {net} {container}")
        docker("network", "disconnect", net, container)


def heal_links():
    for net, container, ip in CUT_LINKS:
        print(f"[{now()}] >>> docker network connect --ip {ip} {net} {container}")
        docker("network", "connect", "--ip", ip, net, container)


class Tracker:
    def __init__(self):
        self.start = time.time()
        self.samples = []
        self.transitions = []
        self.last_state = {}
        self.phase = "init"

    def elapsed(self):
        return time.time() - self.start

    def _record(self, round_samples):
        for s in round_samples:
            self.samples.append(s)
            prev = self.last_state.get(s.node)
            if s.fsm_state and prev and prev != s.fsm_state:
                t = self.elapsed()
                self.transitions.append((t, self.phase, s.node, prev, s.fsm_state))
                print(f"  *** [{self.phase}] TRANSITION @ {t:6.0f}s  {s.node}: {prev} -> {s.fsm_state} ***")
            if s.fsm_state:
                self.last_state[s.node] = s.fsm_state

    def poll(self, seconds, interval=2.0, phase=None):
        if phase:
            self.phase = phase
        deadline = time.time() + seconds
        while time.time() < deadline:
            round_samples = sample_all_nodes()
            self._record(round_samples)
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
            self._record(round_samples)
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

    print("=== Phase 2: cut 2 pairwise links (perfect matching), expect ANCHORED->SUSPICIOUS ===")
    cut_links()
    tr.wait_for_state("SUSPICIOUS", 60, phase="P2_p2p_degraded")

    print(
        "=== Phase 3: keep holding, expect SUSPICIOUS->SOVEREIGN (MaxSuspiciousTime gray-failure escalation) ==="
    )
    tr.wait_for_state("SOVEREIGN", 120, phase="P3_sustained_suspicious_escalate")

    print("=== Phase 4: reconnect both links, expect SOVEREIGN->RECOVERING ===")
    heal_links()
    tr.wait_for_state("RECOVERING", 60, phase="P4_recovery_start")

    print(
        "=== Phase 5: let health stabilize, run to real ANCHORED (needs watch_and_prove.sh running) ==="
    )
    reached = tr.wait_for_state("ANCHORED", 600, phase="P5_final_recovery")

    ts_label = time.strftime("%Y%m%dT%H%M%S")
    csv_path = os.path.join(RESULTS_DIR, f"p2p_lifecycle_test_{ts_label}.csv")
    write_csv(tr.samples, csv_path)

    summary_path = os.path.join(RESULTS_DIR, f"p2p_lifecycle_test_{ts_label}_summary.md")
    with open(summary_path, "w") as f:
        f.write(
            "# LIVE full-lifecycle fault-injection test (real P2P link cut, docker network disconnect)\n\n"
        )
        f.write(f"Total duration: {tr.elapsed():.0f}s. Final phase reached ANCHORED: {reached}\n\n")
        f.write("## Real transitions observed\n\n")
        f.write("| t (s) | Phase | Node | From | To |\n|---:|---|---|---|---|\n")
        for t, phase, node, frm, to in tr.transitions:
            f.write(f"| {t:.0f} | {phase} | {node} | {frm} | {to} |\n")
    print(f"\nwrote {len(tr.samples)} samples to {csv_path}")
    print(f"wrote summary to {summary_path}")


if __name__ == "__main__":
    main()
