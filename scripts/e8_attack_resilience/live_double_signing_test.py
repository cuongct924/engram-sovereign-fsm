#!/usr/bin/env python3
"""LIVE Double-signing detection test against the real testnet -- docs/EXPERIMENT.md's E8
"Double-signing" row. Starts a SECOND `engramd` process holding node04's exact
priv_validator_key.json but its own separate priv_validator_state.json (deliberately not
sharing node04's signing-history file, which is what normally prevents double-signing).

Detection channel: recordDetectedEvidence (x/sovereignty/preblock.go) prints "SLASHABLE
EVIDENCE DETECTED" on every honest validator once CometBFT's evidence pool reports real
DuplicateVoteEvidence. This script log-greps for that marker -- no Query RPC exposes it yet,
though it is committed to keeper state.

Usage:
    python3 -u scripts/e8_attack_resilience/live_double_signing_test.py
"""

import os
import re
import shutil
import subprocess
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "framework"))
from logger import sample_all_nodes, write_csv  # noqa: E402

RESULTS_DIR = os.path.join(os.path.dirname(__file__), "results_live")
REPO_ROOT = os.path.join(os.path.dirname(__file__), "..", "..")
DUPLICATE_HOME = os.path.join(REPO_ROOT, "testnet-data", "engram-node04-double-sign")
REAL_NODE04_KEY = os.path.join(
    REPO_ROOT, "testnet-data", "engram-node04", "config", "priv_validator_key.json"
)
REAL_GENESIS = os.path.join(
    REPO_ROOT, "testnet-data", "engram-node01", "config", "genesis.json"
)
DUPLICATE_SERVICE = "engram-node04-double-sign"
# All 6 real honest validators (node04 is the one being impersonated) --
# was hardcoded to only 3 (node01-03) from the old 4-node cluster.
WITNESS_CONTAINERS = ["engram-node01", "engram-node02", "engram-node03", "engram-node05", "engram-node06", "engram-node07"]
EVIDENCE_MARKER = re.compile(
    r"SLASHABLE EVIDENCE DETECTED type=(\S+) validator=(\S+) offense_height=(\d+) detected_at_height=(\d+)"
)


def now() -> str:
    return time.strftime("%H:%M:%S", time.gmtime())


def compute_persistent_peers() -> str:
    """Builds the id@host:26656 list for the other real validators (not node04, whose identity
    is being impersonated). Node IDs come from each node's own /status RPC.
    """
    import json

    peers = WITNESS_CONTAINERS
    port_map = {
        "engram-node01": 26657, "engram-node02": 26757, "engram-node03": 26857,
        "engram-node05": 27057, "engram-node06": 27157, "engram-node07": 27257,
    }
    node_ids = {}
    for node in peers:
        proc = subprocess.run(
            ["curl", "-s", f"http://localhost:{port_map[node]}/status"],
            capture_output=True,
            text=True,
            timeout=10,
        )
        node_ids[node] = json.loads(proc.stdout)["result"]["node_info"]["id"]
    return ",".join(f"{node_ids[n]}@{n}:26656" for n in peers)


def stage_duplicate_identity() -> None:
    """Copies node04's real signing key + genesis onto the host at the duplicate's home dir
    before the container starts (a Docker Desktop virtiofs limitation blocks nested bind-mounts
    for this). Wipes stale config/ first -- a failed mount can leave blocking placeholder files.
    """
    config_dir = os.path.join(DUPLICATE_HOME, "config")
    data_dir = os.path.join(DUPLICATE_HOME, "data")
    if os.path.isdir(config_dir):
        shutil.rmtree(config_dir)
    if os.path.isdir(data_dir):
        shutil.rmtree(data_dir)
    os.makedirs(config_dir, exist_ok=True)
    os.makedirs(data_dir, exist_ok=True)
    shutil.copy(REAL_NODE04_KEY, os.path.join(config_dir, "priv_validator_key.json"))
    shutil.copy(REAL_GENESIS, os.path.join(config_dir, "genesis.json"))
    # FilePV's state file: `engramd init` bails out early since config already
    # exists (pre-copied above), so this must be written directly. Starts at
    # height 0, deliberately NOT copied from node04's real state -- the point
    # of this harness.
    with open(os.path.join(data_dir, "priv_validator_state.json"), "w") as f:
        f.write('{"height":"0","round":0,"step":0}')


