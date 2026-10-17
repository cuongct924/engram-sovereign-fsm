#!/usr/bin/env python3
"""Five composite-fault runs on the multi-host cluster (Testbed B), each on a fresh genesis (snapshot arm, proof-only recovery),
the protocol of the paper. After every run, before the next teardown: ExtendVote ms per height and the prover log are saved.

    ENGRAM_CLUSTER=ssh python3 -u scripts/e9_composite_fault/run_composite_x5.py --runs 5
"""
import argparse
import os
import re
import subprocess
import sys
import time

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.abspath(os.path.join(HERE, "..", ".."))
sys.path.insert(0, os.path.join(REPO, "scripts", "framework"))
sys.path.insert(0, os.path.join(REPO, "scripts", "e7_consensus_overhead"))
from cluster import CLUSTER  # noqa: E402
import run_cluster_ab as ab  # noqa: E402

EXTEND_RE = re.compile(r"extend_vote height=(\d+) ms=([0-9.]+)")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--runs", type=int, default=5)
    ap.add_argument("--start", type=int, default=1)
    ap.add_argument("--out", default=os.path.join(HERE, "results_live", "testbed_b_snapshot"))
    args = ap.parse_args()
    if CLUSTER.name != "ssh":
        sys.exit("run with ENGRAM_CLUSTER=ssh")
    for var in ("DOCKERHUB_USER", "ENGRAM_IMAGE_TAG", "BITCOIN_RPC_PASSWORD"):
        if not os.environ.get(var):
            sys.exit(f"{var} is not set")
    os.makedirs(args.out, exist_ok=True)
    for i in range(args.start, args.start + args.runs):
        print(f"=== composite run {i}: fresh deployment ({time.strftime('%H:%M:%S')})", flush=True)
        ab.deploy("snapshot")
        ab.check_config("snapshot")
        if not ab.wait_anchored(1800):
            sys.exit(f"run {i}: chain not ANCHORED before the faults")
        t_start = int(time.time())
        env = dict(os.environ, ENGRAM_E9_RESULTS_DIR=args.out)
        if subprocess.run([sys.executable, "-u", os.path.join(HERE, "live_composite_fault.py")], env=env, cwd=REPO).returncode != 0:
            sys.exit(f"run {i}: composite script failed")
        with open(os.path.join(args.out, f"run{i}_extend_vote_ms.csv"), "w") as f:
            f.write("node,height,ms\n")
            for n in CLUSTER.nodes:
                for m in EXTEND_RE.finditer(CLUSTER.logs(n)):
                    f.write(f"{n},{m.group(1)},{m.group(2)}\n")
        with open(os.path.join(args.out, f"run{i}_prover.log"), "w") as f:
            f.write(CLUSTER.prover_logs(t_start - 60))
        print(f"=== composite run {i} saved", flush=True)


if __name__ == "__main__":
    main()
