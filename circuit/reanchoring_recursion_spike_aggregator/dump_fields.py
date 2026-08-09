#!/usr/bin/env python3
# One-off helper for this feasibility spike: dumps a raw bb proof/vk/public_inputs
# binary file as a TOML array of decimal-string Field values (32-byte
# big-endian chunks), for pasting into Prover.toml. Not part of the real
# production pipeline (scripts/reanchoring_prover already does the equivalent
# for the deployed circuit's own public_inputs via cmd/engramd's big.Int
# decimal decoding, see x/sovereignty/types/recovery_header.go's ReduceToField).
import sys

path = sys.argv[1]
data = open(path, "rb").read()
assert len(data) % 32 == 0, f"{path}: {len(data)} bytes not a multiple of 32"
fields = [str(int.from_bytes(data[i : i + 32], "big")) for i in range(0, len(data), 32)]
print(f"# {path}: {len(fields)} fields")
print("[" + ", ".join(f'"{f}"' for f in fields) + "]")
