#!/usr/bin/env python3
"""Byte accounting of the shipped Txs[0] (ENGRAM_AGGREGATED_HEALTH_CERT_V1) on a live chain, read-only over RPC.

AggregatedHealthCertificate (proto/engram/sovereignty/v1/vote_extension.proto) carries one SignedHealthCertificate per
validator in the quorum (address, round, the raw vote-extension bytes, the extension signature) plus the leader's claimed
aggregation. The certificate is a copy of the vote extensions, because ProcessProposal receives none under ABCI 2.0; it is
separate from the block's last_commit. This script splits the bytes by field with a wire-level protobuf reader (no codegen),
so the per-validator cost b and the fixed part a of size(n) = a + n*b come from blocks, not from a fixture.

    python3 scripts/e7_consensus_overhead/certificate_size_scan.py [--last-n 200] [--port 26657]
"""

import argparse
import base64
import collections
import json
import os
import statistics as st
import urllib.request

MARKER = b"ENGRAM_AGGREGATED_HEALTH_CERT_V1|"
OUT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "results_live")


def rpc(port, path):
    with urllib.request.urlopen(f"http://localhost:{port}{path}", timeout=10) as r:
        return json.load(r)["result"]


def varint(b, i):
    v = s = 0
    while True:
        c = b[i]
        i += 1
        v |= (c & 0x7F) << s
        if not c & 0x80:
            return v, i
        s += 7


def fields(b):
    """(field number, wire type, value or bytes, encoded length including tag and length prefix)."""
    i = 0
    while i < len(b):
        start = i
        tag, i = varint(b, i)
        f, w = tag >> 3, tag & 7
        if w == 0:
            v, i = varint(b, i)
        elif w == 2:
            n, i = varint(b, i)
            v, i = b[i : i + n], i + n
        elif w == 1:
            v, i = b[i : i + 8], i + 8
        elif w == 5:
            v, i = b[i : i + 4], i + 4
        else:
            raise ValueError(f"wire type {w}")
        yield f, w, v, i - start


def split(raw):
    agg = raw[len(MARKER):]
    row = {"total": len(raw), "marker": len(MARKER), "n_certs": 0, "cert_total": 0, "addr": 0, "round": 0, "ext": 0, "sig": 0,
           "other": 0, "state": ""}
    for f, w, v, ln in fields(agg):
        if f == 2:
            row["n_certs"] += 1
            row["cert_total"] += ln
            for g, _, cv, cl in fields(v):
                key = {1: "addr", 2: "round", 3: "ext", 4: "sig"}.get(g, "other")
                row[key] += cl
        else:
            row["other"] += ln
            if f == 3:
                row["state"] = v.decode()
    return row


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--port", type=int, default=26657)
    ap.add_argument("--last-n", type=int, default=200)
    a = ap.parse_args()
    tip = int(rpc(a.port, "/status")["sync_info"]["latest_block_height"])
    rows = []
    for h in range(max(2, tip - a.last_n + 1), tip + 1):
        blk = rpc(a.port, f"/block?height={h}")["block"]
        txs = blk["data"]["txs"]
        if not txs:
            continue
        raw = base64.b64decode(txs[0])
        if raw.startswith(MARKER):
            r = split(raw)
            r["height"] = h
            r["commit_sigs"] = len(blk["last_commit"]["signatures"])
            rows.append(r)
    if not rows:
        raise SystemExit("no certificate blocks found")
    by = collections.defaultdict(list)
    for r in rows:
        by[(r["state"], r["n_certs"])].append(r)
    lines = [f"# Txs[0] byte accounting, last {a.last_n} blocks up to {tip} ({len(rows)} with a certificate)", "",
             "| FSM state | certificates in quorum | blocks | Txs[0] bytes (mean) | per validator (mean) | fixed part (mean) | address | round | extension | signature |",
             "|---|---|---|---|---|---|---|---|---|---|"]
    for (state, n), rs in sorted(by.items()):
        m = lambda k: st.mean(r[k] for r in rs)
        per = m("cert_total") / n
        lines.append(f"| {state} | {n} | {len(rs)} | {m('total'):.0f} | {per:.1f} | {m('total') - m('cert_total'):.0f} | "
                     f"{m('addr') / n:.1f} | {m('round') / n:.1f} | {m('ext') / n:.1f} | {m('sig') / n:.1f} |")
    lines += ["", "Each validator's entry is address + round + extension + signature (bytes per validator, including field tags and length prefixes).",
              "The fixed part is the marker plus the leader's claimed aggregation (state, receipts, flags). The block's last_commit is separate."]
    os.makedirs(OUT, exist_ok=True)
    with open(os.path.join(OUT, "certificate_size_scan.md"), "w") as f:
        f.write("\n".join(lines) + "\n")
    print("\n".join(lines))


if __name__ == "__main__":
    main()
