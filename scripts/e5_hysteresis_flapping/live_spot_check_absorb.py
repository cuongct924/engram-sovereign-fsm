#!/usr/bin/env python3
"""LIVE spot-check for E5's 5b/5c sub-scenarios (docs/EXPERIMENT.md) --
down-hysteresis (ANCHORED -> SUSPICIOUS, DownHysteresisThreshold) and
SUSPICIOUS-exit hysteresis (SUSPICIOUS -> ANCHORED, SuspiciousHysteresisWait)
against the real 4-node testnet. Sibling to live_spot_check.py (5a), kept
separate so a bug here can't regress the already-proven 5a workflow.
Both params are genesis params, not compile-time-fixed: set
ENGRAM_PARAM_DOWN_HYSTERESIS_THRESHOLD / ENGRAM_PARAM_SUSPICIOUS_HYSTERESIS_WAIT
in .env, wipe testnet-data/, regenerate genesis, redeploy, then run with
--edge/--value matching what was deployed (docs/DEVELOPMENT.md §3).

Noise source: celestia-bridge stop/start (the live-proven mechanism
live_spot_check.py uses for noisy_da). DA unhealthiness is warning-level only
(types.IsDAHealthy has no critical path, x/sovereignty/types/predicates.go),
so it exercises down/SUSPICIOUS-exit hysteresis's absorb path without risking
an accidental critical-level bypass -- unlike a BTC-gap-based noise source
(calibrating h_btc_current into the Suspicious..Sovereign band via
ANCHOR_SUBMISSION_PAUSED_FILE needs tight closed-loop gap monitoring this
script doesn't attempt).

--edge down (5b): cluster assumed to start ANCHORED (the chain's own genesis
default) -- measures directly.
--edge suspicious_exit (5c): a pre-phase stops celestia-bridge and polls until
every node reports SUSPICIOUS (bounded by --drive-timeout-s) before the
measurement window starts, mirroring tests/e2e/suspicious_exit_sweep_test.go's
driveToSuspicious.

--env stable vs noisy_da mirrors live_spot_check.py's (5a) same flag, but the MEANING inverts for
--edge suspicious_exit -- see ENVIRONMENTS below. stable never toggles celestia-bridge during the
measurement window; for --edge down that's a 100%-healthy control, for --edge suspicious_exit
(which still needs the drive-phase stop to reach SUSPICIOUS first) it's the opposite: celestia-bridge
stays down for the whole window, a sustained-warning control. noisy_da toggles on the
--noise-on-s/--noise-off-s cadence, defaulted asymmetrically per edge (see --noise-off-s's help)
to approximate tests/e2e/{hysteresis,suspicious_exit}_sweep_test.go's actual
noiseProbability=0.2 model -- down's noise is a minority warning blip against a healthy baseline
(matches 5a's own noisy_da), but suspicious_exit's noise is a minority HEALTHY blip against a
sustained-warning baseline (inverted), so a symmetric 50/50 toggle would misrepresent it.

Usage, once per (edge, value, env) combination already deployed:

    # after redeploying with ENGRAM_PARAM_DOWN_HYSTERESIS_THRESHOLD=4:
    python3 -u scripts/e5_hysteresis_flapping/live_spot_check_absorb.py --edge down --value 4 --env stable
    python3 -u scripts/e5_hysteresis_flapping/live_spot_check_absorb.py --edge down --value 4 --env noisy_da
    # after redeploying with ENGRAM_PARAM_SUSPICIOUS_HYSTERESIS_WAIT=4:
    python3 -u scripts/e5_hysteresis_flapping/live_spot_check_absorb.py --edge suspicious_exit --value 4 --env stable
    python3 -u scripts/e5_hysteresis_flapping/live_spot_check_absorb.py --edge suspicious_exit --value 4 --env noisy_da

Granularity caveat (shared with live_spot_check.py): metrics are computed
from fixed-interval RPC polling, not per-block state -- a transition that
starts and fully reverses within one polling interval can be missed.
"""

