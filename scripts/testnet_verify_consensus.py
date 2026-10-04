#!/usr/bin/env python3
"""Deep check that every validator of the running testnet executes the same chain.

Height or "container is up" is not enough: a validator whose application state differs from the others fails at
height 2 with "wrong Block.Header.AppHash" and stays at height 1 while the rest advance, which silently turns an
N-node experiment into an (N-1)-node one. The checks:
  1. every node answers, is not catching_up, and reports the same chain_id and the same genesis file bytes;
  2. every node holds the same block hash and the same app_hash at a height all of them have;
  3. that block's commit carries a precommit from every validator (a quorum is enough for liveness, but fewer
     than N means one validator is not following);
  4. no node's log contains the AppHash-mismatch signature, and no container has restarted.

Exit 0 when all hold, 1 otherwise with one line per failed check.

    python3 scripts/testnet_verify_consensus.py [--wait-s 120]
"""

import argparse
import hashlib
import json
import os
import subprocess
import sys
import time
import urllib.request

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(HERE, "framework"))
from cluster import CLUSTER  # noqa: E402

REPO = os.path.dirname(HERE)
MISMATCH = "wrong Block.Header.AppHash"


def rpc(port, path):
    with urllib.request.urlopen(f"http://127.0.0.1:{port}{path}", timeout=10) as r:
        return json.load(r)["result"]


def docker(*args):
    return subprocess.run(["docker", *args], capture_output=True, text=True, timeout=120)


def check(nodes):
    ports = {n: CLUSTER.rpc_port(n) for n in nodes}
    bad = []
    status = {}
    for n, p in ports.items():
        try:
            status[n] = rpc(p, "/status")
        except Exception as e:  # noqa: BLE001
            bad.append(f"{n}: RPC unreachable ({e})")
    if bad:
        return bad
    for n, s in status.items():
        if s["sync_info"]["catching_up"]:
            bad.append(f"{n}: catching_up at height {s['sync_info']['latest_block_height']}")
    nets = {n: s["node_info"]["network"] for n, s in status.items()}
    if len(set(nets.values())) != 1:
        bad.append(f"chain_id differs: {nets}")

    genesis = {}
    for n in nodes:
        path = os.path.join(REPO, "testnet-data", n, "config", "genesis.json")
        if os.path.exists(path):
            with open(path, "rb") as f:
                genesis[n] = hashlib.sha256(f.read()).hexdigest()[:12]
    if len(set(genesis.values())) > 1:
        bad.append(f"genesis.json differs between nodes: {genesis}")

    heights = {n: int(s["sync_info"]["latest_block_height"]) for n, s in status.items()}
    top = max(heights.values())
    lagging = {n: h for n, h in heights.items() if top - h > 10}
    if lagging:
        return bad + [f"{n}: stuck at height {h} while the others reach {top}" for n, h in lagging.items()]
    probe = min(heights.values()) - 1
    if probe < 1:
        return bad + [f"no common committed height yet: {heights}"]

    blocks = {}
    for n, p in ports.items():
        try:
            b = rpc(p, f"/block?height={probe}")
            blocks[n] = (b["block_id"]["hash"], b["block"]["header"]["app_hash"])
        except Exception as e:  # noqa: BLE001
            bad.append(f"{n}: cannot read block {probe} ({e})")
    if len({v[0] for v in blocks.values()}) > 1:
        bad.append(f"block {probe} hash differs: { {n: v[0][:12] for n, v in blocks.items()} }")
    if len({v[1] for v in blocks.values()}) > 1:
        bad.append(f"block {probe} app_hash differs: { {n: v[1][:12] for n, v in blocks.items()} }")

    try:
        sigs = rpc(ports[nodes[0]], f"/commit?height={probe}")["signed_header"]["commit"]["signatures"]
        signed = sum(1 for x in sigs if x["block_id_flag"] == 2)
        if signed < len(nodes):
            bad.append(f"block {probe} commit holds {signed}/{len(nodes)} precommits")
    except Exception as e:  # noqa: BLE001
        bad.append(f"cannot read commit {probe} ({e})")

    for n in nodes:
        logs = docker("logs", n)
        if MISMATCH in logs.stdout or MISMATCH in logs.stderr:
            bad.append(f"{n}: log contains '{MISMATCH}'")
        restarts = docker("inspect", n, "--format", "{{.RestartCount}}").stdout.strip()
        if restarts not in ("", "0"):
            bad.append(f"{n}: container restarted {restarts} time(s)")
    return bad


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--wait-s", type=float, default=120, help="keep retrying this long before failing")
    args = ap.parse_args()
    nodes = list(CLUSTER.nodes)
    deadline = time.time() + args.wait_s
    while True:
        bad = check(nodes)
        if not bad:
            print(f"consensus verified across {len(nodes)} validators")
            return 0
        if time.time() > deadline:
            print("\n".join(f"FAIL {b}" for b in bad))
            return 1
        time.sleep(10)


if __name__ == "__main__":
    sys.exit(main())
