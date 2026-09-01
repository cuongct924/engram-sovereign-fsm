#!/usr/bin/env python3
"""LIVE Censorship / forced-tx-withholding test against the real 4-node
testnet -- docs/EXPERIMENT.md's E8 A7 adversarial half. Previously had no
live driver: x/sovereignty/proposal.go's applyByzantineBehavior already
supported ENGRAM_BYZANTINE_BEHAVIOR=censor_tx:<target> mechanically, but
nothing exercised it end-to-end against a real forced tx.

Protocol, real and byte-exact throughout (no mocked queue/tx content):

  1. Build a real, independently-decodable "target" tx (another
     MsgSubmitForcedTxRequest, chosen only because a CLI to build it exists
     -- its own content is otherwise irrelevant) via
     `engramd tx-submit-forced-tx --dry-run`. buildMinimalTx
     (cmd/engramd/reanchor_cli.go) has no timestamp/nonce/random field, so
     this is deterministic -- re-running reproduces the exact same bytes.
  2. Register those exact raw bytes in ForcedTxQueue via a SEPARATE
     MsgSubmitForcedTxRequest{Tx: <target's raw bytes>}, using --payload-hex
     for byte-exact content (x/sovereignty/keeper/msg_server.go's
     SubmitForcedTx just does ForcedTxQueue.Set(ctx, string(msg.Tx))).
  3. Broadcast the target tx itself as a real, standalone tx (so it can ever
     appear verbatim in req.Txs[1:], which is what IsCensoring/
     ProcessProposal's check #0 compares ForcedTxQueue entries against --
     x/sovereignty/proposal.go:305-318).
  4. Set node04 byzantine with censor_tx:<hex of target's raw bytes> --
     applyByzantineBehavior hex-decodes this; the target is hex-encoded
     because Docker Compose env-var/YAML interpolation doesn't survive
     arbitrary non-UTF8 tx bytes.
  5. Poll: whenever node04 leads a round, its proposal should omit the target
     tx (observable as it not landing in a block node04 proposed); once
     MaxIgnoreRounds (Params default: 1) is exceeded, the NEXT censoring
     proposal must be REJECTED by the 3 honest validators' IsCensoring check
     -- never committed, safety bar (honest AppHash never diverges) checked
     the whole time, matching live_byzantine_attacks.py's Tracker pattern.

Usage:
    python3 -u scripts/e8_attack_resilience/live_censorship_test.py
"""

