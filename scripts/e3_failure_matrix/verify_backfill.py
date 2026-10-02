#!/usr/bin/env python3
"""Independent check that a SOVEREIGN interval's block data is retrievable from Celestia.

The DA backfill gate (RECOVERING -> ANCHORED) rests on a quorum-reported contiguous published
height. This re-fetches the data itself: it scans a range of Celestia heights through a bridge's
blob.GetAll, parses the publisher's batches (repeated [height(8) | len(4) | data], data = the
block's txs framed len(4) || raw), and compares every Engram height of the interval with the
transactions of the real Engram block from CometBFT's /block. Stdlib only.

Usage:
    python3 verify_backfill.py --engram-from 1500 --engram-to 1600 --celestia-from 120 \
        [--bridge-url http://localhost:26658] [--engram-rpc http://localhost:26657] [--out backfill.csv]
The bridge token is read from the environment or .env (CELESTIA_BRIDGE_NODE01_AUTH_TOKEN).
"""

import argparse
import base64
import csv
import json
import os
import struct
import sys
import time
import urllib.error
import urllib.request

REPO_ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
MARKER_TAG = b"ENGRAMDA"


def namespace_bytes(ns_id: str) -> bytes:
    """x/da's NewNamespace: version byte 0, the id right-aligned in 29 bytes."""
    ns = bytearray(29)
    ns[29 - len(ns_id):] = ns_id.encode()
    return bytes(ns)


def load_token(var: str) -> str:
    if os.environ.get(var):
        return os.environ[var]
    with open(os.path.join(REPO_ROOT, ".env")) as f:
        for line in f:
            if line.startswith(var + "="):
                return line.split("=", 1)[1].strip()
    sys.exit(f"{var} not set in the environment or .env")


def rpc(url: str, token: str, method: str, params: list, timeout: float = 30.0):
    body = json.dumps({"jsonrpc": "2.0", "id": 1, "method": method, "params": params}).encode()
    req = urllib.request.Request(url, data=body, headers={"Authorization": f"Bearer {token}", "Content-Type": "application/json"})
    out = None
    for attempt in range(4):  # a bridge under load drops connections now and then
        try:
            with urllib.request.urlopen(req, timeout=timeout) as resp:
                out = json.loads(resp.read())
            break
        except (urllib.error.URLError, ConnectionError, TimeoutError, OSError):
            if attempt == 3:
                raise
            time.sleep(5 * (attempt + 1))
    if out.get("error"):
        raise RuntimeError(out["error"].get("message", out["error"]))
    return out.get("result")


def parse_batch(data: bytes):
    """Yields (engram_height, block_data) from one batch blob; markers yield nothing."""
    if data.startswith(MARKER_TAG) and len(data) == len(MARKER_TAG) + 8:
        return
    i = 0
    while i + 12 <= len(data):
        height, length = struct.unpack(">QI", data[i:i + 12])
        i += 12
        if i + length > len(data):
            return
        yield height, data[i:i + length]
        i += length


def collect_published(url, token, ns_b64, c_from, c_to):
    """engram height -> set of distinct payloads seen on Celestia in [c_from, c_to]."""
    seen = {}
    for ch in range(c_from, c_to + 1):
        try:
            blobs = rpc(url, token, "blob.GetAll", [ch, [ns_b64]])
        except RuntimeError as e:
            if "not found" in str(e):
                continue
            raise
        for blob in blobs or []:
            for h, payload in parse_batch(base64.b64decode(blob["data"])):
                seen.setdefault(h, set()).add(payload)
    return seen


def engram_block_data(engram_rpc: str, height: int) -> bytes:
    with urllib.request.urlopen(f"{engram_rpc}/block?height={height}", timeout=15) as resp:
        txs = json.loads(resp.read())["result"]["block"]["data"]["txs"] or []
    out = b""
    for t in txs:
        raw = base64.b64decode(t)
        out += struct.pack(">I", len(raw)) + raw
    return out


def verify(args) -> dict:
    token = load_token(args.token_var)
    ns_b64 = base64.b64encode(namespace_bytes(args.namespace)).decode()
    c_to = args.celestia_to or int(rpc(args.bridge_url, token, "header.LocalHead", [])["header"]["height"])
    published = collect_published(args.bridge_url, token, ns_b64, args.celestia_from, c_to)

    rows, matched, found = [], 0, 0
    for h in range(args.engram_from, args.engram_to + 1):
        want = engram_block_data(args.engram_rpc, h)
        copies = published.get(h, set())
        is_found = bool(copies)
        is_match = is_found and all(c == want for c in copies)
        found += is_found
        matched += is_match
        rows.append({"engram_height": h, "published": is_found, "match": is_match, "distinct_payloads": len(copies)})
    total = args.engram_to - args.engram_from + 1
    summary = {"engram_from": args.engram_from, "engram_to": args.engram_to, "celestia_from": args.celestia_from,
               "celestia_to": c_to, "heights": total, "published": found, "matched": matched,
               "published_pct": round(100 * found / total, 2), "match_pct": round(100 * matched / total, 2)}
    if args.out:
        with open(args.out, "w", newline="") as f:
            w = csv.DictWriter(f, fieldnames=list(rows[0]) if rows else ["engram_height"])
            w.writeheader()
            w.writerows(rows)
    return summary


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--engram-from", type=int, required=True)
    ap.add_argument("--engram-to", type=int, required=True)
    ap.add_argument("--celestia-from", type=int, required=True)
    ap.add_argument("--celestia-to", type=int, default=0, help="default: the bridge's local head")
    ap.add_argument("--bridge-url", default="http://localhost:26658")
    ap.add_argument("--engram-rpc", default="http://localhost:26657")
    ap.add_argument("--namespace", default="engramda01")
    ap.add_argument("--token-var", default="CELESTIA_BRIDGE_NODE01_AUTH_TOKEN")
    ap.add_argument("--out", default="")
    args = ap.parse_args()
    s = verify(args)
    print(json.dumps(s))
    sys.exit(0 if s["matched"] == s["heights"] else 1)


if __name__ == "__main__":
    main()
