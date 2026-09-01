#!/usr/bin/env python3
"""LIVE vanilla-vs-extended throughput/latency A/B under real load --
docs/EXPERIMENT.md's E7.

"Extended" is measured against the REAL 7-validator Docker cluster
(engram-validator-cluster, already running -- the same deployment E2/E4/E5/E6
collect their data against), not a throwaway local process: real multi-node
BFT voting overhead is what this experiment is actually trying to capture,
and a single-node process can't produce that. "Vanilla" stays a local
single-process comparison (standing up a second full 7-node vanilla-flagged
cluster is a separate, large infra undertaking this script doesn't attempt)
-- this is a deliberate, documented simplification, not an oversight: it
answers "does the extended payload's per-block byte/CPU overhead matter"
without needing a second cluster, at the cost of not isolating cluster-size
effects between the two arms. Treat the vanilla number as a lower bound on
overhead-free throughput, not a strict apples-to-apples cluster comparison.

Load mechanism: `engramd tx-submit-forced-tx --payload <label>-<i> --dry-run`
(cmd/engramd/e8_cli.go) builds N distinct, deterministic tx byte strings
up front (this app's txs are unsigned envelopes, no x/auth -- no signing to
replicate), varied per-submission so CometBFT's mempool doesn't dedup
identical bytes (buildMinimalTx has no nonce/timestamp field). Each is then
POSTed directly to /broadcast_tx_sync at a controlled rate -- the same
build-then-broadcast-raw pattern scripts/e8_attack_resilience/
live_censorship_test.py already uses, just looped instead of one-shot.

Latency/throughput: reuses scripts/e2_fault_injection/live_scenario_matrix.py's
block_interval_stats() verbatim (imported, not reimplemented) for Mean/p50/p95
seconds-between-height-increments, plus real blocks/sec and tx-accepted/sec
(broadcast_tx_sync responses with code==0).

blocks_per_sec vs. mean/p50/p95 interval are NOT directly comparable and must
not be read as contradicting each other: blocks_per_sec is
(height range)/(full elapsed wall time), so it includes any stretch of the
window where height didn't advance at all (round-skips, mempool backpressure,
sampling-boundary rounding); block_interval_stats only averages over the
CONSECUTIVE samples where height DID increase, so it's blind to those gaps by
construction. A run with real stalls can show mean interval ~= 1/target-rate
(steady-state, when it's moving) while blocks_per_sec reads well below
1/mean_interval (because part of the window wasn't moving). This script now
reports both explicitly plus a `gap_fraction` diagnostic
(1 - n_intervals*mean_s/elapsed) quantifying how much of the window that
gap-blindness hid, instead of presenting the two numbers side by side with no
explanation.

Usage (extended = the real running Docker cluster; vanilla = one local
process spun up by this script):
    python3 -u scripts/e7_consensus_overhead/live_throughput_latency.py
"""

import json
import os
import subprocess
import sys
import time
import urllib.request

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "framework"))
from logger import query_node  # noqa: E402

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "e2_fault_injection"))
from live_scenario_matrix import block_interval_stats  # noqa: E402

REPO_ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
RESULTS_DIR = os.path.join(os.path.dirname(__file__), "results_live")
BIN = "/tmp/engramd-load-comparison"
HOME_VANILLA = "/tmp/engramd-load-vanilla"
# "extended" targets the real, already-running 7-validator Docker cluster
# (engram-node01's host-mapped RPC port -- scripts/framework/logger.py's
# NODE_RPC_PORTS) instead of a local process; this script never starts or
# stops that cluster. "vanilla" stays a local single process this script
# owns the lifecycle of (module doc: why these two arms aren't symmetric).
PORT_EXTENDED_CLUSTER = 26657
PORT_VANILLA = 36657

MODES = {
    "extended": {"port": PORT_EXTENDED_CLUSTER, "local": False},
    "vanilla": {"home": HOME_VANILLA, "port": PORT_VANILLA, "local": True},
}


def now() -> str:
    return time.strftime("%H:%M:%S", time.gmtime())


def run(cmd, **kwargs):
    print(f"[{now()}] >>> {' '.join(cmd)}")
    return subprocess.run(cmd, cwd=REPO_ROOT, check=True, capture_output=True, text=True, **kwargs)


def build_binary():
    run(["go", "build", "-o", BIN, "./cmd/engramd"])


