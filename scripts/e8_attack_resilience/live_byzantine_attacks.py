#!/usr/bin/env python3
"""LIVE Byzantine-mode validator test against the real 4-node testnet --
docs/EXPERIMENT.md's E8 A3 (Data Withholding), A4 (Forged BTC Receipt), A6
(Malicious Proposer). Swaps engram-node04's compose definition for
docker/engram-validator-node04-byzantine.yml's override (adds
ENGRAM_BYZANTINE_BEHAVIOR), which x/sovereignty/proposal.go's
applyByzantineBehavior reads to deliberately misbehave on its own
PrepareProposal calls, then reverts it back to the honest definition.

node04 stays IN the real validator set (N=4,f=1) throughout -- this is NOT a
new/5th node, matching this repo's tuned Byzantine-fault assumption.

Safety bar checked every round: the 3 HONEST nodes' AppHash must stay
identical to each other at every height sampled (never adopting node04's
malicious claim) -- the same bar x/sovereignty/proposal_test.go's in-process
tests already enforce, now demonstrated through real ABCI tx routing on a
real multi-node network instead of a direct Go call.

Known limitation, honestly flagged rather than silently skipped: A7's
adversarial half (deliberately censoring a specific forced tx) needs a real
MsgSubmitForcedTx submitted first to have something to censor -- no CLI
command for that exists yet in cmd/engramd (only the ABCI Msg type and
in-process test coverage do). Not attempted here; x/sovereignty/proposal.go's
applyByzantineBehavior already supports the censor_tx:<hash> behavior
mechanically, it just has no live driver script yet for the tx-submission
half.

Usage:
    python3 -u scripts/e8_attack_resilience/live_byzantine_attacks.py a6_fake_fsm_state
    python3 -u scripts/e8_attack_resilience/live_byzantine_attacks.py a4_forge_btc_hash
    python3 -u scripts/e8_attack_resilience/live_byzantine_attacks.py a3_false_da_attestation
    python3 -u scripts/e8_attack_resilience/live_byzantine_attacks.py --all
"""

import argparse
import os
import subprocess
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "framework"))
from logger import sample_all_nodes, write_csv, NODE_RPC_PORTS  # noqa: E402

RESULTS_DIR = os.path.join(os.path.dirname(__file__), "results_live")
BYZANTINE_SERVICE = "engram-node04"
HONEST_NODES = [n for n in NODE_RPC_PORTS if n != BYZANTINE_SERVICE]

SCENARIOS = {
    "a6_fake_fsm_state": {
        "behavior": "fake_fsm_state:SOVEREIGN",
        "description": "A6 Malicious Proposer: node04 claims SOVEREIGN while real conditions are healthy",
    },
    "a4_forge_btc_hash": {
        "behavior": "forge_btc_hash",
        "description": "A4 Forged BTC Receipt: node04 claims an advanced checkpoint with a tampered hash",
    },
    "a3_false_da_attestation": {
        "behavior": "false_da_attestation",
        "description": "A3 Data Withholding: node04 claims DA attestation succeeded with no real submission behind it",
    },
}


def now() -> str:
    return time.strftime("%H:%M:%S", time.gmtime())


def enable_byzantine(behavior: str) -> None:
    print(
        f"[{now()}] >>> recreating {BYZANTINE_SERVICE} with ENGRAM_BYZANTINE_BEHAVIOR={behavior!r}"
    )
    env = dict(os.environ)
    env["ENGRAM_BYZANTINE_BEHAVIOR"] = behavior
    subprocess.run(
        [
            "docker",
            "compose",
            "-f",
            "compose.yml",
            "-f",
            "docker/engram-validator-node04-byzantine.yml",
            "up",
            "-d",
            "--no-deps",
            "--force-recreate",
            BYZANTINE_SERVICE,
        ],
        capture_output=True,
        text=True,
        timeout=60,
        check=True,
        env=env,
    )


def disable_byzantine() -> None:
    print(f"[{now()}] >>> reverting {BYZANTINE_SERVICE} to its honest definition")
    subprocess.run(
        [
            "docker",
            "compose",
            "-f",
            "compose.yml",
            "up",
            "-d",
            "--no-deps",
            "--force-recreate",
            BYZANTINE_SERVICE,
        ],
        capture_output=True,
        text=True,
        timeout=60,
    )


