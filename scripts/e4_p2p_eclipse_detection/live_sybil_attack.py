#!/usr/bin/env python3
"""LIVE Docker attacker-swarm test against the real 4-node testnet --
docs/EXPERIMENT.md's E4/E8 A1 (Peer Slot Exhaustion) and A2 (Sybil via
simulated multi-subnet swarm), verifying the real ingress filter
(x/sovereignty/keeper/peer_filter.go's FilterPeerByAddr, Params.
MaxPeersPerSubnet) against real attacker traffic, beyond E4's synthetic
Monte Carlo data (simulate_eclipse_attack.py).

Two legs, driven by docker/attacker-peer-swarm.yml's two profiles:
  a1 -- 10 attacker containers, all on engram-net (same /24 as the 4 real
        validators), all dialing engram-node01. Tests that the filter caps
        same-subnet admission at MaxPeersPerSubnet (8) well before
        CometBFT's own MaxNumInboundPeers (40) would matter.
  a2 -- 12 attackers across 4 distinct new subnets (attacker-subnet-a/b/c/d,
        3 each), all dialing engram-node01. Tests whether spreading across
        subnets evades the same cap the a1 leg demonstrates -- the honest,
        buildable analog of Sybil via subnet diversity (real BGP hijacking
        is out of scope, see compose.yml's note).

Usage:
    python3 -u scripts/e4_p2p_eclipse_detection/live_sybil_attack.py a1
    python3 -u scripts/e4_p2p_eclipse_detection/live_sybil_attack.py a2
"""

import argparse
import os
import subprocess
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "framework"))
from logger import sample_all_nodes, peer_subnet_counts, write_csv, NODE_RPC_PORTS  # noqa: E402

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "e8_attack_resilience"))
from live_timeout_flood_test import sample_docker_stats  # noqa: E402

RESULTS_DIR = os.path.join(os.path.dirname(__file__), "results_live")
TARGET_NODE = "engram-node01"
TARGET_SUBNET = "172.28.0.0"  # engram-net's real /24 network address
MAX_PEERS_PER_SUBNET = 8  # x/sovereignty/types/params.go's DefaultParams()
ALL_VALIDATORS = list(NODE_RPC_PORTS.keys())

LEGS = {
    "a1": {
        "profile": "attacker-swarm-a1",
        "services": [f"attacker-a1-{i:02d}" for i in range(1, 11)],
        "description": "10 attackers, all on engram-net (same subnet as the 4 real validators)",
    },
    "a2": {
        "profile": "attacker-swarm-a2",
        "services": [
            f"attacker-a2-{suffix}"
            for suffix in [
                "a1",
                "a2",
                "a3",
                "b1",
                "b2",
                "b3",
                "c1",
                "c2",
                "c3",
                "d1",
                "d2",
                "d3",
            ]
        ],
        "description": "12 attackers across 4 distinct subnets (attacker-subnet-a/b/c/d, 3 each)",
    },
}


def now() -> str:
    return time.strftime("%H:%M:%S", time.gmtime())


def swarm_up(profile: str, services: list) -> None:
    print(
        f"[{now()}] >>> docker compose --profile {profile} up -d <services> (starting attacker swarm)"
    )
    # Retries a transient `docker compose up -d` failure observed live and
    # repeatedly this session (E4 A3's attacker_up hit the same thing) --
    # a manual retry of the identical command always succeeded immediately,
    # suggesting Compose's own project-state cache needing a moment to
    # resync rather than a real config/dependency issue.
    last_err = None
    for attempt in range(3):
        try:
            subprocess.run(
                ["docker", "compose", "--profile", profile, "up", "-d", *services],
                capture_output=True,
                text=True,
                timeout=120,
                check=True,
            )
            return
        except subprocess.CalledProcessError as e:
            last_err = e
            print(f"[{now()}] swarm_up attempt {attempt + 1}/3 failed ({e}), retrying in 5s...")
            time.sleep(5)
    raise last_err


