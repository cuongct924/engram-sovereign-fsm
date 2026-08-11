#!/usr/bin/env python3
"""LIVE Censorship / forced-tx-withholding test against the real 4-node
testnet -- docs/EXPERIMENT.md's E8 A7 adversarial half. Previously had no
live driver: x/sovereignty/proposal.go's applyByzantineBehavior already
supported ENGRAM_BYZANTINE_BEHAVIOR=censor_tx:<target> mechanically, but
nothing exercised it end-to-end against a real forced tx.

Protocol, real and byte-exact throughout (no mocked queue/tx content):

  1. Build a real, independently-decodable "target" tx (another
     MsgSubmitForcedTxRequest, chosen only because a CLI to build it already
     exists -- its own content is otherwise irrelevant) via
     `engramd tx-submit-forced-tx --dry-run`. buildMinimalTx
     (cmd/engramd/reanchor_cli.go) has no timestamp/nonce/random field, so
     this is deterministic -- the same command run again reproduces the
     exact same bytes.
  2. Register those exact raw bytes in ForcedTxQueue via a SEPARATE
     MsgSubmitForcedTxRequest{Tx: <target's raw bytes>}, using --payload-hex
     for byte-exact content (x/sovereignty/keeper/msg_server.go's
     SubmitForcedTx just does ForcedTxQueue.Set(ctx, string(msg.Tx))).
  3. Broadcast the target tx itself as a real, standalone tx (so it can
     ever appear verbatim in req.Txs[1:], which is what
     IsCensoring/ProcessProposal's check #0 compares ForcedTxQueue entries
     against -- x/sovereignty/proposal.go:305-318).
  4. Set node04 byzantine with censor_tx:<hex of target's raw bytes> --
     applyByzantineBehavior hex-decodes this; the target is hex-encoded
     rather than a raw string because Docker Compose env-var/YAML
     interpolation does not survive arbitrary non-UTF8 tx bytes.
  5. Poll: whenever node04 leads a round, its proposal should omit the
     target tx (observable as it not landing in a block node04 proposed);
     once MaxIgnoreRounds (Params default: 1) is exceeded, the NEXT
     censoring proposal from node04 must be REJECTED by the 3 honest
     validators' IsCensoring check -- never committed, safety bar (honest
     AppHash never diverges) checked the whole time, matching
     live_byzantine_attacks.py's Tracker pattern (including its same-height
     AppHash-comparison fix).

Usage:
    python3 -u scripts/e8_attack_resilience/live_censorship_test.py
"""

import os
import re
import subprocess
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "framework"))
from logger import sample_all_nodes, write_csv, NODE_RPC_PORTS  # noqa: E402

RESULTS_DIR = os.path.join(os.path.dirname(__file__), "results_live")
BYZANTINE_SERVICE = "engram-node04"
HONEST_NODES = [n for n in NODE_RPC_PORTS if n != BYZANTINE_SERVICE]
NODE_URL = f"http://localhost:{NODE_RPC_PORTS['engram-node01']}"


def now() -> str:
    return time.strftime("%H:%M:%S", time.gmtime())


def run_cli(*args: str) -> subprocess.CompletedProcess:
    proc = subprocess.run(
        ["engramd", *args, "--node", NODE_URL],
        capture_output=True,
        text=True,
        timeout=45,
    )
    return proc


def build_target_tx_hex() -> str:
    """--dry-run: builds the deterministic raw bytes of a throwaway
    MsgSubmitForcedTxRequest WITHOUT broadcasting it -- this is the "other
    tx" that will be registered as forced AND separately broadcast for
    real, per this module's doc.
    """
    proc = run_cli(
        "tx-submit-forced-tx", "--payload", "e8-a7-censorship-target", "--dry-run"
    )
    if proc.returncode != 0:
        raise RuntimeError(f"--dry-run failed: {proc.stdout} {proc.stderr}")
    return proc.stdout.strip()


