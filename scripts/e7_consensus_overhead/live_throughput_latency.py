#!/usr/bin/env python3
"""LIVE vanilla-vs-extended throughput/latency A/B under real load -- docs/EXPERIMENT.md's E7.

"Extended" targets the REAL 7-validator Docker cluster (already running -- real multi-node BFT
overhead can't come from a single-node process). "Vanilla" stays a local single process (a
second full 7-node vanilla cluster is a separate infra undertaking) -- a deliberate
simplification: treat it as a lower bound on overhead-free throughput, not a strict
cluster-vs-cluster comparison.

Load mechanism: builds N distinct forced-tx byte strings up front, POSTs each directly to
/broadcast_tx_sync at a controlled rate (same pattern as live_censorship_test.py, looped).
`--senders` splits that rate across concurrent broadcaster threads; for `extended` those threads
target *different validators'* RPC ports (scripts/framework/logger.py's NODE_RPC_PORTS) round-
robin, not just node01 -- closer to real multi-client load than one process hammering one node.

blocks_per_sec and mean/p50/p95 interval measure different things and can disagree without
contradicting each other: blocks_per_sec includes any stalled stretch of the window (round-skips,
backpressure), while interval stats only average over consecutive height-increasing samples, so
they're blind to those gaps. `steady_blocks_per_sec` (1/mean interval) and `gap_fraction`
reconcile the two explicitly instead of reporting them side by side unexplained.

`--ramp` sweeps RAMP_RATES_HZ instead of a single `--rate-hz`, to find the real throughput
ceiling (where tx-accepted/s stops tracking sent/s) rather than assuming one fixed rate is
representative.

Usage (extended = the real running Docker cluster; vanilla = one local
process spun up by this script):
    python3 -u scripts/e7_consensus_overhead/live_throughput_latency.py
    python3 -u scripts/e7_consensus_overhead/live_throughput_latency.py --ramp --senders 4
"""

import os as _os, sys as _sys
_sys.path.insert(0, _os.path.join(_os.path.dirname(_os.path.abspath(__file__)), "..", "framework"))
from stats import ci95_halfwidth  # noqa: E402
import json
import os
import subprocess
import sys
import threading
import time
import urllib.request

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "framework"))
from logger import NODE_RPC_PORTS, query_node  # noqa: E402

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

# Ramp stages for --ramp: finds the real ceiling instead of assuming one fixed
# rate is representative (docs/EXPERIMENT.md's E7 previously only measured 5 tx/s).
RAMP_RATES_HZ = [5.0, 20.0, 50.0, 100.0]

MODES = {
    "extended": {"port": PORT_EXTENDED_CLUSTER, "local": False},
    "vanilla": {"home": HOME_VANILLA, "port": PORT_VANILLA, "local": True},
}


def sender_ports_for(mode: str, n_senders: int) -> list:
    """Concurrent-sender RPC ports for `mode`. `extended` spreads senders round-robin
    across real validators (different nodes' RPCs, not just node01); `vanilla` has only
    one process to target, so every sender hits the same port."""
    if mode == "extended":
        all_ports = list(NODE_RPC_PORTS.values())
        return [all_ports[i % len(all_ports)] for i in range(n_senders)]
    return [PORT_VANILLA] * n_senders


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