def init_home(home: str, moniker: str, shift_ports: bool) -> None:
    subprocess.run(["rm", "-rf", home])
    subprocess.run([BIN, "init", moniker, "--home", home], capture_output=True, text=True, check=True)
    if shift_ports:
        # Exact same fixed 26xxx->36xxx shift vanilla_comparison.sh applies, so
        # both nodes can run simultaneously on one machine without port clashes.
        cfg = os.path.join(home, "config", "config.toml")
        with open(cfg) as f:
            text = f.read()
        text = (
            text.replace("tcp://127.0.0.1:26657", "tcp://127.0.0.1:36657")
            .replace("tcp://0.0.0.0:26656", "tcp://0.0.0.0:36656")
            .replace("tcp://127.0.0.1:26658", "tcp://127.0.0.1:36658")
            .replace(":26660", ":36660")
        )
        with open(cfg, "w") as f:
            f.write(text)


def start_node(home: str, vanilla: bool, log_path: str) -> subprocess.Popen:
    args = [BIN, "start", "--home", home]
    if vanilla:
        args.insert(2, "--vanilla")
    log = open(log_path, "w")
    return subprocess.Popen(args, cwd=REPO_ROOT, stdout=log, stderr=subprocess.STDOUT)


def build_forced_tx_hexes(label: str, count: int, node_url: str) -> list:
    hexes = []
    for i in range(count):
        proc = subprocess.run(
            [BIN, "tx-submit-forced-tx", "--payload", f"{label}-load-{i}", "--dry-run", "--node", node_url],
            capture_output=True, text=True, timeout=15,
        )
        if proc.returncode != 0:
            raise RuntimeError(f"--dry-run failed for {label}-{i}: {proc.stdout} {proc.stderr}")
        hexes.append(proc.stdout.strip())
    return hexes


def broadcast_sync(node_url: str, tx_hex: str) -> bool:
    """POSTs directly to /broadcast_tx_sync (bypassing the CLI's own broadcast
    for rate control, same as live_censorship_test.py's broadcast_target_tx).
    Returns whether the mempool accepted it (code == 0)."""
    try:
        with urllib.request.urlopen(f"{node_url}/broadcast_tx_sync?tx=0x{tx_hex}", timeout=5) as resp:
            result = json.loads(resp.read())
        return result.get("result", {}).get("code", -1) == 0
    except Exception as e:  # noqa: BLE001 -- a failed submission is a real data point, not a crash
        print(f"    broadcast failed: {e}")
        return False


def run_load_phase(mode: str, port: int, duration_s: float, rate_hz: float) -> dict:
    node_url = f"http://127.0.0.1:{port}"
    count = int(duration_s * rate_hz) + 20
    print(f"[{now()}] building {count} distinct forced-tx payloads for {mode}...")
    hexes = build_forced_tx_hexes(mode, count, node_url)

    samples = []
    accepted = 0
    sent = 0
    start = time.time()
    deadline = start + duration_s
    interval = 1.0 / rate_hz
    next_send = start
    next_sample = start

    while time.time() < deadline:
        t = time.time()
        if t >= next_send and sent < len(hexes):
            if broadcast_sync(node_url, hexes[sent]):
                accepted += 1
            sent += 1
            next_send += interval
        if t >= next_sample:
            samples.append(query_node(mode, port=port))
            next_sample += 1.0  # sample height once per second, independent of send rate
        time.sleep(0.02)

    elapsed = time.time() - start
    stats = block_interval_stats(samples) or {"mean_s": None, "p50_s": None, "p95_s": None, "n_intervals": 0}
    heights = [s.height for s in samples if s.height > 0]
    blocks_per_sec = (max(heights) - min(heights)) / elapsed if len(heights) >= 2 else None
    # steady_blocks_per_sec: what throughput WOULD be if the whole window
    # moved at the measured steady-state pace (1/mean_s) -- the module doc's
    # blocks_per_sec-vs-interval reconciliation. gap_fraction: how much of
    # elapsed isn't accounted for by n_intervals*mean_s (round-skips,
    # backpressure, or just the sampling boundary) -- 0 means the two
    # measures agree; a positive fraction quantifies the real stall.
    steady_blocks_per_sec = 1.0 / stats["mean_s"] if stats["mean_s"] else None
    gap_fraction = (
        1.0 - (stats["n_intervals"] * stats["mean_s"]) / elapsed
        if stats["mean_s"] and elapsed > 0
        else None
    )

    print(
        f"[{now()}] {mode}: sent={sent} accepted={accepted} elapsed={elapsed:.1f}s "
        f"blocks/s={blocks_per_sec} steady_blocks/s={steady_blocks_per_sec} "
        f"gap_fraction={gap_fraction} tx-accepted/s={accepted / elapsed:.2f}"
    )
    return {
        "mode": mode,
        "sent": sent,
        "accepted": accepted,
        "elapsed_s": elapsed,
        "blocks_per_sec": blocks_per_sec,
        "steady_blocks_per_sec": steady_blocks_per_sec,
        "gap_fraction": gap_fraction,
        "tx_accepted_per_sec": accepted / elapsed if elapsed > 0 else None,
        "mean_s": stats["mean_s"],
        "p50_s": stats["p50_s"],
        "p95_s": stats["p95_s"],
        "n_intervals": stats["n_intervals"],
    }


