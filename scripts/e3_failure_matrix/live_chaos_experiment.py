#!/usr/bin/env python3
"""E3/E4 LIVE (real docker testnet, real Pumba injection) -- distinct from
scripts/e3_failure_matrix/measure_latency.py (reads tests/e2e/results/*.csv,
in-process mock-sensor harness) and scripts/e4_p2p_eclipse_detection/
simulate_eclipse_attack.py (synthetic Monte Carlo). Drives the REAL 4-node
engram-nodeNN testnet with a REAL Pumba chaos profile (root compose.yml's
pumba-latency/loss/kill/eclipse services) and polls real nodes' RPC/ABCI-query
state throughout via scripts/framework/logger.py.

Usage:
    python3 scripts/e3_failure_matrix/live_chaos_experiment.py chaos-delay
    python3 scripts/e3_failure_matrix/live_chaos_experiment.py chaos-loss
    python3 scripts/e3_failure_matrix/live_chaos_experiment.py chaos-crash
    python3 scripts/e3_failure_matrix/live_chaos_experiment.py chaos-eclipse

Run from the repo root (needs `docker compose` to resolve compose.yml).
"""

import os
import sys
import time
import argparse

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "framework"))
from logger import (
    poll_timeline,
    write_csv,
    sample_all_nodes,
    NODE_RPC_PORTS,
)  # noqa: E402
import injector  # noqa: E402

RESULTS_DIR = os.path.join(os.path.dirname(__file__), "results_live")

CHAOS_TARGET_CONTAINER = {
    "chaos-delay": None,
    "chaos-loss": None,
    "chaos-crash": "engram-node04",
    "chaos-eclipse": "engram-node01",
}


def fmt_sample(s):
    return f"{s.node}: h={s.height} state={s.fsm_state or '?'} err={s.error or '-'}"


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("profile", choices=list(injector.PROFILE_TO_SERVICE))
    parser.add_argument("--baseline-s", type=float, default=20.0)
    parser.add_argument("--interval-s", type=float, default=3.0)
    parser.add_argument("--post-s", type=float, default=45.0)
    args = parser.parse_args()

    os.makedirs(RESULTS_DIR, exist_ok=True)
    profile = args.profile
    print(f"=== LIVE chaos experiment: {profile} ===")
    print(f"    {injector.PROFILE_DESCRIPTIONS[profile]}")

    target = CHAOS_TARGET_CONTAINER[profile]
    if target:
        print(
            f"target container status before: {target} = {injector.container_status(target)}"
        )

    print(f"\n-- baseline ({args.baseline_s:.0f}s) --")
    all_samples = poll_timeline(
        args.baseline_s,
        args.interval_s,
        on_sample=lambda r: print(" ", " | ".join(fmt_sample(s) for s in r)),
    )

    print(f"\n-- injecting {profile} (polling until Pumba's own --duration elapses) --")
    injector.wait_for_no_active_netem()
    container = injector.start_pumba_profile(profile)
    print(f"  {container} confirmed running/exited")
    during_samples = []
    while injector.is_running(container):
        round_samples = sample_all_nodes()
        during_samples.extend(round_samples)
        print(" ", " | ".join(fmt_sample(s) for s in round_samples))
        time.sleep(args.interval_s)
    all_samples.extend(during_samples)
    print(
        f"pumba container {container} finished, status={injector.container_status(container)}"
    )
    injector.cleanup_profile(profile)

    if target:
        print(
            f"target container status right after chaos: {target} = {injector.container_status(target)}"
        )

    print(f"\n-- post-recovery window ({args.post_s:.0f}s) --")
    post_samples = poll_timeline(
        args.post_s,
        args.interval_s,
        on_sample=lambda r: print(" ", " | ".join(fmt_sample(s) for s in r)),
    )
    all_samples.extend(post_samples)

    if target:
        print(
            f"target container status after recovery window: {target} = {injector.container_status(target)}"
        )

    ts_label = time.strftime("%Y%m%dT%H%M%S")
    csv_path = os.path.join(RESULTS_DIR, f"{profile}_{ts_label}.csv")
    write_csv(all_samples, csv_path)
    print(f"\nwrote {len(all_samples)} samples to {csv_path}")

    # Real summary derived from what was actually observed -- not projected.
    heights_by_node = {}
    errors_by_node = {}
    for s in all_samples:
        heights_by_node.setdefault(s.node, []).append(s.height)
        if s.error:
            errors_by_node.setdefault(s.node, 0)
            errors_by_node[s.node] += 1

    summary_path = os.path.join(RESULTS_DIR, f"{profile}_{ts_label}_summary.md")
    with open(summary_path, "w") as f:
        f.write(f"# LIVE chaos experiment: {profile}\n\n")
        f.write(f"{injector.PROFILE_DESCRIPTIONS[profile]}\n\n")
        f.write(f"Samples: {len(all_samples)}, CSV: `{os.path.basename(csv_path)}`\n\n")
        f.write("| Node | Height range | RPC errors observed |\n|---|---|---:|\n")
        for node in NODE_RPC_PORTS:
            hs = [h for h in heights_by_node.get(node, []) if h >= 0]
            rng = f"{min(hs)}..{max(hs)}" if hs else "n/a"
            f.write(f"| {node} | {rng} | {errors_by_node.get(node, 0)} |\n")
        if target:
            f.write(
                f"\nTarget container `{target}` final status: `{injector.container_status(target)}`\n"
            )
    print(f"wrote summary to {summary_path}")


if __name__ == "__main__":
    main()