def register_forced_tx(target_hex: str) -> None:
    print(
        f"[{now()}] >>> registering target (hex, {len(target_hex)//2} bytes) in ForcedTxQueue"
    )
    proc = run_cli("tx-submit-forced-tx", "--payload-hex", target_hex)
    print(
        f"    exit={proc.returncode} stdout={proc.stdout.strip()!r} stderr={proc.stderr.strip()!r}"
    )
    if proc.returncode != 0:
        raise RuntimeError("failed to register the forced tx")


def broadcast_target_tx(target_hex: str) -> None:
    """Broadcasts the SAME raw bytes as their own standalone tx, via
    CometBFT's broadcast_tx_sync RPC directly (not through the CLI, which
    would build a DIFFERENT tx from --payload-hex -- MsgSubmitForcedTxRequest
    wrapping this content, not this content itself as a top-level tx).
    """
    print(
        f"[{now()}] >>> broadcasting the target tx itself (so it can appear in req.Txs verbatim)"
    )
    proc = subprocess.run(
        [
            "curl",
            "-s",
            f"{NODE_URL}/broadcast_tx_sync?tx=0x{target_hex}",
        ],
        capture_output=True,
        text=True,
        timeout=15,
    )
    print(f"    {proc.stdout.strip()}")


def enable_byzantine(target_hex: str) -> None:
    behavior = f"censor_tx:{target_hex}"
    print(
        f"[{now()}] >>> recreating {BYZANTINE_SERVICE} with ENGRAM_BYZANTINE_BEHAVIOR={behavior!r}"
    )
    env = dict(os.environ)
    env["ENGRAM_BYZANTINE_BEHAVIOR"] = behavior
    subprocess.run(
        [
            "docker",
            "compose",
            "-f",
            "compose.yml",
            "-f",
            "docker/engram-node04-byzantine.yml",
            "up",
            "-d",
            "--no-deps",
            "--force-recreate",
            BYZANTINE_SERVICE,
        ],
        capture_output=True,
        text=True,
        timeout=60,
        check=True,
        env=env,
    )


def disable_byzantine() -> None:
    print(f"[{now()}] >>> reverting {BYZANTINE_SERVICE} to its honest definition")
    subprocess.run(
        [
            "docker",
            "compose",
            "-f",
            "compose.yml",
            "up",
            "-d",
            "--no-deps",
            "--force-recreate",
            BYZANTINE_SERVICE,
        ],
        capture_output=True,
        text=True,
        timeout=60,
    )


def scan_logs_for_reject(container: str, since: str) -> int:
    """Best-effort count of round-skip/reject activity in a node's own logs
    since a given timestamp -- a real observable side effect of IsCensoring
    tripping (nil prevote -> round timeout -> f+1 quorum round-skip, per
    root CLAUDE.md's M0b), not a proxy for anything unobservable.
    """
    proc = subprocess.run(
        ["docker", "logs", "--since", since, container],
        capture_output=True,
        text=True,
        timeout=15,
    )
    text = proc.stdout + proc.stderr
    return len(
        re.findall(
            r"round-skip|enterPrevote.*nil|received invalid proposal",
            text,
            re.IGNORECASE,
        )
    )


class Tracker:
    def __init__(self):
        self.start = time.time()
        self.samples = []
        self.divergence_events = []

    def elapsed(self):
        return time.time() - self.start

    def poll_for(self, seconds: float, interval: float, phase: str):
        deadline = time.time() + seconds
        while time.time() < deadline:
            t = self.elapsed()
            all_samples = sample_all_nodes()
            self.samples.extend(all_samples)

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
            by_height = {}
            for node, h in honest_heights.items():
                by_height.setdefault(h, {})[node] = honest_hashes[node]
            for h, hashes_at_h in by_height.items():
                if len(hashes_at_h) > 1 and len(set(hashes_at_h.values())) > 1:
                    self.divergence_events.append((t, phase, h, dict(hashes_at_h)))
                    print(
                        f"  *** [{phase}] SAFETY VIOLATION @ {t:6.0f}s height={h}: {hashes_at_h} ***"
                    )

            states = {s.node: (s.height, s.fsm_state) for s in all_samples}
            print(f"[{t:6.0f}s][{phase}] {states}")
            time.sleep(interval)


