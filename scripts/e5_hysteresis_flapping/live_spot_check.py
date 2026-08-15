#!/usr/bin/env python3
"""LIVE hysteresis spot-check against the real 4-node testnet --
docs/EXPERIMENT.md's E5, scoped as a small confirmatory check (2 values x 2
environments), NOT a full 6x5 sweep. The in-process sweep
(tests/e2e/hysteresis_sweep_test.go) already produced the real finding this
checks: anchored_uptime decreases and flapping_count INCREASES monotonically
as HysteresisWait grows under sustained noise -- no interior sweet spot (see
docs/EXPERIMENT.md's E5 for the mechanism). Purpose here is confirming that
holds under REAL consensus timing noise (round-skip stalls, real jitter),
which the in-process harness can't reproduce -- not re-deriving it.

A full live sweep is impractical unattended: each HysteresisWait value needs
its own fresh genesis + redeploy cycle (several minutes minimum). HysteresisWait
is NOT compile-time-fixed -- ENGRAM_PARAM_HYSTERESIS_WAIT in .env, read by
`engramd testnet init-files` at genesis-generation time and carried to the
running Keeper by app/app.go's newInitChainer (gs.Params.ToParams()),
overriding params.go's DefaultParams() without a rebuild (docs/DEVELOPMENT.md
§3). This script measures ONE already-deployed cluster per invocation -- set
ENGRAM_PARAM_HYSTERESIS_WAIT in .env, wipe testnet-data/, regenerate genesis,
redeploy, THEN run with --hysteresis-wait matching what was deployed:

    # after setting ENGRAM_PARAM_HYSTERESIS_WAIT=2 and regenerating genesis:
    python3 -u scripts/e5_hysteresis_flapping/live_spot_check.py --hysteresis-wait 2 --env noisy_da
    # after setting ENGRAM_PARAM_HYSTERESIS_WAIT=10 and regenerating genesis:
    python3 -u scripts/e5_hysteresis_flapping/live_spot_check.py --hysteresis-wait 10 --env stable
    python3 -u scripts/e5_hysteresis_flapping/live_spot_check.py --hysteresis-wait 10 --env noisy_da

Metric caveat, honestly flagged: FlappingCount/AnchoredUptime are computed
from fixed-interval RPC polling, not the in-process harness's per-BLOCK
timeline -- a transition that starts AND fully reverses within one polling
interval can be missed. A real granularity difference, reported explicitly
in the summary output.
"""

import argparse
import os
import subprocess
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "framework"))
from logger import sample_all_nodes, write_csv  # noqa: E402
from injector import start_pumba_wan_profile, cleanup_wan_profile  # noqa: E402

RESULTS_DIR = os.path.join(os.path.dirname(__file__), "results_live")
CELESTIA_BRIDGE = "celestia-bridge"

ENVIRONMENTS = {
    "stable": {"description": "no injected noise, control group"},
    "noisy_da": {
        "description": "repeated docker stop/start celestia-bridge cycling, proven mechanism from live_lifecycle_test.py"
    },
}


def now() -> str:
    return time.strftime("%H:%M:%S", time.gmtime())


def docker(*args):
    subprocess.run(["docker", *args], capture_output=True, text=True, timeout=30)


def compute_metrics(states_by_node: dict) -> dict:
    """Mirrors tests/e2e/harness.go's ComputeMetrics (FlappingCount) and
    hysteresis_sweep_test.go's anchoredUptime, applied per-node to a
    fixed-interval sample sequence -- see module doc for the granularity
    caveat versus the in-process per-block version.
    """
    out = {}
    for node, states in states_by_node.items():
        states = [s for s in states if s]  # drop empty/error samples
        if not states:
            out[node] = {
                "flapping_count": 0,
                "total_transitions": 0,
                "anchored_uptime": 0.0,
                "samples": 0,
            }
            continue
        prev = states[0]
        last_transition = None
        flapping = 0
        total_transitions = 0
        for s in states[1:]:
            if s != prev:
                total_transitions += 1
                transition = f"{prev}->{s}"
                reverse = f"{s}->{prev}"
                if last_transition == reverse:
                    flapping += 1
                last_transition = transition
            prev = s
        anchored_uptime = sum(1 for s in states if s == "ANCHORED") / len(states)
        out[node] = {
            "flapping_count": flapping,
            "total_transitions": total_transitions,
            "anchored_uptime": round(anchored_uptime, 4),
            "samples": len(states),
        }
    return out


