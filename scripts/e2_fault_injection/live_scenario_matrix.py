#!/usr/bin/env python3
"""LIVE E2 fault-injection scenario matrix against the real 4-node testnet --
docs/EXPERIMENT.md's S1-S7, driven by REAL docker fault injection (not
tests/e2e's in-process mock harness).

One continuous 7-phase run (chains phases like live_lifecycle_test.py, since
a cold restabilization between 7 INDEPENDENT runs costs ~250-300s of peer-tenure
buildup each). Each phase's "before" is the previous phase's healed "after".

    S1 baseline        -- no injection, confirm ANCHORED
    S2 BTC congestion   -- pause checkpoint submission (AnchorTracker's
                           ANCHOR_SUBMISSION_PAUSED_FILE): h_btc_anchored
                           freezes while h_btc_current climbs, growing btc_gap.
    S3 DA unavailable   -- docker stop/start celestia-bridge.
    S4 P2P eclipse partial -- 300ms+-50ms delay on all 6 real
                           validator-link-01-0N interfaces to/from node01
                           (see NODE01_P2P_LINK_TARGETS's doc).
    S5 Anchor isolation -- existing chaos-eclipse profile (100% loss on
                           engram-node01, 3m); ActiveAnchors is not
                           confirmable via Query.State (stale field, see
                           logger.py), so cross-checks via /net_info.
    S6 combined BTC+DA failure -- docker stop bitcoin-node01 AND
                           celestia-bridge. Doubles as the live regression
                           check for anchor.go/btcGapMetric's error-swallowing
                           fix (previously only unit-tested).
    S7 recovery         -- restart everything, run to real ANCHORED via the
                           real ZK re-anchoring pipeline. REQUIRES
                           scripts/reanchoring_prover/watch_and_prove.sh
                           running separately in the background.

Output CSVs named s{N}_<name>.csv, matching tests/e2e/results/s*.csv's
naming so measure_latency_live.py can read them with the same glob.

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

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))
from utils import (  # noqa: E402
    setup_academic_plot_style,
    figsize_single,
    FIG_WIDTH_DOUBLE_COL,
    savefig_academic,
)

import matplotlib.pyplot as plt  # noqa: E402

RESULTS_DIR = os.path.join(os.path.dirname(__file__), "results_live")
# Figures (unlike raw CSVs/markdown) land alongside live_figure_builder.py's
# output, not results_live/ -- one figures directory for all of E2's charts.
FIGURES_DIR = os.path.join(os.path.dirname(__file__), "results")
CELESTIA_BRIDGE = "celestia-bridge"
BITCOIN_NODE = "bitcoin-node01"
ENGRAM_NODES = ["engram-node01", "engram-node02", "engram-node03", "engram-node04"]
ANCHOR_SUBMISSION_PAUSED_FILE = "/tmp/anchor_submission_paused"
NET_INFO_LOG = os.path.join(RESULTS_DIR, "s4_s5_net_info_check.md")


def block_interval_stats(samples_for_node):
    """Block-interval throughput/latency proxy (docs/EXPERIMENT.md's E2
    Metrics table): consecutive-sample time deltas where height actually
    increased, so it measures real block cadence, not polling-interval noise.
    Not per-round consensus latency (fixed-interval RPC polling can't see
    sub-interval timing) -- an honest proxy, stated as such in the docs.
    """
    samples_for_node = sorted(samples_for_node, key=lambda s: s.timestamp)
    deltas = []
    for prev, cur in zip(samples_for_node, samples_for_node[1:]):
        if cur.height > prev.height:
            deltas.append((cur.timestamp - prev.timestamp) / (cur.height - prev.height))
    if not deltas:
        return None
    deltas.sort()
    n = len(deltas)
    mean = sum(deltas) / n
    p50 = deltas[n // 2]
    p95 = deltas[min(n - 1, int(n * 0.95))]
    return {"mean_s": mean, "p50_s": p50, "p95_s": p95, "n_intervals": n}


# S4's real P2P eclipse mechanism: delay on node01's 3 dedicated pairwise-link
# interfaces to its peers AND each peer's own link back to node01 --
# bidirectional, all 3 links. Pumba's netem defaults to eth0, which on this
# cluster maps to bitcoin-net for every validator (never a P2P-carrying
# interface), so eth0-only loss/delay produced zero FSM-visible effect even
# at 40% loss; the real inter-validator gossip runs over
# validator-link-01-0N (docs/ARCHITECTURE.md's pairwise-link topology), and
# targeting these 6 links produces the full ANCHORED->SUSPICIOUS->SOVEREIGN
# ->RECOVERING->ANCHORED cycle, matching the in-process gray-failure-timeout
# finding. One-sided delay (only node01's own egress) is NOT enough for the
# CONSECUTIVE-streak-gated absorb (DownHysteresisThreshold) to reliably trip:
# node01's degraded view only enters a proposal on node01's round-robin turn,
# so bidirectional delay keeps every proposer's own view degraded every round.
NODE01_P2P_LINK_TARGETS = [
    ("engram-node01", "eth3"),  # node01 -> node02
    ("engram-node01", "eth4"),  # node01 -> node03
    ("engram-node01", "eth5"),  # node01 -> node04
    ("engram-node02", "eth2"),  # node02 -> node01
    ("engram-node03", "eth4"),  # node03 -> node01
    ("engram-node04", "eth3"),  # node04 -> node01
]


def start_p2p_eclipse_partial(delay_ms=300, jitter_ms=50, duration="2m"):
    for container, iface in NODE01_P2P_LINK_TARGETS:
        subprocess.run(
            [
                "docker", "run", "-d", "--rm",
                "-v", "/var/run/docker.sock:/var/run/docker.sock",
                "gaiaadm/pumba:latest",
                "netem", f"--duration={duration}", "--interface", iface,
                "delay", f"--time={delay_ms}", f"--jitter={jitter_ms}",
                container,
            ],
            capture_output=True, text=True, timeout=30,
        )


def cleanup_p2p_eclipse_partial():
    r = subprocess.run(
        ["docker", "ps", "-q", "--filter", "ancestor=gaiaadm/pumba:latest"],
        capture_output=True, text=True,
    )
    for cid in r.stdout.split():
        subprocess.run(["docker", "stop", cid], capture_output=True, timeout=30)


def now() -> str:
    return time.strftime("%H:%M:%S", time.gmtime())


def docker(*args):
    subprocess.run(["docker", *args], capture_output=True, text=True, timeout=30)


class Tracker:
    def __init__(self):
        self.start = time.time()
        # (scenario_key, NodeSample) -- scenario_key set by set_scenario(), so
        # each write_scenario_csv() writes ONLY that scenario's own samples
        # (per-scenario files, not a cumulative dump).
        self.samples = []
        self.transitions = []
        self.last_state = {}
        self.phase = "init"
        self.scenario_key = "init"
        self.throughput_stats = {}

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
        every phase run so far."""
        path = os.path.join(RESULTS_DIR, f"{name}.csv")
        scenario_samples = [s for key, s in self.samples if key == scenario_key]
        write_csv(scenario_samples, path)
        print(
            f"wrote {len(scenario_samples)} samples (scenario={scenario_key}) to {path}"
        )
        self.throughput_stats[scenario_key] = self._block_interval_summary(scenario_samples)
        return path

    def _block_interval_summary(self, scenario_samples):
        """Aggregates block_interval_stats across all 4 nodes for one
        scenario -- one throughput/latency-proxy number per scenario (all 4
        validators stay in lockstep in every measured run)."""
        by_node = {}
        for s in scenario_samples:
            by_node.setdefault(s.node, []).append(s)
        all_stats = [block_interval_stats(v) for v in by_node.values()]
        all_stats = [s for s in all_stats if s]
        if not all_stats:
            return None
        return {
            "mean_s": sum(s["mean_s"] for s in all_stats) / len(all_stats),
            "p50_s": sum(s["p50_s"] for s in all_stats) / len(all_stats),
            "p95_s": max(s["p95_s"] for s in all_stats),
            "n_nodes": len(all_stats),
        }

    def write_throughput_summary(self, path):
        with open(path, "w") as f:
            f.write("# E2 live throughput/latency (block-interval proxy)\n\n")
            f.write(
                "Mean/p50/p95 seconds between consecutive height increments, averaged "
                "across all 4 validators per scenario (p95 is the max across nodes). "
                "Fixed-interval RPC polling, not per-round consensus timing -- a "
                "throughput proxy, not a true latency percentile.\n\n"
            )
            f.write("| Scenario | Mean (s) | p50 (s) | p95 (s) |\n")
            f.write("|---|---:|---:|---:|\n")
            for key, stats in self.throughput_stats.items():
                if not stats:
                    f.write(f"| {key} | n/a | n/a | n/a |\n")
                    continue
                f.write(
                    f"| {key} | {stats['mean_s']:.2f} | {stats['p50_s']:.2f} | {stats['p95_s']:.2f} |\n"
                )
        print(f"wrote throughput/latency summary to {path}")


