#!/usr/bin/env python3
# Builds Prover.toml for the aggregator from real leaf proof dirs (each holding
# `proof` and `public_inputs` from `bb prove -t noir-recursive`). Padding slots
# repeat the last real leaf: verify_honk_proof is unconditional (see main.nr).
# Usage: gen_prover_toml.py K leaf_dir [leaf_dir ...] > Prover.toml
# AGG_OVERRIDE="count=999,n_real=3" replaces top-level keys -- used by negative_tests.sh to
# build witnesses a sound aggregator must reject.
import os
import sys

K = int(sys.argv[1])
dirs = sys.argv[2:]
assert 1 <= len(dirs) <= K

def fields(p):
    b = open(p, "rb").read()
    assert len(b) % 32 == 0
    return [int.from_bytes(b[i:i + 32], "big") for i in range(0, len(b), 32)]

slots = dirs + [dirs[-1]] * (K - len(dirs))
proofs = [fields(f"{d}/proof") for d in slots]
pis = [fields(f"{d}/public_inputs") for d in slots]
assert all(len(p) == 7 for p in pis)

real = pis[: len(dirs)]
q = lambda xs: "[" + ", ".join(f'"{x}"' for x in xs) + "]"
top = {
    "rt_last": real[0][0],
    "rt_new": real[-1][1],
    "count": sum(p[2] for p in real),
    "first_height": real[0][3],
    "first_block_hash": real[0][4],
    "last_block_hash": real[-1][5],
    "validator_set_hash": real[0][6],
    "n_real": len(dirs),
}
for kv in filter(None, os.environ.get("AGG_OVERRIDE", "").split(",")):
    k, v = kv.split("=")
    assert k in top, k
    top[k] = v
for k, v in top.items():
    print(f'{k} = "{v}"')
print("leaf_proofs = [" + ", ".join(q(p) for p in proofs) + "]")
print("leaf_pi = [" + ", ".join(q(p) for p in pis) + "]")