class Tracker:
    def __init__(self):
        self.start = time.time()
        self.samples = []
        self.divergence_events = []

    def elapsed(self):
        return time.time() - self.start

    def poll_for(self, seconds: float, interval: float, phase: str):
        deadline = time.time() + seconds
        while time.time() < deadline:
            t = self.elapsed()
            all_samples = sample_all_nodes()
            self.samples.extend(all_samples)

            honest_hashes = {
                s.node: s.app_hash
                for s in all_samples
                if s.node in HONEST_NODES and s.height > 0
            }
            distinct_hashes = set(honest_hashes.values())
            if len(distinct_hashes) > 1:
                self.divergence_events.append((t, phase, dict(honest_hashes)))
                print(
                    f"  *** [{phase}] SAFETY VIOLATION @ {t:6.0f}s: honest nodes disagree on AppHash: "
                    f"{honest_hashes} ***"
                )

            states = {s.node: (s.height, s.fsm_state) for s in all_samples}
            print(f"[{t:6.0f}s][{phase}] {states}")
            time.sleep(interval)


def run_scenario(name: str, attack_s: float, recovery_s: float, interval_s: float):
    scenario = SCENARIOS[name]
    os.makedirs(RESULTS_DIR, exist_ok=True)
    tr = Tracker()

    print(f"=== E8 live Byzantine-mode test '{name}': {scenario['description']} ===")

    print("=== Phase 1: baseline (honest node04) ===")
    tr.poll_for(15.0, interval_s, "baseline")

    print(f"=== Phase 2: attack ({attack_s:.0f}s, node04 byzantine) ===")
    enable_byzantine(scenario["behavior"])
    tr.poll_for(attack_s, interval_s, "attack")

    print(f"=== Phase 3: recovery ({recovery_s:.0f}s, node04 reverted to honest) ===")
    disable_byzantine()
    tr.poll_for(recovery_s, interval_s, "recovery")

    ts_label = time.strftime("%Y%m%dT%H%M%S")
    csv_path = os.path.join(RESULTS_DIR, f"byzantine_{name}_{ts_label}.csv")
    write_csv(tr.samples, csv_path)

    summary_path = os.path.join(RESULTS_DIR, f"byzantine_{name}_{ts_label}_summary.md")
    safety_held = len(tr.divergence_events) == 0
    with open(summary_path, "w") as f:
        f.write(f"# LIVE Byzantine-mode validator test: {name}\n\n")
        f.write(f"{scenario['description']}\n\n")
        f.write(
            f"Total duration: {tr.elapsed():.0f}s. Behavior: `{scenario['behavior']}`.\n\n"
        )
        f.write(f"## Verdict\n\n")
        f.write(
            f"- Safety held (3 honest nodes' AppHash never diverged): **{safety_held}**\n"
        )
        f.write(f"- Divergence events: {len(tr.divergence_events)}\n\n")
        if tr.divergence_events:
            f.write("### Divergence detail\n\n")
            for t, phase, hashes in tr.divergence_events:
                f.write(f"- t={t:.0f}s phase={phase}: {hashes}\n")

    print(f"\nwrote {len(tr.samples)} samples to {csv_path}")
    print(f"wrote summary to {summary_path}")
    print(
        f"\nVERDICT for {name}: safety_held={safety_held} divergence_events={len(tr.divergence_events)}"
    )
    return safety_held


def main():
    parser = argparse.ArgumentParser(
        description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter
    )
    parser.add_argument("scenario", nargs="?", choices=list(SCENARIOS.keys()))
    parser.add_argument(
        "--all", action="store_true", help="run every scenario in sequence"
    )
    parser.add_argument("--attack-s", type=float, default=60.0)
    parser.add_argument("--recovery-s", type=float, default=30.0)
    parser.add_argument("--interval-s", type=float, default=3.0)
    args = parser.parse_args()

    if not args.all and not args.scenario:
        parser.error("must specify a scenario or --all")

    names = list(SCENARIOS.keys()) if args.all else [args.scenario]
    results = {}
    for name in names:
        results[name] = run_scenario(
            name, args.attack_s, args.recovery_s, args.interval_s
        )

    print("\n=== SUMMARY ===")
    for name, held in results.items():
        print(f"{name}: safety_held={held}")
    if not all(results.values()):
        sys.exit(1)


if __name__ == "__main__":
    main()
