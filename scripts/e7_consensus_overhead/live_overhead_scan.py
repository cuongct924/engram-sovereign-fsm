#!/usr/bin/env python3
"""E7 LIVE (real docker testnet) -- distinct from measure_overhead.py, which
reads `go test ./tests/benchmark/...`'s synthetic V0-V5 payload benchmarks.
This script instead scans a real, already-committed range of blocks from the
live engram-node01 RPC and measures the REAL ExtendedProposal marker
overhead actually landing in Txs[0] on the running chain -- no synthetic
payload construction, no mocked ProcessProposal call.

Split into TWO regimes, not one blended average, per docs/EXPERIMENT.md's
E7 reconciliation -- a flat average both understates the common healthy
case's near-zero cost and hides the recovery path's real, bounded cost:

  - "steady-state tax": the ExtendedProposal marker (fsm_state/da_receipt/
    btc_receipt/zk_proof_ref) present on EVERY block regardless of health,
    bucketed by the real fsm_state decoded from that block's own marker
    JSON (Go's `encoding/json` marshals the marker body directly after the
    ENGRAM_EXTENDED_PROPOSAL_V1| prefix -- decoded here without needing a
    Python protobuf/Go-JSON codegen dependency).
  - "recovery-event cost": blocks whose OTHER (non-marker) tx content is
    unusually large -- a heuristic flag for a likely SubmitRecoveryProof
    message (the real ~14,656-byte UltraHonk proof, measured in E6's
    table6b_scaling.csv, travels as a SEPARATE tx, not inside
    ExtendedProposal itself -- see x/sovereignty/proposal.go's ZKProofRef
    doc). This is a byte-size heuristic, not real protobuf type decoding
    (no Python codegen available) -- flagged explicitly as such in the
    output, not presented as exact classification.

Usage:
    python3 scripts/e7_consensus_overhead/live_overhead_scan.py [--last-n 50]
"""

import argparse
import base64
import json
import os
import sys
import urllib.request

MARKER = b"ENGRAM_EXTENDED_PROPOSAL_V1|"
RESULTS_DIR = os.path.join(os.path.dirname(__file__), "results_live")
# A real SubmitRecoveryProof tx carries ~14,656 real proof bytes (E6's real
# measurement) plus public inputs/envelope overhead -- comfortably clear of
# any ordinary tx this app produces today (no bank/staking msgs exist, see
# app/app.go's TODO), so a large gap above "normal" other_tx_bytes is a
# reasonable heuristic trigger.
RECOVERY_PROOF_SIZE_HEURISTIC_BYTES = 5000


def rpc_get(port, path):
    with urllib.request.urlopen(f"http://localhost:{port}{path}", timeout=5) as resp:
        return json.loads(resp.read())


def block_txs(port, height):
    d = rpc_get(port, f"/block?height={height}")
    return d["result"]["block"]["data"]["txs"], d["result"]["block"]["header"]["time"]


