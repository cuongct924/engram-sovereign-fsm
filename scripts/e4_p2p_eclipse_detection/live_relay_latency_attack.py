#!/usr/bin/env python3
"""LIVE Docker relay-latency attack against the real 4-node testnet -- docs/EXPERIMENT.md's E4 A4
(Relay Node Attack / relay-node latency inflation), beyond the existing synthetic Monte Carlo data
(tests/e2e/p2p_detector_comparison_test.go's A4_RelayNodeAttack).

Same live-observability mechanism as A3 (live_churn_attack.py, see its module doc for the full
explanation): PeerLatency is read from CometBFT's real p2p.Peer.RTT() by
cmd/engramd/main.go's vanillaP2PHealthAdapter (the real max RTT across a node's own peers), folded
into IsP2PQualityHealthy, and the derived Healthy boolean is committed consensus state via
ExtendedProposal.Healthy -- so, like A3, this attack's live success signal is a real, bounded FSM
degradation during the attack window followed by hysteresis-gated recovery, the opposite polarity
from A1/A2 (whose success signal is the FSM staying untouched, since FilterPeerByAddr absorbs
those attacks at ingress before the FSM ever sees them).

Mechanism: one real validator (engram-node04) gets ITS ACTUAL P2P LINKS delayed via direct `tc
qdisc add ... netem delay` (350ms +-50ms), applied through a throwaway privileged debug container
sharing node04's network namespace (docker run --net=container:engram-node04 --cap-add=NET_ADMIN)
-- clearly above MaxPeerLatency=200ms (x/sovereignty/types/params.go), and comfortably above the
existing WAN baseline's own max (140ms+-25ms, which never crosses 200ms and so can't double as
this attack).

NOT via Pumba's `netem delay <container>` command, despite that being every other netem profile's
mechanism in this file's family (compose.yml). Confirmed live: Pumba applies its delay to
whichever interface Docker assigns as `eth0`, and this cluster's real P2P gossip does NOT run over
that interface. `docker/engram-validator-cluster.yml` gives node04 SIX networks (bitcoin-net,
celestia-net, engram-net, plus three dedicated pairwise validator-link-0X-04 networks, one per
peer -- this is a pairwise-link topology, not a shared-bridge one). Docker assigns eth0..eth5 by
network *attachment* order, which does not match the YAML's `networks:` block order -- confirmed
live via `ip -o addr show` inside node04's netns: eth0 is bitcoin-net (172.21.0.130), completely
unrelated to consensus traffic. The real per-peer gossip links are the three validator-link-0X-04
interfaces (172.40.0.0/16 addresses) -- discovered dynamically at runtime by IP prefix, not
hardcoded as eth3/eth4/eth5, since attachment order is not a stable contract. A first live run
using Pumba's `dev eth0` genuinely applied a confirmed-active 350ms delay for the whole 240s attack
window (verified via `tc qdisc show`) with ZERO effect on any validator's measured PeerLatency,
because it delayed the wrong network entirely.

Cluster-wide-visibility note (the opposite of A3's proposer-visibility caveat): delaying ALL THREE
of node04's pairwise links means every other validator's real p.RTT() measurement of ITS
connection to node04 goes up simultaneously, so all three non-target validators should see the
same degraded reading regardless of whose turn it is to propose -- a cleaner, less intermittent
signal is expected here than for A3.

Quorum note (see live_churn_attack.py's module doc for A3's full BFT-quorum explanation): unlike
A3's single-peer-link churn (visible to only 1-2 of 4 validators' own local view), delaying node04
degrades every OTHER validator's own measurement of that link too -- all 4 validators should
independently compute Healthy=false, well past the >2/3 quorum ProcessProposal
(x/sovereignty/proposal.go:293-303) requires to commit a transition. This attack is not
minority-visible the way A3's was, so a real FSM transition is the expected outcome here.

Window duration: `RTT()` (../engram-consensus-core/p2p/conn/connection.go) only updates once per
real PacketPing/PacketPong exchange, and CometBFT only pings each peer every `defaultPingInterval`
(60s) -- a 120s attack window can span as few as 0-1 real exchanges depending on the ping timer's
phase relative to when the attack starts, an unreliable sample size. --attack-s defaults to 240s
(~4 ping cycles) so at least one elevated-RTT exchange is captured with real margin, regardless of
phase.

Usage (against an already-deployed, default-params, healthy ANCHORED testnet -- no genesis change
needed, MaxPeerLatency is already DefaultParams()):

    python3 -u scripts/e4_p2p_eclipse_detection/live_relay_latency_attack.py
"""

