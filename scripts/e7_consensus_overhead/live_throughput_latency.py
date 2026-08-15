#!/usr/bin/env python3
"""LIVE vanilla-vs-extended throughput/latency A/B under real load --
docs/EXPERIMENT.md's E7. `vanilla_comparison.sh` already runs a real local
2-process A/B (one normal `engramd`, one `--vanilla`) but only samples them
IDLE -- its own output says block interval "does not meaningfully differ"
because CometBFT's `timeout_commit` dominates both when there's no tx load.
This script is the same real 2-process harness, but drives real
MsgSubmitForcedTxRequest load at both nodes so throughput/latency actually
measure something, not idle noise.

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
(broadcast_tx_sync responses with code==0) -- neither existed anywhere before
this script.

Usage (no docker/testnet needed -- two local engramd processes only):
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
HOME_NORMAL = "/tmp/engramd-load-normal"
HOME_VANILLA = "/tmp/engramd-load-vanilla"
PORT_NORMAL = 26657
PORT_VANILLA = 36657

MODES = {
    "extended": {"home": HOME_NORMAL, "port": PORT_NORMAL, "vanilla_flag": False},
    "vanilla": {"home": HOME_VANILLA, "port": PORT_VANILLA, "vanilla_flag": True},
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

    print(
        f"[{now()}] {mode}: sent={sent} accepted={accepted} elapsed={elapsed:.1f}s "
        f"blocks/s={blocks_per_sec} tx-accepted/s={accepted / elapsed:.2f}"
    )
    return {
        "mode": mode,
        "sent": sent,
        "accepted": accepted,
        "elapsed_s": elapsed,
        "blocks_per_sec": blocks_per_sec,
        "tx_accepted_per_sec": accepted / elapsed if elapsed > 0 else None,
        "mean_s": stats["mean_s"],
        "p50_s": stats["p50_s"],
        "p95_s": stats["p95_s"],
        "n_intervals": stats["n_intervals"],
    }


def main():
    import argparse
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--warmup-s", type=float, default=8.0)
    parser.add_argument("--duration-s", type=float, default=60.0)
    parser.add_argument("--rate-hz", type=float, default=5.0)
    args = parser.parse_args()

    os.makedirs(RESULTS_DIR, exist_ok=True)
    build_binary()
    init_home(HOME_NORMAL, "normal-node", shift_ports=False)
    init_home(HOME_VANILLA, "vanilla-node", shift_ports=True)

    procs = []
    try:
        print(f"[{now()}] starting extended (normal) node...")
        procs.append(start_node(HOME_NORMAL, False, "/tmp/engramd-load-normal.log"))
        time.sleep(1)
        print(f"[{now()}] starting vanilla node...")
        procs.append(start_node(HOME_VANILLA, True, "/tmp/engramd-load-vanilla.log"))

        print(f"[{now()}] warming up ({args.warmup_s:.0f}s)...")
        time.sleep(args.warmup_s)

        results = {}
        for mode, cfg in MODES.items():
            print(f"=== load phase: {mode} ({args.duration_s:.0f}s @ {args.rate_hz:.1f} tx/s) ===")
            results[mode] = run_load_phase(mode, cfg["port"], args.duration_s, args.rate_hz)
    finally:
        print(f"[{now()}] stopping both nodes...")
        for p in procs:
            p.terminate()
        for p in procs:
            try:
                p.wait(timeout=10)
            except subprocess.TimeoutExpired:
                p.kill()

    ts_label = time.strftime("%Y%m%dT%H%M%S")
    summary_path = os.path.join(RESULTS_DIR, f"throughput_latency_{ts_label}_summary.md")
    with open(summary_path, "w") as f:
        f.write("# LIVE E7 throughput/latency A/B under real load\n\n")
        f.write(
            f"Real MsgSubmitForcedTxRequest load, {args.rate_hz:.1f} tx/s target, "
            f"{args.duration_s:.0f}s per mode, against two local real `engramd` processes "
            "(extended vs. `--vanilla`), not the shared docker testnet.\n\n"
        )
        f.write("| Mode | Sent | Accepted | Blocks/s | Tx-accepted/s | Mean interval (s) | p50 (s) | p95 (s) |\n")
        f.write("|---|---:|---:|---:|---:|---:|---:|---:|\n")
        for mode, r in results.items():
            f.write(
                f"| {mode} | {r['sent']} | {r['accepted']} | "
                f"{r['blocks_per_sec']:.3f} | {r['tx_accepted_per_sec']:.2f} | "
                f"{r['mean_s']:.3f} | {r['p50_s']:.3f} | {r['p95_s']:.3f} |\n"
                if r["mean_s"] is not None
                else f"| {mode} | {r['sent']} | {r['accepted']} | n/a | n/a | n/a | n/a | n/a |\n"
            )

    print(f"\nwrote summary to {summary_path}")
    print(f"\nRESULTS: {results}")


if __name__ == "__main__":
    main()
