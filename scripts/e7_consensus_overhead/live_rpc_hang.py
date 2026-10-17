#!/usr/bin/env python3
"""Hung Bitcoin RPC on the multi-host cluster (Testbed B): `docker pause bitcoin-node01` freezes the daemon, so every
validator's TCP connect succeeds and every request then waits -- the "slow/timeout" sensor of the advisor's test, not a
refused connection. Measures, per phase, with the chain's own FSM state in every sample:

  baseline (60 s)  ->  hang (HANG_S)  ->  resume, until all seven validators are ANCHORED again (RECOVER_TIMEOUT_S)

Per phase: blocks committed and block interval (polled heights), ExtendVote ms (mean / p95 / max) from the
`extend_vote height=.. ms=..` log lines, the FSM states seen, and -- on the snapshot arm -- the stale-snapshot check: no
`sensor_snapshot` line after the freshness window (maxAge 5 s + margin) may report btc_gap == 0 while the daemon is frozen,
which would be the old healthy value kept alive.

    ENGRAM_CLUSTER=ssh python3 -u scripts/e7_consensus_overhead/live_rpc_hang.py --arm snapshot            # arm already deployed
    ENGRAM_CLUSTER=ssh python3 -u scripts/e7_consensus_overhead/live_rpc_hang.py --arm sync --deploy        # fresh genesis first

The daemon is always unpaused on exit (finally), also on Ctrl-C.
"""

import argparse
import csv
import json
import os
import re
import statistics
import sys
import time

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
sys.path.insert(0, os.path.join(HERE, "..", "framework"))
from cluster import CLUSTER  # noqa: E402
from logger import query_node  # noqa: E402
import run_cluster_ab as ab  # noqa: E402

RESULTS = os.path.join(HERE, "results_live")
EXTEND_RE = re.compile(r"extend_vote height=(\d+) ms=([0-9.]+)")
SNAP_RE = re.compile(r"sensor_snapshot height=(\d+) btc_gap=(\d+) .*?da_gap=(\d+) .*?snapshot_degraded=(true|false)")
FRESH_MARGIN_S = 10.0  # maxAge 5 s plus the reader loop's own period and a block or two


def pct(xs, p):
    if not xs:
        return None
    xs = sorted(xs)
    return xs[min(len(xs) - 1, int(round(p * (len(xs) - 1))))]


def sample_all():
    out = []
    for n in CLUSTER.nodes:
        try:
            s = query_node(n)
            out.append((n, s.height, s.fsm_state, ""))
        except Exception as exc:  # noqa: BLE001 -- a node mid-sample is recorded, not fatal
            out.append((n, 0, "", str(exc)[:80]))
    return out


def docker(cmd: str) -> None:
    r = CLUSTER.run(CLUSTER.infra, f"{CLUSTER.sudo(CLUSTER.infra)} docker {cmd}".strip())
    if r.returncode != 0:
        raise RuntimeError(f"docker {cmd} on infra failed: {r.stderr.strip()}")