def main():
    parser = argparse.ArgumentParser(
        description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter
    )
    parser.add_argument(
        "--hysteresis-wait",
        type=int,
        required=True,
        help="the HysteresisWait value params.go was rebuilt+redeployed with (label only, not enforced)",
    )
    parser.add_argument("--env", choices=list(ENVIRONMENTS.keys()), required=True)
    parser.add_argument("--duration-s", type=float, default=300.0)
    parser.add_argument("--interval-s", type=float, default=3.0)
    parser.add_argument(
        "--noise-on-s",
        type=float,
        default=20.0,
        help="noisy_da: seconds celestia-bridge stays down per cycle",
    )
    parser.add_argument(
        "--noise-off-s",
        type=float,
        default=20.0,
        help="noisy_da: seconds celestia-bridge stays up per cycle",
    )
    parser.add_argument(
        "--wan-profile",
        choices=["none", "chaos-wan-latency", "chaos-wan-loss"],
        default="chaos-wan-latency",
        help="per-validator WAN-realism baseline held for the whole run (each of the 4 "
        "validators gets a DIFFERENT delay/jitter or loss%%, approximating distinct real "
        "regions rather than one uniform condition) -- 'none' reproduces the old uniform-LAN "
        "behavior. Mutually exclusive with itself only: never combine latency+loss, both add a "
        "root netem qdisc on the same interface and the second one fails to apply.",
    )
    args = parser.parse_args()
    if args.wan_profile != "none" and args.duration_s > 570:
        print(
            f"WARNING: --duration-s={args.duration_s:.0f} exceeds the WAN profile's own "
            "10-minute (600s) --duration -- it will expire mid-run, silently reverting to "
            "uniform-LAN conditions for the remainder.",
            file=sys.stderr,
        )

    os.makedirs(RESULTS_DIR, exist_ok=True)
    print(
        f"=== E5 live spot-check: HysteresisWait={args.hysteresis_wait} env={args.env} "
        f"({ENVIRONMENTS[args.env]['description']}) wan_profile={args.wan_profile} ==="
    )
    print(
        "NOTE: this script does NOT verify the deployed cluster actually has this HysteresisWait "
        "value -- confirm ENGRAM_PARAM_HYSTERESIS_WAIT + genesis regeneration + redeploy matched "
        "--hysteresis-wait before trusting this run's label."
    )

    if args.wan_profile != "none":
        print(f"[{now()}] >>> starting {args.wan_profile} (WAN-realism baseline)")
        start_pumba_wan_profile(args.wan_profile)

    states_by_node: dict = {}
    all_samples = []
    start = time.time()
    deadline = start + args.duration_s
    next_noise_toggle = start
    da_currently_down = False

    try:
        while time.time() < deadline:
            t = time.time() - start
            round_samples = sample_all_nodes()
            all_samples.extend(round_samples)
            for s in round_samples:
                states_by_node.setdefault(s.node, []).append(s.fsm_state)
            states = {s.node: s.fsm_state for s in round_samples}
            print(f"[{t:6.0f}s] {states}")

            if args.env == "noisy_da" and time.time() >= next_noise_toggle:
                if da_currently_down:
                    print(f"[{now()}] >>> restoring celestia-bridge (noise cycle)")
                    docker("start", CELESTIA_BRIDGE)
                    next_noise_toggle = time.time() + args.noise_off_s
                else:
                    print(f"[{now()}] >>> stopping celestia-bridge (noise cycle)")
                    docker("stop", CELESTIA_BRIDGE)
                    next_noise_toggle = time.time() + args.noise_on_s
                da_currently_down = not da_currently_down

            time.sleep(args.interval_s)

        if args.env == "noisy_da" and da_currently_down:
            print(f"[{now()}] >>> restoring celestia-bridge (cleanup)")
            docker("start", CELESTIA_BRIDGE)
    finally:
        if args.wan_profile != "none":
            print(f"[{now()}] >>> stopping {args.wan_profile} (WAN-realism baseline)")
            cleanup_wan_profile(args.wan_profile)

    metrics = compute_metrics(states_by_node)

    ts_label = time.strftime("%Y%m%dT%H%M%S")
    csv_path = os.path.join(
        RESULTS_DIR,
        f"live_spot_check_hw{args.hysteresis_wait}_{args.env}_{ts_label}.csv",
    )
    write_csv(all_samples, csv_path)

    summary_path = os.path.join(
        RESULTS_DIR,
        f"live_spot_check_hw{args.hysteresis_wait}_{args.env}_{ts_label}_summary.md",
    )
    with open(summary_path, "w") as f:
        f.write(
            f"# LIVE E5 spot-check: HysteresisWait={args.hysteresis_wait}, env={args.env}\n\n"
        )
        f.write(
            f"{ENVIRONMENTS[args.env]['description']}. Duration: {args.duration_s:.0f}s, "
            f"polling interval: {args.interval_s:.0f}s, WAN-realism baseline: {args.wan_profile}.\n\n"
        )
        f.write(
            "**Granularity caveat:** metrics below are computed from fixed-interval polling, not "
            "per-block state -- see this script's module doc.\n\n"
        )
        f.write(
            "| Node | Samples | Total Transitions | Flapping Count | Anchored Uptime |\n"
        )
        f.write("|---|---:|---:|---:|---:|\n")
        for node, m in metrics.items():
            f.write(
                f"| {node} | {m['samples']} | {m['total_transitions']} | {m['flapping_count']} | "
                f"{m['anchored_uptime']:.2%} |\n"
            )

    print(f"\nwrote {len(all_samples)} samples to {csv_path}")
    print(f"wrote summary to {summary_path}")
    print(f"\nMetrics: {metrics}")


if __name__ == "__main__":
    main()