def mean_stdev_ci95(values):
    """(mean, stdev, 95% CI half-width) via a normal approximation --
    adequate for the small repeat counts (5-20) this script targets, not a
    claim of exact small-sample coverage."""
    n = len(values)
    if n == 0:
        return None, None, None
    m = sum(values) / n
    if n < 2:
        return m, 0.0, 0.0
    var = sum((v - m) ** 2 for v in values) / (n - 1)
    stdev = var ** 0.5
    ci95 = 1.96 * stdev / (n ** 0.5)
    return m, stdev, ci95


def main():
    import argparse
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--warmup-s", type=float, default=8.0)
    parser.add_argument("--duration-s", type=float, default=60.0)
    parser.add_argument("--rate-hz", type=float, default=20.0)
    parser.add_argument(
        "--repeats", type=int, default=5,
        help="independent load-phase repeats per mode (nodes stay up across repeats; "
        "each repeat is its own timed window) -- reports mean/stdev/95%% CI across repeats, "
        "not a single-run point estimate (the advisor's confidence-interval/effect-size ask).",
    )
    args = parser.parse_args()

    os.makedirs(RESULTS_DIR, exist_ok=True)
    build_binary()

    # extended: verify the real cluster is up and ANCHORED -- this script
    # never starts/stops/redeploys it (module doc). vanilla: this script's
    # own local process, same as before.
    try:
        with urllib.request.urlopen(f"http://127.0.0.1:{PORT_EXTENDED_CLUSTER}/status", timeout=5) as resp:
            cluster_status = json.loads(resp.read())
        cluster_height = int(cluster_status["result"]["sync_info"]["latest_block_height"])
        print(f"[{now()}] extended target: real cluster at height {cluster_height}, reachable.")
    except Exception as e:
        raise RuntimeError(
            f"real cluster not reachable on port {PORT_EXTENDED_CLUSTER} -- this script targets the "
            f"already-running engram-validator-cluster and does not start it itself: {e}"
        ) from e

    init_home(HOME_VANILLA, "vanilla-node", shift_ports=True)

    procs = []
    try:
        print(f"[{now()}] starting vanilla node...")
        procs.append(start_node(HOME_VANILLA, True, "/tmp/engramd-load-vanilla.log"))

        print(f"[{now()}] warming up ({args.warmup_s:.0f}s)...")
        time.sleep(args.warmup_s)

        # repeat_results[mode] = [run_load_phase() dict, ...], one per repeat.
        repeat_results = {mode: [] for mode in MODES}
        for rep in range(args.repeats):
            for mode, cfg in MODES.items():
                print(
                    f"=== repeat {rep + 1}/{args.repeats}, load phase: {mode} "
                    f"({args.duration_s:.0f}s @ {args.rate_hz:.1f} tx/s) ==="
                )
                repeat_results[mode].append(run_load_phase(mode, cfg["port"], args.duration_s, args.rate_hz))
    finally:
        print(f"[{now()}] stopping vanilla node (extended target is the real cluster -- left running)...")
        for p in procs:
            p.terminate()
        for p in procs:
            try:
                p.wait(timeout=10)
            except subprocess.TimeoutExpired:
                p.kill()

    # Per-mode aggregate stats across repeats (mean/stdev/95% CI), not just
    # the last repeat's numbers.
    aggregates = {}
    for mode, runs in repeat_results.items():
        blocks_per_sec = [r["blocks_per_sec"] for r in runs]
        steady_bps = [r["steady_blocks_per_sec"] for r in runs if r["steady_blocks_per_sec"] is not None]
        gap_fracs = [r["gap_fraction"] for r in runs if r["gap_fraction"] is not None]
        tx_per_sec = [r["tx_accepted_per_sec"] for r in runs]
        p95s = [r["p95_s"] for r in runs if r["p95_s"] is not None]
        aggregates[mode] = {
            "blocks_per_sec": mean_stdev_ci95(blocks_per_sec),
            "steady_blocks_per_sec": mean_stdev_ci95(steady_bps),
            "gap_fraction": mean_stdev_ci95(gap_fracs),
            "tx_accepted_per_sec": mean_stdev_ci95(tx_per_sec),
            "p95_s": mean_stdev_ci95(p95s),
            "n": len(runs),
        }

    ts_label = time.strftime("%Y%m%dT%H%M%S")
    summary_path = os.path.join(RESULTS_DIR, f"throughput_latency_{ts_label}_summary.md")
    with open(summary_path, "w") as f:
        f.write("# LIVE E7 throughput/latency A/B under real load\n\n")
        f.write(
            f"Real MsgSubmitForcedTxRequest load, {args.rate_hz:.1f} tx/s target, "
            f"{args.duration_s:.0f}s x {args.repeats} repeats per mode. `extended` is measured "
            "against the real 7-validator engram-validator-cluster (already running, the same "
            "deployment E2/E4/E5/E6's data comes from); `vanilla` is a local single `engramd "
            "--vanilla` process this script owns -- a deliberate simplification (a second full "
            "7-node vanilla cluster is a separate infra undertaking), so treat `vanilla` as a "
            "lower bound on overhead-free throughput, not a strict cluster-vs-cluster comparison. "
            "`blocks_per_sec` and `mean/p50/p95 interval` measure different things (module "
            "docstring) -- `steady_blocks_per_sec` (=1/mean interval) and `gap_fraction` "
            "reconcile them explicitly below.\n\n"
        )
        f.write("## Aggregate across repeats (mean ± 95% CI)\n\n")
        f.write("| Mode | n | Blocks/s | Steady blocks/s (1/mean interval) | Gap fraction | Tx-accepted/s | p95 interval (s) |\n")
        f.write("|---|---:|---:|---:|---:|---:|---:|\n")
        for mode, agg in aggregates.items():
            bps_m, _, bps_ci = agg["blocks_per_sec"]
            sbps_m, _, sbps_ci = agg["steady_blocks_per_sec"]
            gap_m, _, gap_ci = agg["gap_fraction"]
            tps_m, _, tps_ci = agg["tx_accepted_per_sec"]
            p95_m, _, p95_ci = agg["p95_s"]
            f.write(
                f"| {mode} | {agg['n']} | {bps_m:.3f} ± {bps_ci:.3f} | "
                + (f"{sbps_m:.3f} ± {sbps_ci:.3f}" if sbps_m is not None else "n/a")
                + " | "
                + (f"{gap_m:.3f} ± {gap_ci:.3f}" if gap_m is not None else "n/a")
                + " | "
                f"{tps_m:.2f} ± {tps_ci:.2f} | "
                + (f"{p95_m:.3f} ± {p95_ci:.3f}" if p95_m is not None else "n/a")
                + " |\n"
            )
        f.write("\n## Per-repeat detail\n\n")
        f.write(
            "| Mode | Repeat | Sent | Accepted | Blocks/s | Steady blocks/s | Gap fraction | "
            "Tx-accepted/s | Mean interval (s) | p50 (s) | p95 (s) |\n"
        )
        f.write("|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|\n")
        for mode, runs in repeat_results.items():
            for i, r in enumerate(runs):
                f.write(
                    f"| {mode} | {i + 1} | {r['sent']} | {r['accepted']} | "
                    f"{r['blocks_per_sec']:.3f} | "
                    + (f"{r['steady_blocks_per_sec']:.3f}" if r["steady_blocks_per_sec"] is not None else "n/a")
                    + " | "
                    + (f"{r['gap_fraction']:.3f}" if r["gap_fraction"] is not None else "n/a")
                    + f" | {r['tx_accepted_per_sec']:.2f} | "
                    + (
                        f"{r['mean_s']:.3f} | {r['p50_s']:.3f} | {r['p95_s']:.3f} |\n"
                        if r["mean_s"] is not None
                        else "n/a | n/a | n/a |\n"
                    )
                )

    print(f"\nwrote summary to {summary_path}")
    print(f"\nAGGREGATES: {aggregates}")


if __name__ == "__main__":
    main()
