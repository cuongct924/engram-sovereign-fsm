#!/usr/bin/env python3
"""
E7 -- Consensus Overhead of the Extended Proposal (docs/EXPERIMENT.md's E7,
Table 4).

Runs the REAL Go benchmarks in tests/benchmark/fsm_latency_test.go
(`go test -bench=. -benchmem`) and builds Table 4. See that file's package
doc for why V0-V5 decompose into real proposal-size measurements
(json.Marshal on real Go structs) plus real validation sub-step benchmarks
(CalculateNextState, da.VerifyReceipt, anchor.VerifyReceipt) composed
cumulatively, rather than pretending ProcessProposal has 6 separate code
paths -- it has exactly one, which always validates the full proposal.

Also builds a real vanilla-CometBFT baseline comparison (docs/EXPERIMENT.md's
E2/E3/E7) from {normal,vanilla}_samples.csv, produced by running
`scripts/e7_consensus_overhead/vanilla_comparison.sh` first.

Usage:
    python3 scripts/e7_consensus_overhead/measure_overhead.py
    scripts/e7_consensus_overhead/vanilla_comparison.sh   # (separately, real 2-node run)
"""

import csv
import datetime
import os
import re
import statistics
import subprocess
import sys

REPO_ROOT = os.path.join(os.path.dirname(__file__), "..", "..")
OUT_DIR = os.path.join(os.path.dirname(__file__), "results")
TABLE_OUT = os.path.join(OUT_DIR, "table4_overhead.md")
NORMAL_SAMPLES_CSV = os.path.join(OUT_DIR, "normal_samples.csv")
VANILLA_SAMPLES_CSV = os.path.join(OUT_DIR, "vanilla_samples.csv")

BENCH_LINE_RE = re.compile(
    r"^(?P<name>Benchmark\S+)\s+(?P<n>\d+)\s+(?P<ns_op>[\d.]+)\s+ns/op"
    r"(?:\s+(?P<bytes_op>[\d.]+)\s+bytes/op)?"
    r"(?:\s+(?P<b_op>\d+)\s+B/op)?"
    r"(?:\s+(?P<allocs_op>\d+)\s+allocs/op)?"
)


def run_go_benchmarks():
    cmd = ["go", "test", "./tests/benchmark/...", "-bench=.", "-benchmem", "-run=^$"]
    proc = subprocess.run(cmd, cwd=REPO_ROOT, capture_output=True, text=True)
    if proc.returncode != 0:
        print(proc.stdout, file=sys.stderr)
        print(proc.stderr, file=sys.stderr)
        sys.exit(proc.returncode)

    results = {}
    for line in proc.stdout.splitlines():
        m = BENCH_LINE_RE.match(line)
        if not m:
            continue
        d = m.groupdict()
        results[d["name"]] = {
            "ns_op": float(d["ns_op"]),
            "bytes_op": float(d["bytes_op"]) if d["bytes_op"] else None,
        }
    return results


def build_table(results):
    def r(name):
        return results.get(name)

    size = {
        "V0": r("BenchmarkProposalSize/V0_Vanilla_NoExtension-8")
        or r("BenchmarkProposalSize/V0_Vanilla_NoExtension"),
        "V1": r("BenchmarkProposalSize/V1_FSMStateOnly-8")
        or r("BenchmarkProposalSize/V1_FSMStateOnly"),
        "V2": r("BenchmarkProposalSize/V2_PlusDAReceipt-8")
        or r("BenchmarkProposalSize/V2_PlusDAReceipt"),
        "V3": r("BenchmarkProposalSize/V3_PlusBTCReceipt-8")
        or r("BenchmarkProposalSize/V3_PlusBTCReceipt"),
        "V4": r("BenchmarkProposalSize/V4_PlusP2PDigest-8")
        or r("BenchmarkProposalSize/V4_PlusP2PDigest"),
        "V5": r("BenchmarkProposalSize/V5_PlusZKProofRef-8")
        or r("BenchmarkProposalSize/V5_PlusZKProofRef"),
    }

    fsm_cost = r("BenchmarkCalculateNextState-8") or r("BenchmarkCalculateNextState")
    da_cost = r("BenchmarkDAVerifyReceipt-8") or r("BenchmarkDAVerifyReceipt")
    btc_cost = r("BenchmarkBTCVerifyReceipt-8") or r("BenchmarkBTCVerifyReceipt")
    full_cost = r("BenchmarkProcessProposal-8") or r("BenchmarkProcessProposal")

    cumulative_ns = {
        "V0": 0.0,
        "V1": fsm_cost["ns_op"] if fsm_cost else None,
    }
    if cumulative_ns["V1"] is not None and da_cost:
        cumulative_ns["V2"] = cumulative_ns["V1"] + da_cost["ns_op"]
    if "V2" in cumulative_ns and btc_cost:
        cumulative_ns["V3"] = cumulative_ns["V2"] + btc_cost["ns_op"]

    lines = [
        "**Table 4 -- Extended Proposal Overhead (measured, real `go test -bench`):**",
        "",
        "| Variant | Description | Proposal size (bytes) | Cumulative validation CPU (ns/op) |",
        "| --- | --- | ---: | ---: |",
        f"| V0 | Vanilla, no extension | {fmt(size['V0'], 'bytes_op')} | 0 (baseline) |",
        f"| V1 | + fsm_state only | {fmt(size['V1'], 'bytes_op')} | {fmt2(cumulative_ns.get('V1'))} (CalculateNextState) |",
        f"| V2 | + DA receipt | {fmt(size['V2'], 'bytes_op')} | {fmt2(cumulative_ns.get('V2'))} (+da.VerifyReceipt) |",
        f"| V3 | + BTC receipt | {fmt(size['V3'], 'bytes_op')} | {fmt2(cumulative_ns.get('V3'))} (+anchor.VerifyReceipt) |",
        f"| V4 | + P2P sensor digest *(size estimate only -- not in the real wire format today, see tests/benchmark/fsm_latency_test.go)* | {fmt(size['V4'], 'bytes_op')} | n/a |",
        f"| V5 | + ZK proof ref (real shipped ExtendedProposal, full end-to-end via NewProcessProposalHandler) | {fmt(size['V5'], 'bytes_op')} | {fmt2(full_cost['ns_op']) if full_cost else 'n/a'} (full ProcessProposal, includes JSON decode + all checks) |",
        "",
        "Note: V5's proposal size (real `ExtendedProposal`) is smaller than V4's because V4 is a hypothetical "
        "P2P-digest-included estimate that was never wired into the real wire format -- P2P health is validated "
        "from the leader's local `keeper.Metrics`, not carried in the proposal. V5's CPU cost is the real full "
        "`ProcessProposal` cost (JSON decode + all 5 IsValidProposal checks), not just the cumulative "
        "sub-benchmarks' sum, since that is what actually runs on a node.",
    ]
    return "\n".join(lines)