def decode_extended_proposal_json(raw0: bytes):
    """Decodes the marker body as JSON -- ExtendedProposal's Go struct tags
    (fsm_state/da_receipt/btc_receipt/zk_proof_ref) map directly onto JSON
    keys, so no separate schema definition is needed here. Returns None on
    any decode failure (best-effort observability, not consensus-critical).
    """
    body = raw0[len(MARKER) :]
    try:
        return json.loads(body)
    except (ValueError, UnicodeDecodeError):
        return None


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--port", type=int, default=26657)
    parser.add_argument("--last-n", type=int, default=50)
    args = parser.parse_args()

    status = rpc_get(args.port, "/status")
    tip = int(status["result"]["sync_info"]["latest_block_height"])
    start = max(1, tip - args.last_n + 1)

    rows = []
    for h in range(start, tip + 1):
        try:
            txs, block_time = block_txs(args.port, h)
        except Exception as e:
            print(f"height {h}: skip ({e})", file=sys.stderr)
            continue
        marker_bytes = 0
        marker_present = False
        fsm_state = ""
        zk_proof_ref_present = False
        other_tx_bytes = (
            sum(len(base64.b64decode(t)) for t in txs[1:]) if len(txs) > 1 else 0
        )
        if txs:
            raw0 = base64.b64decode(txs[0])
            if raw0.startswith(MARKER):
                marker_present = True
                marker_bytes = len(raw0)
                ext = decode_extended_proposal_json(raw0)
                if ext:
                    fsm_state = ext.get("fsm_state", "")
                    zk_proof_ref_present = bool(ext.get("zk_proof_ref"))
        likely_recovery_proof_tx = other_tx_bytes >= RECOVERY_PROOF_SIZE_HEURISTIC_BYTES
        rows.append(
            {
                "height": h,
                "block_time": block_time,
                "num_txs": len(txs),
                "marker_present": marker_present,
                "extended_proposal_bytes": marker_bytes,
                "fsm_state": fsm_state,
                "zk_proof_ref_present": zk_proof_ref_present,
                "other_tx_bytes": other_tx_bytes,
                "likely_recovery_proof_tx": likely_recovery_proof_tx,
            }
        )

    os.makedirs(RESULTS_DIR, exist_ok=True)
    csv_path = os.path.join(RESULTS_DIR, "table4_live_overhead.csv")
    import csv

    with open(csv_path, "w", newline="") as f:
        w = csv.DictWriter(f, fieldnames=list(rows[0].keys()) if rows else [])
        w.writeheader()
        for r in rows:
            w.writerow(r)

    present = [r for r in rows if r["marker_present"]]
    coverage = len(present) / len(rows) * 100 if rows else 0

    # Steady-state tax: bucketed by real fsm_state.
    by_state: dict = {}
    for r in present:
        st = r["fsm_state"] or "(unknown)"
        by_state.setdefault(st, []).append(r["extended_proposal_bytes"])

    recovery_blocks = [r for r in rows if r["likely_recovery_proof_tx"]]

    md_path = os.path.join(RESULTS_DIR, "table4_live_overhead.md")
    with open(md_path, "w") as f:
        f.write(
            "# E7 LIVE -- Consensus Overhead, split by regime (real 4-node testnet)\n\n"
        )
        f.write(
            f"Scanned real heights {start}..{tip} on engram-node01 (port {args.port}).\n\n"
        )
        f.write(f"- Blocks scanned: {len(rows)}\n")
        f.write(
            f"- Blocks with real `ENGRAM_EXTENDED_PROPOSAL_V1` marker in Txs[0]: {len(present)} ({coverage:.1f}%)\n\n"
        )

        f.write(
            "## Steady-state tax (per-block, paid always, bucketed by real fsm_state)\n\n"
        )
        if by_state:
            f.write(
                "| fsm_state | Blocks | Avg ExtendedProposal bytes |\n|---|---:|---:|\n"
            )
            for st, sizes in sorted(by_state.items()):
                avg = sum(sizes) / len(sizes)
                f.write(f"| {st} | {len(sizes)} | {avg:.1f} |\n")
        else:
            f.write("(no blocks with a decodable marker in this scan window)\n")
        f.write("\n")

        f.write(
            "## Recovery-event cost (heuristic: blocks with unusually large non-marker tx content)\n\n"
        )
        f.write(
            f"Heuristic threshold: other_tx_bytes >= {RECOVERY_PROOF_SIZE_HEURISTIC_BYTES} bytes "
            f"(real UltraHonk proof measured at ~14,656 B in E6's table6b_scaling.csv; this is a "
            f"byte-size heuristic, NOT real protobuf type decoding -- no Python codegen dependency "
            f"available, flagged as such rather than presented as exact classification).\n\n"
        )
        if recovery_blocks:
            f.write(
                "| Height | fsm_state | other_tx_bytes | zk_proof_ref_present |\n|---:|---|---:|---|\n"
            )
            for r in recovery_blocks:
                f.write(
                    f"| {r['height']} | {r['fsm_state']} | {r['other_tx_bytes']} | "
                    f"{r['zk_proof_ref_present']} |\n"
                )
        else:
            f.write(
                f"No blocks matched the heuristic in this scan window (heights {start}..{tip}) -- "
                f"this scan likely never passed through a real RECOVERING proof-submission cycle. "
                f"Re-run after driving the cluster through one (e.g. scripts/e3_failure_matrix/"
                f"live_lifecycle_test.py's phase 7, or scripts/e2_fault_injection/"
                f"live_scenario_matrix.py's S7 phase) to capture a real sample.\n"
            )
        f.write("\n")

        f.write("## Full per-block detail\n\n")
        f.write(
            "| Height | Txs | fsm_state | ExtendedProposal bytes | Other tx bytes | Likely recovery-proof tx |\n"
        )
        f.write("|---:|---:|---|---:|---:|---|\n")
        for r in rows:
            f.write(
                f"| {r['height']} | {r['num_txs']} | {r['fsm_state']} | {r['extended_proposal_bytes']} | "
                f"{r['other_tx_bytes']} | {'yes' if r['likely_recovery_proof_tx'] else 'no'} |\n"
            )

    print(f"scanned {len(rows)} real blocks ({start}..{tip})")
    print(f"marker coverage: {coverage:.1f}%")
    print(f"steady-state buckets: {[(st, len(v)) for st, v in by_state.items()]}")
    print(f"recovery-event candidate blocks: {len(recovery_blocks)}")
    print(f"wrote {csv_path}\nwrote {md_path}")


if __name__ == "__main__":
    main()