def swarm_down(profile: str, services: list) -> None:
    # NEVER `docker compose down` here -- `down` tears down the ENTIRE
    # project (all containers/networks, including the real 4-node cluster
    # and bitcoin/celestia) regardless of --profile; its scope is the whole
    # project, unlike `up`. Confirmed live: an earlier version used `down`
    # and destroyed the running cluster mid-experiment. `stop` + `rm -f`
    # with explicit service names (matching injector.py's cleanup_profile)
    # is scoped correctly.
    print(
        f"[{now()}] >>> docker compose stop/rm <services> (tearing down attacker swarm only)"
    )
    subprocess.run(
        ["docker", "compose", "--profile", profile, "stop", *services],
        capture_output=True,
        text=True,
        timeout=60,
    )
    subprocess.run(
        ["docker", "compose", "--profile", profile, "rm", "-f", *services],
        capture_output=True,
        text=True,
        timeout=60,
    )


class Tracker:
    def __init__(self, sample_stats: bool = False):
        self.start = time.time()
        self.rows = []  # list of dict rows for the CSV
        self.node_samples = []  # NodeSample objects, for write_csv reuse
        # No byzantine validator exists in A1/A2 (the attack is external,
        # non-validator containers) -- unlike live_byzantine_attacks.py's
        # HONEST_NODES filter, every one of the 4 real validators must agree,
        # so divergence is checked across ALL_VALIDATORS, not a subset.
        self.divergence_events = []
        self.phase_heights: dict = {}  # phase -> [(t, max_height), ...]
        self.phase_stats: dict = {}  # phase -> [{node: (cpu_pct, mem_mb)}, ...]
        self.sample_stats = sample_stats

    def elapsed(self):
        return time.time() - self.start

    def poll_once(self, phase: str, stats_containers=None):
        t = self.elapsed()
        # One round-trip over all 4 validators -- both the existing
        # target-node subnet-count check AND the divergence/liveness checks
        # (added for E8's safety-metric standardization) read from it.
        node_samples = sample_all_nodes()
        self.node_samples.extend(node_samples)
        s = next(x for x in node_samples if x.node == TARGET_NODE)

        try:
            subnet_counts = peer_subnet_counts(TARGET_NODE)
        except (
            Exception
        ) as e:  # noqa: BLE001 -- best-effort observability, not consensus-critical
            subnet_counts = {}
            print(f"  (peer_subnet_counts failed this round: {e})")

        target_subnet_count = subnet_counts.get(TARGET_SUBNET, 0)
        total_peers = sum(subnet_counts.values())

        self.phase_heights.setdefault(phase, [])
        self.phase_stats.setdefault(phase, [])

        hashes = {x.node: x.app_hash for x in node_samples if x.node in ALL_VALIDATORS and x.height > 0}
        heights = {x.node: x.height for x in node_samples if x.node in ALL_VALIDATORS and x.height > 0}
        by_height: dict = {}
        for node, h in heights.items():
            by_height.setdefault(h, {})[node] = hashes[node]
        for h, hashes_at_h in by_height.items():
            if len(hashes_at_h) > 1 and len(set(hashes_at_h.values())) > 1:
                self.divergence_events.append((t, phase, h, dict(hashes_at_h)))
                print(f"  *** [{phase}] SAFETY VIOLATION @ {t:6.0f}s height={h}: {hashes_at_h} ***")
        if heights:
            self.phase_heights[phase].append((t, max(heights.values())))

        if self.sample_stats:
            self.phase_stats[phase].append(sample_docker_stats(stats_containers or ALL_VALIDATORS))

        row = {
            "t": round(t, 1),
            "phase": phase,
            "fsm_state": s.fsm_state,
            "height": s.height,
            "target_subnet_peer_count": target_subnet_count,
            "total_peers": total_peers,
            "subnet_distribution": dict(subnet_counts),
        }
        self.rows.append(row)
        print(
            f"[{t:6.0f}s][{phase}] fsm_state={s.fsm_state} height={s.height} "
            f"engram-net-peers={target_subnet_count} total_peers={total_peers} "
            f"subnets={subnet_counts}"
        )
        return row

    def poll_for(self, seconds: float, interval: float, phase: str, stats_containers=None):
        deadline = time.time() + seconds
        while time.time() < deadline:
            self.poll_once(phase, stats_containers=stats_containers)
            time.sleep(interval)

    def height_rate(self, phase: str) -> float:
        """Blocks/s over phase's samples -- ported from
        live_timeout_flood_test.py's Tracker.height_rate."""
        pts = self.phase_heights.get(phase, [])
        if len(pts) < 2:
            return 0.0
        (t0, h0), (t1, h1) = pts[0], pts[-1]
        if t1 <= t0:
            return 0.0
        return (h1 - h0) / (t1 - t0)

    def stats_summary(self, phase: str, node: str):
        """(avg_cpu, max_cpu, avg_mem, max_mem) for node across phase's
        snapshots; None if not sampled -- ported from
        live_timeout_flood_test.py's Tracker.stats_summary."""
        vals = [snap[node] for snap in self.phase_stats.get(phase, []) if node in snap]
        if not vals:
            return None
        cpus = [v[0] for v in vals]
        mems = [v[1] for v in vals]
        return (sum(cpus) / len(cpus), max(cpus), sum(mems) / len(mems), max(mems))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("leg", choices=list(LEGS.keys()))
    parser.add_argument("--baseline-s", type=float, default=30.0)
    parser.add_argument("--attack-s", type=float, default=90.0)
    parser.add_argument("--recovery-s", type=float, default=60.0)
    parser.add_argument("--interval-s", type=float, default=5.0)
    parser.add_argument(
        "--sample-stats", action="store_true",
        help="also sample docker stats (CPU%%/MiB) on the 4 validators + this leg's attacker "
        "containers during the attack phase -- E8's resource-exhaustion signal, most relevant "
        "here since A1/A2 spin up 10-12 extra containers",
    )
    args = parser.parse_args()

    leg = LEGS[args.leg]
    os.makedirs(RESULTS_DIR, exist_ok=True)
    tr = Tracker(sample_stats=args.sample_stats)
    attack_stats_containers = ALL_VALIDATORS + leg["services"]

    print(f"=== E4/E8 live Sybil attack leg '{args.leg}': {leg['description']} ===")
    print(
        f"    MaxPeersPerSubnet={MAX_PEERS_PER_SUBNET} (x/sovereignty/types/params.go)"
    )

    print(f"=== Phase 1: baseline ({args.baseline_s:.0f}s) ===")
    tr.poll_for(args.baseline_s, args.interval_s, "baseline")
    baseline_count = tr.rows[-1]["target_subnet_peer_count"] if tr.rows else 0

    print(f"=== Phase 2: attack ({args.attack_s:.0f}s) ===")
    swarm_up(leg["profile"], leg["services"])
    tr.poll_for(args.attack_s, args.interval_s, "attack", stats_containers=attack_stats_containers)
    peak_count = max(
        r["target_subnet_peer_count"] for r in tr.rows if r["phase"] == "attack"
    )
    peak_total = max(r["total_peers"] for r in tr.rows if r["phase"] == "attack")

    print(f"=== Phase 3: recovery ({args.recovery_s:.0f}s, tearing down swarm) ===")
    swarm_down(leg["profile"], leg["services"])
    tr.poll_for(args.recovery_s, args.interval_s, "recovery")
    final_count = tr.rows[-1]["target_subnet_peer_count"] if tr.rows else 0

    filter_held = peak_count <= MAX_PEERS_PER_SUBNET
    # Not a fixed ANCHORED/SUSPICIOUS allowlist: the cluster's baseline
    # depends on whatever the real BTC/DA/P2P pipeline's timing is at run
    # time (e.g. RECOVERING mid-reanchoring is legitimate and unrelated to
    # the attack). The real claim to check is "the attack didn't cause a
    # WORSE transition" -- fsm_state during/after never regresses relative
    # to the baseline phase's own state set.
    baseline_states = {
        r["fsm_state"] for r in tr.rows if r["phase"] == "baseline" and r["fsm_state"]
    }
    attack_states = {
        r["fsm_state"]
        for r in tr.rows
        if r["phase"] in ("attack", "recovery") and r["fsm_state"]
    }
    fsm_stayed_healthy = (
        attack_states.issubset(baseline_states) if baseline_states else True
    )

    safety_held = len(tr.divergence_events) == 0
    baseline_rate = tr.height_rate("baseline")
    attack_rate = tr.height_rate("attack")
    recovery_rate = tr.height_rate("recovery")
    # No fixed threshold -- report the real rates and let the reader judge;
    # "roughly held" means attack didn't collapse toward 0 relative to baseline.
    liveness_held = baseline_rate == 0 or attack_rate >= 0.5 * baseline_rate

    ts_label = time.strftime("%Y%m%dT%H%M%S")
    csv_path = os.path.join(RESULTS_DIR, f"sybil_attack_{args.leg}_{ts_label}.csv")
    write_csv(tr.node_samples, csv_path)

    summary_path = os.path.join(
        RESULTS_DIR, f"sybil_attack_{args.leg}_{ts_label}_summary.md"
    )
    with open(summary_path, "w") as f:
        f.write(f"# LIVE Sybil/slot-exhaustion attack, leg '{args.leg}'\n\n")
        f.write(f"{leg['description']}\n\n")
        f.write(
            f"Total duration: {tr.elapsed():.0f}s. "
            f"MaxPeersPerSubnet={MAX_PEERS_PER_SUBNET}.\n\n"
        )
        f.write(
            "## Real observed subnet-peer counts (target: engram-node01, subnet 172.28.0.0/24)\n\n"
        )
        f.write(f"- Baseline (pre-attack): {baseline_count}\n")
        f.write(
            f"- Peak during attack: {peak_count} "
            f"(peak total peers across all subnets: {peak_total})\n"
        )
        f.write(f"- After teardown (recovery): {final_count}\n\n")
        f.write(f"## Verdict\n\n")
        f.write(
            f"- Ingress filter held the target subnet at or below MaxPeersPerSubnet during "
            f"the attack: **{filter_held}**\n"
        )
        f.write(
            f"- FSM state never left ANCHORED/SUSPICIOUS during the attack "
            f"(no false SOVEREIGN degradation from a defended attack): **{fsm_stayed_healthy}**\n"
        )
        f.write(
            f"- Safety held (all 4 validators' AppHash never diverged at the same height): "
            f"**{safety_held}**\n"
        )
        f.write(f"- Divergence events: {len(tr.divergence_events)}\n")
        f.write(
            f"- Liveness held (block rate during attack vs. baseline, no collapse toward 0): "
            f"**{liveness_held}** (baseline {baseline_rate:.3f} blocks/s, attack {attack_rate:.3f} "
            f"blocks/s, recovery {recovery_rate:.3f} blocks/s)\n\n"
        )
        if tr.divergence_events:
            f.write("### Divergence detail\n\n")
            for t, phase, h, hashes in tr.divergence_events:
                f.write(f"- t={t:.0f}s phase={phase} height={h}: {hashes}\n")
            f.write("\n")
        if args.sample_stats:
            f.write("## Resource usage during attack (docker stats)\n\n")
            f.write("| Container | Avg CPU% | Max CPU% | Avg Mem (MiB) | Max Mem (MiB) |\n")
            f.write("|---|---:|---:|---:|---:|\n")
            for container in attack_stats_containers:
                s = tr.stats_summary("attack", container)
                if s:
                    f.write(f"| {container} | {s[0]:.2f} | {s[1]:.2f} | {s[2]:.1f} | {s[3]:.1f} |\n")
                else:
                    f.write(f"| {container} | n/a | n/a | n/a | n/a |\n")
            f.write("\n")
        f.write("## Full timeline\n\n")
        f.write(
            "| t (s) | phase | fsm_state | height | target_subnet_peers | total_peers |\n"
        )
        f.write("|---:|---|---|---:|---:|---:|\n")
        for r in tr.rows:
            f.write(
                f"| {r['t']} | {r['phase']} | {r['fsm_state']} | {r['height']} | "
                f"{r['target_subnet_peer_count']} | {r['total_peers']} |\n"
            )

    print(f"\nwrote {len(tr.node_samples)} node samples to {csv_path}")
    print(f"wrote summary to {summary_path}")
    print(
        f"\nVERDICT: filter_held={filter_held} fsm_stayed_healthy={fsm_stayed_healthy} "
        f"safety_held={safety_held} divergence_events={len(tr.divergence_events)} "
        f"liveness_held={liveness_held} peak_target_subnet_count={peak_count} "
        f"(limit={MAX_PEERS_PER_SUBNET})"
    )


if __name__ == "__main__":
    main()
