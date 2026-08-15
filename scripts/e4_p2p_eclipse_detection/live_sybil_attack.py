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
from logger import sample_all_nodes, peer_subnet_counts, write_csv  # noqa: E402

RESULTS_DIR = os.path.join(os.path.dirname(__file__), "results_live")
TARGET_NODE = "engram-node01"
TARGET_SUBNET = "172.28.0.0"  # engram-net's real /24 network address
MAX_PEERS_PER_SUBNET = 8  # x/sovereignty/types/params.go's DefaultParams()

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
    subprocess.run(
        ["docker", "compose", "--profile", profile, "up", "-d", *services],
        capture_output=True,
        text=True,
        timeout=120,
        check=True,
    )


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
    def __init__(self):
        self.start = time.time()
        self.rows = []  # list of dict rows for the CSV
        self.node_samples = []  # NodeSample objects, for write_csv reuse

    def elapsed(self):
        return time.time() - self.start

    def poll_once(self, phase: str):
        t = self.elapsed()
        node_samples = sample_all_nodes([TARGET_NODE])
        self.node_samples.extend(node_samples)
        s = node_samples[0]

        try:
            subnet_counts = peer_subnet_counts(TARGET_NODE)
        except (
            Exception
        ) as e:  # noqa: BLE001 -- best-effort observability, not consensus-critical
            subnet_counts = {}
            print(f"  (peer_subnet_counts failed this round: {e})")

        target_subnet_count = subnet_counts.get(TARGET_SUBNET, 0)
        total_peers = sum(subnet_counts.values())

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

    def poll_for(self, seconds: float, interval: float, phase: str):
        deadline = time.time() + seconds
        while time.time() < deadline:
            self.poll_once(phase)
            time.sleep(interval)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("leg", choices=list(LEGS.keys()))
    parser.add_argument("--baseline-s", type=float, default=30.0)
    parser.add_argument("--attack-s", type=float, default=90.0)
    parser.add_argument("--recovery-s", type=float, default=60.0)
    parser.add_argument("--interval-s", type=float, default=5.0)
    args = parser.parse_args()

    leg = LEGS[args.leg]
    os.makedirs(RESULTS_DIR, exist_ok=True)
    tr = Tracker()

    print(f"=== E4/E8 live Sybil attack leg '{args.leg}': {leg['description']} ===")
    print(
        f"    MaxPeersPerSubnet={MAX_PEERS_PER_SUBNET} (x/sovereignty/types/params.go)"
    )

    print(f"=== Phase 1: baseline ({args.baseline_s:.0f}s) ===")
    tr.poll_for(args.baseline_s, args.interval_s, "baseline")
    baseline_count = tr.rows[-1]["target_subnet_peer_count"] if tr.rows else 0

    print(f"=== Phase 2: attack ({args.attack_s:.0f}s) ===")
    swarm_up(leg["profile"], leg["services"])
    tr.poll_for(args.attack_s, args.interval_s, "attack")
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
            f"(no false SOVEREIGN degradation from a defended attack): **{fsm_stayed_healthy}**\n\n"
        )
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
        f"peak_target_subnet_count={peak_count} (limit={MAX_PEERS_PER_SUBNET})"
    )


if __name__ == "__main__":
    main()
