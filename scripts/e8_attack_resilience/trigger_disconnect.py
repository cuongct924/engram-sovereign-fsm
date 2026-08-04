#!/usr/bin/env python3
"""
E8 -- Attack-Resilience Test Suite (docs/EXPERIMENT.md's E8, Table 3).

Runs the REAL Go safety-property tests in x/sovereignty/proposal_test.go
(via `go test -json`) and maps each pass/fail result onto docs/EXPERIMENT.md's
E8 attack table. These tests exercise the exact same ProcessProposal/
CommitFSMTransition code path a running node uses (IsValidProposal ported
branch-for-branch from spec/core/EngramTendermint.tla:281-307), so a "Reject"
result here is a real ABCI-level rejection, not a simulated one.

Scope note: two E8 rows genuinely cannot be covered by these in-process unit
tests and are reported as not-covered rather than faked -- "Timeout flooding"
and "Double-signing" both require a live multi-node consensus engine
(round/timer behavior, CometBFT's evidence module) that this environment does
not have running (see M0b/M7 in CLAUDE.md's "Not yet done" section).

Usage:
    python3 scripts/e8_attack_resilience/trigger_disconnect.py
"""

import json
import os
import subprocess
import sys

REPO_ROOT = os.path.join(os.path.dirname(__file__), "..", "..")
OUT_DIR = os.path.join(os.path.dirname(__file__), "results")
TABLE_OUT = os.path.join(OUT_DIR, "table3_attack_resilience.md")

# Maps docs/EXPERIMENT.md's E8 attack rows to the real Go test(s) that cover
# them. A row with an empty test list is explicitly not covered by this
# in-process harness.
ATTACK_TO_TESTS = {
    "Byzantine proposer sets fake fsm_state=ANCHORED under critical local sensors": (
        ["TestProcessProposal_RejectsFSMStateMismatch"],
        "Honest validators prevote nil",
    ),
    "DA attestation false but proposal contains block body/header": (
        ["TestProcessProposal_RejectsMissingDAAttestation"],
        "Reject",
    ),
    "BTC receipt rollback / forged checkpoint hash": (
        ["TestProcessProposal_RejectsForgedBTCHash"],
        "Reject",
    ),
    "Withdrawal tx during SOVEREIGN": (
        ["TestProcessProposal_RejectsWithdrawalWhileSovereign"],
        "Blocked",
    ),
    "Leader censorship of forced tx queue": (
        ["TestProcessProposal_RejectsCensoringProposal", "TestProcessProposal_AcceptsCensoredTxOnceIncluded"],
        "Rejected while omitted; accepted once included (M0d)",
    ),
    "Premature/fake ZK re-anchoring proof claim (bonus, not in original E8 list)": (
        ["TestProcessProposal_RejectsPrematureZKProofClaim"],
        "Reject",
    ),
    "Timeout flooding by Byzantine nodes": ([], "NOT COVERED -- needs live multi-node consensus (M0b)"),
    "Double-signing": ([], "NOT COVERED -- needs CometBFT evidence module on a live node (M7)"),
}


def run_go_tests():
    cmd = ["go", "test", "./x/sovereignty/...", "-run", "TestProcessProposal|TestCommitFSMTransition", "-v", "-json"]
    proc = subprocess.run(cmd, cwd=REPO_ROOT, capture_output=True, text=True)
    results = {}
    for line in proc.stdout.splitlines():
        try:
            rec = json.loads(line)
        except json.JSONDecodeError:
            continue
        test = rec.get("Test")
        action = rec.get("Action")
        if test and action in ("pass", "fail"):
            results[test] = {"action": action, "elapsed": rec.get("Elapsed", 0)}
    return results, proc.returncode


def build_table(results):
    lines = [
        "**Table 3 -- Attack-Resilience Test Suite (measured, real `go test` results):**",
        "",
        "| Attack | Covering test(s) | Result | Expected (docs/EXPERIMENT.md) |",
        "| --- | --- | --- | --- |",
    ]
    for attack, (tests, expected) in ATTACK_TO_TESTS.items():
        if not tests:
            lines.append(f"| {attack} | -- | not covered | {expected} |")
            continue
        outcomes = [results.get(t, {}).get("action", "MISSING") for t in tests]
        status = "PASS" if all(o == "pass" for o in outcomes) else f"FAIL ({outcomes})"
        lines.append(f"| {attack} | {', '.join(tests)} | {status} | {expected} |")
    return "\n".join(lines)


def main():
    results, returncode = run_go_tests()
    table = build_table(results)

    os.makedirs(OUT_DIR, exist_ok=True)
    with open(TABLE_OUT, "w") as f:
        f.write(table + "\n")

    print(table)
    print(f"\nTable written to {TABLE_OUT}")
    n_covered = sum(1 for tests, _ in ATTACK_TO_TESTS.values() if tests)
    n_total = len(ATTACK_TO_TESTS)
    print(f"{n_covered}/{n_total} attack rows covered by real in-process tests; go test exit code {returncode}")
    if returncode != 0:
        sys.exit(returncode)


if __name__ == "__main__":
    main()
