#!/usr/bin/env python3
"""Live non-stationary hysteresis sweep (E5c): static k=2 and k=4 against the adaptive threshold (k0=2, kmax=8) with
StableWindow in {16, 64, 128}, each at several `offset` values (how long the chain was healthy before the outage).

Every run gets a fresh genesis (the adaptive level lives on the chain, and repeated DA outages lengthen recoveries on
a single chain). The run is live_nonstationary.py. .env is restored afterwards.

    python3 -u scripts/e5_hysteresis_flapping/run_nonstationary_sweep.py [--trials 3] [--offsets 0,60]
"""

import argparse
import os
import shutil
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import run_absorb_sweep as ras  # noqa: E402
import run_adaptive_sweep as rad  # noqa: E402

# name -> (DOWN, MAX, STABLE_WINDOW)
CONFIGS = {"static2": (2, 2, 16), "static4": (4, 4, 16), "adaptiveW16": (2, 8, 16), "adaptiveW64": (2, 8, 64),
           "adaptiveW128": (2, 8, 128), "adaptiveK4W64": (2, 4, 64)}


def ensure_stable_window(value: int) -> None:
    with open(ras.ENV_PATH) as f:
        has = "ENGRAM_PARAM_STABLE_WINDOW=" in f.read()
    if not has:
        with open(ras.ENV_PATH, "a") as g:
            g.write("\nENGRAM_PARAM_STABLE_WINDOW=16\n")
    ras.set_env_param("ENGRAM_PARAM_STABLE_WINDOW", value)


def run_one(label: str, offset: int, health_timeout_s: float, runner: str = "live_nonstationary.py") -> str:
    if not ras.wait_for_healthy_anchored(timeout_s=health_timeout_s):
        return "SKIPPED-unhealthy"
    try:
        extra = ["--offset-s", str(offset)] if runner == "live_nonstationary.py" else []
        ras.run([sys.executable, "-u", os.path.join(HERE, runner), "--label", label] + extra)
        return "OK"
    except subprocess.CalledProcessError:
        return "FAILED"


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--configs", default=",".join(CONFIGS))
    ap.add_argument("--trials", type=int, default=3)
    ap.add_argument("--offsets", default="0,60")
    ap.add_argument("--health-timeout-s", type=float, default=1500.0)
    ap.add_argument("--runner", default="live_nonstationary.py",
                    help="live_nonstationary.py (three-phase noise) or live_repeated_trigger.py (adversary pulses)")
    args = ap.parse_args()

    backup = shutil.copy(ras.ENV_PATH, ras.ENV_PATH + ".nonstationary_sweep.bak")
    offsets = [int(x) for x in args.offsets.split(",")]
    runs = []
    try:
        for name in args.configs.split(","):
            down, mx, window = CONFIGS[name]
            ensure_stable_window(window)
            # Every run gets a fresh genesis, static or not: each run ends with a DA outage, and recoveries on one
            # chain get longer with every outage (a b9 run began its fourth trial permanently in RECOVERING).
            for trial in range(1, args.trials + 1):
                for off in offsets:
                    ras.log(f"=== {name} trial {trial} offset {off}: fresh genesis")
                    rad.redeploy(down, mx)
                    status = run_one(f"{name}_o{off}_t{trial}", off, args.health_timeout_s, args.runner)
                    runs.append({"config": name, "trial": trial, "offset": off, "status": status})
                    ras.log(str(runs[-1]))
    finally:
        shutil.move(backup, ras.ENV_PATH)
        ras.log("restored .env")


if __name__ == "__main__":
    main()
