#!/usr/bin/env python3
"""Adaptive vs static down-hysteresis, live (docs/NEXT_SESSION_PLAN.md item 16, reduced form).

The ANCHORED->SUSPICIOUS threshold at static k in {2, 4, 6, 8} and adaptive (k0=2, kmax=8), measured under WAN
jitter (stable), DA blips (noisy_da) and a sustained DA outage (detect: detection delay). The configurations the
claim compares (static2, static6, adaptive) get --trials-main trials, the others complete the curve over k. The
adaptive threshold keeps its state (flap_level) on the chain, so it gets a fresh genesis for every run:

    staticK    DOWN=K, MAX=K   no hidden state: one genesis serves all its runs
    adaptive   DOWN=2, MAX=8   starts responsive, backs off after flaps, relaxes when stable

DOWN == MAX is what makes a configuration static: with MAX above DOWN the backoff doubling is active, which is
why E5's earlier "static" values were not purely static when MAX stayed at 8.

Each trial runs live_spot_check_absorb.py (fixed window, WAN latency/jitter profile, optional DA noise) and records
flaps, ANCHORED uptime and demotions per validator. Detection delay is NOT measured here; claims about it stay
with the in-process sweep. .env is restored afterwards.

    python3 -u scripts/e5_hysteresis_flapping/run_adaptive_sweep.py
"""

import argparse
import json
import os
import shutil
import subprocess
import sys
import time

HERE = os.path.dirname(__file__)
sys.path.insert(0, HERE)
import run_absorb_sweep as ras  # noqa: E402

CONFIGS = {"static2": (2, 2), "static4": (4, 4), "static6": (6, 6), "static8": (8, 8), "adaptive": (2, 8)}
# The three configurations the claim compares get several trials; the others only complete the curve over k.
MAIN_CONFIGS = ("static2", "static6", "adaptive")
RESULTS_DIR = os.path.join(HERE, "results_live")


def redeploy(down: int, max_down: int) -> None:
    ras.set_env_param("ENGRAM_PARAM_DOWN_HYSTERESIS_THRESHOLD", down)
    ras.set_env_param("ENGRAM_PARAM_MAX_DOWN_HYSTERESIS_THRESHOLD", max_down)
    ras.log("stopping validators before wiping testnet-data/")
    cmd = ["docker", "compose", "--env-file", ".env", "-f", "docker-compose.yml"]
    subprocess.run(cmd + ["stop", *ras.VALIDATOR_SERVICES], cwd=ras.REPO_ROOT)
    subprocess.run(cmd + ["rm", "-f", *ras.VALIDATOR_SERVICES], cwd=ras.REPO_ROOT)
    ras.run(["make", "testnet-up"])


def measure(name: str, trial: int, env: str, down: int, args) -> str:
    if not ras.wait_for_healthy_anchored(timeout_s=args.health_timeout_s):
        return "SKIPPED-unhealthy"
    label = f"adaptive_{name}_t{trial}_{env}"
    if env == "detect":
        cmd = [sys.executable, "-u", os.path.join(HERE, "live_detection_probe.py"), "--label", label]
    else:
        cmd = [sys.executable, "-u", os.path.join(HERE, "live_spot_check_absorb.py"), "--edge", "down",
               "--value", str(down), "--env", env, "--duration-s", str(args.duration_s),
               "--wan-profile", args.wan_profile, "--label", label]
    try:
        ras.run(cmd)
        return "OK"
    except subprocess.CalledProcessError:
        return "FAILED"


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--configs", default=",".join(CONFIGS))
    ap.add_argument("--trials-main", type=int, default=3, help="trials for static2, static6 and adaptive")
    ap.add_argument("--trials-other", type=int, default=1, help="trials for the remaining static points")
    ap.add_argument("--envs", default="stable,noisy_da,detect",
                    help="stable (WAN jitter only), noisy_da (DA blips), detect (sustained outage: detection delay)")
    ap.add_argument("--duration-s", type=float, default=300.0)
    ap.add_argument("--wan-profile", default="chaos-wan-latency")
    ap.add_argument("--health-timeout-s", type=float, default=1500.0)
    ap.add_argument("--stable-window", type=int, default=None,
                    help="StableWindow for the whole sweep (healthy ANCHORED blocks that lower the flap level by one); "
                    "result labels get an SW<n> suffix so they never mix with the default-window runs")
    args = ap.parse_args()

    backup = shutil.copy(ras.ENV_PATH, ras.ENV_PATH + ".adaptive_sweep.bak")
    envs = args.envs.split(",")
    runs = []
    suffix = f"SW{args.stable_window}" if args.stable_window else ""
    try:
        if args.stable_window:
            ras.set_env_param("ENGRAM_PARAM_STABLE_WINDOW", args.stable_window)
        for name in args.configs.split(","):
            down, max_down = CONFIGS[name]
            trials = args.trials_main if name in MAIN_CONFIGS else args.trials_other
            # A static threshold (DOWN == MAX) keeps no hidden state, so one genesis serves all its trials and
            # environments. The adaptive one keeps flap_level on the chain: a fresh genesis for every run.
            fresh_each_run = down != max_down
            if not fresh_each_run:
                ras.log(f"=== {name} (DOWN={down}, MAX={max_down}): one genesis, {trials} trial(s) ===")
                redeploy(down, max_down)
            for trial in range(1, trials + 1):
                for env in envs:
                    if fresh_each_run:
                        ras.log(f"=== {name} (DOWN={down}, MAX={max_down}) trial {trial} env {env}: fresh genesis ===")
                        redeploy(down, max_down)
                    status = measure(name + suffix, trial, env, down, args)
                    runs.append({"config": name, "trial": trial, "env": env, "status": status})
    finally:
        shutil.move(backup, ras.ENV_PATH)
        ras.log("restored .env")
    os.makedirs(RESULTS_DIR, exist_ok=True)
    with open(os.path.join(RESULTS_DIR, f"adaptive_sweep_runs_{time.strftime('%Y%m%dT%H%M%S')}.json"), "w") as f:
        json.dump(runs, f, indent=1)
    for r in runs:
        ras.log(str(r))


if __name__ == "__main__":
    main()
