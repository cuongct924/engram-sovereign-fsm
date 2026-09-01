#!/usr/bin/env python3
"""LIVE E9 combined-failure trace against the real 4-node testnet --
docs/EXPERIMENT.md's E9 capstone: ONE continuous run layering multiple
overlapping peripheral failures (not sequential isolated toggles like
live_lifecycle_test.py), a genuinely compounding stress trace across all 3
peripheral systems.

Run LAST, after these prerequisite mechanisms have each been individually
validated in isolation -- betting a long combined run on unproven mechanisms
compounds the risk of an inconclusive result:
  - live_scenario_matrix.py's S2 leg (confirms chaos-btc-delay actually
    produces a gradual btc_gap ramp).
  - live_sybil_attack.py or injector.toggle_profile_bursts (confirms real
    P2P churn behavior).

Layering, one continuous timeline (not sequential):
  1. chaos-btc-delay ramp starts (BTC congestion pressure builds).
  2. WHILE BTC pressure is active and the FSM has likely reached SOVEREIGN,
     overlay docker stop celestia-bridge (DA outage layered ON TOP).
  3. WHILE both are still active, overlay a P2P churn burst
     (injector.toggle_profile_bursts("chaos-loss", ...)) -- three
     simultaneous fault classes at once.
  4. Heal in reverse order: P2P churn stops, then DA restored, then BTC
     delay profile expires/is cleaned up.
  5. Recovery to real ANCHORED via the real ZK re-anchoring pipeline
     (REQUIRES scripts/reanchoring_prover/watch_and_prove.sh running in the
     background separately, same requirement as live_lifecycle_test.py's/
     live_scenario_matrix.py's final phase).

Usage:
    python3 -u scripts/e9_trace_driven/live_combined_trace.py
    python3 -u scripts/e9_trace_driven/live_combined_trace.py --wan-profile none  # old uniform-LAN behavior
"""

import argparse
import os
import subprocess
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "framework"))
from logger import sample_all_nodes, write_csv, NODE_RPC_PORTS  # noqa: E402
from injector import (
    start_pumba_profile,
    cleanup_profile,
    wait_for_no_active_netem,
    start_pumba_wan_profile,
    cleanup_wan_profile,
)  # noqa: E402
from sensor_log_scraper import scrape_sensor_snapshots, nearest_timestamp  # noqa: E402

RESULTS_DIR = os.path.join(os.path.dirname(__file__), "results_live")
CELESTIA_BRIDGE = "celestia-bridge"
BITCOIN_NODE = "bitcoin-node01"
ALL_NODES = list(NODE_RPC_PORTS.keys())


def now() -> str:
    return time.strftime("%H:%M:%S", time.gmtime())


def docker(*args):
    subprocess.run(["docker", *args], capture_output=True, text=True, timeout=30)


