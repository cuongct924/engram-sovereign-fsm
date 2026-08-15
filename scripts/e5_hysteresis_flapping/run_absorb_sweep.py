#!/usr/bin/env python3
"""Orchestrates E5's 5b/5c live spot-check sweep (docs/EXPERIMENT.md) across the
in-process sweep's value set, DownHysteresisThreshold / SuspiciousHysteresisWait
in {1,2,4,6,8}, x2 environments (stable, noisy_da -- matching 5a's 5x2 design):
for each (edge, value) pair, redeploys the real 4-node testnet with that param
baked into a fresh genesis, then runs live_spot_check_absorb.py's 300s
measurement window once per env against that same deployed genesis (no
redeploy needed between envs, just re-waiting for a healthy ANCHORED
baseline). live_spot_check_absorb.py itself only measures one already-deployed
(value, env) pair; this script is what drives the redeploy between values.

Bitcoin regtest and Celestia stay running for the whole sweep (only engramd's
genesis/Params changes between iterations, not their state) -- `make
testnet-up`'s wallet-funding and token-fetch steps are idempotent
(scripts/testnet_fund_wallet.sh), so re-running it per iteration only pays the
cost of a fresh genesis + validator restart, not a full from-scratch bring-up.
Re-mining is never triggered: CLAUDE.md's "Bitcoin regtest must mine
continuously" -- this script never bursts blocks itself.

Every fresh genesis cold-starts h_btc_anchored=0, so btc_gap is huge (real
live BTC tip minus 0) until the first checkpoint reaches k_deep_finality+1
confirmations (~60-90s of real BTC blocks) -- IsCriticalCondition has no
hysteresis (5a's own finding), so this reliably forces SOVEREIGN immediately
after every redeploy, self-resolving through RECOVERING back to ANCHORED only
once the anchor/ZK pipeline catches up. Measured once at ~17 minutes
wall-clock on this machine. --health-timeout-s must cover this, not just
RPC-reachability, or the measurement window starts mid-recovery and
contaminates the very absorb-rate/uptime numbers being measured.

Usage (from repo root, needs `make testnet-up` prerequisites already met --
.env with BITCOIN_RPC_USER/PASSWORD filled in, docker daemon running):

    python3 -u scripts/e5_hysteresis_flapping/run_absorb_sweep.py
    python3 -u scripts/e5_hysteresis_flapping/run_absorb_sweep.py --edges down --values 1,4
"""

import argparse
import os
import re
import subprocess
import sys
import time

REPO_ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
ENV_PATH = os.path.join(REPO_ROOT, ".env")

sys.path.insert(0, os.path.join(REPO_ROOT, "scripts", "framework"))
from logger import sample_all_nodes  # noqa: E402

PARAM_FIELD = {
    "down": "ENGRAM_PARAM_DOWN_HYSTERESIS_THRESHOLD",
    "suspicious_exit": "ENGRAM_PARAM_SUSPICIOUS_HYSTERESIS_WAIT",
}
VALIDATOR_SERVICES = [
    "engram-node01",
    "engram-node02",
    "engram-node03",
    "engram-node04",
    "reanchoring-prover",
]


def now() -> str:
    return time.strftime("%H:%M:%S", time.gmtime())


def log(msg: str) -> None:
    print(f"[{now()}] {msg}", flush=True)


def run(cmd, **kwargs):
    log(f">>> {' '.join(cmd)}")
    subprocess.run(cmd, cwd=REPO_ROOT, check=True, **kwargs)


def set_env_param(field: str, value: int) -> None:
    with open(ENV_PATH) as f:
        lines = f.readlines()
    pattern = re.compile(rf"^{re.escape(field)}=")
    found = False
    for i, line in enumerate(lines):
        if pattern.match(line):
            comment = ""
            if "#" in line:
                comment = "  #" + line.split("#", 1)[1].rstrip("\n")
            lines[i] = f"{field}={value}{comment}\n"
            found = True
            break
    if not found:
        raise RuntimeError(f"{field} not found in {ENV_PATH}")
    with open(ENV_PATH, "w") as f:
        f.writelines(lines)
    log(f"set {field}={value} in .env")


