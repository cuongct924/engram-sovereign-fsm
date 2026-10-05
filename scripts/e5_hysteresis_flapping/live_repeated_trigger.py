#!/usr/bin/env python3
"""One live run of the repeated-trigger adversary (docs/EXPERIMENT.md E5c').

The adversary needs no validator: it makes the chain's periphery look bad for a few seconds, over and over, by
stopping every Celestia bridge for --pulse-s seconds and restarting them, --gap-s apart. A static threshold k is
crossed by a pulse that lasts about k blocks every time; the adaptive threshold backs off after each demotion, so
later pulses of the same length are absorbed (the in-process simulation, tests/e2e/adaptive_nonstationary_test.go,
shows the model; this is the same behaviour on the real cluster).

Reported over --duration-s seconds, state sampled once a second on node01: pulses sent and the actual time the bridges
were down, demotions (ANCHORED -> SUSPICIOUS), flaps, share of time in ANCHORED, and whether SOVEREIGN was reached.

    python3 -u scripts/e5_hysteresis_flapping/live_repeated_trigger.py --label static2_t1
"""

import argparse
import json
import os
import sys
import threading
import time

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(HERE, "..", "framework"))
sys.path.insert(0, HERE)
from live_nonstationary import flaps_and_demotions, state  # noqa: E402
from logger import start_all_da_bridges, stop_all_da_bridges  # noqa: E402

RESULTS_DIR = os.path.join(HERE, "results_live")


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--label", required=True)
    ap.add_argument("--duration-s", type=float, default=600.0)
    ap.add_argument("--pulse-s", type=float, default=3.0)
    ap.add_argument("--gap-s", type=float, default=12.0)
    ap.add_argument("--recover-timeout-s", type=float, default=900.0)
    args = ap.parse_args()

    if state()[0] != "ANCHORED":
        sys.exit("cluster is not ANCHORED")
    states, stop = [], threading.Event()

    def sampler():
        while not stop.is_set():
            try:
                states.append(state()[0])
            except Exception:  # noqa: BLE001
                pass
            time.sleep(1.0)

    th = threading.Thread(target=sampler, daemon=True)
    th.start()
    pulses, down_s = 0, []
    end = time.time() + args.duration_s
    try:
        while time.time() < end:
            t0 = time.time()
            stop_all_da_bridges()
            time.sleep(args.pulse_s)
            start_all_da_bridges()
            down_s.append(time.time() - t0)
            pulses += 1
            time.sleep(args.gap_s)
    finally:
        start_all_da_bridges()
        stop.set()
        th.join(timeout=5)
    flaps, demotions = flaps_and_demotions(states)
    deadline = time.time() + args.recover_timeout_s
    recovered = False
    while time.time() < deadline:
        if state()[0] == "ANCHORED":
            recovered = True
            break
        time.sleep(5)
    out = {"label": args.label, "pulses": pulses, "pulse_s_nominal": args.pulse_s,
           "bridge_down_s_mean": round(sum(down_s) / len(down_s), 1) if down_s else None,
           "demotions": demotions, "flaps": flaps,
           "anchored_share": round(states.count("ANCHORED") / max(len(states), 1), 4),
           "reached_sovereign": "SOVEREIGN" in states, "samples": len(states), "recovered": recovered}
    os.makedirs(RESULTS_DIR, exist_ok=True)
    path = os.path.join(RESULTS_DIR, f"repeated_trigger_{args.label}_{time.strftime('%Y%m%dT%H%M%S')}.json")
    json.dump(out, open(path, "w"), indent=1)
    print(json.dumps(out), "->", path)


if __name__ == "__main__":
    main()