class Tracker:
    def __init__(self):
        self.start = time.time()
        self.samples = []
        self.transitions = []
        self.last_state = {}
        self.phase = "init"
        self.markers = (
            []
        )  # (t, event) for fault-injection/healing events, for the timeline figure

    def elapsed(self):
        return time.time() - self.start

    def mark(self, event: str):
        t = self.elapsed()
        self.markers.append((t, event))
        print(f"[{t:6.0f}s] >>> {event}")

    def poll(self, seconds, interval=3.0, phase=None):
        if phase:
            self.phase = phase
        deadline = time.time() + seconds
        while time.time() < deadline:
            round_samples = sample_all_nodes()
            self.samples.extend(round_samples)
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
            self.samples.extend(round_samples)
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


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument(
        "--wan-profile",
        choices=["none", "chaos-wan-latency", "chaos-wan-loss"],
        default="chaos-wan-latency",
        help="per-validator WAN-realism baseline (each of the 4 validators gets a DIFFERENT "
        "delay/jitter or loss%%, approximating distinct real regions) held for the whole trace "
        "EXCEPT Phase 4 -- chaos-wan-* and chaos-loss both add a root netem qdisc on the same "
        "interfaces, so the WAN baseline is paused before Phase 4 and resumed after. 'none' "
        "reproduces the old uniform-LAN behavior. The profile's own --duration is 10 minutes; a "
        "slow Phase 7 can outlast it, silently reverting to uniform-LAN for the remainder -- not "
        "fatal, Phase 7 only waits for recovery.",
    )
    args = parser.parse_args()

    os.makedirs(RESULTS_DIR, exist_ok=True)
    tr = Tracker()

    wan_active = False
    if args.wan_profile != "none":
        print(f"[{now()}] >>> starting {args.wan_profile} (WAN-realism baseline)")
        start_pumba_wan_profile(args.wan_profile)
        wan_active = True

    print("=== Phase 1: baseline ===")
    tr.poll(15, phase="P1_baseline")

    print("=== Phase 2: BTC pressure begins (chaos-btc-delay) ===")
    wait_for_no_active_netem()
    start_pumba_profile("chaos-btc-delay")
    tr.mark("chaos-btc-delay started (BTC congestion pressure)")
    tr.poll(60, interval=3.0, phase="P2_btc_pressure_building")

    print(
        "=== Phase 3: layer DA outage on top of BTC pressure (docker stop celestia-bridge) ==="
    )
    docker("stop", CELESTIA_BRIDGE)
    tr.mark("celestia-bridge stopped (DA outage layered on BTC pressure)")
    tr.poll(60, phase="P3_btc_plus_da")

    print(
        "=== Phase 4: layer P2P churn burst on top of both (chaos-loss, 3 short cycles) ==="
    )
    if wan_active:
        # chaos-wan-latency/loss and chaos-loss both add a root netem qdisc
        # on the same engram-node01/02 interfaces -- a second root qdisc add
        # fails outright, so the WAN baseline must step aside for this phase.
        print(f"[{now()}] >>> pausing {args.wan_profile} for Phase 4 (root qdisc conflict with chaos-loss)")
        cleanup_wan_profile(args.wan_profile)
    tr.mark("chaos-loss churn burst starting (3rd simultaneous fault class)")
    for i in range(3):
        wait_for_no_active_netem()
        start_pumba_profile("chaos-loss")
        tr.poll(20, interval=2.0, phase=f"P4_triple_fault_churn_cycle_{i + 1}_on")
        cleanup_profile("chaos-loss")
        tr.poll(10, interval=2.0, phase=f"P4_triple_fault_churn_cycle_{i + 1}_off")
    tr.mark("P2P churn burst complete")
    if wan_active:
        print(f"[{now()}] >>> resuming {args.wan_profile} after Phase 4")
        wait_for_no_active_netem()
        start_pumba_wan_profile(args.wan_profile)

    print("=== Phase 5: hold at peak triple-fault state briefly ===")
    tr.poll(30, phase="P5_peak_triple_fault")

    print("=== Phase 6: heal in reverse order ===")
    cleanup_profile("chaos-btc-delay")
    tr.mark("chaos-btc-delay cleaned up")
    docker("start", CELESTIA_BRIDGE)
    tr.mark("celestia-bridge restarted")
    tr.poll(30, phase="P6_healing")

    print(
        "=== Phase 7: recovery to real ANCHORED (requires watch_and_prove.sh running separately) ==="
    )
    reached = tr.wait_for_state("ANCHORED", 600, phase="P7_recovery")

    if wan_active:
        print(f"[{now()}] >>> stopping {args.wan_profile} (WAN-realism baseline)")
        cleanup_wan_profile(args.wan_profile)

    ts_label = time.strftime("%Y%m%dT%H%M%S")
    csv_path = os.path.join(RESULTS_DIR, f"e9_combined_trace_{ts_label}.csv")
    write_csv(tr.samples, csv_path)

    summary_path = os.path.join(RESULTS_DIR, f"e9_combined_trace_{ts_label}_summary.md")
    with open(summary_path, "w") as f:
        f.write(
            "# LIVE E9 combined-failure trace (BTC + DA + P2P churn, overlapping)\n\n"
        )
        f.write(
            f"Total duration: {tr.elapsed():.0f}s. Final phase reached ANCHORED: {reached}. "
            f"WAN-realism baseline: {args.wan_profile}.\n\n"
        )
        f.write("## Fault-injection / healing event markers\n\n")
        f.write("| t (s) | Event |\n|---:|---|\n")
        for t, event in tr.markers:
            f.write(f"| {t:.0f} | {event} |\n")
        f.write("\n## Real transitions observed\n\n")
        f.write("| t (s) | Phase | Node | From | To |\n|---:|---|---|---|---|\n")
        for t, phase, node, frm, to in tr.transitions:
            f.write(f"| {t:.0f} | {phase} | {node} | {frm} | {to} |\n")

    print(f"\nwrote {len(tr.samples)} samples to {csv_path}")
    print(f"wrote summary to {summary_path}")

    # Diagnostic-only per-validator BTC/DA/P2P sensor reads, scraped from
    # x/sovereignty/sensors_refresh.go's sensor_snapshot log line (never
    # committed state -- see the note below). Best-effort: a scrape failure
    # or empty result just means these 3 extra panels won't be available,
    # not a run failure.
    sensor_rows = []
    for node in ALL_NODES:
        node_samples = [s for s in tr.samples if s.node == node]
        # scrape_sensor_snapshots reads the container's ENTIRE log history
        # (its own doc), not just this run -- a long-lived container carries
        # sensor_snapshot lines from every prior run/session. Dropping rows
        # outside this run's real sampled height range keeps stale heights
        # (e.g. genesis-era startup noise) out of nearest_timestamp, which
        # would otherwise snap them to a misleadingly plausible in-run time.
        heights_this_run = {s.height for s in node_samples}
        lo, hi = min(heights_this_run), max(heights_this_run)
        for row in scrape_sensor_snapshots(node):
            if not (lo <= row["height"] <= hi):
                continue
            row["approx_timestamp"] = nearest_timestamp(row["height"], node_samples)
            sensor_rows.append(row)

    sensors_csv_path = os.path.join(RESULTS_DIR, f"e9_combined_trace_{ts_label}_sensors.csv")
    if sensor_rows:
        import csv as csv_mod

        with open(sensors_csv_path, "w", newline="") as f:
            writer = csv_mod.DictWriter(f, fieldnames=list(sensor_rows[0].keys()))
            writer.writeheader()
            writer.writerows(sensor_rows)
        print(f"wrote {len(sensor_rows)} diagnostic sensor_snapshot rows to {sensors_csv_path}")
    else:
        print(
            "no sensor_snapshot log lines scraped (empty docker logs, or the deployed engramd "
            "predates this diagnostic line) -- BTC/DA/P2P panels will be unavailable this run."
        )

    print(
        f"\nNote: BTC gap/DA gap/P2P health are each validator's own LOCAL sensor reads, never "
        f"committed state (x/sovereignty/preblock.go's NewPreBlocker doc) -- they can legitimately "
        f"differ between validators, unlike every other metric here. {sensors_csv_path if sensor_rows else '(not written this run)'} "
        f"holds them as diagnostic-only data, scraped from `docker logs`, height-keyed rather than "
        f"timestamped (approx_timestamp is derived from the nearest real RPC sample at that "
        f"height). live_figure_builder.py plots them as 3 additional panels, clearly labeled as "
        f"local/diagnostic, not committed/agreed data like the other 6."
    )


if __name__ == "__main__":
    main()
