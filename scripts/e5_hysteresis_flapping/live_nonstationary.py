#!/usr/bin/env python3
"""One live run of the non-stationary hysteresis experiment (docs/EXPERIMENT.md E5c).

The case for an ADAPTIVE down threshold is a network whose noise changes, not stationary jitter (there a well-chosen
static value already removes flapping). One run goes through three phases on a cluster that starts ANCHORED:

  1. quiet   --quiet-s seconds, no injected noise;
  2. noisy   --noisy-s seconds under the WAN latency/jitter profile (the noise that produced the flaps of the
             stationary sweep), then the profile is removed;
  3. quiet   wait until the chain is ANCHORED again, stay ANCHORED for --offset-s more seconds (the time the adaptive
             level needs to decay), then stop every Celestia bridge and time the first non-ANCHORED state
             (detection delay) and SOVEREIGN. The bridges are restored afterwards.

Reported: flaps / demotions / ANCHORED share of the noisy phase (state sampled once a second on node01: the FSM state is
committed, so every validator sees the same), the time to regain ANCHORED, and the detection delay after `offset`.

    python3 -u scripts/e5_hysteresis_flapping/live_nonstationary.py --label static2_o0_t1 --offset-s 0
"""

import argparse
import json
import os
import sys
import time

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(HERE, "..", "framework"))
from injector import cleanup_wan_profile, start_pumba_wan_profile  # noqa: E402
from logger import query_node, start_all_da_bridges, stop_all_da_bridges  # noqa: E402

RESULTS_DIR = os.path.join(HERE, "results_live")


def state():
    s = query_node("engram-node01")
    return s.fsm_state, s.height


def poll(seconds, sink):
    end = time.time() + seconds
    while time.time() < end:
        sink.append(state()[0])
        time.sleep(1.0)


def flaps_and_demotions(states):
    """A flap is a transition that reverses the previous one (live_spot_check.compute_metrics)."""
    prev, last = states[0], None
    flaps = demotions = 0
    for s in states[1:]:
        if s != prev:
            if last == f"{s}->{prev}":
                flaps += 1
            if prev == "ANCHORED" and s == "SUSPICIOUS":
                demotions += 1
            last = f"{prev}->{s}"
        prev = s
    return flaps, demotions


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--label", required=True)
    ap.add_argument("--quiet-s", type=float, default=150.0)
    ap.add_argument("--noisy-s", type=float, default=250.0)
    ap.add_argument("--offset-s", type=float, default=0.0)
    ap.add_argument("--wan-profile", default="chaos-wan-latency")
    ap.add_argument("--regain-timeout-s", type=float, default=600.0)
    ap.add_argument("--detect-timeout-s", type=float, default=240.0)
    ap.add_argument("--recover-timeout-s", type=float, default=900.0)
    args = ap.parse_args()

    if state()[0] != "ANCHORED":
        sys.exit("cluster is not ANCHORED")
    quiet1, noisy = [], []
    poll(args.quiet_s, quiet1)
    start_pumba_wan_profile(args.wan_profile)
    try:
        poll(args.noisy_s, noisy)
    finally:
        cleanup_wan_profile(args.wan_profile)
    flaps, demotions = flaps_and_demotions(noisy)

    t_end_noise = time.time()
    regained = None
    while time.time() - t_end_noise < args.regain_timeout_s:
        if state()[0] == "ANCHORED":
            regained = time.time() - t_end_noise
            break
        time.sleep(1.0)
    out = {"label": args.label, "offset_s": args.offset_s, "noisy_flaps": flaps, "noisy_demotions": demotions,
           "noisy_anchored_share": round(noisy.count("ANCHORED") / len(noisy), 4),
           "quiet1_anchored_share": round(quiet1.count("ANCHORED") / len(quiet1), 4),
           "regain_anchored_s": round(regained, 1) if regained is not None else None,
           "detection_delay_s": None, "blocks_to_detection": None, "time_to_sovereign_s": None,
           "left_anchored_during_offset": False, "recovered": None}
    if regained is not None:
        end = time.time() + args.offset_s
        while time.time() < end:
            if state()[0] != "ANCHORED":
                out["left_anchored_during_offset"] = True
                break
            time.sleep(1.0)
        if not out["left_anchored_during_offset"]:
            h0 = state()[1]
            t0 = time.time()
            stop_all_da_bridges()
            susp = sov = None
            try:
                while time.time() - t0 < args.detect_timeout_s and sov is None:
                    st, h = state()
                    now = time.time() - t0
                    if susp is None and st != "ANCHORED":
                        susp, out["blocks_to_detection"] = now, h - h0
                    if st == "SOVEREIGN":
                        sov = now
                    time.sleep(1.0)
            finally:
                start_all_da_bridges()
            out["detection_delay_s"] = round(susp, 1) if susp is not None else None
            out["time_to_sovereign_s"] = round(sov, 1) if sov is not None else None
            deadline = time.time() + args.recover_timeout_s
            out["recovered"] = False
            while time.time() < deadline:
                if state()[0] == "ANCHORED":
                    out["recovered"] = True
                    break
                time.sleep(5)
    os.makedirs(RESULTS_DIR, exist_ok=True)
    path = os.path.join(RESULTS_DIR, f"nonstationary_{args.label}_{time.strftime('%Y%m%dT%H%M%S')}.json")
    json.dump(out, open(path, "w"), indent=1)
    print(json.dumps(out), "->", path)


if __name__ == "__main__":
    main()
