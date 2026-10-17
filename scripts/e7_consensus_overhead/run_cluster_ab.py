#!/usr/bin/env python3
"""Fresh-deployment load comparison on the multi-host cluster (Testbed B): one genesis per (arm, trial), one 60 s
open-loop measurement at 100 tx/s, the settings of the earlier Testbed B load runs (64 senders, 32-byte payload).

Arms (the critical-path comparison of the vote extension):
  vanilla   plain CometBFT, vote extensions off at genesis (ENGRAM_DISABLE_VOTE_EXTENSIONS=1), no prover
  empty     Engram with vote extensions on, ExtendVote signs a zero observation (ENGRAM_SENSOR_MODE=empty)
  sync      Engram, ExtendVote reads Bitcoin/Celestia RPC inline (ENGRAM_SENSOR_MODE=sync): the "before" arm
  snapshot  Engram, ExtendVote composes the cached snapshot (default): the "after" arm

    export DOCKERHUB_USER=... ENGRAM_IMAGE_TAG=... BITCOIN_RPC_PASSWORD=...
    ENGRAM_CLUSTER=ssh python3 -u scripts/e7_consensus_overhead/run_cluster_ab.py --arms vanilla --trials 3

An Engram chain is measured only after all seven validators report ANCHORED; if it does not get there in 30 min the
deployment is redone (at most twice), and every attempt is printed. Raw output is the sweep's CSV/summary per point.
"""

import argparse
import os
import re
import subprocess
import sys
import time

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.abspath(os.path.join(HERE, "..", ".."))
ANSIBLE = os.path.join(REPO, "deploy", "ansible")
sys.path.insert(0, os.path.join(REPO, "scripts", "framework"))
from cluster import CLUSTER  # noqa: E402
from logger import query_node  # noqa: E402

ARMS = {
    "vanilla": dict(engram_mode="vanilla", sensor_mode="snapshot", prover=False, sweep_arm="vanilla7"),
    "empty": dict(engram_mode="engram", sensor_mode="empty", prover=True, sweep_arm="engram"),
    "sync": dict(engram_mode="engram", sensor_mode="sync", prover=True, sweep_arm="engram"),
    "snapshot": dict(engram_mode="engram", sensor_mode="snapshot", prover=True, sweep_arm="engram"),
}


def engram_params() -> dict:
    """The ENGRAM_PARAM_* of group_vars/all.yml, read from the file so the arms cannot drift from the paper's parameters."""
    params, on = {}, False
    for line in open(os.path.join(ANSIBLE, "group_vars", "all.yml")):
        if line.startswith("engram_params:"):
            on = True
        elif on and re.match(r"^\s+ENGRAM_PARAM_\w+:", line):
            k, v = line.split("#")[0].strip().split(":", 1)
            params[k] = v.strip().strip('"')
        elif on and line.strip() and not line.startswith(" ") and not line.startswith("#"):
            break
    if len(params) < 5:
        sys.exit(f"cannot read engram_params from group_vars/all.yml: {params}")
    return params


def operator_cidr() -> str:
    ip = subprocess.run(["curl", "-s", "-m", "10", "https://checkip.amazonaws.com"], capture_output=True, text=True).stdout.strip()
    return f"{ip}/32"


def ansible(playbook: str, extra: dict) -> None:
    import json
    cmd = ["ansible-playbook", playbook, "-e", json.dumps(extra)]
    print("+", " ".join(cmd[:3]), "...", flush=True)
    if subprocess.run(cmd, cwd=ANSIBLE).returncode != 0:
        raise RuntimeError(f"{playbook} failed")


def deploy(arm: str, extra: dict | None = None) -> None:
    a = ARMS[arm]
    params = engram_params()
    if arm == "vanilla":
        params["ENGRAM_DISABLE_VOTE_EXTENSIONS"] = "1"
    ansible("teardown.yml", {"wipe_generated": True})
    ansible("site.yml", {"operator_cidrs": [operator_cidr()], "engram_mode": a["engram_mode"], "sensor_mode": a["sensor_mode"],
                         "prover_enabled": a["prover"], "engram_params": params, **(extra or {})})


