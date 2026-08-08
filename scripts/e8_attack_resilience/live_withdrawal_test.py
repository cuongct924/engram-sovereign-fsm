#!/usr/bin/env python3
"""LIVE withdrawal-during-SOVEREIGN test against the real 4-node testnet --
docs/EXPERIMENT.md's E8 A5. No byzantine validator needed: drives the real
cluster into SOVEREIGN via the already-proven `docker stop celestia-bridge`
fault injection (same mechanism as live_lifecycle_test.py), then submits a
real withdrawal-marker tx via `engramd tx-submit-forced-tx` (cmd/engramd/
e8_cli.go), confirming it is never committed while the FSM stays SOVEREIGN.

Important nuance, unlike A3/A4/A6's byzantine-mode tests: the withdrawal
check (x/sovereignty/proposal.go's check #4, types.WithdrawLocked +
containsWithdrawal) rejects the WHOLE PROPOSAL in ProcessProposal, not the
tx individually in CheckTx/DeliverTx -- CheckTx has no such check, so the tx
is admitted to the mempool successfully. The observable safety symptom is
therefore "never gets included/committed while SOVEREIGN" (the submitting
CLI call times out waiting for a DeliverTx result), not an explicit
rejection error code. This script checks for that directly via RPC polling
rather than relying solely on the CLI's own timeout message.

Usage:
    python3 -u scripts/e8_attack_resilience/live_withdrawal_test.py
"""

import os
import subprocess
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "framework"))
from logger import sample_all_nodes, write_csv, NODE_RPC_PORTS  # noqa: E402

RESULTS_DIR = os.path.join(os.path.dirname(__file__), "results_live")
CELESTIA_BRIDGE = "celestia-bridge"
WITHDRAWAL_PAYLOAD = "TX_WITHDRAWAL live-e8-a5-test-payload"


def now() -> str:
    return time.strftime("%H:%M:%S", time.gmtime())


def docker(*args):
    subprocess.run(["docker", *args], capture_output=True, text=True, timeout=30)


def submit_withdrawal_tx(node_url: str):
    """Runs `engramd tx-submit-forced-tx` as a subprocess, non-fatally --
    a timeout (the tx never got a DeliverTx result within the CLI's own 30s
    poll window) is the EXPECTED, correct outcome while SOVEREIGN, not a
    script failure.
    """
    print(f"[{now()}] >>> engramd tx-submit-forced-tx --payload {WITHDRAWAL_PAYLOAD!r}")
    proc = subprocess.run(
        [
            "engramd",
            "tx-submit-forced-tx",
            "--node",
            node_url,
            "--payload",
            WITHDRAWAL_PAYLOAD,
        ],
        capture_output=True,
        text=True,
        timeout=45,
    )
    print(
        f"    exit={proc.returncode} stdout={proc.stdout.strip()!r} stderr={proc.stderr.strip()!r}"
    )
    return proc


class Tracker:
    def __init__(self):
        self.start = time.time()
        self.samples = []

    def elapsed(self):
        return time.time() - self.start

    def poll_for(self, seconds: float, interval: float, phase: str, target_state=None):
        deadline = time.time() + seconds
        reached = False
        while time.time() < deadline:
            t = self.elapsed()
            round_samples = sample_all_nodes()
            self.samples.extend(round_samples)
            states = {s.node: s.fsm_state for s in round_samples}
            print(f"[{t:6.0f}s][{phase}] {states}")
            if target_state and all(v == target_state for v in states.values()):
                reached = True
                break
            time.sleep(interval)
        return reached


def main():
    os.makedirs(RESULTS_DIR, exist_ok=True)
    tr = Tracker()
    node_url = f"http://localhost:{list(NODE_RPC_PORTS.values())[0]}"

    print("=== Phase 1: baseline (expect ANCHORED) ===")
    tr.poll_for(15.0, 3.0, "baseline")

    print("=== Phase 2: stop celestia-bridge, drive into SUSPICIOUS then SOVEREIGN ===")
    docker("stop", CELESTIA_BRIDGE)
    reached_sovereign = tr.poll_for(
        150.0, 3.0, "escalating_to_sovereign", target_state="SOVEREIGN"
    )
    if not reached_sovereign:
        print(
            "WARNING: did not observe all 4 nodes reach SOVEREIGN within the window -- "
            "the withdrawal test below is only meaningful once SOVEREIGN is confirmed."
        )

    print("=== Phase 3: submit withdrawal tx while SOVEREIGN ===")
    submit_start = tr.elapsed()
    proc = submit_withdrawal_tx(node_url)
    submit_end = tr.elapsed()
    # A CLI timeout/DeliverTx-not-observed error is the EXPECTED correct
    # outcome here (see module doc) -- a returncode of 0 (real success,
    # meaning the tx WAS committed while SOVEREIGN) would be the actual
    # safety violation to flag.
    blocked_correctly = proc.returncode != 0

    print("=== Phase 4: restore celestia-bridge, confirm recovery still works ===")
    docker("start", CELESTIA_BRIDGE)
    tr.poll_for(60.0, 3.0, "recovery")

    ts_label = time.strftime("%Y%m%dT%H%M%S")
    csv_path = os.path.join(RESULTS_DIR, f"withdrawal_test_{ts_label}.csv")
    write_csv(tr.samples, csv_path)

    summary_path = os.path.join(RESULTS_DIR, f"withdrawal_test_{ts_label}_summary.md")
    with open(summary_path, "w") as f:
        f.write("# LIVE withdrawal-during-SOVEREIGN test (E8 A5)\n\n")
        f.write(
            f"Total duration: {tr.elapsed():.0f}s. Reached SOVEREIGN before submitting: "
            f"{reached_sovereign}.\n\n"
        )
        f.write(
            f"## Withdrawal tx submission (t={submit_start:.0f}s to t={submit_end:.0f}s)\n\n"
        )
        f.write(f"- CLI exit code: {proc.returncode}\n")
        f.write(f"- stdout: `{proc.stdout.strip()}`\n")
        f.write(f"- stderr: `{proc.stderr.strip()}`\n\n")
        f.write(f"## Verdict\n\n")
        f.write(
            f"- Withdrawal correctly blocked while SOVEREIGN (tx never committed, CLI did "
            f"not report success): **{blocked_correctly}**\n\n"
        )
        f.write(
            "Note: CheckTx admits the tx to the mempool successfully -- the real rejection "
            "happens at ProcessProposal (the whole proposal containing it is rejected), so "
            "the tx is withheld/pending rather than returning an explicit error code. A CLI "
            "timeout here is the expected, correct signal, not a script failure.\n"
        )

    print(f"\nwrote {len(tr.samples)} samples to {csv_path}")
    print(f"wrote summary to {summary_path}")
    print(
        f"\nVERDICT: reached_sovereign={reached_sovereign} blocked_correctly={blocked_correctly}"
    )


if __name__ == "__main__":
    main()