import argparse
import os
import subprocess
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "framework"))
from logger import sample_all_nodes, write_csv  # noqa: E402

RESULTS_DIR = os.path.join(os.path.dirname(__file__), "results_live")
TARGET_NODE = "engram-node04"
MAX_PEER_LATENCY_MS = 200  # x/sovereignty/types/params.go's DefaultParams()
DELAY_MS = 350
JITTER_MS = 50
# Pairwise validator-link networks (docker/engram-validator-cluster.yml) all
# sit in 172.40.0.0/16 -- distinguishes node04's real P2P interfaces from
# bitcoin-net/celestia-net/engram-net, none of which carry consensus gossip.
PAIRWISE_LINK_PREFIX = "172.40."
_NETEM_HELPER_IMAGE = "nicolaka/netshoot"


def now() -> str:
    return time.strftime("%H:%M:%S", time.gmtime())


def pairwise_link_interfaces(container: str) -> list:
    """Discovers container's real per-peer P2P interface names by IP subnet,
    not a hardcoded eth index -- see this module's doc for why attachment
    order (and hence eth0/eth1/...) is not a stable contract here."""
    proc = subprocess.run(
        ["docker", "run", "--rm", "--net", f"container:{container}", _NETEM_HELPER_IMAGE,
         "ip", "-o", "addr", "show"],
        capture_output=True, text=True, timeout=30, check=True,
    )
    ifaces = []
    for line in proc.stdout.splitlines():
        parts = line.split()
        if len(parts) >= 4 and parts[3].startswith(PAIRWISE_LINK_PREFIX):
            ifaces.append(parts[1])
    return ifaces


def apply_delay(container: str, ifaces: list, delay_ms: int, jitter_ms: int) -> None:
    for iface in ifaces:
        subprocess.run(
            ["docker", "run", "--rm", "--net", f"container:{container}", "--cap-add", "NET_ADMIN",
             _NETEM_HELPER_IMAGE, "tc", "qdisc", "add", "dev", iface, "root", "netem",
             "delay", f"{delay_ms}ms", f"{jitter_ms}ms"],
            capture_output=True, text=True, timeout=30, check=True,
        )


def clear_delay(container: str, ifaces: list) -> None:
    """Best-effort -- `tc qdisc del` on an interface with no active qdisc
    exits non-zero, which is fine, there's nothing to remove."""
    for iface in ifaces:
        subprocess.run(
            ["docker", "run", "--rm", "--net", f"container:{container}", "--cap-add", "NET_ADMIN",
             _NETEM_HELPER_IMAGE, "tc", "qdisc", "del", "dev", iface, "root"],
            capture_output=True, text=True, timeout=30,
        )


