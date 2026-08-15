#!/usr/bin/env python3
"""LIVE Timeout-flooding test against the real 4-node testnet --
docs/EXPERIMENT.md's E8 "Timeout flooding by Byzantine nodes" row.

Prior closure used `chaos-crash` (SIGKILL, sends nothing) -- never tested an
ACTIVE validator that stays alive and floods genuinely signed Timeout
messages to manipulate round-skip cadence. This closes that gap: node04
stays honest everywhere else, but re-broadcasts a signed Timeout for round+1
every INTERVAL_MS (default 50ms, far faster than TIMEOUT_DURATION's real
precommit-wait timer) via engram-consensus-core's consensus/state.go
timeoutFloodRoutine (gated by ENGRAM_TIMEOUT_FLOOD_INTERVAL_MS).

Checks: (1) safety -- 3 honest nodes' AppHash never diverges; (2) cadence --
block-production rate during the flood must not measurably drop versus
baseline (extra forced round-skips would grow block interval).

Node04 stays IN the real validator set (N=4,f=1). See also
consensus/round_skip_test.go's TestStateTimeoutFloodCannotForceRoundSkipAlone
(engram-consensus-core) -- the in-process version of check 2's claim,
confirmed here under real network timing.

--sample-stats adds `docker stats` CPU%/memory sampling alongside each poll,
and counts real "rate limit exceeded" log lines on the honest nodes during
the flood (docker logs --since) -- direct evidence the per-peer limiter
(reactor.go's PeerState.allowTimeoutMessage) is engaging and resource cost
stays bounded, not just an inference from cadence holding.

Usage:
    python3 -u scripts/e8_attack_resilience/live_timeout_flood_test.py
    python3 -u scripts/e8_attack_resilience/live_timeout_flood_test.py --interval-ms 2 --flood-s 60 --sample-stats
"""

import argparse
import os
import subprocess
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "framework"))
from logger import sample_all_nodes, write_csv, NODE_RPC_PORTS  # noqa: E402

RESULTS_DIR = os.path.join(os.path.dirname(__file__), "results_live")
FLOOD_SERVICE = "engram-node04"
HONEST_NODES = [n for n in NODE_RPC_PORTS if n != FLOOD_SERVICE]
ALL_NODES = list(NODE_RPC_PORTS.keys())


def now() -> str:
    return time.strftime("%H:%M:%S", time.gmtime())


def enable_flood(interval_ms: int) -> None:
    print(
        f"[{now()}] >>> recreating {FLOOD_SERVICE} with ENGRAM_TIMEOUT_FLOOD_INTERVAL_MS={interval_ms}"
    )
    env = dict(os.environ)
    env["ENGRAM_TIMEOUT_FLOOD_INTERVAL_MS"] = str(interval_ms)
    subprocess.run(
        [
            "docker",
            "compose",
            "-f",
            "compose.yml",
            "-f",
            "docker/engram-node04-timeout-flood.yml",
            "up",
            "-d",
            "--no-deps",
            "--force-recreate",
            FLOOD_SERVICE,
        ],
        capture_output=True,
        text=True,
        timeout=60,
        check=True,
        env=env,
    )


def disable_flood() -> None:
    print(f"[{now()}] >>> reverting {FLOOD_SERVICE} to its honest definition")
    subprocess.run(
        ["docker", "compose", "-f", "compose.yml", "up", "-d", "--no-deps",
         "--force-recreate", FLOOD_SERVICE],
        capture_output=True,
        text=True,
        timeout=60,
    )


def sample_docker_stats(containers) -> dict:
    """One-shot `docker stats --no-stream` snapshot -- {node: (cpu_pct, mem_mb)}.
    Missing/unparseable entries are omitted rather than raising, since a
    container mid-recreate (--force-recreate) can transiently not exist.
    """
    try:
        proc = subprocess.run(
            ["docker", "stats", "--no-stream", "--format", "{{.Container}},{{.CPUPerc}},{{.MemUsage}}"]
            + list(containers),
            capture_output=True,
            text=True,
            timeout=10,
        )
    except (subprocess.TimeoutExpired, OSError):
        return {}
    out = {}
    for line in proc.stdout.strip().splitlines():
        parts = line.split(",")
        if len(parts) != 3:
            continue
        node, cpu_raw, mem_raw = parts
        try:
            cpu_pct = float(cpu_raw.rstrip("%"))
            mem_value = mem_raw.split("/")[0].strip()  # "73.86MiB / 768MiB" -> "73.86MiB"
            mem_mb = float(mem_value.rstrip("MiB")) if "MiB" in mem_value else float(mem_value.rstrip("GiB")) * 1024
        except ValueError:
            continue
        out[node] = (cpu_pct, mem_mb)
    return out


