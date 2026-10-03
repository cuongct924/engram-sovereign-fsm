#!/usr/bin/env python3
"""Detection delay of the ANCHORED -> SUSPICIOUS edge under a sustained DA outage (adaptive-hysteresis sweep).

With the cluster ANCHORED, stop every Celestia bridge and watch node01 once per second: the time to the first
non-ANCHORED state is the detection delay, the time to SOVEREIGN is the escalation delay. Then restore the bridges
and wait for ANCHORED. Run it on a chain whose flap_level is still 0 to measure the responsive start of the
adaptive threshold; run_adaptive_sweep.py gives it a fresh genesis for the adaptive configuration.

    python3 -u scripts/e5_hysteresis_flapping/live_detection_probe.py --label adaptive_t1_detect
"""

import argparse
import json
import os
import sys
import time

HERE = os.path.dirname(__file__)
sys.path.insert(0, os.path.join(HERE, "..", "framework"))
from logger import query_node, start_all_da_bridges, stop_all_da_bridges  # noqa: E402

RESULTS_DIR = os.path.join(HERE, "results_live")


def state():
    s = query_node("engram-node01")
    return s.fsm_state, s.height


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--label", required=True)
    ap.add_argument("--max-wait-s", type=float, default=240.0)
    ap.add_argument("--recover-timeout-s", type=float, default=900.0)
    args = ap.parse_args()

    st, _ = state()
    if st != "ANCHORED":
        sys.exit(f"cluster is {st}, not ANCHORED")
    t0 = time.time()
    h0 = state()[1]
    stop_all_da_bridges()
    susp = sov = None
    h_susp = h_sov = None
    try:
        while time.time() - t0 < args.max_wait_s and sov is None:
            st, h = state()
            now = time.time() - t0
            if susp is None and st != "ANCHORED":
                susp, h_susp = now, h
            if st == "SOVEREIGN":
                sov, h_sov = now, h
            time.sleep(1.0)
    finally:
        start_all_da_bridges()
    deadline = time.time() + args.recover_timeout_s
    recovered = False
    while time.time() < deadline:
        if state()[0] == "ANCHORED":
            recovered = True
            break
        time.sleep(5)
    out = {"label": args.label, "detection_delay_s": round(susp, 1) if susp is not None else None,
           "blocks_to_detection": (h_susp - h0) if h_susp else None,
           "time_to_sovereign_s": round(sov, 1) if sov is not None else None, "recovered": recovered}
    os.makedirs(RESULTS_DIR, exist_ok=True)
    path = os.path.join(RESULTS_DIR, f"detection_{args.label}_{time.strftime('%Y%m%dT%H%M%S')}.json")
    json.dump(out, open(path, "w"), indent=1)
    print(json.dumps(out), "->", path)


if __name__ == "__main__":
    main()
