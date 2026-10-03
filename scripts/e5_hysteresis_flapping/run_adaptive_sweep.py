#!/usr/bin/env python3
"""Adaptive vs static down-hysteresis, live (docs/NEXT_SESSION_PLAN.md item 16, reduced form).

Three configurations of the ANCHORED->SUSPICIOUS threshold, each on a FRESH genesis per trial (the adaptive
threshold keeps its state -- flap_level -- on the chain, so repeated trials on one chain would not be independent):

    static2    DOWN=2, MAX=2   the old default, too low under WAN jitter in E5
    static6    DOWN=6, MAX=6
    adaptive   DOWN=2, MAX=8   starts responsive, backs off after flaps, relaxes when stable

DOWN == MAX is what makes a configuration static: with MAX above DOWN the backoff doubling is active, which is
why E5's earlier "static" values were not purely static when MAX stayed at 8.

Each trial runs live_spot_check_absorb.py (fixed window, WAN latency/jitter profile, optional DA noise) and records
flaps, ANCHORED uptime and demotions per validator. Detection delay is NOT measured here; claims about it stay
with the in-process sweep. .env is restored afterwards.

    python3 -u scripts/e5_hysteresis_flapping/run_adaptive_sweep.py --trials 3 --envs stable
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

CONFIGS = {"static2": (2, 2), "static6": (6, 6), "adaptive": (2, 8)}
RESULTS_DIR = os.path.join(HERE, "results_live")


def redeploy(down: int, max_down: int) -> None:
    ras.set_env_param("ENGRAM_PARAM_DOWN_HYSTERESIS_THRESHOLD", down)
    ras.set_env_param("ENGRAM_PARAM_MAX_DOWN_HYSTERESIS_THRESHOLD", max_down)
    ras.log("stopping validators before wiping testnet-data/")
    cmd = ["docker", "compose", "--env-file", ".env", "-f", "docker-compose.yml"]
    subprocess.run(cmd + ["stop", *ras.VALIDATOR_SERVICES], cwd=ras.REPO_ROOT)
    subprocess.run(cmd + ["rm", "-f", *ras.VALIDATOR_SERVICES], cwd=ras.REPO_ROOT)
    ras.run(["make", "testnet-up"])


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--configs", default="static2,static6,adaptive")
    ap.add_argument("--trials", type=int, default=3)
    ap.add_argument("--envs", default="stable", help="stable (WAN jitter only) and/or noisy_da")
    ap.add_argument("--duration-s", type=float, default=300.0)
    ap.add_argument("--wan-profile", default="chaos-wan-latency")
    ap.add_argument("--health-timeout-s", type=float, default=1500.0)
    args = ap.parse_args()

    original = shutil.copy(ras.ENV_PATH, ras.ENV_PATH + ".adaptive_sweep.bak")
    runs = []
    try:
        for name in args.configs.split(","):
            down, max_down = CONFIGS[name]
            for trial in range(1, args.trials + 1):
                ras.log(f"=== {name} (DOWN={down}, MAX={max_down}) trial {trial}/{args.trials}: fresh genesis ===")
                redeploy(down, max_down)
                for env in args.envs.split(","):
                    if not ras.wait_for_healthy_anchored(timeout_s=args.health_timeout_s):
                        runs.append({"config": name, "trial": trial, "env": env, "status": "SKIPPED-unhealthy"})
                        continue
                    cmd = [sys.executable, "-u", os.path.join(HERE, "live_spot_check_absorb.py"), "--edge", "down",
                           "--value", str(down), "--env", env, "--duration-s", str(args.duration_s),
                           "--wan-profile", args.wan_profile, "--label", f"adaptive_{name}_t{trial}_{env}"]
                    try:
                        ras.run(cmd)
                        runs.append({"config": name, "trial": trial, "env": env, "status": "OK"})
                    except subprocess.CalledProcessError:
                        runs.append({"config": name, "trial": trial, "env": env, "status": "FAILED"})
    finally:
        shutil.move(original, ras.ENV_PATH)
        ras.log("restored .env")
    os.makedirs(RESULTS_DIR, exist_ok=True)
    with open(os.path.join(RESULTS_DIR, f"adaptive_sweep_runs_{time.strftime('%Y%m%dT%H%M%S')}.json"), "w") as f:
        json.dump(runs, f, indent=1)
    for r in runs:
        ras.log(str(r))


if __name__ == "__main__":
    main()