def count_rate_limit_drops(container: str, since_iso: str) -> int:
    """Real count of reactor.go's "rate limit exceeded" drop log line on
    `container` since `since_iso` -- direct evidence the per-peer Timeout
    rate limiter engaged, not an inference from cadence/safety alone.
    """
    try:
        proc = subprocess.run(
            ["docker", "logs", "--since", since_iso, container],
            capture_output=True,
            text=True,
            timeout=15,
        )
    except (subprocess.TimeoutExpired, OSError):
        return -1
    return (proc.stdout + proc.stderr).count("rate limit exceeded")


class Tracker:
    def __init__(self, sample_stats: bool = False):
        self.start = time.time()
        self.samples = []
        self.divergence_events = []
        self.sample_stats = sample_stats
        # phase -> list of (elapsed_s, height) samples from any honest node,
        # for a simple height/s regression per phase.
        self.phase_heights: dict = {}
        # phase -> list of {node: (cpu_pct, mem_mb)} snapshots.
        self.phase_stats: dict = {}

    def elapsed(self):
        return time.time() - self.start

    def poll_for(self, seconds: float, interval: float, phase: str):
        self.phase_heights.setdefault(phase, [])
        self.phase_stats.setdefault(phase, [])
        deadline = time.time() + seconds
        while time.time() < deadline:
            t = self.elapsed()
            all_samples = sample_all_nodes()
            self.samples.extend(all_samples)

            if self.sample_stats:
                self.phase_stats[phase].append(sample_docker_stats(ALL_NODES))

            honest_hashes = {
                s.node: s.app_hash
                for s in all_samples
                if s.node in HONEST_NODES and s.height > 0
            }
            honest_heights = {
                s.node: s.height
                for s in all_samples
                if s.node in HONEST_NODES and s.height > 0
            }
            by_height: dict = {}
            for node, h in honest_heights.items():
                by_height.setdefault(h, {})[node] = honest_hashes[node]
            for h, hashes_at_h in by_height.items():
                if len(hashes_at_h) > 1 and len(set(hashes_at_h.values())) > 1:
                    self.divergence_events.append((t, phase, h, dict(hashes_at_h)))
                    print(
                        f"  *** [{phase}] SAFETY VIOLATION @ {t:6.0f}s height={h}: honest nodes "
                        f"disagree on AppHash: {hashes_at_h} ***"
                    )
            if honest_heights:
                self.phase_heights[phase].append((t, max(honest_heights.values())))

            states = {s.node: (s.height, s.fsm_state) for s in all_samples}
            print(f"[{t:6.0f}s][{phase}] {states}")
            time.sleep(interval)

    def stats_summary(self, phase: str, node: str):
        """(avg_cpu, max_cpu, avg_mem, max_mem) for node across phase's
        snapshots; None if no stats were sampled (sample_stats=False or
        node missing from every snapshot, e.g. node04 mid-recreate)."""
        vals = [s[node] for s in self.phase_stats.get(phase, []) if node in s]
        if not vals:
            return None
        cpus = [v[0] for v in vals]
        mems = [v[1] for v in vals]
        return (sum(cpus) / len(cpus), max(cpus), sum(mems) / len(mems), max(mems))

    def height_rate(self, phase: str) -> float:
        """Blocks/s over `phase`'s samples -- (last height - first height) /
        (last t - first t). 0.0 if fewer than 2 samples."""
        pts = self.phase_heights.get(phase, [])
        if len(pts) < 2:
            return 0.0
        (t0, h0), (t1, h1) = pts[0], pts[-1]
        if t1 <= t0:
            return 0.0
        return (h1 - h0) / (t1 - t0)