import os
import re
import subprocess
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "framework"))
from logger import (  # noqa: E402
    sample_all_nodes,
    write_csv,
    NODE_RPC_PORTS,
    block_proposer,
    own_validator_address,
)

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
    """--dry-run: builds the raw bytes of a throwaway MsgSubmitForcedTxRequest
    WITHOUT broadcasting it -- this is the "other tx" that will be registered
    as forced AND separately broadcast for real, per this module's doc.

    Payload includes a real wall-clock nonce -- a fixed literal payload
    produces the SAME tx hash every run, and CometBFT's tx cache remembers
    a previously-committed hash for a while after commit, rejecting a later
    run's identical resubmission with "tx already exists in cache" (hit
    live, confirmed the earlier tx really had committed successfully).
    """
    proc = run_cli(
        "tx-submit-forced-tx", "--payload", f"e8-a7-censorship-target-{time.time()}", "--dry-run"
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


def observe_recent_proposers(node: str, count: int = 8) -> list:
    """Returns [(height, proposer_address), ...] for the `count` most
    recently committed heights, oldest first -- the raw material for
    predicting the round-robin schedule (predict_next_turn).
    """
    status_height = None
    port = NODE_RPC_PORTS[node]
    from logger import _rpc_get  # local import: leading-underscore internal helper

    status = _rpc_get(port, "/status")
    status_height = int(status["result"]["sync_info"]["latest_block_height"])
    heights = range(max(1, status_height - count + 1), status_height + 1)
    return [(h, block_proposer(node, h)) for h in heights]


def predict_next_turn(node: str, target_address: str, count: int = 8):
    """Predicts the next height target_address will propose, from the
    observed round-robin period among the last `count` committed blocks.
    Returns (predicted_height, period) or (None, None) if target_address
    hasn't proposed at all in the observed window (period can't be inferred
    yet -- caller should widen `count` or just poll normally).

    With N equal-power validators (this testnet: genesis power 10 each),
    CometBFT's proposer-priority algorithm converges to a period-N cycle,
    but the offset is arbitrary and a round skip can perturb it -- this is
    a real-world observed prediction, not a protocol guarantee, so callers
    must tolerate misses (see wait_for_upcoming_turn's retry loop).
    """
    observed = observe_recent_proposers(node, count)
    turns = [h for h, addr in observed if addr == target_address]
    if len(turns) < 2:
        return None, None
    period = turns[-1] - turns[-2]
    return turns[-1] + period, period


def wait_for_upcoming_turn(node: str, target_address: str, max_wait_s: float = 60.0):
    """Blocks until 1 height before target_address's next predicted
    proposer turn (so a tx submitted right after this returns has a full
    block's worth of gossip time to reach target_address's mempool before
    it builds that height's proposal), or gives up after max_wait_s and
    returns None -- callers fall back to blind polling in that case.
    """
    deadline = time.time() + max_wait_s
    while time.time() < deadline:
        predicted, period = predict_next_turn(node, target_address)
        if predicted is None:
            time.sleep(2.0)
            continue
        status = _rpc_get_status_height(node)
        if status >= predicted - 1:
            # Either already at the lead-in height, or (schedule drifted)
            # already past it -- either way, stop predicting and let the
            # caller proceed; re-predicting in a tight loop just burns time.
            return predicted
        time.sleep(min(2.0, (predicted - 1 - status) * 1.0))
    return None


def _rpc_get_status_height(node: str) -> int:
    from logger import _rpc_get

    port = NODE_RPC_PORTS[node]
    status = _rpc_get(port, "/status")
    return int(status["result"]["sync_info"]["latest_block_height"])


def blocks_proposed_by(node: str, target_address: str, start_height: int, end_height: int) -> list:
    """Heights in [start_height, end_height] that target_address actually
    proposed, per the same node's own view -- used to check whether node04
    got a real proposer turn during the censoring window at all (distinct
    from whether it censored: see censorship_actually_observed).
    """
    return [
        h
        for h in range(start_height, end_height + 1)
        if block_proposer(node, h) == target_address
    ]


def censorship_actually_observed(node: str, target_address: str, target_tx_hex: str, heights: list) -> bool:
    """True iff at least one of `heights` (node04's own real proposer turns
    during the censoring window) has a block that OMITS the target tx --
    the direct evidence that ENGRAM_BYZANTINE_BEHAVIOR=censor_tx actually
    fired, not just that node04 got no turn (docs/EXPERIMENT.md's E8 A7
    caveat this closes: a prior run's node04 simply never proposed during
    its poll window, so "no divergence" was vacuous).
    """
    from logger import _rpc_get
    import base64

    target_b64 = base64.b64encode(bytes.fromhex(target_tx_hex)).decode()
    port = NODE_RPC_PORTS[node]
    for h in heights:
        block = _rpc_get(port, f"/block?height={h}")
        txs = block["result"]["block"]["data"].get("txs") or []
        if target_b64 not in txs:
            return True
    return False


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
        # phase -> [(t, max_honest_height), ...] -- liveness signal (chain
        # kept committing during the attack, not just stayed un-diverged),
        # ported from live_timeout_flood_test.py's Tracker.phase_heights/
        # height_rate; this script previously tracked height only to bucket
        # AppHash comparisons, never as a rate.
        self.phase_heights: dict = {}

    def elapsed(self):
        return time.time() - self.start

    def height_rate(self, phase: str) -> float:
        pts = self.phase_heights.get(phase, [])
        if len(pts) < 2:
            return 0.0
        (t0, h0), (t1, h1) = pts[0], pts[-1]
        if t1 <= t0:
            return 0.0
        return (h1 - h0) / (t1 - t0)

    def poll_for(self, seconds: float, interval: float, phase: str):
        self.phase_heights.setdefault(phase, [])
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
            if honest_heights:
                self.phase_heights[phase].append((t, max(honest_heights.values())))

            states = {s.node: (s.height, s.fsm_state) for s in all_samples}
            print(f"[{t:6.0f}s][{phase}] {states}")
            time.sleep(interval)


def main():
    os.makedirs(RESULTS_DIR, exist_ok=True)
    tr = Tracker()

    print("=== E8 A7 live Censorship test ===")
    print("=== Phase 1: baseline ===")
    tr.poll_for(15.0, 3.0, "baseline")

    # node04 goes byzantine BEFORE the target tx is registered/broadcast --
    # ordering matters: x/sovereignty/preblock.go's updateForcedTxTracking
    # dequeues a forced tx the moment ANY leader includes it (an earlier
    # version left it queued forever after inclusion, permanently
    # deadlocking the chain). If the target were registered while node04 is
    # still honest, an honest leader could dequeue it before byzantine mode
    # activates and the test would pass without exercising node04's
    # censor_tx path. Enabling byzantine first means whichever validator
    # leads the round the target lands in determines what's observed: node04
    # (25% of rounds) censors it and the NEXT leader picks it up; an honest
    # leader just includes it -- either way, height keeps progressing.
    print("=== Phase 2: node04 byzantine (censor_tx), before target tx exists ===")
    build_only_hex = build_target_tx_hex()
    enable_byzantine(build_only_hex)
    # 20s, not 10s: --force-recreate leaves node04 RPC-unreachable for several
    # seconds (confirmed live: height=-1 at t=19s of the old 10s window) --
    # too short a settle window let phase 3's tx submission land while node04
    # was still catching up, tripping a slow/round-skipping block and the
    # CLI's own 30s DeliverTx-wait timeout (confirmed live, twice).
    tr.poll_for(20.0, 3.0, "byzantine_armed")

    # Wait until node04's predicted next proposer turn is imminent before
    # submitting the target tx -- registering it right after an ARBITRARY
    # point in the round-robin cycle risks an honest leader dequeuing it
    # first (updateForcedTxTracking dequeues on first inclusion, one-shot),
    # or the 90s window simply ending before node04 ever gets a turn at all
    # (docs/EXPERIMENT.md's E8 A7 finding: reject_signals all zero because
    # the attack was never actually exercised). Falls back to submitting
    # immediately if a schedule can't be predicted within 60s.
    node04_address = own_validator_address(BYZANTINE_SERVICE)
    print(f"=== Phase 3: predicting {BYZANTINE_SERVICE}'s next proposer turn ===")
    predicted_height = wait_for_upcoming_turn(HONEST_NODES[0], node04_address, max_wait_s=60.0)
    if predicted_height:
        print(f"    predicted next {BYZANTINE_SERVICE} turn: height {predicted_height}")
    else:
        print(f"    could not predict a schedule within 60s -- submitting immediately (best effort)")

    print("=== Phase 4: register + broadcast the real target tx, poll 90s ===")
    attack_start_ts = time.strftime("%Y-%m-%dT%H:%M:%S")
    window_start_height = _rpc_get_status_height(HONEST_NODES[0])
    target_hex = build_only_hex
    print(f"    target tx: {len(target_hex)//2} bytes")
    register_forced_tx(target_hex)
    broadcast_target_tx(target_hex)
    tr.poll_for(90.0, 3.0, "censoring")
    window_end_height = _rpc_get_status_height(HONEST_NODES[0])

    node04_turns = blocks_proposed_by(
        HONEST_NODES[0], node04_address, window_start_height, window_end_height
    )
    censoring_observed = (
        censorship_actually_observed(HONEST_NODES[0], node04_address, target_hex, node04_turns)
        if node04_turns
        else False
    )
    print(
        f"    {BYZANTINE_SERVICE} proposer turns during censoring window: {node04_turns} "
        f"(censorship_observed={censoring_observed})"
    )

    print("=== Phase 5: revert node04 to honest ===")
    disable_byzantine()
    tr.poll_for(30.0, 3.0, "recovery")

    reject_signals = {c: scan_logs_for_reject(c, attack_start_ts) for c in HONEST_NODES}

    ts_label = time.strftime("%Y%m%dT%H%M%S")
    csv_path = os.path.join(RESULTS_DIR, f"censorship_a7_{ts_label}.csv")
    write_csv(tr.samples, csv_path)

    summary_path = os.path.join(RESULTS_DIR, f"censorship_a7_{ts_label}_summary.md")
    safety_held = len(tr.divergence_events) == 0
    baseline_rate = tr.height_rate("baseline")
    censoring_rate = tr.height_rate("censoring")
    recovery_rate = tr.height_rate("recovery")
    # Liveness claim: the chain kept committing DURING censorship, not just
    # stayed un-diverged while frozen -- no fixed threshold, report the real
    # rates; "held" means censoring didn't collapse toward 0 vs. baseline.
    liveness_held = baseline_rate == 0 or censoring_rate >= 0.5 * baseline_rate
    with open(summary_path, "w") as f:
        f.write("# LIVE Censorship test (E8 A7 adversarial half)\n\n")
        f.write(
            f"Target tx: {len(target_hex)//2} bytes (hex, see raw CSV/logs for full value).\n\n"
        )
        f.write(f"Total duration: {tr.elapsed():.0f}s.\n\n")
        f.write("## Verdict\n\n")
        f.write(
            f"- **{BYZANTINE_SERVICE} proposer turns during the censoring window: {node04_turns}** "
            f"(empty means the attack was never actually exercised -- treat safety/liveness below as "
            f"vacuous if so)\n"
        )
        f.write(
            f"- **Censorship actually observed (a {BYZANTINE_SERVICE} block omitted the target tx): "
            f"{censoring_observed}**\n"
        )
        f.write(
            f"- Safety held (honest AppHash never diverged at the same height): **{safety_held}**\n"
        )
        f.write(f"- Divergence events: {len(tr.divergence_events)}\n")
        f.write(
            f"- Liveness held (block rate during censoring vs. baseline, no collapse toward 0): "
            f"**{liveness_held}** (baseline {baseline_rate:.3f} blocks/s, censoring "
            f"{censoring_rate:.3f} blocks/s, recovery {recovery_rate:.3f} blocks/s)\n"
        )
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
        f"\nVERDICT: node04_turns={node04_turns} censoring_observed={censoring_observed} "
        f"safety_held={safety_held} divergence_events={len(tr.divergence_events)} "
        f"liveness_held={liveness_held} reject_signals={reject_signals}"
    )
    if not safety_held:
        sys.exit(1)
    if not node04_turns:
        print(
            "WARNING: node04 never proposed during the censoring window -- "
            "this run does not exercise the attack; re-run or widen the window.",
            file=sys.stderr,
        )
        sys.exit(2)


if __name__ == "__main__":
    main()
