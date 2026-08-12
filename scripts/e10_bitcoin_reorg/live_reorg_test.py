#!/usr/bin/env python3
"""LIVE Bitcoin reorg test against the real 4-node testnet -- new E10,
exercising a code path formally specified (spec/core/EngramConsensus.tla's
BitcoinReorg action, spec/README.md's SS7.3 "Threat Model Boundary: Bitcoin
Reorg Depth") but never previously exercised by any live or in-process
experiment in this repo (E1-E9 grep for "reorg" turn up nothing outside the
spec itself).

Mechanism: bitcoin-node01 is the only bitcoind every validator's
AnchorTracker/BTC sensor actually queries (BITCOIN_HOST); bitcoin-node02 is
node01's regtest sync peer, never dialed directly by engramd
(docker/bitcoin-regtest-cluster.yml, compose.yml's comment). Isolating
node02 via `setnetworkactive false` (not `disconnectnode` -- node01's
`-addnode=bitcoin-node02` config would just silently redial and re-sync),
mining extra blocks on node02 alone via RPC (no P2P needed for
generatetoaddress), then reconnecting forces node01 -- and so every
validator watching it -- through a real reorg onto node02's now-longer
chain, orphaning whatever blocks (and checkpoint OP_RETURN transactions)
only existed on node01's abandoned branch.

Two scenarios, matching spec/README.md SS7.3's own split:
    Shallow (reorg depth < KDeepFinality, default 2): the spec proves this
        is SAFE -- IsKDeep/CanElect's ANCHORED/SUSPICIOUS branch structurally
        can't re-elect a checkpoint lost to the reorg. Concretely realized
        by AnchorTracker.VerifyAnchor (x/anchor/anchor.go), which re-derives
        BlockContainsTag fresh via getblockhash(height) every call -- no
        caching, so a reorg that replaces the block at a previously-verified
        height is caught on the very next re-check. Expect: is_btc_spv_failed
        goes true, FSM reacts (IsCriticalCondition), no silent bad-anchor
        acceptance.
    Deep (reorg depth >= KDeepFinality): spec/README.md SS7.3 says this is
        DELIBERATELY OUT OF SCOPE ("no protocol mechanism re-verifies an
        already-K-deep-confirmed checkpoint... the only lever is choosing
        K_DEEP_FINALITY conservatively"). Nothing here is expected to
        "pass" -- the point is observing what the concrete system actually
        does when this precondition is violated, not verifying a guarantee.

Usage:
    python3 -u scripts/e10_bitcoin_reorg/live_reorg_test.py --depth shallow
    python3 -u scripts/e10_bitcoin_reorg/live_reorg_test.py --depth deep --reorg-depth 5
"""

import argparse
import os
import subprocess
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "framework"))
from logger import sample_all_nodes, write_csv, bitcoin_cli, bitcoin_height  # noqa: E402

RESULTS_DIR = os.path.join(os.path.dirname(__file__), "results_live")
NODE01 = "bitcoin-node01"
NODE02 = "bitcoin-node02"
MINER_LOOP = "bitcoin-miner-loop"
MINER_PAUSE_FILE = "/tmp/miner_interval_override"
MINER_PAUSE_INTERVAL = "99999"


def now() -> str:
    return time.strftime("%H:%M:%S", time.gmtime())


def poll(seconds: float, samples: list, interval: float = 3.0, label: str = ""):
    deadline = time.time() + seconds
    while time.time() < deadline:
        round_samples = sample_all_nodes()
        samples.extend(round_samples)
        states = {s.node: s.fsm_state for s in round_samples}
        print(f"[{now()}][{label}] btc_h1={bitcoin_height(NODE01)} states={states}")
        time.sleep(interval)


