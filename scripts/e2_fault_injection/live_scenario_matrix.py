#!/usr/bin/env python3
"""LIVE E2 fault-injection scenario matrix against the real 4-node testnet --
docs/EXPERIMENT.md's S1-S7, driven by REAL docker fault injection (not
tests/e2e's in-process mock harness), continuing this session's standing
requirement that scripts/ read from the live cluster.

One continuous 7-phase run (chains phases like
scripts/e3_failure_matrix/live_lifecycle_test.py already does, since a cold
restabilization between 7 INDEPENDENT runs would cost ~250-300s of peer-
tenure buildup each, per this session's own empirical numbers -- prohibitive
seven times over). Each phase's "before" state is the previous phase's
already-healed "after" state.

    S1 baseline        -- no injection, confirm ANCHORED
    S2 BTC congestion   -- new chaos-btc-delay Pumba profile on bitcoin-node01.
                           GENUINELY UNPROVEN whether this produces a
                           GRADUAL btc_gap growth (the doc's expectation) or
                           an instant jump like S6 already shows -- this
                           script reports whichever it actually observes,
                           does not assume the expected shape.
    S3 DA unavailable   -- docker stop/start celestia-bridge (proven
                           mechanism, live_lifecycle_test.py).
    S4 P2P eclipse partial -- existing chaos-loss profile (5% loss,
                           engram-node01+02, 2m).
    S5 Anchor isolation -- existing chaos-eclipse profile (100% loss,
                           engram-node01, 3m). ActiveAnchors is NOT
                           independently confirmable via Query.State (that
                           field is documented stale/never committed, see
                           logger.py's _decode_query_state comment) -- this
                           phase cross-checks via /net_info peer counts on
                           the isolated node instead (logger.net_info).
    S6 combined BTC+DA failure -- docker stop bitcoin-node01 AND
                           celestia-bridge simultaneously. Also doubles as
                           the live regression check for this session's
                           anchor.go/btcGapMetric error-swallowing fix,
                           applied earlier but never live-verified for a
                           real BTC outage specifically.
    S7 recovery         -- restart everything, run to real ANCHORED via the
                           real ZK re-anchoring pipeline. REQUIRES
                           scripts/reanchoring_prover/watch_and_prove.sh
                           already running in the background separately.

Output CSVs named s{N}_<name>.csv to match tests/e2e/results/s*.csv's
existing naming convention, so scripts/e3_failure_matrix/measure_latency_live.py
can read them with the same glob pattern.

Usage:
    python3 -u scripts/e2_fault_injection/live_scenario_matrix.py
"""

import os
import subprocess
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "framework"))
from logger import sample_all_nodes, net_info, write_csv  # noqa: E402
from injector import (
    start_pumba_profile,
    cleanup_profile,
    wait_for_no_active_netem,
)  # noqa: E402

RESULTS_DIR = os.path.join(os.path.dirname(__file__), "results_live")
CELESTIA_BRIDGE = "celestia-bridge"
BITCOIN_NODE = "bitcoin-node01"


def now() -> str:
    return time.strftime("%H:%M:%S", time.gmtime())


def docker(*args):
    subprocess.run(["docker", *args], capture_output=True, text=True, timeout=30)


class Tracker:
    def __init__(self):
        self.start = time.time()
        # (scenario_key, NodeSample) -- scenario_key set by set_scenario(),
        # so each write_scenario_csv() call writes ONLY that scenario's own
        # samples, matching tests/e2e/results/s*.csv's per-scenario
        # convention -- NOT a cumulative dump of every phase run so far.
        self.samples = []
        self.transitions = []
        self.last_state = {}
        self.phase = "init"
        self.scenario_key = "init"

    def elapsed(self):
        return time.time() - self.start

    def set_scenario(self, scenario_key: str):
        self.scenario_key = scenario_key

    def poll(self, seconds, interval=3.0, phase=None):
        if phase:
            self.phase = phase
        deadline = time.time() + seconds
        while time.time() < deadline:
            round_samples = sample_all_nodes()
            self.samples.extend((self.scenario_key, s) for s in round_samples)
            for s in round_samples:
                prev = self.last_state.get(s.node)
                if s.fsm_state and prev and prev != s.fsm_state:
                    t = self.elapsed()
                    self.transitions.append((t, self.phase, s.node, prev, s.fsm_state))
                    print(
                        f"  *** [{self.phase}] TRANSITION @ {t:6.0f}s  {s.node}: {prev} -> {s.fsm_state} ***"
                    )
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
            self.samples.extend((self.scenario_key, s) for s in round_samples)
            for s in round_samples:
                prev = self.last_state.get(s.node)
                if s.fsm_state and prev and prev != s.fsm_state:
                    t = self.elapsed()
                    self.transitions.append((t, self.phase, s.node, prev, s.fsm_state))
                    print(
                        f"  *** [{self.phase}] TRANSITION @ {t:6.0f}s  {s.node}: {prev} -> {s.fsm_state} ***"
                    )
                if s.fsm_state:
                    self.last_state[s.node] = s.fsm_state
            states = {s.node: s.fsm_state for s in round_samples}
            print(f"[{self.elapsed():6.0f}s][{self.phase}] {states}")
            if all(v == target_state for v in states.values()):
                print(f"[{self.elapsed():6.0f}s] all 4 nodes reached {target_state}")
                return True
            time.sleep(2.0)
        print(
            f"[{self.elapsed():6.0f}s] TIMEOUT waiting for {target_state} after {timeout_s}s"
        )
        return False

    def write_scenario_csv(self, name: str, scenario_key: str):
        """Writes ONLY samples recorded under scenario_key (see set_scenario)
        -- each scenario's CSV is self-contained, not a cumulative dump of
        every phase run so far in this whole 7-phase script."""
        path = os.path.join(RESULTS_DIR, f"{name}.csv")
        scenario_samples = [s for key, s in self.samples if key == scenario_key]
        write_csv(scenario_samples, path)
        print(
            f"wrote {len(scenario_samples)} samples (scenario={scenario_key}) to {path}"
        )
        return path