def check_config(arm: str) -> None:
    """Every validator runs the arm's mode and sensor mode; a silently wrong container env would label wrong data."""
    a = ARMS[arm]
    for n in CLUSTER.nodes:
        mode = CLUSTER.container_env(n, "ENGRAM_MODE") or "engram"
        sensor = CLUSTER.container_env(n, "ENGRAM_SENSOR_MODE")
        if mode != a["engram_mode"] or (a["engram_mode"] == "engram" and sensor != a["sensor_mode"]):
            raise RuntimeError(f"{n}: ENGRAM_MODE={mode} ENGRAM_SENSOR_MODE={sensor}, arm {arm} needs {a['engram_mode']}/{a['sensor_mode']}")


def wait_anchored(timeout_s: float, stall_s: float = 300.0) -> bool:
    """True once all validators report ANCHORED. False at the timeout, or as soon as no validator's height has advanced for
    stall_s: a stalled chain (the empty arm stuck at height 2) does not recover by waiting, and the cluster bills meanwhile."""
    deadline = time.time() + timeout_s
    best_h, best_t = -1, time.time()
    while time.time() < deadline:
        try:
            samples = [query_node(n) for n in CLUSTER.nodes]
            if all(s.fsm_state == "ANCHORED" for s in samples):
                return True
            h = max(s.height for s in samples)
            if h > best_h:
                best_h, best_t = h, time.time()
            elif time.time() - best_t > stall_s:
                print(f"  chain stalled at height {h} for {stall_s:.0f} s, states {[s.fsm_state for s in samples]}", flush=True)
                return False
        except Exception as exc:  # noqa: BLE001 -- retried until the deadline
            print("  waiting:", exc, flush=True)
        time.sleep(10)
    return False


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--arms", required=True, help="comma-separated: " + ",".join(ARMS))
    ap.add_argument("--trials", type=int, default=3)
    ap.add_argument("--rate", type=float, default=100.0)
    ap.add_argument("--tag", default="b2")
    ap.add_argument("--attempts", type=int, default=1, help="deployments per point; 1 stops at the first failure so the cluster can be destroyed")
    args = ap.parse_args()
    if CLUSTER.name != "ssh":
        sys.exit("run with ENGRAM_CLUSTER=ssh")
    for var in ("DOCKERHUB_USER", "ENGRAM_IMAGE_TAG", "BITCOIN_RPC_PASSWORD"):
        if not os.environ.get(var):
            sys.exit(f"{var} is not set")

    for arm in args.arms.split(","):
        for trial in range(1, args.trials + 1):
            label = f"{args.tag}_{arm}_t{trial}"
            for attempt in range(1, args.attempts + 1):
                print(f"=== {label}: fresh deployment, attempt {attempt} ({time.strftime('%H:%M:%S')}) tag={os.environ['ENGRAM_IMAGE_TAG']}", flush=True)
                deploy(arm)
                check_config(arm)
                if ARMS[arm]["engram_mode"] != "engram" or wait_anchored(1800):
                    break
                print(f"{label}: chain did not reach ANCHORED (stalled or 30 min elapsed)", flush=True)
            else:
                sys.exit(f"{label}: no usable deployment after {args.attempts} attempt(s)")
            sweep = [sys.executable, "-u", os.path.join(HERE, "live_throughput_sweep.py"), "--arm", ARMS[arm]["sweep_arm"],
                     "--rates", str(args.rate), "--repeats", "1", "--duration-s", "60", "--senders", "64", "--tag", label]
            if subprocess.run(sweep, cwd=REPO).returncode != 0:
                sys.exit(f"{label}: measurement failed")


if __name__ == "__main__":
    main()
