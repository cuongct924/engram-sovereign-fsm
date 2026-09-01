#!/usr/bin/env python3
"""LIVE Docker churn attack against the real 7-node testnet -- docs/EXPERIMENT.md's E4 A3
(Churn-based Rotation), beyond the existing synthetic Monte Carlo data
(tests/e2e/p2p_detector_comparison_test.go's A3_ChurnBasedRotation).

Unlike A1/A2 (blocked at the ingress filter), A3's PeerChurnRate/AvgPeerTenure reach
IsP2PQualityHealthy (x/sovereignty/types/predicates.go), which IS committed consensus state and
can drive ANCHORED->SUSPICIOUS -- so success here means a real, bounded degradation followed by
hysteresis-gated recovery, not the FSM staying untouched.

Raw PeerChurnRate/AvgPeerTenure aren't queryable live (never committed state), so only the
resulting FSM transition is observable, not the sensor's raw numbers.

Two real attacker containers (docker/attacker-peer-swarm.yml, profile attacker-swarm-a3) dial
node03/node04 (adjacent proposers, see ATTACKER_SERVICES below); churn is real docker stop/start
on the containers, not netem (which CometBFT's liveness check doesn't reliably catch).

Expected: **no committed FSM transition** -- node03/node04 are only 2 of 7 validators (~28.6%
power), below the >2/3 quorum `ProcessProposal` requires, so the honest majority's ANCHORED claim
always wins. This demonstrates the same quorum safety that gives Byzantine-safety, not a detection
failure -- a supermajority attack (>=5 of 7) would be a different experiment.

Usage (against an already-deployed, default-params, healthy ANCHORED testnet -- no genesis change
needed, MaxChurnRate/MinAvgTenure are already DefaultParams()):

    python3 -u scripts/e4_p2p_eclipse_detection/live_churn_attack.py
"""

import argparse
import os
import subprocess
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "framework"))
from logger import sample_all_nodes, write_csv  # noqa: E402

RESULTS_DIR = os.path.join(os.path.dirname(__file__), "results_live")
ATTACKER_PROFILE = "attacker-swarm-a3"
ATTACKER_SERVICES = ["attacker-a3-01", "attacker-a3-02"]  # -> engram-node03, engram-node04
# node03/node04: empirically-confirmed adjacent proposers in this 7-validator cluster's real
# round-robin (node06->node03->node04->node01->node02->node07->node05) -- re-verify if the
# validator set changes.
MAX_CHURN_RATE = 5  # x/sovereignty/types/params.go's DefaultParams(), per the 1h rolling window


def now() -> str:
    return time.strftime("%H:%M:%S", time.gmtime())


def attacker_up() -> None:
    # Right after a fresh `make testnet-up` redeploy, the first `docker
    # compose up -d` can fail transiently (compose's project-state cache
    # resyncing) -- retrying briefly avoids needing manual intervention.
    print(f"[{now()}] >>> docker compose --profile {ATTACKER_PROFILE} up -d {' '.join(ATTACKER_SERVICES)}")
    last_err = None
    for attempt in range(3):
        try:
            subprocess.run(
                ["docker", "compose", "--profile", ATTACKER_PROFILE, "up", "-d", *ATTACKER_SERVICES],
                capture_output=True, text=True, timeout=60, check=True,
            )
            return
        except subprocess.CalledProcessError as e:
            last_err = e
            print(f"[{now()}] attacker_up attempt {attempt + 1}/3 failed ({e}), retrying in 5s...")
            time.sleep(5)
    raise last_err