def plot_latency_whiskers(throughput_stats, out_dir):
    """Combined p50-bar / p95-whisker / mean-marker latency chart -- one
    view instead of 3 separate Mean/p50/p95 panels. Linear, full-range
    y-axis (no truncated axis): S4's dramatic p95 tail is the real signal
    for the graceful-degradation-under-fault narrative, not noise to hide."""
    setup_academic_plot_style()
    order = ["s1", "s2", "s3", "s4", "s5", "s6", "s7"]
    keys = [k for k in order if throughput_stats.get(k)]
    names = [k.upper() for k in keys]
    p50s = [throughput_stats[k]["p50_s"] for k in keys]
    p95s = [throughput_stats[k]["p95_s"] for k in keys]
    means = [throughput_stats[k]["mean_s"] for k in keys]
    x = list(range(len(keys)))

    fig, ax = plt.subplots(figsize=figsize_single(width=FIG_WIDTH_DOUBLE_COL))
    ax.bar(x, p50s, width=0.5, color="#2a78d6", zorder=3, label="p50 (typical)")
    ax.errorbar(
        x, p50s,
        yerr=[[0.0] * len(keys), [p95 - p50 for p50, p95 in zip(p50s, p95s)]],
        fmt="none", ecolor="#52514e", elinewidth=1.5, capsize=6, capthick=1.5,
        zorder=4,
    )
    ax.scatter(x, means, marker="D", s=26, color="#52514e", zorder=5, label="Mean")

    if throughput_stats.get("s1"):
        s1_p50 = throughput_stats["s1"]["p50_s"]
        ax.axhline(s1_p50, color="#c3c2b7", linestyle="--", linewidth=1, zorder=1)
        ax.text(-0.4, s1_p50, f"S1 baseline p50 = {s1_p50:.2f}s",
                fontsize=8, color="#898781", va="bottom", ha="left")

    if "s4" in keys:
        i4 = keys.index("s4")
        ax.text(i4, p95s[i4], f"p95={p95s[i4]:.2f}s", fontsize=8,
                color="#52514e", va="bottom", ha="center")

    ax.set_xticks(x)
    ax.set_xticklabels(names)
    ax.set_ylabel("Block interval (s)")
    ax.set_ylim(0, max(p95s) * 1.15)
    ax.set_title(
        "E2 Block-Interval Latency by Scenario\n"
        "(bar = p50, whisker = p95 tail, diamond = mean; live 4-node Docker testnet)",
        fontsize=10,
    )
    ax.legend(loc="upper left", fontsize=8, frameon=False)
    fig.tight_layout()
    savefig_academic(fig, out_dir, "figure3_latency_whiskers_live")
    print(f"wrote latency whisker chart to {out_dir}/figure3_latency_whiskers_live.{{png,pdf}}")


