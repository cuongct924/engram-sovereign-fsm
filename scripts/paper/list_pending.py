#!/usr/bin/env python3
"""List every \\pending{...} of the manuscript with its file and line, so that a result that has arrived is easy to place.

Usage: scripts/paper/list_pending.py [--paper docs/paper]
"""
import argparse
import pathlib
import re

ap = argparse.ArgumentParser()
ap.add_argument("--paper", default="docs/paper")
root = pathlib.Path(ap.parse_args().paper)
pat = re.compile(r"\\pending\{")
n = 0
for f in sorted(root.rglob("*.tex")):
    text = f.read_text()
    for m in pat.finditer(text):
        depth, i = 1, m.end()
        while i < len(text) and depth:
            depth += {"{": 1, "}": -1}.get(text[i], 0)
            i += 1
        body = " ".join(text[m.end():i - 1].split())
        n += 1
        print(f"{n:2d}. {f.relative_to(root)}:{text.count(chr(10), 0, m.start()) + 1}: {body[:150]}")
print(f"{n} pending")
