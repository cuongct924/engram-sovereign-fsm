#!/usr/bin/env python3
"""LIVE RQ2 trial against the real testnet -- docs/EXPERIMENT.md's E11: liveness under peripheral
degradation for one comparison system (hard-gated, engram, soft) and one fault.

The cluster must already be deployed in the mode under test (`make testnet-up MODE=<mode>`); the
script reads ENGRAM_MODE from the running container and refuses to run on a mismatch. One trial is
three phases on one continuous chain: healthy baseline -> fault -> recovery.

Faults (all keep Bitcoin mining; see docs/DEVELOPMENT.md). Under ENGRAM_CLUSTER=ssh they run on
the deploy/ hosts (scripts/framework/cluster.py):
  btc   checkpoint-submission pause: BTC keeps advancing, no new checkpoint, so btc_gap grows
        (the only mechanism confirmed live to grow it -- see live_scenario_matrix.py's S2)
  da    docker stop of every validator's Celestia bridge
  both  both at once

A withdrawal-marker tx is broadcast every 2 s for the whole trial. A withdrawal committed while
the FSM state at that height is SOVEREIGN/RECOVERING is "unsafe" (state read with abci_query at the
block's own height). Phase boundaries are recorded as block heights, not wall-clock times, so the
host/container clock skew cannot misassign a block to a phase.

Usage:
    python3 -u scripts/e11_hard_gated_baseline/live_hard_gated_vs_engram.py --mode hard-gated --fault da
    python3 -u scripts/e11_hard_gated_baseline/live_hard_gated_vs_engram.py --mode engram --fault both --trials 3
"""

import argparse
import base64
import csv
import json
import os
import subprocess
import sys
import threading
import time
import urllib.request
from datetime import datetime, timezone

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(HERE, "..", "framework"))
from cluster import CLUSTER  # noqa: E402
from logger import QUERY_STATE_PATH, _decode_query_state, query_node  # noqa: E402

RESULTS_DIR = os.path.join(HERE, "results_live")
SUMMARY_CSV = os.path.join(RESULTS_DIR, "e11_summary.csv")
NODES = list(CLUSTER.nodes)
OBSERVER = NODES[0]
OBSERVER_URL = f"http://{CLUSTER.rpc_host(OBSERVER)}:{CLUSTER.rpc_port(OBSERVER)}"
UNSAFE_STATES = ("SOVEREIGN", "RECOVERING")
STALL_GAP_S = 30.0  # a longest block gap above this inside the fault window counts as a stall
SUMMARY_FIELDS = [
    "mode", "fault", "trial", "tag", "baseline_s", "fault_s", "recovery_s",
    "baseline_blocks", "baseline_blocks_per_s", "fault_blocks", "fault_blocks_per_s",
    "longest_gap_s", "stalled", "max_round_in_fault", "resume_after_heal_s",
    "withdrawals_submitted", "withdrawals_committed", "withdrawals_unsafe",
    "fsm_states_in_fault", "t_fault_start", "t_heal", "timeline_csv", "blocks_csv",
]


def now() -> str:
    return time.strftime("%H:%M:%S", time.gmtime())


def rpc(path: str, timeout: float = 5.0) -> dict:
    with urllib.request.urlopen(f"{OBSERVER_URL}{path}", timeout=timeout) as r:
        return json.load(r)["result"]


def block_time(s: str) -> float:
    head, _, frac = s.rstrip("Z").partition(".")
    dt = datetime.strptime(head, "%Y-%m-%dT%H:%M:%S").replace(tzinfo=timezone.utc)
    return dt.timestamp() + (float("0." + frac) if frac else 0.0)


def latest_height() -> int:
    return int(rpc("/status")["sync_info"]["latest_block_height"])


def container_mode(node: str) -> str:
    return CLUSTER.container_env(node, "ENGRAM_MODE") or "engram"


def inject(fault: str) -> None:
    if fault in ("btc", "both"):
        CLUSTER.anchor_pause(True)
    if fault in ("da", "both"):
        CLUSTER.da_stop_all()


def heal() -> None:
    """Idempotent: restores every fault regardless of which one ran."""
    CLUSTER.anchor_pause(False)
    CLUSTER.da_start_all()


class Sampler(threading.Thread):
    """Height/round every 0.5 s from the observer, FSM state every 2 s, for the whole trial."""

    def __init__(self, t0: float):
        super().__init__(daemon=True)
        self.t0, self.rows, self._halt = t0, [], threading.Event()
        self._state, self._state_at = "", 0.0

    def run(self):
        while not self._halt.is_set():
            t = time.time()
            row = {"t": round(t - self.t0, 2), "height": -1, "round": -1, "fsm_state": self._state}
            try:
                row["height"] = latest_height()
                hrs = rpc("/consensus_state")["round_state"]["height/round/step"].split("/")
                row["round"] = int(hrs[1])
            except Exception:  # noqa: BLE001 -- a mid-trial container restart must not kill the sampler
                pass
            if t - self._state_at >= 2.0:
                self._state, self._state_at = query_node(OBSERVER).fsm_state, t
                row["fsm_state"] = self._state
            self.rows.append(row)
            self._halt.wait(0.5)

    def stop(self):
        self._halt.set()
        self.join(timeout=10)