def start_duplicate(persistent_peers: str) -> None:
    stage_duplicate_identity()
    print(
        f"[{now()}] >>> starting {DUPLICATE_SERVICE} with DUPLICATE_PERSISTENT_PEERS={persistent_peers!r}"
    )
    env = dict(os.environ)
    env["DUPLICATE_PERSISTENT_PEERS"] = persistent_peers
    subprocess.run(
        [
            "docker",
            "compose",
            "--profile",
            "double-sign-harness",
            "up",
            "-d",
            "--build",
            DUPLICATE_SERVICE,
        ],
        capture_output=True,
        text=True,
        timeout=300,
        check=True,
        env=env,
    )


def stop_duplicate() -> None:
    # NEVER `docker compose down` -- tears down the WHOLE project, not just
    # this profile (see live_sybil_attack.py's swarm_down).
    print(f"[{now()}] >>> stopping {DUPLICATE_SERVICE}")
    subprocess.run(
        [
            "docker",
            "compose",
            "--profile",
            "double-sign-harness",
            "stop",
            DUPLICATE_SERVICE,
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
            "double-sign-harness",
            "rm",
            "-f",
            DUPLICATE_SERVICE,
        ],
        capture_output=True,
        text=True,
        timeout=60,
    )


def scan_logs_for_evidence(container: str) -> list:
    # --tail bounded: an unbounded `docker logs` dump on a long-running cluster
    # can exceed the timeout; only recent output matters for this test's window.
    proc = subprocess.run(
        ["docker", "logs", "--tail", "20000", container],
        capture_output=True, text=True, timeout=60,
    )
    return [m.groups() for m in EVIDENCE_MARKER.finditer(proc.stdout + proc.stderr)]


class Tracker:
    """AppHash-divergence tracker for the honest witnesses (mirrors
    live_byzantine_attacks.py's Tracker; node04 excluded since its identity
    is being impersonated) -- tracks detection latency and safety together.
    """

    def __init__(self):
        self.start = time.time()
        self.samples = []
        self.divergence_events = []

    def elapsed(self):
        return time.time() - self.start

    def poll_once(self, phase: str):
        t = self.elapsed()
        all_samples = sample_all_nodes(WITNESS_CONTAINERS)
        self.samples.extend(all_samples)
        hashes = {s.node: s.app_hash for s in all_samples if s.height > 0}
        heights = {s.node: s.height for s in all_samples if s.height > 0}
        by_height: dict = {}
        for node, h in heights.items():
            by_height.setdefault(h, {})[node] = hashes[node]
        for h, hashes_at_h in by_height.items():
            if len(hashes_at_h) > 1 and len(set(hashes_at_h.values())) > 1:
                self.divergence_events.append((t, phase, h, dict(hashes_at_h)))
                print(f"  *** [{phase}] SAFETY VIOLATION @ {t:6.0f}s height={h}: {hashes_at_h} ***")
        print(f"[{t:6.0f}s][{phase}] heights={heights}")

    def poll_for(self, seconds: float, interval: float, phase: str):
        deadline = time.time() + seconds
        while time.time() < deadline:
            self.poll_once(phase)
            time.sleep(interval)