def main():
    os.makedirs(RESULTS_DIR, exist_ok=True)
    tr = Tracker()

    print("=== E8 A7 live Censorship test ===")
    print("=== Phase 1: baseline ===")
    tr.poll_for(15.0, 3.0, "baseline")

    # node04 goes byzantine BEFORE the target tx is ever registered/
    # broadcast -- ordering matters: x/sovereignty/preblock.go's
    # updateForcedTxTracking dequeues a forced tx the moment ANY leader
    # (honest or not) includes it -- an earlier version left it queued
    # forever after inclusion, permanently deadlocking the chain, since the
    # tx can never reappear once consumed. If the target were registered
    # while node04 is still
    # honest, an honest leader could dequeue it before byzantine mode even
    # activates, and the test would trivially pass without ever exercising
    # node04's censor_tx path. Enabling byzantine first means whichever
    # validator leads the round the target lands in mempool determines
    # what's actually observed: node04 (25% of rounds) censors it and the
    # NEXT leader picks it up; an honest leader just includes it directly --
    # either way, height must keep progressing normally the whole time.
    print("=== Phase 2: node04 byzantine (censor_tx), before target tx exists ===")
    build_only_hex = build_target_tx_hex()
    enable_byzantine(build_only_hex)
    tr.poll_for(10.0, 3.0, "byzantine_armed")

    print("=== Phase 3: register + broadcast the real target tx, poll 90s ===")
    attack_start_ts = time.strftime("%Y-%m-%dT%H:%M:%S")
    target_hex = build_only_hex
    print(f"    target tx: {len(target_hex)//2} bytes")
    register_forced_tx(target_hex)
    broadcast_target_tx(target_hex)
    tr.poll_for(90.0, 3.0, "censoring")

    print("=== Phase 4: revert node04 to honest ===")
    disable_byzantine()
    tr.poll_for(30.0, 3.0, "recovery")

    reject_signals = {c: scan_logs_for_reject(c, attack_start_ts) for c in HONEST_NODES}

    ts_label = time.strftime("%Y%m%dT%H%M%S")
    csv_path = os.path.join(RESULTS_DIR, f"censorship_a7_{ts_label}.csv")
    write_csv(tr.samples, csv_path)

    summary_path = os.path.join(RESULTS_DIR, f"censorship_a7_{ts_label}_summary.md")
    safety_held = len(tr.divergence_events) == 0
    with open(summary_path, "w") as f:
        f.write("# LIVE Censorship test (E8 A7 adversarial half)\n\n")
        f.write(
            f"Target tx: {len(target_hex)//2} bytes (hex, see raw CSV/logs for full value).\n\n"
        )
        f.write(f"Total duration: {tr.elapsed():.0f}s.\n\n")
        f.write("## Verdict\n\n")
        f.write(
            f"- Safety held (honest AppHash never diverged at the same height): **{safety_held}**\n"
        )
        f.write(f"- Divergence events: {len(tr.divergence_events)}\n")
        f.write(
            f"- Reject/round-skip log signals per honest node during the censoring window: {reject_signals}\n\n"
        )
        if tr.divergence_events:
            f.write("### Divergence detail\n\n")
            for t, phase, h, hashes in tr.divergence_events:
                f.write(f"- t={t:.0f}s phase={phase} height={h}: {hashes}\n")

    print(f"\nwrote {len(tr.samples)} samples to {csv_path}")
    print(f"wrote summary to {summary_path}")
    print(
        f"\nVERDICT: safety_held={safety_held} divergence_events={len(tr.divergence_events)} reject_signals={reject_signals}"
    )
    if not safety_held:
        sys.exit(1)


if __name__ == "__main__":
    main()
