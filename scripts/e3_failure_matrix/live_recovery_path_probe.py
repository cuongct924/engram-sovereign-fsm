#!/usr/bin/env python3
"""Which path made the chain leave RECOVERING: a verified recovery proof, or the BTC-height heuristic?

The state exposes only reanchoring_proof_valid, not the path that set it. This probe runs one cluster-wide DA outage
and records, for the recovery that follows:
  * the timeline of fsm_state / safe_blocks / reanchoring_proof_valid on node01,
  * whether the reanchoring-prover container reported an accepted proof ("proof #N accepted") or a failure
    (witness not buildable, proof rejected),
  * how many heights of the SOVEREIGN interval were committed in a round other than 0 (the circuit only supports
    round 0, so such heights make a proof impossible for the interval they bound).
Run it once per ENGRAM_PARAM_PROOF_ONLY_RECOVERY setting; with proof-only on, a chain that never leaves RECOVERING
shows the proof path cannot recover that interval.

    python3 -u scripts/e3_failure_matrix/live_recovery_path_probe.py --label heuristic_allowed
"""

import argparse
import json
import os
import re
import subprocess
import sys
import time
import urllib.request

HERE = os.path.dirname(__file__)
sys.path.insert(0, os.path.join(HERE, "..", "framework"))
from logger import NODE_RPC_PORTS, query_node, start_all_da_bridges, stop_all_da_bridges  # noqa: E402

RESULTS_DIR = os.path.join(HERE, "results_live")
PORT = NODE_RPC_PORTS["engram-node01"]


def rpc(path: str):
    with urllib.request.urlopen(f"http://127.0.0.1:{PORT}{path}", timeout=10) as r:
        return json.load(r)["result"]


def snap():
    s = query_node("engram-node01")
    return {"t": time.time(), "height": s.height, "state": s.fsm_state, "safe": s.safe_blocks, "proof_valid": s.reanchoring_proof_valid}


def wait_for(state: str, timeout_s: float):
    d = time.time() + timeout_s
    while time.time() < d:
        s = snap()
        if s["state"] == state:
            return s
        time.sleep(3)
    return None


def prover_log(since: float) -> list:
    p = subprocess.run(["docker", "logs", "--since", str(int(since)), "reanchoring-prover"], capture_output=True, text=True, timeout=120)
    keep = re.compile(r"accepted|rejected|cannot build|firing|round|ERROR|error|aggregat|chunk", re.I)
    return [l[:260] for l in (p.stdout + p.stderr).splitlines() if keep.search(l)][-40:]


def nonzero_round_heights(lo: int, hi: int) -> list:
    out = []
    for h in range(lo, hi + 1):
        try:
            if int(rpc(f"/commit?height={h}")["signed_header"]["commit"]["round"]) != 0:
                out.append(h)
        except Exception:  # noqa: BLE001
            pass
    return out


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--label", required=True)
    ap.add_argument("--outage-s", type=int, default=120)
    ap.add_argument("--recovery-timeout-s", type=float, default=900.0)
    args = ap.parse_args()

    if not wait_for("ANCHORED", 900):
        sys.exit("cluster not ANCHORED before the probe")
    t0 = time.time()
    stop_all_da_bridges()
    sov = None
    while time.time() - t0 < args.outage_s:
        s = snap()
        if sov is None and s["state"] == "SOVEREIGN":
            sov = s
        time.sleep(3)
    start_all_da_bridges()

    timeline, rec, anchored = [], None, None
    deadline = time.time() + args.recovery_timeout_s
    while time.time() < deadline:
        s = snap()
        timeline.append(s)
        if rec is None and s["state"] == "RECOVERING":
            rec = s
        if s["state"] == "ANCHORED" and rec is not None:
            anchored = s
            break
        time.sleep(3)

    lo = sov["height"] if sov else 0
    hi = (rec or timeline[-1])["height"] if timeline else lo
    bad = nonzero_round_heights(lo, hi) if sov else []
    logs = prover_log(t0)
    result = {
        "label": args.label, "outage_s": args.outage_s,
        "sovereign_height": lo, "recovering_height": rec["height"] if rec else None,
        "interval_blocks": hi - lo, "left_recovering": anchored is not None,
        "seconds_in_recovering": round(anchored["t"] - rec["t"], 1) if (anchored and rec) else None,
        "proof_valid_first_true_after_recovering_s": next((round(x["t"] - rec["t"], 1) for x in timeline if rec and x["proof_valid"] and x["t"] >= rec["t"]), None),
        "heights_with_round_gt_0": bad, "prover_accepted": any("accepted" in l and "proof #" in l for l in logs),
        "prover_log": logs, "timeline_tail": timeline[-6:],
    }
    os.makedirs(RESULTS_DIR, exist_ok=True)
    path = os.path.join(RESULTS_DIR, f"recovery_path_probe_{args.label}_{time.strftime('%Y%m%dT%H%M%S')}.json")
    json.dump(result, open(path, "w"), indent=1)
    print(json.dumps({k: v for k, v in result.items() if k not in ("prover_log", "timeline_tail")}, indent=1))
    print("wrote", path)


if __name__ == "__main__":
    main()