def main():
    os.makedirs(RESULTS_DIR, exist_ok=True)
    tr = Tracker()

    print("=== S1: baseline ===")
    tr.set_scenario("s1")
    tr.poll(15, phase="S1_baseline")
    tr.write_scenario_csv("s1_normal", "s1")

    print("=== S2: BTC congestion (chaos-btc-delay, dry-run behavior unproven) ===")
    tr.set_scenario("s2")
    wait_for_no_active_netem()
    start_pumba_profile("chaos-btc-delay")
    tr.poll(
        130, interval=3.0, phase="S2_btc_congestion"
    )  # profile's own --duration=2m + margin
    cleanup_profile("chaos-btc-delay")
    tr.poll(20, phase="S2_btc_congestion_cooldown")
    tr.write_scenario_csv("s2_btc_congestion", "s2")

    print("=== S3: DA unavailable (docker stop celestia-bridge) ===")
    tr.set_scenario("s3")
    docker("stop", CELESTIA_BRIDGE)
    tr.poll(60, phase="S3_da_unavailable")
    docker("start", CELESTIA_BRIDGE)
    tr.wait_for_state("ANCHORED", 90, phase="S3_da_recovery")
    tr.write_scenario_csv("s3_da_unavailable", "s3")

    print("=== S4: P2P eclipse partial (chaos-loss) ===")
    tr.set_scenario("s4")
    wait_for_no_active_netem()
    start_pumba_profile("chaos-loss")
    tr.poll(
        130, interval=3.0, phase="S4_p2p_eclipse_partial"
    )  # profile's own --duration=2m + margin
    cleanup_profile("chaos-loss")
    tr.poll(20, phase="S4_p2p_cooldown")
    tr.write_scenario_csv("s4_p2p_eclipse_partial", "s4")

    print(
        "=== S5: Anchor isolation (chaos-eclipse) -- cross-checking via /net_info, "
        "not Query.State (documented stale field) ==="
    )
    tr.set_scenario("s5")
    wait_for_no_active_netem()
    start_pumba_profile("chaos-eclipse")
    deadline = time.time() + 190  # profile's own --duration=3m + margin
    while time.time() < deadline:
        tr.poll(10, interval=3.0, phase="S5_anchor_isolation")
        try:
            info = net_info("engram-node01")
            n_peers = info.get("result", {}).get("n_peers", "?")
            print(f"    [S5] engram-node01 real /net_info n_peers={n_peers}")
        except Exception as e:  # noqa: BLE001 -- best-effort cross-check
            print(f"    [S5] /net_info check failed: {e}")
    cleanup_profile("chaos-eclipse")
    tr.poll(20, phase="S5_cooldown")
    tr.write_scenario_csv("s5_anchor_isolation", "s5")

    print(
        "=== S6: combined BTC+DA failure (also the live regression check for "
        "anchor.go/btcGapMetric's error-swallowing fix) ==="
    )
    tr.set_scenario("s6")
    docker("stop", BITCOIN_NODE)
    docker("stop", CELESTIA_BRIDGE)
    tr.poll(90, phase="S6_combined_btc_da_failure")
    tr.write_scenario_csv("s6_combined_btc_da_failure", "s6")

    print(
        "=== S7: recovery to real ANCHORED (requires watch_and_prove.sh running separately) ==="
    )
    tr.set_scenario("s7")
    docker("start", BITCOIN_NODE)
    docker("start", CELESTIA_BRIDGE)
    reached = tr.wait_for_state("ANCHORED", 600, phase="S7_recovery")
    tr.write_scenario_csv("s7_recovery", "s7")

    print(
        f"\n=== DONE: total duration {tr.elapsed():.0f}s, S7 reached ANCHORED: {reached} ==="
    )
    print(f"Total real transitions observed: {len(tr.transitions)}")
    for t, phase, node, frm, to in tr.transitions:
        print(f"  t={t:.0f}s [{phase}] {node}: {frm} -> {to}")


if __name__ == "__main__":
    main()