def main():
    os.makedirs(RESULTS_DIR, exist_ok=True)
    os.makedirs(FIGURES_DIR, exist_ok=True)
    tr = Tracker()

    print("=== S1: baseline ===")
    tr.set_scenario("s1")
    tr.poll(15, phase="S1_baseline")
    tr.write_scenario_csv("s1_normal", "s1")

    print(
        "=== S2: BTC congestion (checkpoint-submission pause, confirmed live "
        "to grow btc_gap and force SOVEREIGN -- see docs/EXPERIMENT.md's S2 "
        "writeup) ==="
    )
    # Two earlier mechanisms were confirmed live NOT to grow btc_gap:
    # chaos-btc-delay's RPC-level netem delay (~40x smaller than the 20s
    # natural cadence, doesn't touch block-height deltas) and a global
    # mining-rate slowdown (h_btc_current and h_btc_anchored derive from the
    # same slowed stream, staying in sync). What works:
    # AnchorTracker.SetSubmissionPausedFile (x/anchor/anchor.go) -- MaybeSubmit
    # skips NEW checkpoints while the file exists, freezing h_btc_anchored
    # while h_btc_current climbs. Grew btc_gap to 12 (past SovereignThreshold=8)
    # and committed all 4 validators to SOVEREIGN in lockstep.
    tr.set_scenario("s2")
    for node in ENGRAM_NODES:
        docker("exec", node, "touch", ANCHOR_SUBMISSION_PAUSED_FILE)
    tr.poll(180, interval=3.0, phase="S2_btc_congestion")
    for node in ENGRAM_NODES:
        docker("exec", node, "rm", "-f", ANCHOR_SUBMISSION_PAUSED_FILE)
    # wait_for_state, not a fixed poll -- same reasoning as S4/S5's cooldown
    # fix below: a fixed window isn't guaranteed long enough for every
    # validator to reach ANCHORED before S3 starts.
    tr.wait_for_state("ANCHORED", 90, phase="S2_btc_congestion_cooldown")
    tr.write_scenario_csv("s2_btc_congestion", "s2")

    print("=== S3: DA unavailable (docker stop celestia-bridge) ===")
    tr.set_scenario("s3")
    docker("stop", CELESTIA_BRIDGE)
    tr.poll(60, phase="S3_da_unavailable")
    docker("start", CELESTIA_BRIDGE)
    tr.wait_for_state("ANCHORED", 90, phase="S3_da_recovery")
    tr.write_scenario_csv("s3_da_unavailable", "s3")

    net_info_log = open(NET_INFO_LOG, "w")
    net_info_log.write("# S4/S5 real /net_info peer-count cross-check\n\n")
    net_info_log.write(
        "Query.State never carries ActiveAnchors/p2p_healthy (documented stale field, "
        "logger.py's _decode_query_state) -- this cross-checks engram-node01's real "
        "connected-peer count via CometBFT's own /net_info RPC during the isolation "
        "window, independent of the app-level FSM state.\n\n"
    )
    net_info_log.write("| t (s) | phase | n_peers |\n|---:|---|---:|\n")

    def log_net_info(phase: str):
        try:
            info = net_info("engram-node01")
            n_peers = info.get("result", {}).get("n_peers", "?")
            print(f"    [{phase}] engram-node01 real /net_info n_peers={n_peers}")
            net_info_log.write(f"| {tr.elapsed():.0f} | {phase} | {n_peers} |\n")
            net_info_log.flush()
        except Exception as e:  # noqa: BLE001 -- best-effort cross-check
            print(f"    [{phase}] /net_info check failed: {e}")
            net_info_log.write(f"| {tr.elapsed():.0f} | {phase} | error: {e} |\n")

    print(
        "=== S4: P2P eclipse partial (300ms+-50ms delay on all 6 real "
        "validator-link-01-0N interfaces to/from node01) -- cross-checking via "
        "/net_info too ==="
    )
    tr.set_scenario("s4")
    wait_for_no_active_netem()
    start_p2p_eclipse_partial()
    deadline = time.time() + 130  # profile's own --duration=2m + margin
    while time.time() < deadline:
        tr.poll(10, interval=3.0, phase="S4_p2p_eclipse_partial")
        log_net_info("S4_p2p_eclipse_partial")
    cleanup_p2p_eclipse_partial()
    # wait_for_state, not a fixed poll: a fixed 20s cooldown isn't always
    # enough for every validator to reach ANCHORED before S5 -- node01 (S5's
    # isolation target) can still be mid-RECOVERING from S4, confounding S5
    # (its slow recovery becomes unattributable between the two phases).
    tr.wait_for_state("ANCHORED", 90, phase="S4_p2p_cooldown")
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
        log_net_info("S5_anchor_isolation")
    cleanup_profile("chaos-eclipse")
    # Same reasoning as S4's cooldown: S6 stops real infrastructure
    # (bitcoin-node01, celestia-bridge); starting it against a cluster that
    # hasn't fully reconverged from S5 would confound S6's result.
    tr.wait_for_state("ANCHORED", 90, phase="S5_cooldown")
    tr.write_scenario_csv("s5_anchor_isolation", "s5")
    net_info_log.close()

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

    tr.write_throughput_summary(os.path.join(RESULTS_DIR, "s2e_throughput_latency.md"))
    plot_latency_whiskers(tr.throughput_stats, FIGURES_DIR)

    print(
        f"\n=== DONE: total duration {tr.elapsed():.0f}s, S7 reached ANCHORED: {reached} ==="
    )
    print(f"Total real transitions observed: {len(tr.transitions)}")
    for t, phase, node, frm, to in tr.transitions:
        print(f"  t={t:.0f}s [{phase}] {node}: {frm} -> {to}")


if __name__ == "__main__":
    main()