def run_load_phase(mode: str, ports: list, duration_s: float, rate_hz: float) -> dict:
    """Runs one timed load phase, splitting `rate_hz` total across len(ports) concurrent
    sender threads (each posting to its own port -- different validators for `extended`,
    module doc). Sampling always reads from ports[0], since all validators converge on the
    same height regardless of which node's RPC serviced a given broadcast."""
    sample_port = ports[0]
    n_senders = len(ports)
    per_sender_rate = rate_hz / n_senders
    count = int(duration_s * rate_hz) + 20 * n_senders
    print(f"[{now()}] building {count} distinct forced-tx payloads for {mode} ({n_senders} sender(s))...")
    hexes = build_forced_tx_hexes(mode, count, f"http://127.0.0.1:{sample_port}")
    sender_hexes = [hexes[i::n_senders] for i in range(n_senders)]

    lock = threading.Lock()
    counters = {"accepted": 0, "sent": 0}
    samples = []
    start = time.time()
    deadline = start + duration_s

    def sender_loop(port: int, my_hexes: list) -> None:
        url = f"http://127.0.0.1:{port}"
        interval = 1.0 / per_sender_rate
        next_send = start
        idx = 0
        while time.time() < deadline and idx < len(my_hexes):
            t = time.time()
            if t >= next_send:
                ok = broadcast_sync(url, my_hexes[idx])
                idx += 1
                with lock:
                    counters["sent"] += 1
                    if ok:
                        counters["accepted"] += 1
                next_send += interval
            time.sleep(0.005)

    threads = [
        threading.Thread(target=sender_loop, args=(p, h), daemon=True)
        for p, h in zip(ports, sender_hexes)
    ]
    for th in threads:
        th.start()

    next_sample = start
    while time.time() < deadline:
        t = time.time()
        if t >= next_sample:
            samples.append(query_node(mode, port=sample_port))
            next_sample += 1.0  # sample height once per second, independent of send rate
        time.sleep(0.02)
    for th in threads:
        th.join(timeout=5)

    sent, accepted = counters["sent"], counters["accepted"]
    elapsed = time.time() - start
    stats = block_interval_stats(samples) or {"mean_s": None, "p50_s": None, "p95_s": None, "n_intervals": 0}
    heights = [s.height for s in samples if s.height > 0]
    blocks_per_sec = (max(heights) - min(heights)) / elapsed if len(heights) >= 2 else None
    # steady_blocks_per_sec: throughput if the whole window moved at the measured
    # steady-state pace (1/mean_s). gap_fraction: how much of elapsed that pace
    # doesn't account for (round-skips, backpressure) -- see module doc.
    steady_blocks_per_sec = 1.0 / stats["mean_s"] if stats["mean_s"] else None
    gap_fraction = (
        1.0 - (stats["n_intervals"] * stats["mean_s"]) / elapsed
        if stats["mean_s"] and elapsed > 0
        else None
    )

    print(
        f"[{now()}] {mode}: rate_hz={rate_hz} senders={n_senders} sent={sent} accepted={accepted} "
        f"elapsed={elapsed:.1f}s blocks/s={blocks_per_sec} steady_blocks/s={steady_blocks_per_sec} "
        f"gap_fraction={gap_fraction} tx-accepted/s={accepted / elapsed:.2f}"
    )
    return {
        "mode": mode,
        "rate_hz": rate_hz,
        "senders": n_senders,
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
    """(mean, stdev, 95% confidence interval half-width) via a normal approximation --
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
    ci95 = ci95_halfwidth(stdev, n)
    return m, stdev, ci95


def cohens_d(a, b):
    """Cohen's d (pooled stdev) between two repeat-sample groups of equal metric --
    e.g. extended vs. vanilla `blocks_per_sec` at the same rate. Only meaningful
    between two real repeat groups (n>=2 each); returns None otherwise."""
    na, nb = len(a), len(b)
    if na < 2 or nb < 2:
        return None
    ma, mb = sum(a) / na, sum(b) / nb
    var_a = sum((v - ma) ** 2 for v in a) / (na - 1)
    var_b = sum((v - mb) ** 2 for v in b) / (nb - 1)
    pooled_sd = (((na - 1) * var_a + (nb - 1) * var_b) / (na + nb - 2)) ** 0.5
    if pooled_sd == 0:
        return None
    return (ma - mb) / pooled_sd


def main():
    import argparse
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--warmup-s", type=float, default=8.0)
    parser.add_argument("--duration-s", type=float, default=None, help="default: 30s if --ramp, else 60s")
    parser.add_argument("--rate-hz", type=float, default=20.0, help="ignored if --ramp")
    parser.add_argument(
        "--senders", type=int, default=1,
        help="concurrent broadcaster threads per load phase, splitting --rate-hz across them; "
        "`extended` spreads them round-robin across different validators' RPCs (not just node01).",
    )
    parser.add_argument(
        "--ramp", action="store_true",
        help=f"sweep {RAMP_RATES_HZ} tx/s instead of one fixed --rate-hz, to find the real "
        "throughput ceiling (where tx-accepted/s stops tracking sent/s).",
    )
    parser.add_argument(
        "--repeats", type=int, default=None,
        help="independent load-phase repeats per mode/rate (nodes stay up across repeats; "
        "each repeat is its own timed window) -- reports mean/stdev/95%% confidence interval across repeats, "
        "not a single-run point estimate (the advisor's confidence-interval/effect-size ask). "
        "default: 1 if --ramp (several rate stages already cost wall-clock), else 5.",
    )
    args = parser.parse_args()
    duration_s = args.duration_s if args.duration_s is not None else (30.0 if args.ramp else 60.0)
    repeats = args.repeats if args.repeats is not None else (1 if args.ramp else 5)
    rates_hz = RAMP_RATES_HZ if args.ramp else [args.rate_hz]

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

        # results[(mode, rate_hz)] = [run_load_phase() dict, ...], one per repeat.
        # Single-rate runs (the common case) key on the one rate in rates_hz.
        results = {(mode, rate): [] for mode in MODES for rate in rates_hz}
        for rate in rates_hz:
            for rep in range(repeats):
                for mode in MODES:
                    ports = sender_ports_for(mode, args.senders)
                    print(
                        f"=== rate {rate:.1f} tx/s, repeat {rep + 1}/{repeats}, load phase: {mode} "
                        f"({duration_s:.0f}s, {len(ports)} sender(s)) ==="
                    )
                    results[(mode, rate)].append(run_load_phase(mode, ports, duration_s, rate))
    finally:
        print(f"[{now()}] stopping vanilla node (extended target is the real cluster -- left running)...")
        for p in procs:
            p.terminate()
        for p in procs:
            try:
                p.wait(timeout=10)
            except subprocess.TimeoutExpired:
                p.kill()

    # Aggregate stats per (mode, rate) across repeats (mean/stdev/95% confidence interval), not
    # just the last repeat's numbers.
    aggregates = {}
    for key, runs in results.items():
        blocks_per_sec = [r["blocks_per_sec"] for r in runs]
        steady_bps = [r["steady_blocks_per_sec"] for r in runs if r["steady_blocks_per_sec"] is not None]
        gap_fracs = [r["gap_fraction"] for r in runs if r["gap_fraction"] is not None]
        tx_per_sec = [r["tx_accepted_per_sec"] for r in runs]
        p95s = [r["p95_s"] for r in runs if r["p95_s"] is not None]
        aggregates[key] = {
            "blocks_per_sec": mean_stdev_ci95(blocks_per_sec),
            "steady_blocks_per_sec": mean_stdev_ci95(steady_bps),
            "gap_fraction": mean_stdev_ci95(gap_fracs),
            "tx_accepted_per_sec": mean_stdev_ci95(tx_per_sec),
            "p95_s": mean_stdev_ci95(p95s),
            "n": len(runs),
        }

    # Effect size (Cohen's d, pooled stdev) between extended and vanilla at each rate --
    # only meaningful because both are repeat groups of matching n; there's no such
    # second condition for any other E7 metric, so this is the one place E7 uses it.
    effect_sizes = {}
    for rate in rates_hz:
        ext_bps = [r["blocks_per_sec"] for r in results.get(("extended", rate), [])]
        van_bps = [r["blocks_per_sec"] for r in results.get(("vanilla", rate), [])]
        ext_gap = [r["gap_fraction"] for r in results.get(("extended", rate), []) if r["gap_fraction"] is not None]
        van_gap = [r["gap_fraction"] for r in results.get(("vanilla", rate), []) if r["gap_fraction"] is not None]
        effect_sizes[rate] = {
            "blocks_per_sec_d": cohens_d(ext_bps, van_bps),
            "gap_fraction_d": cohens_d(ext_gap, van_gap),
        }

    ts_label = time.strftime("%Y%m%dT%H%M%S")
    kind = "ramp" if args.ramp else "throughput_latency"
    summary_path = os.path.join(RESULTS_DIR, f"{kind}_{ts_label}_summary.md")
    with open(summary_path, "w") as f:
        f.write("# LIVE E7 throughput/latency A/B under real load\n\n")
        rate_desc = f"ramp {rates_hz} tx/s" if args.ramp else f"{rates_hz[0]:.1f} tx/s target"
        f.write(
            f"Real MsgSubmitForcedTxRequest load, {rate_desc}, {duration_s:.0f}s x {repeats} "
            f"repeat(s) per mode/rate, {args.senders} concurrent sender(s) per phase "
            f"({'round-robin across validators for extended' if args.senders > 1 else 'single sender'}). "
            "`extended` is measured against the real 7-validator engram-validator-cluster (already "
            "running, the same deployment E2/E4/E5/E6's data comes from); `vanilla` is a local "
            "single `engramd --vanilla` process this script owns -- a deliberate simplification (a "
            "second full 7-node vanilla cluster is a separate infra undertaking), so treat "
            "`vanilla` as a lower bound on overhead-free throughput, not a strict cluster-vs-cluster "
            "comparison. `blocks_per_sec` and `mean/p50/p95 interval` measure different things "
            "(module docstring) -- `steady_blocks_per_sec` (=1/mean interval) and `gap_fraction` "
            "reconcile them explicitly below.\n\n"
        )
        f.write("## Aggregate across repeats (mean ± 95% confidence interval)\n\n")
        f.write(
            "| Mode | Rate (tx/s) | n | Blocks/s | Steady blocks/s (1/mean interval) | "
            "Gap fraction | Tx-accepted/s | p95 interval (s) |\n"
        )
        f.write("|---|---:|---:|---:|---:|---:|---:|---:|\n")
        for (mode, rate), agg in aggregates.items():
            bps_m, _, bps_ci = agg["blocks_per_sec"]
            sbps_m, _, sbps_ci = agg["steady_blocks_per_sec"]
            gap_m, _, gap_ci = agg["gap_fraction"]
            tps_m, _, tps_ci = agg["tx_accepted_per_sec"]
            p95_m, _, p95_ci = agg["p95_s"]
            f.write(
                f"| {mode} | {rate:.1f} | {agg['n']} | {bps_m:.3f} ± {bps_ci:.3f} | "
                + (f"{sbps_m:.3f} ± {sbps_ci:.3f}" if sbps_m is not None else "n/a")
                + " | "
                + (f"{gap_m:.3f} ± {gap_ci:.3f}" if gap_m is not None else "n/a")
                + " | "
                f"{tps_m:.2f} ± {tps_ci:.2f} | "
                + (f"{p95_m:.3f} ± {p95_ci:.3f}" if p95_m is not None else "n/a")
                + " |\n"
            )

        if args.ramp:
            f.write(
                "\n## Ramp: throughput ceiling\n\n"
                "Ceiling = the rate stage where `Tx-accepted/s` stops tracking the target rate "
                "(saturation), or where `Gap fraction`/`p95 interval` starts climbing sharply.\n\n"
            )

        if repeats >= 2:
            f.write(
                "\n## Effect size (Cohen's d, extended vs. vanilla, pooled stdev)\n\n"
                "Only meaningful between two repeat groups of matching n -- the one place E7 has a "
                "real paired comparison; not computed for any other metric.\n\n"
            )
            f.write("| Rate (tx/s) | Blocks/s, d | Gap fraction, d |\n")
            f.write("|---:|---:|---:|\n")
            for rate in rates_hz:
                es = effect_sizes[rate]
                bps_d = es["blocks_per_sec_d"]
                gap_d = es["gap_fraction_d"]
                f.write(
                    f"| {rate:.1f} | "
                    + (f"{bps_d:.1f}" if bps_d is not None else "n/a")
                    + " | "
                    + (f"{gap_d:.1f}" if gap_d is not None else "n/a")
                    + " |\n"
                )

        f.write("\n## Per-repeat detail\n\n")
        f.write(
            "| Mode | Rate (tx/s) | Repeat | Sent | Accepted | Blocks/s | Steady blocks/s | "
            "Gap fraction | Tx-accepted/s | Mean interval (s) | p50 (s) | p95 (s) |\n"
        )
        f.write("|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|\n")
        for (mode, rate), runs in results.items():
            for i, r in enumerate(runs):
                f.write(
                    f"| {mode} | {rate:.1f} | {i + 1} | {r['sent']} | {r['accepted']} | "
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