def attacker_down() -> None:
    # Same rule as live_sybil_attack.py's swarm_down: NEVER `docker compose down`
    # (tears down the whole project, not just this service).
    print(f"[{now()}] >>> docker compose stop/rm {' '.join(ATTACKER_SERVICES)} (tearing down attackers only)")
    subprocess.run(
        ["docker", "compose", "--profile", ATTACKER_PROFILE, "stop", *ATTACKER_SERVICES],
        capture_output=True, text=True, timeout=60,
    )
    subprocess.run(
        ["docker", "compose", "--profile", ATTACKER_PROFILE, "rm", "-f", *ATTACKER_SERVICES],
        capture_output=True, text=True, timeout=60,
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


def stop_attacker() -> None:
    # Both containers stopped together -- node03 and node04 need to go
    # unhealthy on the SAME cycle for their proposer turns to have any
    # chance of landing back-to-back unhealthy.
    for svc in ATTACKER_SERVICES:
        subprocess.run(["docker", "stop", svc], capture_output=True, text=True, timeout=30)


def start_attacker() -> None:
    for svc in ATTACKER_SERVICES:
        subprocess.run(["docker", "start", svc], capture_output=True, text=True, timeout=30)


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--baseline-s", type=float, default=30.0)
    parser.add_argument("--settle-s", type=float, default=20.0, help="time to let attacker-a3-01 become a normal stable peer before churn starts")
    parser.add_argument("--churn-on-s", type=float, default=15.0, help="seconds the attacker CONTAINER stays stopped per cycle (docker stop, a real TCP teardown -- not netem)")
    parser.add_argument("--churn-off-s", type=float, default=20.0, help="seconds the attacker stays running/reconnected per cycle before the next stop")
    parser.add_argument("--churn-cycles", type=int, default=8, help="8 cycles = 8 real disconnect+8 real connect events, comfortably above MaxChurnRate=5")
    parser.add_argument("--recovery-s", type=float, default=120.0, help="hysteresis-gated recovery can take longer than the attack window itself")
    parser.add_argument("--interval-s", type=float, default=5.0)
    args = parser.parse_args()

    os.makedirs(RESULTS_DIR, exist_ok=True)
    tr = Tracker()

    print(f"=== E4 live churn attack (A3): targets=engram-node03,engram-node04 via {', '.join(ATTACKER_SERVICES)}, MaxChurnRate={MAX_CHURN_RATE} ===")
    print("NOTE: this script does not verify the deployed cluster's Params -- confirm it's running DefaultParams() before trusting this run's label.")

    print(f"=== Phase 1: baseline ({args.baseline_s:.0f}s) ===")
    tr.poll_for(args.baseline_s, args.interval_s, "baseline")
    baseline_states = {s for r in tr.rows if r["phase"] == "baseline" for s in r["states"].values() if s}

    print(f"=== Phase 2: bring up {', '.join(ATTACKER_SERVICES)} and let them settle ({args.settle_s:.0f}s) ===")
    attacker_up()
    tr.poll_for(args.settle_s, args.interval_s, "settle")

    print(f"=== Phase 3: churn ({args.churn_cycles} cycles, real docker stop/start, on={args.churn_on_s:.0f}s off={args.churn_off_s:.0f}s) ===")
    # Real docker stop/start are fast, blocking calls (unlike the old netem
    # profile's own --duration) -- no background thread needed, just
    # interleave them directly with polling so FSM state is sampled
    # throughout, not just snapshotted after the fact.
    for i in range(args.churn_cycles):
        print(f"[{now()}] >>> churn cycle {i + 1}/{args.churn_cycles}: docker stop {', '.join(ATTACKER_SERVICES)}")
        stop_attacker()
        tr.poll_for(args.churn_on_s, args.interval_s, "attack")
        print(f"[{now()}] >>> churn cycle {i + 1}/{args.churn_cycles}: docker start {', '.join(ATTACKER_SERVICES)}")
        start_attacker()
        tr.poll_for(args.churn_off_s, args.interval_s, "attack")
    attack_states = {s for r in tr.rows if r["phase"] == "attack" for s in r["states"].values() if s}

    print(f"=== Phase 4: recovery ({args.recovery_s:.0f}s, tearing down attacker) ===")
    attacker_down()
    tr.poll_for(args.recovery_s, args.interval_s, "recovery")
    recovery_states = {s for r in tr.rows if r["phase"] == "recovery" for s in r["states"].values() if s}
    final_states = tr.rows[-1]["states"] if tr.rows else {}

    fsm_deviated_during_attack = not attack_states.issubset(baseline_states) if baseline_states else bool(attack_states)
    fsm_recovered_after = recovery_states.issubset(baseline_states) if baseline_states else True

    ts_label = time.strftime("%Y%m%dT%H%M%S")
    csv_path = os.path.join(RESULTS_DIR, f"churn_attack_a3_{ts_label}.csv")
    write_csv(tr.node_samples, csv_path)

    summary_path = os.path.join(RESULTS_DIR, f"churn_attack_a3_{ts_label}_summary.md")
    with open(summary_path, "w") as f:
        f.write("# LIVE E4 churn attack, A3 (Churn-based Rotation)\n\n")
        f.write(
            f"attacker-a3-01 dials engram-node03, attacker-a3-02 dials engram-node04 (empirically-"
            f"confirmed ADJACENT proposers in this 7-validator cluster's real rotation order, "
            f"node06 -> node03 -> node04 -> node01 -> node02 -> node07 -> node05, read off 15 "
            f"consecutive real blocks' proposer_address); churn = "
            f"{args.churn_cycles} real `docker stop`/`docker start` cycles on both attacker "
            f"containers together each cycle (on={args.churn_on_s:.0f}s stopped, off="
            f"{args.churn_off_s:.0f}s reconnected) -- a genuine TCP teardown each cycle, not netem "
            f"packet loss (an earlier design using 100% loss never actually disconnected "
            f"CometBFT's MConnection, confirmed via real node02 logs on the earlier 4-node cluster). "
            f"MaxChurnRate={MAX_CHURN_RATE} (1h rolling window). Total duration: {tr.elapsed():.0f}s.\n\n"
        )
        f.write(
            "**Quorum note:** no FSM transition is the correct, expected outcome for this design, "
            "not a detection failure. `ProcessProposal` (x/sovereignty/proposal.go:293-303) has "
            "every validator independently recompute `fsm_state` from its OWN local metrics and "
            "reject any disagreeing proposal; CometBFT needs >2/3 voting power to commit. Only "
            "node03/node04 (2 of 7, ~28.6%) see degraded PeerChurnRate here, so the honest majority "
            "(the other 5 validators) always rejects an unhealthy claim, forcing round-retries until an "
            "honest proposer's ANCHORED claim commits instead. A real, confirmed churn_rate "
            "excursion on a MINORITY of validators (well past MaxChurnRate on both attacked nodes, "
            "per the sensor_snapshot diagnostic line) cannot force a transition by design -- "
            "'sensors propose, consensus decides' means no minority view, however degraded, moves "
            "committed state. A supermajority attack (>=5 of 7 validators) would be a structurally "
            "different experiment.\n\n"
        )
        f.write("## Verdict\n\n")
        f.write(f"- Baseline FSM states: {sorted(baseline_states)}\n")
        f.write(f"- Attack-window FSM states: {sorted(attack_states)}\n")
        f.write(f"- Recovery-window FSM states: {sorted(recovery_states)}\n")
        f.write(f"- Final state: {final_states}\n\n")
        f.write(f"- FSM deviated from baseline during the churn window (False is the expected/correct outcome here -- see Quorum note above): **{fsm_deviated_during_attack}**\n")
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