def bitcoind_state() -> str:
    r = CLUSTER.run(CLUSTER.infra, f"{CLUSTER.sudo(CLUSTER.infra)} docker inspect -f '{{{{.State.Status}}}} paused={{{{.State.Paused}}}}' bitcoin-node01".strip())
    return r.stdout.strip()


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--arm", choices=["sync", "snapshot"], required=True)
    ap.add_argument("--deploy", action="store_true", help="fresh genesis of the arm before the test")
    ap.add_argument("--hang-s", type=float, default=240)
    ap.add_argument("--baseline-s", type=float, default=60)
    ap.add_argument("--recover-timeout-s", type=float, default=1800)
    ap.add_argument("--tag", default="b2")
    ap.add_argument("--max-age-ms", type=int, default=None, help="ENGRAM_SENSOR_MAX_AGE_MS of the snapshot arm (needs --deploy); default is the code's 5000")
    args = ap.parse_args()
    if CLUSTER.name != "ssh":
        sys.exit("run with ENGRAM_CLUSTER=ssh")
    if args.deploy:
        for var in ("DOCKERHUB_USER", "ENGRAM_IMAGE_TAG", "BITCOIN_RPC_PASSWORD"):
            if not os.environ.get(var):
                sys.exit(f"{var} is not set")
        ab.deploy(args.arm, extra={"sensor_max_age_ms": args.max_age_ms} if args.max_age_ms else None)
    ab.check_config(args.arm)
    if args.max_age_ms:
        for n in CLUSTER.nodes:
            got = CLUSTER.container_env(n, "ENGRAM_SENSOR_MAX_AGE_MS")
            if got != str(args.max_age_ms):
                sys.exit(f"{n}: ENGRAM_SENSOR_MAX_AGE_MS={got}, wanted {args.max_age_ms}")
    if not ab.wait_anchored(1800):
        sys.exit("the chain is not ANCHORED on all validators; refusing to start the hang test")
    if "paused=false" not in bitcoind_state():
        sys.exit(f"bitcoin-node01 is not running unpaused: {bitcoind_state()}")

    rows, marks = [], {}
    t0 = time.time()

    def poll(seconds: float, phase: str, interval: float = 2.0) -> None:
        end = time.time() + seconds
        while time.time() < end:
            t = time.time()
            for n, h, st, err in sample_all():
                rows.append(dict(t=round(t - t0, 2), phase=phase, node=n, height=h, fsm_state=st, error=err))
            time.sleep(max(0.0, interval - (time.time() - t)))

    def tip() -> int:
        hs = [r["height"] for r in rows[-len(CLUSTER.nodes):]]
        return max(hs) if hs else 0

    recovered_after_s = None
    try:
        print(f"== baseline {args.baseline_s:.0f} s", flush=True)
        poll(args.baseline_s, "baseline")
        marks["baseline_end"] = (time.time() - t0, tip())

        print("== hang: docker pause bitcoin-node01", flush=True)
        docker("pause bitcoin-node01")
        marks["hang_start"] = (time.time() - t0, tip())
        poll(args.hang_s, "hang")
        marks["hang_end"] = (time.time() - t0, tip())
    finally:
        docker("unpause bitcoin-node01")
        print("== resumed: " + bitcoind_state(), flush=True)

    marks["resume"] = (time.time() - t0, tip())
    deadline = time.time() + args.recover_timeout_s
    while time.time() < deadline:
        poll(10, "recovery")
        last = rows[-len(CLUSTER.nodes):]
        if all(r["fsm_state"] == "ANCHORED" for r in last):
            recovered_after_s = round(time.time() - t0 - marks["resume"][0], 1)
            break
    marks["end"] = (time.time() - t0, tip())

    print("== reading validator logs", flush=True)
    ev = {}  # height -> [ms over validators]
    snap = []  # (height, btc_gap, da_gap, degraded)
    for n in CLUSTER.nodes:
        for line in CLUSTER.logs(n).splitlines():
            m = EXTEND_RE.search(line)
            if m:
                ev.setdefault(int(m.group(1)), []).append(float(m.group(2)))
                continue
            m = SNAP_RE.search(line)
            if m:
                snap.append((int(m.group(1)), int(m.group(2)), int(m.group(3)), m.group(4) == "true"))

    def phase_heights(name: str):
        lo = {"baseline": max(0, min((r["height"] for r in rows if r["phase"] == "baseline" and r["height"]), default=1) - 1), "hang": marks["baseline_end"][1], "recovery": marks["resume"][1]}[name]
        hi = {"baseline": marks["baseline_end"][1], "hang": marks["hang_end"][1], "recovery": marks["end"][1]}[name]
        return lo + 1, hi

    summary = dict(arm=args.arm, image_tag=os.environ.get("ENGRAM_IMAGE_TAG"), hang_s=args.hang_s, recovered_after_s=recovered_after_s,
                   marks={k: dict(t_s=round(v[0], 1), height=v[1]) for k, v in marks.items()}, phases={})
    t_of = {}
    for r in rows:
        t_of.setdefault(r["node"], []).append((r["t"], r["height"], r["fsm_state"], r["phase"]))
    for name in ("baseline", "hang", "recovery"):
        lo, hi = phase_heights(name)
        ms = [x for h in range(lo, hi + 1) for x in ev.get(h, [])]
        ps = [r for r in rows if r["phase"] == name]
        span = (ps[-1]["t"] - ps[0]["t"]) if ps else 0
        blocks = max(0, hi - lo + 1)
        states = {}
        for r in ps:
            states[r["fsm_state"]] = states.get(r["fsm_state"], 0) + 1
        summary["phases"][name] = dict(
            heights=[lo, hi], blocks=blocks, duration_s=round(span, 1), block_interval_s=round(span / blocks, 3) if blocks else None,
            extend_vote_ms_mean=round(statistics.mean(ms), 2) if ms else None, extend_vote_ms_p95=pct(ms, 0.95),
            extend_vote_ms_max=max(ms) if ms else None, extend_vote_samples=len(ms), fsm_state_samples=states)

    # Stale-snapshot check: the freshness window after the freeze, before the unpause.
    max_age_s = (args.max_age_ms or 5000) / 1000.0
    h_hang_t = marks["hang_start"][0] + max_age_s + 5.0  # the freshness window plus the reader loop and a block or two
    h_from = next((r["height"] for r in rows if r["t"] >= h_hang_t), marks["hang_start"][1] + 1)
    h_to = marks["hang_end"][1]
    window = [s for s in snap if h_from <= s[0] <= h_to]
    b_lo, b_hi = phase_heights("baseline")
    base_window = [s for s in snap if b_lo <= s[0] <= b_hi]
    summary["max_age_ms"] = args.max_age_ms or 5000
    summary["baseline_check"] = dict(snapshot_lines=len(base_window), degraded_lines=sum(1 for s in base_window if s[3]),
                                     note="degraded lines while the daemon is healthy are false positives of the freshness rule")
    summary["stale_check"] = dict(
        heights=[h_from, h_to], snapshot_lines=len(window), btc_gap_zero_lines=sum(1 for s in window if s[1] == 0),
        degraded_lines=sum(1 for s in window if s[3]), btc_gap_values=sorted({s[1] for s in window})[:10],
        note="btc_gap_zero_lines must be 0: a frozen daemon may never read as healthy" if args.arm == "snapshot" else "sync arm prints no sensor_snapshot lines")

    stamp = time.strftime("%Y%m%dT%H%M%S")
    os.makedirs(RESULTS, exist_ok=True)
    base = os.path.join(RESULTS, f"rpc_hang_{args.arm}_{args.tag}_{stamp}")
    with open(base + ".csv", "w", newline="") as f:
        w = csv.DictWriter(f, fieldnames=["t", "phase", "node", "height", "fsm_state", "error"])
        w.writeheader()
        w.writerows(rows)
    with open(base + "_summary.json", "w") as f:
        json.dump(summary, f, indent=2)
    print(json.dumps(summary, indent=2))
    print("wrote", base + ".csv")


if __name__ == "__main__":
    main()
