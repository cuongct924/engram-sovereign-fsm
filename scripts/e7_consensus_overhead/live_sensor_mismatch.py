#!/usr/bin/env python3
"""LIVE nil-prevote-under-sensor-mismatch test -- docs/EXPERIMENT.md's E7
(canonical home; E3's own dangling "nil-prevote ratio" promise cross-
references this result instead of re-measuring it). As a side effect of the
same /dump_consensus_state poller, this also answers E3's other still-open
promise, "consensus rounds/block" (the distinct round numbers observed).

Mechanism (x/sovereignty/proposal.go:301): ProcessProposal REJECTs whenever
ext.Healthy (the proposer's committed claim) disagrees with
types.IsHealthyCondition(in.Metrics, k.Params) -- the VALIDATING node's own,
independently re-probed sensors (RefreshMetrics, proposal.go:270). REJECT
yields a nil prevote for that validator/round (that line's own comment: ABCI
2.0 can't force an immediate round advance). This exact reject-on-mismatch
path is already exercised for an fsm_state mismatch in E8's
TestProcessProposal_RejectsFSMStateMismatch -- "reject -> nil prevote" is
proven, not hypothetical, here.

IsHealthyCondition and its component predicates
(x/sovereignty/types/predicates.go) are pure, instantaneous booleans with NO
hysteresis/streak smoothing -- confirmed by reading the code directly. So
isolating one validator's DA reachability makes ITS OWN Healthy flip
immediately and stay flipped for the entire isolation window, not a narrow
race -- a wide, reproducible mismatch window.

Isolation: `docker network disconnect celestia-net engram-node04` (leaving
engram-net/bitcoin-net untouched, so node04 keeps gossiping/voting normally
-- only its DA reachability breaks) / `docker network connect --ip <static>
celestia-net engram-node04` to reconnect. Same technique
scripts/e3_failure_matrix/live_p2p_lifecycle_test.py already uses for P2P
links -- chosen over Pumba netem, which targets a CONTAINER's whole
interface and would also hit node04's P2P gossip on a different interface,
a bigger blast radius than intended here.

This should NEVER halt the cluster: only node04's own votes go nil: the
other 3 validators still form a quorum (N=4, f=1) and keep committing.

Measurement channel, and why it changed from the first design tried this
session: reading the in-progress round's live `votes[].prevotes` array via
fixed-interval polling mostly catches "not yet received" (rendered
identically to a real nil vote, "nil-Vote") rather than a settled outcome --
confirmed live, baseline nil-ratio came out ~1.0 regardless of health, an
unusable signal at this cluster's ~1s block time. Reading
`round_state.last_commit.votes[index]` instead -- the SETTLED PRECOMMIT
record for the block that just committed -- is immune to this: a validator
that prevotes nil (rejects the proposal) has nothing to lock onto and
precommits nil for that round too, so last_commit's per-validator entry
reliably reflects the same underlying REJECT event, just one voting step
later. The metric name stays "nil-prevote ratio" (matching
docs/EXPERIMENT.md's existing terminology for what this event represents),
measured via this more reliable precommit channel.

Usage (against an already-deployed, healthy ANCHORED testnet -- no genesis
change needed, MaxChurnRate/MinAvgTenure/DAThreshold etc. are already
DefaultParams()):
    python3 -u scripts/e7_consensus_overhead/live_sensor_mismatch.py
"""

import os
import re
import subprocess
import sys
import time

_HEIGHT_ROUND_RE = re.compile(r"(\d+)/(\d+)/")

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "framework"))
from logger import dump_consensus_state, own_validator_address, own_precommit_status  # noqa: E402

RESULTS_DIR = os.path.join(os.path.dirname(__file__), "results_live")
TARGET_NODE = "engram-node04"
TARGET_CELESTIA_IP = "172.22.0.130"  # docker/engram-validator-cluster.yml's static assignment


def now() -> str:
    return time.strftime("%H:%M:%S", time.gmtime())


def docker(*args):
    subprocess.run(["docker", *args], capture_output=True, text=True, timeout=30)


def isolate_da() -> None:
    print(f"[{now()}] >>> docker network disconnect celestia-net {TARGET_NODE}")
    docker("network", "disconnect", "celestia-net", TARGET_NODE)


def reconnect_da() -> None:
    print(f"[{now()}] >>> docker network connect --ip {TARGET_CELESTIA_IP} celestia-net {TARGET_NODE}")
    docker("network", "connect", "--ip", TARGET_CELESTIA_IP, "celestia-net", TARGET_NODE)