class WithdrawalSubmitter(threading.Thread):
    """Broadcasts a unique TX_WITHDRAWAL-marked forced tx every 2 s, without waiting for commit."""

    def __init__(self, tag: str, engramd: str):
        super().__init__(daemon=True)
        self.tag, self.engramd, self.submitted, self._halt = tag, engramd, 0, threading.Event()

    def run(self):
        i = 0
        while not self._halt.is_set():
            payload = f"TX_WITHDRAWAL e11 {self.tag} {i}"
            i += 1
            try:
                raw = subprocess.run([self.engramd, "tx-submit-forced-tx", "--dry-run", "--payload", payload],
                                     capture_output=True, text=True, timeout=15).stdout.strip()
                rpc(f"/broadcast_tx_async?tx=0x{raw}", timeout=5)
                self.submitted += 1
            except Exception:  # noqa: BLE001 -- RPC briefly unavailable; the next tick retries
                pass
            self._halt.wait(2.0)

    def stop(self):
        self._halt.set()
        self.join(timeout=20)


def state_at_height(height: int) -> str:
    path = f"/abci_query?path=%22{QUERY_STATE_PATH}%22&data=0x&height={height}"
    resp = rpc(path)["response"]
    return _decode_query_state(resp.get("value", ""))["fsm_state"] if resp.get("code", 0) == 0 else ""


def scan_blocks(lo: int, hi: int, tag: str) -> list:
    """One row per committed height in [lo, hi]: header time, commit round, tagged withdrawals."""
    rows = []
    needle = f"e11 {tag} ".encode()
    for h in range(lo, hi + 1):
        blk = rpc(f"/block?height={h}")["block"]
        commit = rpc(f"/commit?height={h}")["signed_header"]["commit"]
        txs = [base64.b64decode(t) for t in (blk["data"].get("txs") or [])]
        wd = sum(1 for tx in txs if needle in tx)
        rows.append({
            "height": h, "time": block_time(blk["header"]["time"]), "round": int(commit["round"]),
            "num_txs": len(txs), "withdrawals": wd,
            "state_at_height": state_at_height(h) if wd else "",
        })
    return rows


def wait_ready(mode: str, timeout_s: float) -> bool:
    """Blocks advancing on the observer and, unless hard-gated (never leaves ANCHORED), every
    validator back in ANCHORED."""
    deadline = time.time() + timeout_s
    while time.time() < deadline:
        try:
            h0 = latest_height()
            time.sleep(6)
            advancing = latest_height() > h0
            states = {n: query_node(n).fsm_state for n in NODES}
            if advancing and (mode.startswith("hard-gated") or all(s == "ANCHORED" for s in states.values())):
                return True
            print(f"[{now()}] waiting for readiness: advancing={advancing} states={states}")
        except Exception as e:  # noqa: BLE001
            print(f"[{now()}] waiting for readiness: {e}")
        time.sleep(4)
    return False


def write_csv(path: str, rows: list) -> None:
    with open(path, "w", newline="") as f:
        w = csv.DictWriter(f, fieldnames=list(rows[0].keys()) if rows else ["empty"])
        w.writeheader()
        w.writerows(rows)


def trial_metrics(blocks: list, samples: list, submitted: int, *, h_base: int, h_fault: int, h_heal: int,
                  t_base: float, t_fault: float, t_heal: float) -> dict:
    """Pure function of one trial's scan + sampler rows; t_* are seconds since the trial start."""
    by_h = {b["height"]: b for b in blocks}
    # Longest block-to-block gap from the last pre-fault block through the first post-heal block, so
    # a stall that outlasts the fault window is still counted. None if the chain never resumed.
    after = [b for b in blocks if b["height"] > h_heal]
    last = after[0]["height"] if after else h_heal
    span = [by_h[h_fault]] + [b for b in blocks if h_fault < b["height"] <= last]
    gaps = [b2["time"] - b1["time"] for b1, b2 in zip(span, span[1:])]
    longest_gap = max(gaps) if (gaps and after) else None
    fault_rows = [r for r in samples if t_fault <= r["t"] <= t_heal]
    resume = next((r["t"] - t_heal for r in samples if r["t"] >= t_heal and r["height"] > h_heal), None)
    committed = [b for b in blocks if b["withdrawals"]]
    return {
        "baseline_blocks": h_fault - h_base,
        "baseline_blocks_per_s": round((h_fault - h_base) / (t_fault - t_base), 3),
        "fault_blocks": h_heal - h_fault,
        "fault_blocks_per_s": round((h_heal - h_fault) / (t_heal - t_fault), 3),
        "longest_gap_s": None if longest_gap is None else round(longest_gap, 2),
        "stalled": longest_gap is None or longest_gap >= STALL_GAP_S,
        "max_round_in_fault": max((r["round"] for r in fault_rows), default=-1),
        "resume_after_heal_s": None if resume is None else round(resume, 1),
        "withdrawals_submitted": submitted,
        "withdrawals_committed": sum(b["withdrawals"] for b in committed),
        "withdrawals_unsafe": sum(b["withdrawals"] for b in committed if b["state_at_height"] in UNSAFE_STATES),
        "fsm_states_in_fault": "|".join(sorted({r["fsm_state"] for r in fault_rows if r["fsm_state"]})),
        "t_fault_start": round(t_fault, 1), "t_heal": round(t_heal, 1),
    }


