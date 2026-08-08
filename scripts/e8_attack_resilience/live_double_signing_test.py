#!/usr/bin/env python3
"""LIVE Double-signing detection test against the real 4-node testnet --
docs/EXPERIMENT.md's E8 "Double-signing" row. Starts a SECOND, independent
`engramd` process (docker/engram-validator-node04-duplicate.yml) holding the
exact same priv_validator_key.json as the real engram-node04, but with its
own separate priv_validator_state.json (deliberately NOT sharing node04's
signing-history file -- see that compose file's own doc for why sharing it
would prevent double-signing from ever happening at all, since FilePV's
state file is CometBFT's built-in anti-double-sign safety net).

Detection channel: x/sovereignty/preblock.go's recordDetectedEvidence prints
"SLASHABLE EVIDENCE DETECTED" to stdout on every honest validator once
CometBFT's own (stock, unmodified) evidence pool reports real
DuplicateVoteEvidence -- this script polls `docker logs` on the 3 real
validators (not node04 itself, nor the duplicate) for that marker, rather
than requiring a new gRPC query (none exists for this yet -- the detected
evidence IS committed to queryable keeper state, x/sovereignty/keeper.go's
DetectedEvidenceCount/LastDetectedEvidence, just not yet exposed through a
dedicated Query RPC, which would need a new .proto message + `make
proto-gen`; log-grepping is a real, honest detection channel in the
meantime, not a placeholder).

Usage:
    python3 -u scripts/e8_attack_resilience/live_double_signing_test.py
"""

import os
import re
import subprocess
import sys
import time

DUPLICATE_SERVICE = "engram-node04-duplicate"
WITNESS_CONTAINERS = ["engram-node01", "engram-node02", "engram-node03"]
EVIDENCE_MARKER = re.compile(
    r"SLASHABLE EVIDENCE DETECTED type=(\S+) validator=(\S+) offense_height=(\d+) detected_at_height=(\d+)"
)


def now() -> str:
    return time.strftime("%H:%M:%S", time.gmtime())


def compute_persistent_peers() -> str:
    """Builds the id@host:26656 list for the 3 OTHER real validators (not
    node04 itself, whose identity the duplicate is impersonating). Node IDs
    come from each real, already-running node's own /status RPC -- real and
    authoritative, avoids reimplementing CometBFT's ID-from-pubkey derivation
    here.
    """
    import json

    peers = ["engram-node01", "engram-node02", "engram-node03"]
    port_map = {"engram-node01": 26657, "engram-node02": 26757, "engram-node03": 26857}
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


def start_duplicate(persistent_peers: str) -> None:
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
            DUPLICATE_SERVICE,
        ],
        capture_output=True,
        text=True,
        timeout=120,
        check=True,
        env=env,
    )


def stop_duplicate() -> None:
    # NEVER `docker compose down` (no service names) -- tears down the WHOLE
    # project (real cluster included), not just this profile. See
    # scripts/e4_p2p_eclipse_detection/live_sybil_attack.py's swarm_down doc
    # for the live incident this is fixed from.
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
    proc = subprocess.run(
        ["docker", "logs", container], capture_output=True, text=True, timeout=30
    )
    return [m.groups() for m in EVIDENCE_MARKER.finditer(proc.stdout + proc.stderr)]


def main():
    print("=== E8 live Double-signing detection test ===")
    print("=== Phase 1: baseline (confirm no pre-existing evidence in logs) ===")
    for c in WITNESS_CONTAINERS:
        existing = scan_logs_for_evidence(c)
        if existing:
            print(
                f"WARNING: {c} already has {len(existing)} evidence log line(s) from a PRIOR run -- "
                f"detection-latency numbers below may be stale. Consider a fresh redeploy first."
            )

    print("=== Phase 2: start duplicate-key harness ===")
    persistent_peers = compute_persistent_peers()
    start_duplicate(persistent_peers)

    print(
        "=== Phase 3: poll for real DuplicateVoteEvidence in the 3 witness nodes' logs (up to 300s) ==="
    )
    deadline = time.time() + 300
    found = {}
    while time.time() < deadline and len(found) < len(WITNESS_CONTAINERS):
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

    print(
        f"\nVERDICT: {len(found)}/{len(WITNESS_CONTAINERS)} witness nodes logged real DuplicateVoteEvidence"
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