class Tracker:
    def __init__(self):
        self.start = time.time()
        self.rows = []
        self.node_samples = []

    def elapsed(self):
        return time.time() - self.start

    def poll_once(self, phase: str):
        t = self.elapsed()
        samples = sample_all_nodes()
        self.node_samples.extend(samples)
        states = {s.node: s.fsm_state for s in samples}
        row = {"t": round(t, 1), "phase": phase, "states": dict(states)}
        self.rows.append(row)
        print(f"[{t:6.0f}s][{phase}] {states}")
        return row

    def poll_for(self, seconds: float, interval: float, phase: str):
        deadline = time.time() + seconds
        while time.time() < deadline:
            self.poll_once(phase)
            time.sleep(interval)


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--baseline-s", type=float, default=30.0)
    parser.add_argument("--attack-s", type=float, default=240.0, help="~4x the real 60s CometBFT ping interval, so at least one real elevated-RTT ping/pong exchange is reliably captured regardless of timer phase")
    parser.add_argument("--recovery-s", type=float, default=120.0, help="hysteresis-gated recovery can take longer than the attack window itself")
    parser.add_argument("--interval-s", type=float, default=5.0)
    args = parser.parse_args()

    os.makedirs(RESULTS_DIR, exist_ok=True)
    tr = Tracker()

    print(f"=== E4 live relay-latency attack (A4): target={TARGET_NODE}, MaxPeerLatency={MAX_PEER_LATENCY_MS}ms ===")
    print("NOTE: this script does not verify the deployed cluster's Params -- confirm it's running DefaultParams() before trusting this run's label.")

    ifaces = pairwise_link_interfaces(TARGET_NODE)
    if not ifaces:
        print(f"FATAL: found no {PAIRWISE_LINK_PREFIX}* interfaces on {TARGET_NODE} -- topology "
              f"may have changed, see this script's module doc.", file=sys.stderr)
        sys.exit(1)
    print(f"Discovered {TARGET_NODE}'s real P2P interfaces: {ifaces}")

    print(f"=== Phase 1: baseline ({args.baseline_s:.0f}s) ===")
    tr.poll_for(args.baseline_s, args.interval_s, "baseline")
    baseline_states = {s for r in tr.rows if r["phase"] == "baseline" for s in r["states"].values() if s}

    print(f"=== Phase 2: attack ({args.attack_s:.0f}s, {DELAY_MS}ms+-{JITTER_MS}ms netem delay on {TARGET_NODE}'s {len(ifaces)} pairwise links) ===")
    try:
        apply_delay(TARGET_NODE, ifaces, DELAY_MS, JITTER_MS)
    except subprocess.CalledProcessError as e:
        print(f"FATAL: tc qdisc add failed on {TARGET_NODE}: {e.stderr}", file=sys.stderr)
        clear_delay(TARGET_NODE, ifaces)
        sys.exit(1)
    tr.poll_for(args.attack_s, args.interval_s, "attack")
    attack_states = {s for r in tr.rows if r["phase"] == "attack" for s in r["states"].values() if s}

    print(f"=== Phase 3: recovery ({args.recovery_s:.0f}s, clearing netem delay) ===")
    clear_delay(TARGET_NODE, ifaces)
    tr.poll_for(args.recovery_s, args.interval_s, "recovery")
    recovery_states = {s for r in tr.rows if r["phase"] == "recovery" for s in r["states"].values() if s}
    final_states = tr.rows[-1]["states"] if tr.rows else {}

    fsm_deviated_during_attack = not attack_states.issubset(baseline_states) if baseline_states else bool(attack_states)
    fsm_recovered_after = recovery_states.issubset(baseline_states) if baseline_states else True

    ts_label = time.strftime("%Y%m%dT%H%M%S")
    csv_path = os.path.join(RESULTS_DIR, f"relay_latency_attack_a4_{ts_label}.csv")
    write_csv(tr.node_samples, csv_path)

    summary_path = os.path.join(RESULTS_DIR, f"relay_latency_attack_a4_{ts_label}_summary.md")
    with open(summary_path, "w") as f:
        f.write("# LIVE E4 relay-latency attack, A4 (Relay Node Attack)\n\n")
        f.write(
            f"{DELAY_MS}ms +-{JITTER_MS}ms netem delay held on {TARGET_NODE}'s real P2P interfaces "
            f"({', '.join(ifaces)}, the pairwise validator-link networks -- not eth0/Pumba's "
            f"default, see module doc) for the whole attack window -- not toggled, a sustained "
            f"degradation. MaxPeerLatency={MAX_PEER_LATENCY_MS}ms. Total duration: {tr.elapsed():.0f}s.\n\n"
        )
        f.write(
            "**Quorum note:** unlike A3's single-peer-link churn (visible to only 1-2 of 4 "
            "validators' own local view, see live_churn_attack.py's module doc), delaying all of "
            f"{TARGET_NODE}'s real P2P links degrades every OTHER validator's own RTT measurement "
            f"of its connection to {TARGET_NODE} too -- all 4 validators should independently "
            "compute Healthy=false, past the >2/3 quorum `ProcessProposal` "
            "(x/sovereignty/proposal.go:293-303) requires to commit. A real FSM transition is "
            "therefore the expected outcome here, unlike A3's minority-visible design.\n\n"
        )
        f.write("## Verdict\n\n")
        f.write(f"- Baseline FSM states: {sorted(baseline_states)}\n")
        f.write(f"- Attack-window FSM states: {sorted(attack_states)}\n")
        f.write(f"- Recovery-window FSM states: {sorted(recovery_states)}\n")
        f.write(f"- Final state: {final_states}\n\n")
        f.write(f"- FSM deviated from baseline during the delay window (True is the expected/correct outcome here -- see Quorum note above): **{fsm_deviated_during_attack}**\n")
        f.write(f"- FSM recovered back to baseline states afterward (hysteresis bounded it): **{fsm_recovered_after}**\n\n")
        f.write("## Full timeline\n\n")
        f.write("| t (s) | phase | states |\n")
        f.write("|---:|---|---|\n")
        for r in tr.rows:
            f.write(f"| {r['t']} | {r['phase']} | {r['states']} |\n")

    print(f"\nwrote {len(tr.node_samples)} node samples to {csv_path}")
    print(f"wrote summary to {summary_path}")
    print(f"\nVERDICT: fsm_deviated_during_attack={fsm_deviated_during_attack} fsm_recovered_after={fsm_recovered_after}")


if __name__ == "__main__":
    main()
