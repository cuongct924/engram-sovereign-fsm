#!/usr/bin/env python3
"""Writes TLC configs for the consensus-core refinement run at another bound, plus its reachability (vacuity) checks.

    python3 spec/tools/make_c1_cfgs.py --tag small1 --set MAX_TIMESTAMP=4
    python3 spec/tools/make_c1_cfgs.py --tag small2 --set MAX_TIMESTAMP=4 MAX_BTC_HEIGHT=1

Outputs in spec/core: MC_ConsensusCoreRefinementSafety_<tag>.cfg (the safety run) and
MC_ConsensusCoreVacuity_<tag>_<predicate>.cfg for each NotReached_* predicate of MC_ConsensusCoreVacuity.tla.
Run the vacuity configs first: each must end with a violation, otherwise the bound is too small for the safety run to mean anything.
"""

import argparse
import os
import re

CORE = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "core")
BASE = "MC_ConsensusCoreRefinementSafety.cfg"
PREDICATES = ["NotReached_RoundChange", "NotReached_ByzantineProposal", "NotReached_Decision"]


def override(text, assignments):
    for kv in assignments:
        key, val = kv.split("=")
        text, n = re.subn(rf"(?m)^(\s*){key}\s*=.*$", rf"\g<1>{key} = {val}", text)
        assert n == 1, f"{key} not found exactly once in {BASE}"
    return text


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--tag", required=True)
    ap.add_argument("--set", nargs="+", default=[], metavar="NAME=VALUE")
    args = ap.parse_args()
    base = open(os.path.join(CORE, BASE)).read()
    text = override(base, args.set)
    with open(os.path.join(CORE, f"MC_ConsensusCoreRefinementSafety_{args.tag}.cfg"), "w") as f:
        f.write(text)
    inv_block = re.compile(r"(?ms)^INVARIANTS\n.*?(?=^\S)")
    prop_block = re.compile(r"(?ms)^PROPERTIES\n.*?(?=^\S)")
    assert inv_block.search(text) and prop_block.search(text), "INVARIANTS/PROPERTIES block not found"
    for pred in PREDICATES:
        v = prop_block.sub("", inv_block.sub(f"INVARIANT {pred}\n\n", text, count=1), count=1)
        name = f"MC_ConsensusCoreVacuity_{args.tag}_{pred.replace('NotReached_', '')}.cfg"
        with open(os.path.join(CORE, name), "w") as f:
            f.write(v)
        print("wrote", name)
    print(f"wrote MC_ConsensusCoreRefinementSafety_{args.tag}.cfg")


if __name__ == "__main__":
    main()