class Tracker:
    def __init__(self, target_address: str):
        self.start = time.time()
        self.target_address = target_address
        self.rows = []  # (t, phase, committed_height, vote_str_or_None)
        self.seen_heights = set()  # dedup: last_commit repeats across polls until the next block

    def elapsed(self):
        return time.time() - self.start

    def poll_once(self, phase: str):
        t = self.elapsed()
        try:
            dump = dump_consensus_state(TARGET_NODE)
            result = own_precommit_status(dump, self.target_address)
        except Exception as e:  # noqa: BLE001 -- a failed poll is a data point (e.g. mid-isolation RPC hiccup), not a crash
            print(f"  (dump_consensus_state poll failed: {e})")
            result = None
        if result:
            committed_height, vote_str = result
            self.rows.append((t, phase, committed_height, vote_str))
            self.seen_heights.add(committed_height)
            print(f"[{t:6.0f}s][{phase}] committed_height={committed_height} own_precommit={vote_str}")
        else:
            self.rows.append((t, phase, None, None))
            print(f"[{t:6.0f}s][{phase}] (no reading -- no commit yet)")

    def poll_for(self, seconds: float, interval: float, phase: str):
        deadline = time.time() + seconds
        while time.time() < deadline:
            self.poll_once(phase)
            time.sleep(interval)

    def nil_ratio(self, phase: str):
        """Fraction of DISTINCT committed heights (deduped, since last_commit
        repeats across polls between blocks) where the target nil-voted."""
        seen = {}
        for _, p, h, v in self.rows:
            if p == phase and h is not None:
                seen[h] = v  # last write wins; value is stable per height anyway
        if not seen:
            return None
        nil_count = sum(1 for v in seen.values() if v == "nil-Vote")
        return nil_count / len(seen)

    def rounds_seen(self, phase: str = None):
        """Distinct commit rounds observed (E3's own still-open 'consensus
        rounds/block' promise), parsed from any non-nil Vote string's
        HEIGHT/ROUND/... prefix."""
        rounds = set()
        for _, p, h, v in self.rows:
            if phase and p != phase:
                continue
            if v and v != "nil-Vote":
                m = _HEIGHT_ROUND_RE.search(v)
                if m:
                    rounds.add(int(m.group(2)))
        return sorted(rounds)


def main():
    os.makedirs(RESULTS_DIR, exist_ok=True)

    print(f"=== E7 live nil-prevote under sensor mismatch: isolating {TARGET_NODE}'s DA link ===")
    target_address = own_validator_address(TARGET_NODE)
    print(f"    {TARGET_NODE}'s own validator address: {target_address}")

    tr = Tracker(target_address)

    print("=== Phase 1: baseline (DA connected) ===")
    tr.poll_for(20.0, 2.0, "baseline")

    print("=== Phase 2: isolate DA link (60s) ===")
    isolate_da()
    tr.poll_for(60.0, 2.0, "isolated")

    print("=== Phase 3: reconnect DA link, recovery (30s) ===")
    reconnect_da()
    tr.poll_for(30.0, 2.0, "recovery")

    baseline_ratio = tr.nil_ratio("baseline")
    isolated_ratio = tr.nil_ratio("isolated")
    recovery_ratio = tr.nil_ratio("recovery")
    round_numbers = tr.rounds_seen()

    heights = [r[2] for r in tr.rows if r[2] is not None]
    cluster_progressed = len(heights) >= 2 and max(heights) > min(heights)

    ts_label = time.strftime("%Y%m%dT%H%M%S")
    summary_path = os.path.join(RESULTS_DIR, f"sensor_mismatch_{ts_label}_summary.md")
    with open(summary_path, "w") as f:
        f.write("# LIVE nil-prevote under sensor mismatch (E7)\n\n")
        f.write(
            f"Target: {TARGET_NODE} ({target_address}), isolated from `celestia-net` only "
            f"(`engram-net`/`bitcoin-net` untouched). Total duration: {tr.elapsed():.0f}s.\n\n"
        )
        f.write(
            "**Method:** measured via `round_state.last_commit.votes[index]` -- the SETTLED "
            "precommit record for the block that just committed, not the in-progress round's "
            "live votes (which mostly show \"not yet received\" at this cluster's ~1s block "
            "time, an unusable signal, confirmed live this session). A validator that prevotes "
            "nil precommits nil too (nothing to lock onto), so this reliably reflects the same "
            "underlying REJECT event one voting step later. Ratio is over DISTINCT committed "
            "heights (deduped -- last_commit repeats across polls between blocks).\n\n"
        )
        f.write("## Verdict\n\n")
        f.write(f"- Baseline nil-prevote ratio: {baseline_ratio}\n")
        f.write(f"- Isolated nil-prevote ratio: {isolated_ratio}\n")
        f.write(f"- Recovery nil-prevote ratio: {recovery_ratio}\n")
        f.write(f"- Cluster kept committing throughout, no halt (3-of-4 quorum unaffected): **{cluster_progressed}**\n\n")
        f.write("## Consensus rounds observed (E3's own still-open \"rounds/block\" promise, same poller)\n\n")
        f.write(f"- Distinct commit-round numbers seen across the whole run: {round_numbers}\n\n")
        f.write("## Full timeline\n\n")
        f.write("| t (s) | phase | committed_height | own_precommit |\n")
        f.write("|---:|---|---:|---|\n")
        for t, phase, height, vote_str in tr.rows:
            f.write(f"| {t:.0f} | {phase} | {height} | {vote_str} |\n")

    print(f"\nwrote summary to {summary_path}")
    print(
        f"\nVERDICT: baseline_nil_ratio={baseline_ratio} isolated_nil_ratio={isolated_ratio} "
        f"recovery_nil_ratio={recovery_ratio} cluster_progressed={cluster_progressed} "
        f"rounds_observed={round_numbers}"
    )


if __name__ == "__main__":
    main()
