#!/usr/bin/env python3
"""Open-loop throughput with a FRESH deployment for every (arm, rate, trial) point -- design C of the RQ4 study.

The earlier sweep ran the rates one after another on one chain, so an FSM state reached at 200 tx/s (SOVEREIGN) carried into
the 500 tx/s repeats, and repeats on one chain are not independent deployments. Here every point is its own genesis, and the
interval over trials is a interval over independent deployments. One 60 s open-loop run per point (live_throughput_sweep.py).

    python3 -u scripts/e7_consensus_overhead/run_throughput_fresh.py --arm engram --rates 200,500 --trials 3 \
        --tag da_relaxed --da-rpc-timeout-s 90 --da-check-timeout-ms 2500
    python3 -u scripts/e7_consensus_overhead/run_throughput_fresh.py --arm vanilla7 --rates 500,1000,2000,4000 --trials 3

.env is restored afterwards.
"""

import argparse
import os
import shutil
import subprocess
import sys
import time

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.abspath(os.path.join(HERE, "..", ".."))
ENV = os.path.join(REPO, ".env")


def set_env(name, value):
    with open(ENV) as f:
        lines = f.read().splitlines()
    out, found = [], False
    for line in lines:
        if line.startswith(name + "="):
            out.append(f"{name}={value}")
            found = True
        else:
            out.append(line)
    if not found:
        out.append(f"{name}={value}")
    with open(ENV, "w") as f:
        f.write("\n".join(out) + "\n")


def wait_anchored(timeout_s):
    """Proof-only start-up needs minutes to reach ANCHORED; a load run must start from it."""
    sys.path.insert(0, os.path.join(REPO, "scripts", "framework"))
    from logger import sample_all_nodes  # noqa: E402
    deadline = time.time() + timeout_s
    while time.time() < deadline:
        samples = sample_all_nodes()
        if samples and all(x.fsm_state == "ANCHORED" for x in samples):
            return True
        time.sleep(10)
    return False


def sh(cmd, **kw):
    print("+", " ".join(cmd), flush=True)
    return subprocess.run(cmd, cwd=REPO, **kw)


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--arm", choices=["engram", "vanilla7"], required=True)
    ap.add_argument("--rates", required=True)
    ap.add_argument("--trials", type=int, default=3)
    ap.add_argument("--tag", default="")
    ap.add_argument("--da-rpc-timeout-s", type=int, default=None)
    ap.add_argument("--da-check-timeout-ms", type=int, default=None)
    ap.add_argument("--duration-s", type=float, default=60.0)
    args = ap.parse_args()

    backup = shutil.copy(ENV, ENV + ".throughput_fresh.bak")
    try:
        if args.da_rpc_timeout_s:
            set_env("ENGRAM_DA_RPC_TIMEOUT_S", args.da_rpc_timeout_s)
        if args.da_check_timeout_ms:
            set_env("ENGRAM_DA_CHECK_TIMEOUT_MS", args.da_check_timeout_ms)
        env = dict(os.environ)
        mode = "engram"
        if args.arm == "vanilla7":
            env["ENGRAM_DISABLE_VOTE_EXTENSIONS"] = "1"
            mode = "vanilla"
        for rate in [float(r) for r in args.rates.split(",")]:
            for trial in range(1, args.trials + 1):
                label = f"{args.arm}{'_' + args.tag if args.tag else ''}_r{rate:g}_t{trial}"
                print(f"=== {label}: fresh genesis", flush=True)
                sh([os.path.join(REPO, "scripts", "testnet_redeploy_clean.sh"), mode], env=env)
                ok = sh([sys.executable, os.path.join(REPO, "scripts", "testnet_verify_consensus.py"), "--wait-s", "300"]).returncode == 0
                if not ok:
                    print(f"{label}: consensus verification failed; skipping", flush=True)
                    continue
                if args.arm == "engram" and not wait_anchored(1800):
                    print(f"{label}: never reached ANCHORED; skipping", flush=True)
                    continue
                sh([sys.executable, "-u", os.path.join(HERE, "live_throughput_sweep.py"), "--arm", args.arm, "--rates", str(rate),
                    "--repeats", "1", "--duration-s", str(args.duration_s), "--tag", label])
    finally:
        shutil.move(backup, ENV)
        print("restored .env", flush=True)


if __name__ == "__main__":
    main()