import argparse
import csv
import os
import re
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "framework"))
from logger import sample_all_nodes, write_csv  # noqa: E402
from injector import start_pumba_wan_profile, cleanup_wan_profile  # noqa: E402

RESULTS_DIR = os.path.join(os.path.dirname(__file__), "results_live")
CELESTIA_BRIDGE = "celestia-bridge"

ENVIRONMENTS = {
    "stable": {
        "description": "down (5b): no injected noise, 100% healthy control. "
        "suspicious_exit (5c): sustained warning, 0% healthy blips -- the "
        "opposite control (system must exhaust MaxSuspiciousTime)."
    },
    "noisy_da": {
        "description": "down (5b): ~80% healthy baseline, ~20% warning blips. "
        "suspicious_exit (5c): ~80% warning baseline, ~20% healthy blips -- "
        "inverted from down, matching suspicious_exit_sweep_test.go's actual model."
    },
}


def now() -> str:
    return time.strftime("%H:%M:%S", time.gmtime())


def docker(*args):
    import subprocess

    subprocess.run(["docker", *args], capture_output=True, text=True, timeout=30)


def compute_metrics(edge: str, states_by_node: dict, susp_duration_by_node: dict) -> dict:
    """Mirrors live_spot_check.py's compute_metrics for the shared flapping/
    total_transitions/anchored_uptime fields, plus edge-specific Tier 1 additions
    (docs/EXPERIMENT.md's E5 Metrics table): --edge down adds
    time_outside_anchored/demotion_count, --edge suspicious_exit adds
    exit_count/max_suspicious_duration. Both are direction-specific transition
    counts or a max() over already-polled suspicious_duration samples -- no
    noise-window correlation (that's AbsorbedEvents/AbsorptionRate, still
    in-process-only). See module doc for the granularity caveat versus the
    in-process per-block version.
    """
    out = {}
    for node, states in states_by_node.items():
        states = [s for s in states if s]
        if not states:
            m = {
                "flapping_count": 0,
                "total_transitions": 0,
                "anchored_uptime": 0.0,
                "samples": 0,
            }
            if edge == "down":
                m["time_outside_anchored"] = 0
                m["demotion_count"] = 0
            else:
                m["exit_count"] = 0
                m["max_suspicious_duration"] = 0
            out[node] = m
            continue
        prev = states[0]
        last_transition = None
        flapping = 0
        total_transitions = 0
        demotion_count = 0
        exit_count = 0
        for s in states[1:]:
            if s != prev:
                total_transitions += 1
                transition = f"{prev}->{s}"
                reverse = f"{s}->{prev}"
                if last_transition == reverse:
                    flapping += 1
                last_transition = transition
                if edge == "down" and prev == "ANCHORED" and s == "SUSPICIOUS":
                    demotion_count += 1
                elif edge == "suspicious_exit" and prev == "SUSPICIOUS" and s == "ANCHORED":
                    exit_count += 1
            prev = s
        anchored_uptime = sum(1 for s in states if s == "ANCHORED") / len(states)
        m = {
            "flapping_count": flapping,
            "total_transitions": total_transitions,
            "anchored_uptime": round(anchored_uptime, 4),
            "samples": len(states),
        }
        if edge == "down":
            m["time_outside_anchored"] = sum(1 for s in states if s != "ANCHORED")
            m["demotion_count"] = demotion_count
        else:
            m["exit_count"] = exit_count
            durations = susp_duration_by_node.get(node, [])
            m["max_suspicious_duration"] = max(durations) if durations else 0
        out[node] = m
    return out