def pause_node01_mining(paused: bool):
    """Freezes/resumes bitcoin-miner-loop's own mining on node01 via the
    live-reloadable interval override (bitcoin_miner_loop.sh, added for
    E2's S2 congestion test) -- without this, node01 keeps extending its
    OWN chain during node02's isolation window, and a first shallow-reorg
    run confirmed live (no "reorg"/"disconnect block" log lines in
    bitcoin-node01's logs at all) that node01's own continuous mining can
    outrun node02's synthetic lead, so reconnecting never actually
    triggers a reorg -- the FSM staying ANCHORED throughout proved nothing
    by itself without first confirming a reorg really happened.
    """
    if paused:
        subprocess.run(
            ["docker", "exec", MINER_LOOP, "sh", "-c", f"echo {MINER_PAUSE_INTERVAL} > {MINER_PAUSE_FILE}"],
            check=True,
        )
    else:
        subprocess.run(["docker", "exec", MINER_LOOP, "rm", "-f", MINER_PAUSE_FILE], check=True)


def ensure_node02_wallet() -> str:
    """Returns a mining-destination address on node02, creating its wallet
    if this is the first run (fresh regtest data dirs won't have one)."""
    try:
        return bitcoin_cli("-rpcwallet=reorgwallet", "getnewaddress", node=NODE02)
    except RuntimeError:
        bitcoin_cli("createwallet", "reorgwallet", node=NODE02)
        return bitcoin_cli("-rpcwallet=reorgwallet", "getnewaddress", node=NODE02)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--depth", choices=["shallow", "deep"], required=True)
    parser.add_argument(
        "--reorg-depth",
        type=int,
        default=None,
        help="number of node01's ORIGINAL blocks to orphan -- default 1 for "
        "shallow (< KDeepFinality=2), 5 for deep (>> KDeepFinality=2). "
        "invalidateblock targets height (h1_before - reorg_depth + 1), so "
        "exactly this many original blocks get replaced -- NOT how many "
        "new blocks get mined (that's reorg_depth+1, one longer to win "
        "the longest-chain rule); an earlier version of this script "
        "conflated the two and always invalidated only the tip (depth=1) "
        "regardless of the requested depth, confirmed live via "
        "getblockcount on node02 dropping by exactly 1 after invalidation "
        "no matter what depth was requested.",
    )
    parser.add_argument(
        "--settle-s",
        type=float,
        default=60.0,
        help="seconds to watch node01/Engram after reconnect",
    )
    args = parser.parse_args()
    reorg_depth = args.reorg_depth or (1 if args.depth == "shallow" else 5)

    os.makedirs(RESULTS_DIR, exist_ok=True)
    samples: list = []

    print(f"=== E10 Bitcoin reorg test ({args.depth}, reorg_depth={reorg_depth}) ===")
    print("--- baseline: confirming cluster healthy before injecting anything ---")
    poll(15, samples, label="baseline")

    print(f"[{now()}] freezing node01's own mining (bitcoin-miner-loop) first, then waiting for the "
          f"pause to actually take effect")
    pause_node01_mining(True)
    # bitcoin_miner_loop.sh only re-reads its interval override at the START
    # of its NEXT loop iteration (after its current sleep completes) -- with
    # the normal 20s cadence, that's up to ~20s of lag. Capturing h1_before
    # BEFORE this settle window (an earlier version of this script did) lets
    # node01 mine one more block right as the pause is taking hold, making
    # every later "is height still h1_before" check compare against a
    # stale, already-superseded height -- confirmed live: produced a false
    # "actually_reorged=False" even after a real reorg had happened,
    # because the actual frozen tip was h1_before+1, not h1_before.
    time.sleep(25)

    h1_before = bitcoin_height(NODE01)
    h2_before = bitcoin_height(NODE02)
    print(f"[{now()}] post-freeze heights: node01={h1_before} node02={h2_before}")
    orphan_hash = bitcoin_cli("getblockhash", str(h1_before), node=NODE01)
    print(f"[{now()}] node01's block at height {h1_before} (the one node02's longer chain must orphan): {orphan_hash}")

    print(f"[{now()}] isolating {NODE02}")
    try:
        bitcoin_cli("setnetworkactive", "false", node=NODE02)

        # node02 currently shares node01's exact chain (heights matched before
        # isolation) -- mining on TOP of that shared tip would just extend the
        # SAME chain, not create a competing one (confirmed live: a first
        # attempt without this invalidateblock step produced
        # actually_reorged=False, node02's "new" block just synced back onto
        # node01 as a normal extension). invalidateblock targets
        # fork_height = h1_before - reorg_depth + 1, not h1_before itself --
        # invalidating a block also invalidates every descendant, so this
        # orphans exactly reorg_depth original blocks (fork_height..h1_before)
        # once node02's alternate branch overtakes node01's frozen chain.
        fork_height = h1_before - reorg_depth + 1
        fork_point_hash = bitcoin_cli("getblockhash", str(fork_height), node=NODE02)
        print(f"[{now()}] invalidating node02's block at height {fork_height} ({fork_point_hash}) -- orphans {reorg_depth} original block(s) ({fork_height}..{h1_before})")
        bitcoin_cli("invalidateblock", fork_point_hash, node=NODE02)
        h2_forked_tip = bitcoin_height(NODE02)
        print(f"[{now()}] node02 tip after invalidation: height {h2_forked_tip} (expected {fork_height - 1}, rebuilding from here)")

        addr = ensure_node02_wallet()
        target = h1_before + 1  # one block longer than node01's frozen tip -- enough to win via longest-chain
        to_mine = target - h2_forked_tip
        print(f"[{now()}] mining {to_mine} NEW (competing) blocks on isolated {NODE02} to {addr} (target height {target})")
        if to_mine > 0:
            bitcoin_cli("-rpcwallet=reorgwallet", "generatetoaddress", str(to_mine), addr, node=NODE02)
        h2_after = bitcoin_height(NODE02)
        new_hash_at_fork_height = bitcoin_cli("getblockhash", str(h1_before), node=NODE02)
        print(
            f"[{now()}] node02 height now {h2_after} (node01 frozen, still isolated); "
            f"node02's NEW block at height {h1_before} is {new_hash_at_fork_height} "
            f"(differs from original {orphan_hash}: {new_hash_at_fork_height != orphan_hash})"
        )

        print("--- watching node01/Engram while node02's competing chain builds (node01 frozen, isolated) ---")
        poll(10, samples, label=f"{args.depth}_isolated")

        h1_at_reconnect = bitcoin_height(NODE01)
        print(f"[{now()}] node01 height right before reconnect: {h1_at_reconnect} (frozen at {h1_before} as intended: {h1_at_reconnect == h1_before})")
        print(f"[{now()}] reconnecting {NODE02} (setnetworkactive true) -- forcing the real reorg")
        bitcoin_cli("setnetworkactive", "true", node=NODE02)

        print(f"--- watching node01/Engram reorg + settle ({args.settle_s}s) ---")
        poll(args.settle_s, samples, interval=2.0, label=f"{args.depth}_post_reorg")
    finally:
        print(f"[{now()}] resuming node01's own mining (bitcoin-miner-loop)")
        pause_node01_mining(False)

    h1_after = bitcoin_height(NODE01)
    reorg_hash = bitcoin_cli("getblockhash", str(h1_before), node=NODE01)
    actually_reorged = reorg_hash != orphan_hash
    print(f"[{now()}] node01 height after settle: {h1_after} (was {h1_before}, node02 target {target})")
    print(
        f"[{now()}] REORG VERIFICATION: node01's block at height {h1_before} is now {reorg_hash} "
        f"(was {orphan_hash}) -- actually_reorged={actually_reorged}"
    )

    out = os.path.join(RESULTS_DIR, f"reorg_{args.depth}_{time.strftime('%Y%m%dT%H%M%S', time.gmtime())}.csv")
    write_csv(samples, out)
    print(f"wrote {len(samples)} samples to {out}")
    print(
        f"=== DONE: depth={args.depth} node01 {h1_before}->{h1_after} "
        f"(node02 mined to {h2_after}), reconnect at target={target} ==="
    )


if __name__ == "__main__":
    main()