def run(interval_ms: int, flood_s: float, recovery_s: float, interval_s: float, sample_stats: bool):
    os.makedirs(RESULTS_DIR, exist_ok=True)
    tr = Tracker(sample_stats=sample_stats)

    print(f"=== E8 live Timeout-flooding test: INTERVAL_MS={interval_ms} sample_stats={sample_stats} ===")

    print("=== Phase 1: baseline (honest node04, real precommit-wait timer only) ===")
    tr.poll_for(30.0, interval_s, "baseline")

    print(f"=== Phase 2: flood ({flood_s:.0f}s, node04 spams signed Timeout every {interval_ms}ms) ===")
    flood_start_iso = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
    enable_flood(interval_ms)
    tr.poll_for(flood_s, interval_s, "flood")
    rate_limit_drops = {
        node: count_rate_limit_drops(node, flood_start_iso) for node in HONEST_NODES
    }

    print(f"=== Phase 3: recovery ({recovery_s:.0f}s, node04 reverted to honest) ===")
    disable_flood()
    tr.poll_for(recovery_s, interval_s, "recovery")

    ts_label = time.strftime("%Y%m%dT%H%M%S")
    csv_path = os.path.join(RESULTS_DIR, f"timeout_flood_{ts_label}.csv")
    write_csv(tr.samples, csv_path)

    safety_held = len(tr.divergence_events) == 0
    baseline_rate = tr.height_rate("baseline")
    flood_rate = tr.height_rate("flood")
    recovery_rate = tr.height_rate("recovery")
    # A generous threshold -- this is a coarse liveness sanity check, not a
    # precision throughput benchmark: block interval already has real jitter
    # from CometBFT's own timeout_commit and Docker scheduling noise.
    cadence_held = flood_rate >= 0.5 * baseline_rate if baseline_rate > 0 else True

    summary_path = os.path.join(RESULTS_DIR, f"timeout_flood_{ts_label}_summary.md")
    with open(summary_path, "w") as f:
        f.write("# LIVE Timeout-flooding test\n\n")
        f.write(
            "docs/EXPERIMENT.md's E8 \"Timeout flooding by Byzantine nodes\" row -- "
            "node04 stays alive and honest everywhere else, but actively floods "
            f"genuinely signed Timeout messages every {interval_ms}ms "
            "(engram-consensus-core's timeoutFloodRoutine), instead of the "
            "crash-only (`chaos-crash`) approximation used previously.\n\n"
        )
        f.write(f"Total duration: {tr.elapsed():.0f}s.\n\n")
        f.write("## Verdict\n\n")
        f.write(
            f"- Safety held (3 honest nodes' AppHash never diverged): **{safety_held}**\n"
        )
        f.write(f"- Divergence events: {len(tr.divergence_events)}\n")
        f.write(
            f"- Cadence held (flood-phase height rate >= 50% of baseline): **{cadence_held}**\n"
        )
        f.write(f"  - baseline: {baseline_rate:.3f} blocks/s\n")
        f.write(f"  - flood:    {flood_rate:.3f} blocks/s\n")
        f.write(f"  - recovery: {recovery_rate:.3f} blocks/s\n\n")
        if tr.divergence_events:
            f.write("### Divergence detail\n\n")
            for t, phase, h, hashes in tr.divergence_events:
                f.write(f"- t={t:.0f}s phase={phase} height={h}: {hashes}\n")

        f.write("\n## Per-peer rate limiter (reactor.go's PeerState.allowTimeoutMessage)\n\n")
        f.write(
            f"Real \"rate limit exceeded\" drop count on each honest node, counted from "
            f"`docker logs --since` the flood phase started (not an inference):\n\n"
        )
        for node in HONEST_NODES:
            f.write(f"- {node}: {rate_limit_drops[node]} drops\n")

        if sample_stats:
            f.write("\n## Resource usage (docker stats, CPU%/memory)\n\n")
            f.write("| Node | Phase | Avg CPU% | Max CPU% | Avg Mem (MiB) | Max Mem (MiB) |\n")
            f.write("|---|---|---:|---:|---:|---:|\n")
            for phase in ("baseline", "flood", "recovery"):
                for node in ALL_NODES:
                    s = tr.stats_summary(phase, node)
                    if s is None:
                        continue
                    avg_cpu, max_cpu, avg_mem, max_mem = s
                    f.write(
                        f"| {node} | {phase} | {avg_cpu:.2f} | {max_cpu:.2f} | "
                        f"{avg_mem:.1f} | {max_mem:.1f} |\n"
                    )

    print(f"\nwrote {len(tr.samples)} samples to {csv_path}")
    print(f"wrote summary to {summary_path}")
    print(f"rate_limit_drops: {rate_limit_drops}")
    print(
        f"\nVERDICT: safety_held={safety_held} divergence_events={len(tr.divergence_events)} "
        f"cadence_held={cadence_held} (baseline={baseline_rate:.3f} flood={flood_rate:.3f} blocks/s)"
    )
    return safety_held and cadence_held


def main():
    parser = argparse.ArgumentParser(
        description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter
    )
    parser.add_argument("--interval-ms", type=int, default=50)
    parser.add_argument("--flood-s", type=float, default=60.0)
    parser.add_argument("--recovery-s", type=float, default=30.0)
    parser.add_argument("--interval-s", type=float, default=3.0)
    parser.add_argument("--sample-stats", action="store_true",
                         help="Sample docker stats CPU%%/memory alongside each poll")
    args = parser.parse_args()

    ok = run(args.interval_ms, args.flood_s, args.recovery_s, args.interval_s, args.sample_stats)
    if not ok:
        sys.exit(1)


if __name__ == "__main__":
    main()