def write_summary(
    summary_path: str,
    edge: str,
    value: int,
    env: str,
    metrics: dict,
    duration_s: float = 0.0,
    interval_s: float = 0.0,
    wan_profile: str = "",
    backfilled_from: str = None,
) -> None:
    with open(summary_path, "w") as f:
        f.write(f"# LIVE E5 absorb-edge spot-check: edge={edge}, value={value}, env={env}\n\n")
        if backfilled_from:
            f.write(
                f"Tier 1 fields (time_outside_anchored/demotion_count or exit_count/"
                f"max_suspicious_duration) backfilled from already-collected samples in "
                f"`{backfilled_from}` -- no new Docker run.\n\n"
            )
        else:
            f.write(
                f"{ENVIRONMENTS[env]['description']}. Duration: {duration_s:.0f}s, "
                f"polling interval: {interval_s:.0f}s, WAN-realism baseline: {wan_profile}.\n\n"
            )
        f.write(
            "**Granularity caveat:** metrics below are computed from fixed-interval polling, not "
            "per-block state -- see this script's module doc.\n\n"
        )
        if edge == "down":
            f.write("| Node | Samples | Total Transitions | Flapping Count | Anchored Uptime | Time Outside Anchored | Demotion Count |\n")
            f.write("|---|---:|---:|---:|---:|---:|---:|\n")
            for node, m in metrics.items():
                f.write(
                    f"| {node} | {m['samples']} | {m['total_transitions']} | {m['flapping_count']} | "
                    f"{m['anchored_uptime']:.2%} | {m['time_outside_anchored']} | {m['demotion_count']} |\n"
                )
        else:
            f.write("| Node | Samples | Total Transitions | Flapping Count | Anchored Uptime | Exit Count | Max Suspicious Duration |\n")
            f.write("|---|---:|---:|---:|---:|---:|---:|\n")
            for node, m in metrics.items():
                f.write(
                    f"| {node} | {m['samples']} | {m['total_transitions']} | {m['flapping_count']} | "
                    f"{m['anchored_uptime']:.2%} | {m['exit_count']} | {m['max_suspicious_duration']} |\n"
                )


def backfill_from_csv(args) -> None:
    """Recomputes compute_metrics() (including the Tier 1 fields it didn't have
    when the run was originally collected) from an existing raw samples CSV,
    and rewrites that run's summary .md in place -- no new Docker deployment,
    since fsm_state/suspicious_duration per sample are already in the file.
    """
    states_by_node: dict = {}
    susp_duration_by_node: dict = {}
    with open(args.from_csv, newline="") as f:
        for row in csv.DictReader(f):
            node = row["node"]
            states_by_node.setdefault(node, []).append(row["fsm_state"])
            try:
                duration = int(row["suspicious_duration"])
            except (KeyError, ValueError):
                duration = 0
            susp_duration_by_node.setdefault(node, []).append(duration)

    metrics = compute_metrics(args.edge, states_by_node, susp_duration_by_node)

    # True in-place rewrite: whatever this run's summary was originally named
    # (older CSVs predate the --env flag and have no env in their filename),
    # its summary sits right next to it as the same basename + _summary.md.
    summary_path = re.sub(r"\.csv$", "_summary.md", args.from_csv)
    write_summary(summary_path, args.edge, args.value, args.env, metrics, backfilled_from=args.from_csv)

    print(f"backfilled Tier 1 metrics from {args.from_csv} -> {summary_path}")
    print(f"\nMetrics: {metrics}")


def drive_to_suspicious(interval_s: float, timeout_s: float) -> bool:
    """Stops celestia-bridge and polls all nodes until every one reports SUSPICIOUS,
    or timeout_s elapses. Mirrors tests/e2e/suspicious_exit_sweep_test.go's
    driveToSuspicious. Returns whether every node reached SUSPICIOUS in time.
    """
    print(f"[{now()}] >>> stopping {CELESTIA_BRIDGE} to drive ANCHORED -> SUSPICIOUS")
    docker("stop", CELESTIA_BRIDGE)

    deadline = time.time() + timeout_s
    while time.time() < deadline:
        samples = sample_all_nodes()
        states = {s.node: s.fsm_state for s in samples}
        print(f"[{now()}] drive-phase states: {states}")
        if samples and all(s.fsm_state == "SUSPICIOUS" for s in samples):
            return True
        time.sleep(interval_s)
    return False