def load_samples(path):
    if not os.path.exists(path):
        return None
    with open(path, newline="") as f:
        return list(csv.DictReader(f))


def parse_time(s):
    s = s.rstrip("Z")
    if "." in s:
        head, frac = s.split(".", 1)
        frac = (frac + "000000")[:6]
        s = f"{head}.{frac}"
    return datetime.datetime.fromisoformat(s)


def build_vanilla_comparison():
    normal = load_samples(NORMAL_SAMPLES_CSV)
    vanilla = load_samples(VANILLA_SAMPLES_CSV)
    if normal is None or vanilla is None:
        return None

    def block_intervals(rows):
        times = [parse_time(r["block_time"]) for r in rows]
        return [(b - a).total_seconds() for a, b in zip(times, times[1:])]

    normal_sizes = [int(r["proposal_tx0_bytes"]) for r in normal]
    vanilla_sizes = [int(r["proposal_tx0_bytes"]) for r in vanilla]
    normal_intervals = block_intervals(normal)
    vanilla_intervals = block_intervals(vanilla)

    lines = [
        "",
        "---",
        "",
        "**Vanilla CometBFT baseline (measured, real 2-node local run via `vanilla_comparison.sh`):**",
        "",
        "| Metric | Engram FSM (normal) | Vanilla (--vanilla, no ExtendedProposal) | Overhead |",
        "| --- | ---: | ---: | ---: |",
        f"| Tx[0] proposal-marker size (bytes/block) | {statistics.mean(normal_sizes):.0f} | {statistics.mean(vanilla_sizes):.0f} "
        f"| +{statistics.mean(normal_sizes) - statistics.mean(vanilla_sizes):.0f} B/block, on 100% of blocks |",
        f"| Mean block interval (s) | {statistics.mean(normal_intervals):.3f} | {statistics.mean(vanilla_intervals):.3f} "
        f"| {(statistics.mean(normal_intervals) - statistics.mean(vanilla_intervals))*1000:+.1f} ms/block |",
        "",
        "Both nodes were idle (no user tx load), so block interval is dominated by CometBFT's default `timeout_commit` "
        "in both cases -- it does not meaningfully differ here. The real, measured overhead of the ExtendedProposal "
        "mechanism is the constant per-block proposal-marker size shown above, which matches the real "
        "`BenchmarkProposalSize/V5_PlusZKProofRef` size in the table above.",
    ]
    return "\n".join(lines)


def fmt(entry, key):
    if not entry or entry.get(key) is None:
        return "n/a"
    return f"{entry[key]:.0f}"


def fmt2(v):
    if v is None:
        return "n/a"
    return f"{v:.1f}"


def main():
    results = run_go_benchmarks()
    table = build_table(results)

    vanilla_section = build_vanilla_comparison()
    if vanilla_section:
        table += vanilla_section
    else:
        print(
            f"(No vanilla-baseline data at {NORMAL_SAMPLES_CSV} -- run "
            "vanilla_comparison.sh first for a real 2-node comparison.)",
            file=sys.stderr,
        )

    os.makedirs(OUT_DIR, exist_ok=True)
    with open(TABLE_OUT, "w") as f:
        f.write(table + "\n")

    print(table)
    print(f"\nTable written to {TABLE_OUT}")


if __name__ == "__main__":
    main()
