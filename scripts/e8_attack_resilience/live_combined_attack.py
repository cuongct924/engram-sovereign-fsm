#!/usr/bin/env python3
"""LIVE Combined Attack (A8) against the real 4-node testnet --
docs/EXPERIMENT.md's E8 capstone: overlays MULTIPLE attack vectors from the
A1-A7 matrix simultaneously, confirming safety holds under compounding
adversarial pressure rather than testing each vector in isolation.

Depends on mechanisms built by two earlier parts, both required before this
is meaningful:
  - Part 1/2 (A1/A2): docker/attacker-peer-swarm.yml's A1 leg (peer-slot
    exhaustion swarm against engram-node01) -- scripts/e4_p2p_eclipse_detection/
    live_sybil_attack.py's mechanism, reused directly here.
  - Part 4a (A3/A4/A6): docker/engram-validator-node04-byzantine.yml
    (node04 running with ENGRAM_BYZANTINE_BEHAVIOR set) --
    scripts/e8_attack_resilience/live_byzantine_attacks.py's mechanism.

This overlay: node04 runs fake_fsm_state:SOVEREIGN (A6) for the whole
window, WHILE the A1 attacker swarm simultaneously floods engram-node01's
peer slots -- two independent attack surfaces active at once, neither
defended against by the other's mitigation (Part 1a's ingress filter
doesn't touch consensus-layer proposal content; the honest-validator
rejection path doesn't touch peer admission).

Safety bar: the 3 HONEST, non-byzantine, non-swarm-targeted nodes
(engram-node02, engram-node03 -- engram-node01 is excluded from the
"honest" comparison set here since it's ALSO the A1 swarm's direct target
and node04 is byzantine) must keep an identical AppHash at every height
sampled, the entire time both attacks are active.

Usage:
    python3 -u scripts/e8_attack_resilience/live_combined_attack.py
"""

import os
import subprocess
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "framework"))
from logger import sample_all_nodes, write_csv  # noqa: E402

RESULTS_DIR = os.path.join(os.path.dirname(__file__), "results_live")
BYZANTINE_SERVICE = "engram-node04"
BYZANTINE_BEHAVIOR = "fake_fsm_state:SOVEREIGN"
# Deliberately excludes engram-node01 (the A1 swarm's direct dial target --
# its own peer count/behavior is part of what's under attack, not a clean
# "did safety hold" witness) and engram-node04 (byzantine).
SAFETY_WITNESS_NODES = ["engram-node02", "engram-node03"]


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


A1_SWARM_SERVICES = [f"attacker-a1-{i:02d}" for i in range(1, 11)]


def swarm_up() -> None:
    print(
        f"[{now()}] >>> docker compose --profile attacker-swarm-a1 up -d <services> (A1 leg)"
    )
    subprocess.run(
        [
            "docker",
            "compose",
            "--profile",
            "attacker-swarm-a1",
            "up",
            "-d",
            *A1_SWARM_SERVICES,
        ],
        capture_output=True,
        text=True,
        timeout=120,
        check=True,
    )


def swarm_down() -> None:
    # NEVER `docker compose down` (no service names) -- tears down the WHOLE
    # project (real cluster included), not just this profile. See
    # scripts/e4_p2p_eclipse_detection/live_sybil_attack.py's swarm_down doc
    # for the live incident this is fixed from.
    print(f"[{now()}] >>> docker compose stop/rm <services> (A1 leg only)")
    subprocess.run(
        [
            "docker",
            "compose",
            "--profile",
            "attacker-swarm-a1",
            "stop",
            *A1_SWARM_SERVICES,
        ],
        capture_output=True,
        text=True,
        timeout=60,
    )
    subprocess.run(
        [
            "docker",
            "compose",
            "--profile",
            "attacker-swarm-a1",
            "rm",
            "-f",
            *A1_SWARM_SERVICES,
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

            # Compare AppHash only within the same height -- see
            # live_byzantine_attacks.py's Tracker.poll_for for the false-
            # positive this avoids (found live, same bug class here).
            witness_hashes = {
                s.node: s.app_hash
                for s in all_samples
                if s.node in SAFETY_WITNESS_NODES and s.height > 0
            }
            witness_heights = {
                s.node: s.height
                for s in all_samples
                if s.node in SAFETY_WITNESS_NODES and s.height > 0
            }
            by_height: dict = {}
            for node, h in witness_heights.items():
                by_height.setdefault(h, {})[node] = witness_hashes[node]
            for h, hashes_at_h in by_height.items():
                if len(hashes_at_h) > 1 and len(set(hashes_at_h.values())) > 1:
                    self.divergence_events.append((t, phase, h, dict(hashes_at_h)))
                    print(
                        f"  *** [{phase}] SAFETY VIOLATION @ {t:6.0f}s height={h}: witness nodes "
                        f"disagree: {hashes_at_h} ***"
                    )

            states = {s.node: (s.height, s.fsm_state) for s in all_samples}
            print(f"[{t:6.0f}s][{phase}] {states}")
            time.sleep(interval)


def main():
    os.makedirs(RESULTS_DIR, exist_ok=True)
    tr = Tracker()

    print(
        "=== E8 A8 Combined Attack: byzantine node04 (fake_fsm_state:SOVEREIGN) "
        "+ A1 peer-slot-exhaustion swarm, simultaneous ==="
    )

    print("=== Phase 1: baseline (both attacks off) ===")
    tr.poll_for(15.0, 3.0, "baseline")

    print("=== Phase 2: both attacks active (120s) ===")
    enable_byzantine(BYZANTINE_BEHAVIOR)
    swarm_up()
    tr.poll_for(120.0, 3.0, "combined_attack")

    print("=== Phase 3: recovery (revert both) ===")
    swarm_down()
    disable_byzantine()
    tr.poll_for(60.0, 3.0, "recovery")

    ts_label = time.strftime("%Y%m%dT%H%M%S")
    csv_path = os.path.join(RESULTS_DIR, f"combined_attack_a8_{ts_label}.csv")
    write_csv(tr.samples, csv_path)

    summary_path = os.path.join(
        RESULTS_DIR, f"combined_attack_a8_{ts_label}_summary.md"
    )
    safety_held = len(tr.divergence_events) == 0
    with open(summary_path, "w") as f:
        f.write("# LIVE Combined Attack (E8 A8)\n\n")
        f.write(
            f"Byzantine behavior: `{BYZANTINE_BEHAVIOR}` on {BYZANTINE_SERVICE}, "
            f"simultaneous with the A1 peer-slot-exhaustion swarm (attacker-swarm-a1 profile).\n\n"
        )
        f.write(
            f"Total duration: {tr.elapsed():.0f}s. Safety witness nodes: {SAFETY_WITNESS_NODES}.\n\n"
        )
        f.write(f"## Verdict\n\n")
        f.write(
            f"- Safety held (witness nodes' AppHash never diverged): **{safety_held}**\n"
        )
        f.write(f"- Divergence events: {len(tr.divergence_events)}\n\n")
        if tr.divergence_events:
            f.write("### Divergence detail\n\n")
            for t, phase, h, hashes in tr.divergence_events:
                f.write(f"- t={t:.0f}s phase={phase} height={h}: {hashes}\n")

    print(f"\nwrote {len(tr.samples)} samples to {csv_path}")
    print(f"wrote summary to {summary_path}")
    print(
        f"\nVERDICT: safety_held={safety_held} divergence_events={len(tr.divergence_events)}"
    )
    if not safety_held:
        sys.exit(1)


if __name__ == "__main__":
    main()