def main():
    parser = argparse.ArgumentParser(
        description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter
    )
    parser.add_argument("--edge", choices=["down", "suspicious_exit"], required=True)
    parser.add_argument(
        "--value",
        type=int,
        required=True,
        help="the DownHysteresisThreshold (--edge down) or SuspiciousHysteresisWait "
        "(--edge suspicious_exit) value the deployed cluster was just redeployed with "
        "(label only, not enforced)",
    )
    parser.add_argument("--env", choices=list(ENVIRONMENTS.keys()), required=True)
    parser.add_argument("--duration-s", type=float, default=300.0)
    parser.add_argument("--interval-s", type=float, default=3.0)
    parser.add_argument(
        "--drive-timeout-s",
        type=float,
        default=120.0,
        help="--edge suspicious_exit only: max time to wait for every node to reach "
        "SUSPICIOUS before giving up",
    )
    parser.add_argument(
        "--noise-on-s",
        type=float,
        default=None,
        help="seconds celestia-bridge stays DOWN per cycle (default: 20.0)",
    )
    parser.add_argument(
        "--noise-off-s",
        type=float,
        default=None,
        help="seconds celestia-bridge stays UP per cycle (default: 20.0 for --edge down, "
        "4.0 for --edge suspicious_exit -- see module doc's asymmetric-ratio note)",
    )
    parser.add_argument(
        "--wan-profile",
        choices=["none", "chaos-wan-latency", "chaos-wan-loss"],
        default="chaos-wan-latency",
        help="see live_spot_check.py's identical flag -- per-validator WAN-realism "
        "baseline held for the whole run.",
    )
    parser.add_argument(
        "--from-csv",
        default=None,
        help="skip live polling entirely and recompute metrics from an existing "
        "results_live/live_spot_check_*.csv (already has fsm_state/suspicious_duration per "
        "sample) -- backfills Tier 1 fields into an already-collected run's summary without "
        "a new Docker deployment. Still requires --edge/--value/--env matching that run.",
    )
    args = parser.parse_args()
    if args.from_csv:
        backfill_from_csv(args)
        return
    if args.noise_on_s is None:
        args.noise_on_s = 20.0
    if args.noise_off_s is None:
        # suspicious_exit's "noise" is the MINORITY state (a healthy blip
        # against a sustained-warning baseline) -- 20/4 approximates
        # tests/e2e/suspicious_exit_sweep_test.go's noiseProbability=0.2
        # (hysteresis_sweep_test.go:27) as an 80/20 time split per cycle,
        # not a symmetric toggle. down's own noise IS the minority state
        # already (healthy baseline, warning blip), so it keeps 20/20.
        args.noise_off_s = 4.0 if args.edge == "suspicious_exit" else 20.0

    if args.wan_profile != "none" and args.duration_s > 570:
        print(
            f"WARNING: --duration-s={args.duration_s:.0f} exceeds the WAN profile's own "
            "10-minute (600s) --duration -- it will expire mid-run.",
            file=sys.stderr,
        )

    os.makedirs(RESULTS_DIR, exist_ok=True)
    label = f"{args.edge}{args.value}_{args.env}"
    print(f"=== E5{'b' if args.edge == 'down' else 'c'} live spot-check: edge={args.edge} value={args.value} env={args.env} ({ENVIRONMENTS[args.env]['description']}) wan_profile={args.wan_profile} ===")
    print(
        "NOTE: this script does NOT verify the deployed cluster actually has this "
        "param value -- confirm ENGRAM_PARAM_DOWN_HYSTERESIS_THRESHOLD / "
        "ENGRAM_PARAM_SUSPICIOUS_HYSTERESIS_WAIT + genesis regeneration + redeploy "
        "matched --value before trusting this run's label."
    )

    if args.edge == "suspicious_exit":
        reached = drive_to_suspicious(args.interval_s, args.drive_timeout_s)
        if not reached:
            print(
                f"ERROR: not every node reached SUSPICIOUS within --drive-timeout-s="
                f"{args.drive_timeout_s:.0f}s -- aborting without a measurement window.",
                file=sys.stderr,
            )
            docker("start", CELESTIA_BRIDGE)
            sys.exit(1)
        print(f"[{now()}] >>> all nodes SUSPICIOUS, starting measurement window")
        # Baseline for suspicious_exit is DOWN (sustains the warning holding
        # SUSPICIOUS); "noise" toggles celestia-bridge UP for a healthy blip --
        # the inverse of --edge down's baseline-UP/noise-DOWN cadence.
        baseline_state_is_down = True
    else:
        baseline_state_is_down = False

    if args.wan_profile != "none":
        print(f"[{now()}] >>> starting {args.wan_profile} (WAN-realism baseline)")
        start_pumba_wan_profile(args.wan_profile)

    states_by_node: dict = {}
    susp_duration_by_node: dict = {}
    all_samples = []
    start = time.time()
    deadline = start + args.duration_s
    next_toggle = start
    # da_down tracks the CURRENT toggle state, seeded to match the phase we're
    # already in for --edge suspicious_exit (celestia-bridge is already stopped).
    da_down = baseline_state_is_down

    try:
        while time.time() < deadline:
            t = time.time() - start
            round_samples = sample_all_nodes()
            all_samples.extend(round_samples)
            for s in round_samples:
                states_by_node.setdefault(s.node, []).append(s.fsm_state)
                susp_duration_by_node.setdefault(s.node, []).append(s.suspicious_duration)
            states = {s.node: s.fsm_state for s in round_samples}
            print(f"[{t:6.0f}s] {states}")

            if args.env == "noisy_da" and time.time() >= next_toggle:
                if baseline_state_is_down:
                    # suspicious_exit: baseline DOWN, blip UP.
                    if da_down:
                        print(f"[{now()}] >>> healthy blip: restoring {CELESTIA_BRIDGE}")
                        docker("start", CELESTIA_BRIDGE)
                        next_toggle = time.time() + args.noise_off_s
                    else:
                        print(f"[{now()}] >>> back to baseline: stopping {CELESTIA_BRIDGE}")
                        docker("stop", CELESTIA_BRIDGE)
                        next_toggle = time.time() + args.noise_on_s
                else:
                    # down: baseline UP, blip DOWN.
                    if da_down:
                        print(f"[{now()}] >>> restoring {CELESTIA_BRIDGE} (noise cycle)")
                        docker("start", CELESTIA_BRIDGE)
                        next_toggle = time.time() + args.noise_off_s
                    else:
                        print(f"[{now()}] >>> stopping {CELESTIA_BRIDGE} (noise cycle)")
                        docker("stop", CELESTIA_BRIDGE)
                        next_toggle = time.time() + args.noise_on_s
                da_down = not da_down

            time.sleep(args.interval_s)

        if da_down:
            print(f"[{now()}] >>> restoring {CELESTIA_BRIDGE} (cleanup)")
            docker("start", CELESTIA_BRIDGE)
    finally:
        if args.wan_profile != "none":
            print(f"[{now()}] >>> stopping {args.wan_profile} (WAN-realism baseline)")
            cleanup_wan_profile(args.wan_profile)

    metrics = compute_metrics(args.edge, states_by_node, susp_duration_by_node)

    ts_label = time.strftime("%Y%m%dT%H%M%S")
    csv_path = os.path.join(RESULTS_DIR, f"live_spot_check_{label}_{ts_label}.csv")
    write_csv(all_samples, csv_path)

    summary_path = os.path.join(RESULTS_DIR, f"live_spot_check_{label}_{ts_label}_summary.md")
    write_summary(
        summary_path, args.edge, args.value, args.env, metrics,
        duration_s=args.duration_s, interval_s=args.interval_s, wan_profile=args.wan_profile,
    )

    print(f"\nwrote {len(all_samples)} samples to {csv_path}")
    print(f"wrote summary to {summary_path}")
    print(f"\nMetrics: {metrics}")


if __name__ == "__main__":
    main()