def main():
    os.makedirs(RESULTS_DIR, exist_ok=True)
    tr = Tracker()

    print("=== E8 live Double-signing detection test ===")
    print("=== Phase 1: baseline (confirm no pre-existing evidence in logs) ===")
    for c in WITNESS_CONTAINERS:
        existing = scan_logs_for_evidence(c)
        if existing:
            print(
                f"WARNING: {c} already has {len(existing)} evidence log line(s) from a PRIOR run -- "
                f"detection-latency numbers below may be stale. Consider a fresh redeploy first."
            )
    tr.poll_for(15.0, 3.0, "baseline")

    print("=== Phase 2: start duplicate-key harness ===")
    persistent_peers = compute_persistent_peers()
    start_duplicate(persistent_peers)

    print(
        "=== Phase 3: poll for real DuplicateVoteEvidence in the 3 witness nodes' logs (up to 300s), "
        "sampling AppHash on every round for the safety check alongside it ==="
    )
    deadline = time.time() + 300
    found = {}
    while time.time() < deadline and len(found) < len(WITNESS_CONTAINERS):
        tr.poll_once("double_signing")
        for c in WITNESS_CONTAINERS:
            if c in found:
                continue
            entries = scan_logs_for_evidence(c)
            if entries:
                found[c] = entries
                print(f"  *** {c}: real evidence detected: {entries[-1]} ***")
        if len(found) < len(WITNESS_CONTAINERS):
            print(
                f"[{now()}] waiting... ({len(found)}/{len(WITNESS_CONTAINERS)} witnesses have logged evidence)"
            )
            time.sleep(5)

    print("=== Phase 4: tear down ===")
    stop_duplicate()
    tr.poll_for(30.0, 3.0, "recovery")

    safety_held = len(tr.divergence_events) == 0

    ts_label = time.strftime("%Y%m%dT%H%M%S")
    csv_path = os.path.join(RESULTS_DIR, f"double_signing_{ts_label}.csv")
    write_csv(tr.samples, csv_path)

    summary_path = os.path.join(RESULTS_DIR, f"double_signing_{ts_label}_summary.md")
    with open(summary_path, "w") as f:
        f.write("# LIVE Double-signing detection test (E8 A9)\n\n")
        f.write(
            f"Total duration: {tr.elapsed():.0f}s. Witnesses: {WITNESS_CONTAINERS}.\n\n"
        )
        f.write("## Verdict\n\n")
        f.write(
            f"- {len(found)}/{len(WITNESS_CONTAINERS)} witness nodes logged real "
            f"`DuplicateVoteEvidence`: **{len(found) == len(WITNESS_CONTAINERS)}**\n"
        )
        f.write(
            f"- Safety held (6 honest witnesses' AppHash never diverged at the same height "
            f"during or after the double-signing window): **{safety_held}**\n"
        )
        f.write(f"- Divergence events: {len(tr.divergence_events)}\n\n")
        if tr.divergence_events:
            f.write("### Divergence detail\n\n")
            for t, phase, h, hashes in tr.divergence_events:
                f.write(f"- t={t:.0f}s phase={phase} height={h}: {hashes}\n")
            f.write("\n")
        f.write("## Detection latency\n\n")
        for c, entries in found.items():
            for etype, validator, offense_h, detected_h in entries:
                f.write(
                    f"- {c}: type={etype} validator={validator} offense_height={offense_h} "
                    f"detected_at_height={detected_h} "
                    f"latency={int(detected_h) - int(offense_h)} blocks\n"
                )

    print(f"\nwrote {len(tr.samples)} samples to {csv_path}")
    print(f"wrote summary to {summary_path}")
    print(
        f"\nVERDICT: {len(found)}/{len(WITNESS_CONTAINERS)} witness nodes logged real "
        f"DuplicateVoteEvidence, safety_held={safety_held} divergence_events={len(tr.divergence_events)}"
    )
    for c, entries in found.items():
        for etype, validator, offense_h, detected_h in entries:
            print(
                f"  {c}: type={etype} validator={validator} offense_height={offense_h} "
                f"detected_at_height={detected_h} latency={int(detected_h) - int(offense_h)} blocks"
            )
    if not found:
        print(
            "No evidence detected within the timeout -- this may mean the duplicate process never "
            "actually voted differently from the real node04 (e.g. it was slower to connect, or "
            "CometBFT's own gossip/timing meant they never raced on the same height/round), not "
            "necessarily that detection itself is broken. Re-run, or check both processes' own logs "
            "directly for vote activity."
        )
        sys.exit(1)


if __name__ == "__main__":
    main()