def redeploy(edge: str, value: int) -> None:
    set_env_param(PARAM_FIELD[edge], value)

    log("stopping validators + prover before wiping testnet-data/")
    subprocess.run(
        ["docker", "compose", "--env-file", ".env", "-f", "compose.yml", "stop", *VALIDATOR_SERVICES],
        cwd=REPO_ROOT,
    )
    subprocess.run(
        ["docker", "compose", "--env-file", ".env", "-f", "compose.yml", "rm", "-f", *VALIDATOR_SERVICES],
        cwd=REPO_ROOT,
    )

    run(["make", "testnet-up"])


def wait_for_healthy_anchored(timeout_s: float = 240.0, interval_s: float = 5.0) -> bool:
    """Polls until all 4 validators report catching_up=False, a positive
    height, and fsm_state=ANCHORED on 2 consecutive samples -- the baseline
    live_spot_check_absorb.py's 5b measurement (and 5c's drive-to-SUSPICIOUS
    pre-phase) assumes.
    """
    deadline = time.time() + timeout_s
    consecutive_ok = 0
    while time.time() < deadline:
        samples = sample_all_nodes()
        states = {s.node: (s.fsm_state, s.height, s.catching_up, s.error) for s in samples}
        log(f"health poll: {states}")
        healthy = (
            len(samples) == 4
            and all(not s.error for s in samples)
            and all(not s.catching_up for s in samples)
            and all(s.height > 0 for s in samples)
            and all(s.fsm_state == "ANCHORED" for s in samples)
        )
        if healthy:
            consecutive_ok += 1
            if consecutive_ok >= 2:
                return True
        else:
            consecutive_ok = 0
        time.sleep(interval_s)
    return False


def run_spot_check(edge: str, value: int, env: str, duration_s: float, wan_profile: str) -> None:
    cmd = [
        sys.executable,
        "-u",
        os.path.join(REPO_ROOT, "scripts", "e5_hysteresis_flapping", "live_spot_check_absorb.py"),
        "--edge", edge,
        "--value", str(value),
        "--env", env,
        "--duration-s", str(duration_s),
        "--wan-profile", wan_profile,
    ]
    run(cmd)


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--edges", default="down,suspicious_exit")
    parser.add_argument("--values", default="1,2,4,6,8")
    parser.add_argument("--envs", default="stable,noisy_da", help="matches 5a's 5x2 design; comma-separated subset of stable,noisy_da")
    parser.add_argument("--duration-s", type=float, default=300.0)
    parser.add_argument("--wan-profile", default="chaos-wan-latency", choices=["none", "chaos-wan-latency", "chaos-wan-loss"])
    parser.add_argument("--health-timeout-s", type=float, default=1500.0)
    args = parser.parse_args()

    edges = args.edges.split(",")
    values = [int(v) for v in args.values.split(",")]
    envs = args.envs.split(",")
    for e in edges:
        if e not in PARAM_FIELD:
            sys.exit(f"unknown edge {e!r}, must be one of {list(PARAM_FIELD)}")
    for e in envs:
        if e not in ("stable", "noisy_da"):
            sys.exit(f"unknown env {e!r}, must be one of stable, noisy_da")

    log(f"=== E5b/5c absorb sweep: edges={edges} values={values} envs={envs} duration={args.duration_s:.0f}s ===")
    results = []
    for edge in edges:
        for value in values:
            log(f"--- {edge}={value}: redeploying ---")
            redeploy(edge, value)

            # Same deployed genesis serves every env at this value -- no
            # redeploy between envs, just the ANCHORED-baseline wait each
            # measurement window assumes, re-run before every env including
            # the first.
            for env in envs:
                log(f"waiting for cluster to reach healthy ANCHORED baseline ({edge}={value}, env={env})")
                if not wait_for_healthy_anchored(timeout_s=args.health_timeout_s):
                    log(f"ERROR: cluster did not reach healthy ANCHORED within {args.health_timeout_s:.0f}s, skipping {edge}={value} env={env}")
                    results.append((edge, value, env, "SKIPPED-unhealthy"))
                    continue

                log(f"--- {edge}={value} env={env}: running {args.duration_s:.0f}s measurement ---")
                try:
                    run_spot_check(edge, value, env, args.duration_s, args.wan_profile)
                    results.append((edge, value, env, "OK"))
                except subprocess.CalledProcessError as ex:
                    log(f"ERROR: spot-check failed for {edge}={value} env={env}: {ex}")
                    results.append((edge, value, env, "FAILED"))

    log("=== sweep complete ===")
    for edge, value, env, status in results:
        log(f"  {edge}={value} env={env}: {status}")


if __name__ == "__main__":
    main()
