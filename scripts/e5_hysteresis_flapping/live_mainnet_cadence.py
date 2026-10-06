#!/usr/bin/env python3
"""Live check of the mainnet-cadence conversion (tests/e2e/mainnet_cadence_test.go's prediction).

The testbed mines a regtest block every 20 s, Bitcoin about every 600 s. This run sets the miner to 600 s on a cluster that
starts ANCHORED, watches a healthy baseline (does the FSM leave ANCHORED without a fault?), then pauses checkpoint
submission and times the first SOVEREIGN on node01, with the Bitcoin and Engram blocks that passed. The model predicts
about 50 minutes and 5 Bitcoin blocks for the fault, and a false escalation of a healthy chain about once in eight days
(too rare to be seen in a baseline of an hour or two; the baseline only reports whatever happens).

    python3 -u scripts/e5_hysteresis_flapping/live_mainnet_cadence.py --label mainnet600_t1
"""

import argparse
import json
import os
import subprocess
import sys
import time

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(HERE, "..", "framework"))
from cluster import CLUSTER  # noqa: E402
from logger import bitcoin_block_count, query_node, sample_all_nodes  # noqa: E402

RESULTS = os.path.join(HERE, "results_live")
OVERRIDE = "/tmp/miner_interval_override"
MINER = "bitcoin-miner-loop"


def set_miner(seconds):
    cmd = f"echo {seconds} > {OVERRIDE}" if seconds else f"rm -f {OVERRIDE}"
    subprocess.run(["docker", "exec", MINER, "sh", "-c", cmd], check=True, capture_output=True, timeout=60)


def wait_anchored(timeout_s):
    end = time.time() + timeout_s
    while time.time() < end:
        s = sample_all_nodes()
        if s and all(x.fsm_state == "ANCHORED" for x in s):
            return True
        time.sleep(10)
    return False


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--label", required=True)
    ap.add_argument("--miner-s", type=int, default=600)
    ap.add_argument("--baseline-min", type=float, default=40.0)
    ap.add_argument("--fault-timeout-min", type=float, default=150.0)
    args = ap.parse_args()

    if not wait_anchored(1500):
        sys.exit("cluster not ANCHORED before the run")
    set_miner(args.miner_s)
    try:
        t_start = time.time()
        btc0 = bitcoin_block_count()
        base = []
        while time.time() - t_start < args.baseline_min * 60:
            n = query_node("engram-node01")
            base.append({"t": round(time.time() - t_start, 1), "state": n.fsm_state, "height": n.height})
            time.sleep(5)
        btc_baseline = bitcoin_block_count() - btc0
        base_states = sorted({b["state"] for b in base})
        baseline_sovereign = any(b["state"] == "SOVEREIGN" for b in base)
        left = sum(1 for a, b in zip(base, base[1:]) if a["state"] == "ANCHORED" and b["state"] != "ANCHORED")

        # fault: checkpoint submission paused
        CLUSTER.anchor_pause(True)
        t_fault = time.time()
        btc_f0 = bitcoin_block_count()
        h0 = query_node("engram-node01").height
        t_sus = t_sov = None
        end = t_fault + args.fault_timeout_min * 60
        while time.time() < end:
            n = query_node("engram-node01")
            now = time.time() - t_fault
            if t_sus is None and n.fsm_state != "ANCHORED":
                t_sus = now
            if n.fsm_state == "SOVEREIGN":
                t_sov = now
                h_sov = n.height
                break
            time.sleep(5)
        btc_to_sov = bitcoin_block_count() - btc_f0
        res = {"label": args.label, "miner_s": args.miner_s, "baseline_min": args.baseline_min, "baseline_states": base_states,
               "baseline_btc_blocks": btc_baseline, "baseline_demotions": left, "baseline_reached_sovereign": baseline_sovereign,
               "t_first_non_anchored_s": t_sus, "t_sovereign_s": t_sov, "btc_blocks_to_sovereign": btc_to_sov,
               "engram_blocks_to_sovereign": (h_sov - h0) if t_sov else None}
    finally:
        CLUSTER.anchor_pause(False)
        set_miner(0)
    os.makedirs(RESULTS, exist_ok=True)
    path = os.path.join(RESULTS, f"mainnet_cadence_{args.label}_{time.strftime('%Y%m%dT%H%M%S')}.json")
    with open(path, "w") as f:
        json.dump(res, f, indent=1)
    print(json.dumps(res, indent=1))
    print("wrote", path)


if __name__ == "__main__":
    main()
