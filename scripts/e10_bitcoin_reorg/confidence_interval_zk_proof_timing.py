#!/usr/bin/env python3
"""E10 / item-15 -- real ZK recovery-proof round-trip timing with mean +- 95% confidence interval
across N real accepted proofs, from a `watch_and_prove.sh` log.

`watch_and_prove.sh` (scripts/reanchoring_prover/) already timestamps every
poll ("[HH:MM:SS] check N: ...") and logs "N=X -- firing prove_and_submit.sh
NOW" immediately before running the real query->link->witness->execute->
prove->submit pipeline (prove_and_submit.sh), then the next poll's timestamp
lands right after that pipeline returns. The delta between the "firing"
poll's timestamp and the NEXT poll's timestamp is therefore the real
wall-clock round-trip time for one full proof cycle -- already present in
the log, no new instrumentation needed.

Usage:
    python3 scripts/e10_bitcoin_reorg/ci_zk_proof_timing.py [log_path]
    (defaults to /tmp/watch_and_prove.log)
"""

import os
import re
import sys
from datetime import datetime, timedelta

RESULTS_DIR = os.path.join(os.path.dirname(__file__), "results_live")

CHECK_RE = re.compile(r"^\[(\d\d:\d\d:\d\d)\] check (\d+): N=(\d+)")
FIRING_RE = re.compile(r"firing prove_and_submit\.sh NOW")
ACCEPTED_RE = re.compile(r"proof #(\d+) accepted")
REJECTED_RE = re.compile(r"proof rejected")


def parse_ts(ts: str) -> datetime:
    return datetime.strptime(ts, "%H:%M:%S")


def mean_stdev_ci95(values):
    n = len(values)
    if n == 0:
        return None, None, None
    m = sum(values) / n
    if n < 2:
        return m, 0.0, 0.0
    var = sum((v - m) ** 2 for v in values) / (n - 1)
    stdev = var ** 0.5
    return m, stdev, 1.96 * stdev / (n ** 0.5)


def main():
    log_path = sys.argv[1] if len(sys.argv) > 1 else "/tmp/watch_and_prove.log"
    with open(log_path) as f:
        lines = f.readlines()

    cycles = []  # (round_trip_s, accepted: bool)
    pending_ts = None
    last_ts = None
    for line in lines:
        m = CHECK_RE.match(line)
        if m:
            ts = parse_ts(m.group(1))
            if pending_ts is not None:
                delta = (ts - pending_ts).total_seconds()
                if delta < 0:  # crossed midnight
                    delta += 86400
                cycles.append([delta, None])
                pending_ts = None
            last_ts = ts
            continue
        if FIRING_RE.search(line):
            pending_ts = last_ts
        elif ACCEPTED_RE.search(line) and cycles and cycles[-1][1] is None:
            cycles[-1][1] = True
        elif REJECTED_RE.search(line) and cycles and cycles[-1][1] is None:
            cycles[-1][1] = False

    accepted = [c[0] for c in cycles if c[1] is True]
    rejected = [c[0] for c in cycles if c[1] is False]

    print(f"parsed {len(cycles)} proof cycle(s): {len(accepted)} accepted, {len(rejected)} rejected")
    m, sd, ci = mean_stdev_ci95(accepted)
    if m is not None:
        print(f"accepted round-trip: mean={m:.1f}s +- {ci:.1f}s (95% confidence interval, n={len(accepted)}, stdev={sd:.1f}s)")
        print(f"  values: {[f'{v:.1f}' for v in accepted]}")
    else:
        print("no accepted proof cycles parsed yet")

    os.makedirs(RESULTS_DIR, exist_ok=True)
    ts_label = datetime.now().strftime("%Y%m%dT%H%M%S")
    out_path = os.path.join(RESULTS_DIR, f"zk_proof_timing_confidence_interval_{ts_label}.md")
    with open(out_path, "w") as f:
        f.write("# E10 / item-15: real ZK recovery-proof round-trip timing (mean +- 95% confidence interval)\n\n")
        f.write(
            f"Parsed from `{log_path}` -- round-trip = one full "
            "query -> link chain -> boundary witness -> nargo execute -> bb prove -> "
            "tx-submit-recovery-proof cycle (`prove_and_submit.sh`), timed via "
            "`watch_and_prove.sh`'s own per-poll timestamps (no new instrumentation).\n\n"
        )
        f.write(f"* Accepted: {len(accepted)}, Rejected: {len(rejected)}\n")
        if m is not None:
            f.write(f"* Mean +- 95% confidence interval: {m:.1f}s +- {ci:.1f}s (stdev={sd:.1f}s, n={len(accepted)})\n")
            f.write(f"* Values (s): {', '.join(f'{v:.1f}' for v in accepted)}\n")
    print(f"wrote {out_path}")


if __name__ == "__main__":
    main()