def run_trial(args, trial: int) -> dict:
    ts = time.strftime("%Y%m%dT%H%M%S")
    tag = f"{args.mode}-{args.fault}-t{trial}-{ts}"
    base = os.path.join(RESULTS_DIR, f"e11_{args.mode}_{args.fault}_t{trial}_{ts}")
    t0 = time.time()
    sampler, submitter = Sampler(t0), WithdrawalSubmitter(tag, args.engramd)
    sampler.start()
    submitter.start()
    try:
        h_base, t_base = latest_height(), time.time()
        print(f"[{now()}] {tag}: baseline {args.baseline_s}s from height {h_base}")
        time.sleep(args.baseline_s)
        h_fault, t_fault = latest_height(), time.time()
        print(f"[{now()}] {tag}: INJECT {args.fault} at height {h_fault}")
        inject(args.fault)
        time.sleep(args.fault_s)
        h_heal, t_heal = latest_height(), time.time()
        print(f"[{now()}] {tag}: HEAL at height {h_heal}")
        heal()
        time.sleep(args.recovery_s)
        h_end = latest_height()
    finally:
        heal()
        submitter.stop()
        sampler.stop()

    blocks = scan_blocks(h_base, h_end, tag)
    summary = trial_metrics(
        blocks, sampler.rows, submitter.submitted,
        h_base=h_base, h_fault=h_fault, h_heal=h_heal,
        t_base=t_base - t0, t_fault=t_fault - t0, t_heal=t_heal - t0,
    )
    summary.update({
        "mode": args.mode, "fault": args.fault, "trial": trial, "tag": tag,
        "baseline_s": args.baseline_s, "fault_s": args.fault_s, "recovery_s": args.recovery_s,
        "timeline_csv": os.path.relpath(base + "_timeline.csv", RESULTS_DIR),
        "blocks_csv": os.path.relpath(base + "_blocks.csv", RESULTS_DIR),
    })
    write_csv(base + "_timeline.csv", sampler.rows)
    write_csv(base + "_blocks.csv", blocks)
    new = not os.path.exists(SUMMARY_CSV)
    with open(SUMMARY_CSV, "a", newline="") as f:
        w = csv.DictWriter(f, fieldnames=SUMMARY_FIELDS)
        if new:
            w.writeheader()
        w.writerow(summary)
    print(f"[{now()}] {tag}: {json.dumps(summary)}")
    return summary


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--mode", required=True, choices=["hard-gated", "hard-gated-oob", "engram", "soft"])
    ap.add_argument("--fault", required=True, choices=["btc", "da", "both"])
    ap.add_argument("--trials", type=int, default=1)
    ap.add_argument("--first-trial", type=int, default=1, help="resume a series: number of the first trial to run")
    ap.add_argument("--baseline-s", type=float, default=60)
    ap.add_argument("--fault-s", type=float, default=180)
    ap.add_argument("--recovery-s", type=float, default=120)
    ap.add_argument("--ready-timeout-s", type=float, default=900,
                    help="wait between trials for ANCHORED (engram/soft need the ZK re-anchoring prover running)")
    ap.add_argument("--engramd", default=os.environ.get("ENGRAMD_BIN", os.path.join(HERE, "..", "..", "build", "engramd")))
    args = ap.parse_args()

    try:
        deployed = container_mode(OBSERVER)
    except RuntimeError as e:
        print(f"refusing to run: {e}", file=sys.stderr)
        return 2
    if deployed != args.mode:
        print(f"refusing to run: --mode {args.mode} but {OBSERVER} runs ENGRAM_MODE={deployed} "
              f"(redeploy with `make testnet-up MODE={args.mode}`)", file=sys.stderr)
        return 2
    os.makedirs(RESULTS_DIR, exist_ok=True)

    for trial in range(args.first_trial, args.trials + 1):
        if not wait_ready(args.mode, args.ready_timeout_s):
            print(f"[{now()}] cluster not ready for trial {trial} within {args.ready_timeout_s}s; stopping", file=sys.stderr)
            return 1
        run_trial(args, trial)
    return 0


if __name__ == "__main__":
    sys.exit(main())
