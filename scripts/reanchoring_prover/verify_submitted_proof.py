#!/usr/bin/env python3
"""Verify a recovery proof that is already on chain, from outside the node, the way a third party would.

The chain keeps only the checkpoint (root, height) and an audit pointer; the proof and its public inputs travel in the
MsgSubmitRecoveryProofRequest of a committed block. This reads such a block over RPC, extracts `zk_proof` and
`public_inputs`, and runs `bb verify` against the verification key shipped in x/sovereignty/keeper/zk_assets
(vk for a plain proof of up to 256 headers, vk_aggregate for an aggregate). Nothing here uses the node's own verifier
or the prover's files.

    python3 scripts/reanchoring_prover/verify_submitted_proof.py --height 823 [--node http://127.0.0.1:26657]
    python3 scripts/reanchoring_prover/verify_submitted_proof.py --scan 400   # last 400 blocks, every proof found

Requires `bb` on PATH (or ~/.bb/bb). Exit 0 when every proof found verifies.
"""

import argparse
import base64
import json
import os
import shutil
import subprocess
import sys
import tempfile
import urllib.request

REPO = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
ASSETS = os.path.join(REPO, "x", "sovereignty", "keeper", "zk_assets")
MSG_SUFFIX = "MsgSubmitRecoveryProofRequest"


def rpc(node, path):
    with urllib.request.urlopen(f"{node}{path}", timeout=15) as r:
        return json.load(r)["result"]


def varint(b, i):
    n = shift = 0
    while True:
        c = b[i]
        i += 1
        n |= (c & 0x7F) << shift
        if not c & 0x80:
            return n, i
        shift += 7


def fields(b):
    """Yield (field number, wire type, value) of a protobuf message; value is bytes for wire type 2."""
    i = 0
    while i < len(b):
        key, i = varint(b, i)
        num, wt = key >> 3, key & 7
        if wt == 0:
            v, i = varint(b, i)
        elif wt == 2:
            n, i = varint(b, i)
            v, i = b[i:i + n], i + n
        elif wt == 1:
            v, i = b[i:i + 8], i + 8
        elif wt == 5:
            v, i = b[i:i + 4], i + 4
        else:
            raise ValueError(f"wire type {wt}")
        yield num, wt, v


def proofs_in_tx(raw):
    """(zk_proof, public_inputs) of every recovery-proof message in a Cosmos Tx."""
    out = []
    for num, wt, body in fields(raw):
        if num != 1 or wt != 2:  # Tx.body
            continue
        for mnum, mwt, any_msg in fields(body):
            if mnum != 1 or mwt != 2:  # TxBody.messages
                continue
            url = value = b""
            for anum, awt, v in fields(any_msg):
                if awt != 2:
                    continue
                if anum == 1:
                    url = v
                elif anum == 2:
                    value = v
            if url.decode(errors="ignore").endswith(MSG_SUFFIX):
                f = {n: v for n, w, v in fields(value) if w == 2}
                out.append((f.get(2, b""), f.get(3, b"")))
    return out


def bb_path():
    return shutil.which("bb") or os.path.expanduser("~/.bb/bb")


def verify(proof, public_inputs):
    # A plain proof covers at most 256 headers and is checked against the leaf vk; the aggregate vk is tried second.
    for name in ("vk", "vk_aggregate"):
        with tempfile.TemporaryDirectory() as d:
            for fn, data in (("proof", proof), ("public_inputs", public_inputs)):
                with open(os.path.join(d, fn), "wb") as f:
                    f.write(data)
            r = subprocess.run([bb_path(), "verify", "-k", os.path.join(ASSETS, name), "-p", os.path.join(d, "proof"),
                                "-i", os.path.join(d, "public_inputs")], capture_output=True, text=True)
            if r.returncode == 0:
                return name
    return None


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--node", default="http://127.0.0.1:26657")
    ap.add_argument("--height", type=int)
    ap.add_argument("--scan", type=int, help="look at the last N blocks")
    args = ap.parse_args()
    if not args.height and not args.scan:
        ap.error("give --height or --scan")
    tip = int(rpc(args.node, "/status")["sync_info"]["latest_block_height"])
    heights = [args.height] if args.height else range(max(1, tip - args.scan + 1), tip + 1)
    found = ok = 0
    for h in heights:
        for tx in rpc(args.node, f"/block?height={h}")["block"]["data"]["txs"] or []:
            try:
                msgs = proofs_in_tx(base64.b64decode(tx))
            except (ValueError, IndexError, TypeError):
                continue  # not a protobuf Tx, e.g. the ENGRAM_AGGREGATED_HEALTH_CERT_V1 blob at Txs[0]
            for proof, pub in msgs:
                found += 1
                vk = verify(proof, pub)
                ok += vk is not None
                print(f"height {h}: proof {len(proof)} B, public inputs {len(pub)} B -> "
                      f"{'VERIFIED with ' + vk if vk else 'NOT verified'}")
    print(f"{ok}/{found} proofs verified")
    return 0 if found and ok == found else 1


if __name__ == "__main__":
    sys.exit(main())
