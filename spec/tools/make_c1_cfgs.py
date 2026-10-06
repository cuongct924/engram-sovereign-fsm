#!/usr/bin/env python3
"""Writes TLC configs for the C1 refinement runs at other bounds, plus their reachability (vacuity) checks.

Consensus-core run (environment frozen; FSM transitions are NOT a behaviour of it):
    python3 spec/tools/make_c1_cfgs.py --tag small1 --set MAX_TIMESTAMP=4
    python3 spec/tools/make_c1_cfgs.py --tag small2 --set MAX_TIMESTAMP=4 MAX_BTC_HEIGHT=1

Monolithic run (MC_ServerRefinementSafety: every action, so FSM transitions are part of it):
    python3 spec/tools/make_c1_cfgs.py --layer server --tag L0 --set MAX_ROUND=1 MAX_BTC_HEIGHT=1 MAX_ENGRAM_HEIGHT=1 MAX_TIMESTAMP=3
    python3 spec/tools/make_c1_cfgs.py --layer server --ladder      # writes the rungs L0..L3 in one go

Outputs in spec/core (or --outdir): the safety cfg and one vacuity cfg per NotReached_* predicate
(MC_ConsensusCoreVacuity.tla for --layer core, MC_ServerRefinementVacuity.tla for --layer server).
Run the vacuity configs first: each must end with a violation, otherwise the bound is too small for the safety run to mean anything.
The server safety cfg is written without its Sanity_* invariants: they are EXPECTED to be violated and would stop the run.
"""

import argparse
import os
import re

CORE = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "core")

LAYERS = {
    "core": {
        "base": "MC_ConsensusCoreRefinementSafety.cfg",
        "safety": "MC_ConsensusCoreRefinementSafety_{tag}.cfg",
        "vacuity": "MC_ConsensusCoreVacuity_{tag}_{pred}.cfg",
        "predicates": ["NotReached_RoundChange", "NotReached_ByzantineProposal", "NotReached_Decision"],
        "strip_sanity": False,
    },
    "server": {
        "base": "MC_ServerRefinementSafety.cfg",
        "safety": "MC_ServerRefinementSafety_{tag}.cfg",
        "vacuity": "MC_ServerRefinementVacuity_{tag}_{pred}.cfg",
        "predicates": ["NotReached_RoundChange", "NotReached_ByzantineProposal", "NotReached_Decision",
                       "NotReached_FSMTransition", "NotReached_DecisionAfterFSMTransition"],
        "strip_sanity": True,
    },
}

# Cumulative rungs of the bound ladder of docs/C1_GAP_PLAN.md (Track 2): each rung changes one more bound.
LADDER = [
    ("L0", ["MAX_ROUND=1", "MAX_BTC_HEIGHT=1", "MAX_ENGRAM_HEIGHT=1", "MAX_TIMESTAMP=3"]),
    ("L1", ["MAX_ROUND=1", "MAX_BTC_HEIGHT=1", "MAX_ENGRAM_HEIGHT=2", "MAX_TIMESTAMP=3"]),
    ("L2", ["MAX_ROUND=1", "MAX_BTC_HEIGHT=1", "MAX_ENGRAM_HEIGHT=2", "MAX_TIMESTAMP=4"]),
    ("L3", ["MAX_ROUND=2", "MAX_BTC_HEIGHT=1", "MAX_ENGRAM_HEIGHT=2", "MAX_TIMESTAMP=4"]),
]


def override(text, assignments, base):
    for kv in assignments:
        key, val = kv.split("=")
        text, n = re.subn(rf"(?m)^(\s*){key}\s*=.*$", rf"\g<1>{key} = {val}", text)
        assert n == 1, f"{key} not found exactly once in {base}"
    return text


def strip_sanity(text):
    """Drops the active Sanity_* invariants (expected to fail); commented ones are untouched."""
    return re.sub(r"(?m)^[ \t]+Sanity_\w+[ \t]*\n", "", text)


def write_set(layer, tag, assignments, outdir):
    spec = LAYERS[layer]
    base = open(os.path.join(CORE, spec["base"])).read()
    text = override(base, assignments, spec["base"])
    if spec["strip_sanity"]:
        text = strip_sanity(text)
    inv_block = re.compile(r"(?ms)^INVARIANTS\n.*?(?=^\S)")
    prop_block = re.compile(r"(?ms)^PROPERTIES\n.*?(?=^\S)")
    assert inv_block.search(text) and prop_block.search(text), "INVARIANTS/PROPERTIES block not found"
    name = spec["safety"].format(tag=tag)
    with open(os.path.join(outdir, name), "w") as f:
        f.write(text)
    print("wrote", name)
    for pred in spec["predicates"]:
        v = prop_block.sub("", inv_block.sub(f"INVARIANT {pred}\n\n", text, count=1), count=1)
        name = spec["vacuity"].format(tag=tag, pred=pred.replace("NotReached_", ""))
        with open(os.path.join(outdir, name), "w") as f:
            f.write(v)
        print("wrote", name)


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--layer", choices=sorted(LAYERS), default="core")
    ap.add_argument("--tag")
    ap.add_argument("--set", nargs="+", default=[], metavar="NAME=VALUE")
    ap.add_argument("--ladder", action="store_true", help="write the rungs L0..L3 (server layer only)")
    ap.add_argument("--outdir", default=CORE)
    args = ap.parse_args()
    os.makedirs(args.outdir, exist_ok=True)
    if args.ladder:
        assert args.layer == "server", "--ladder is defined for --layer server"
        for tag, assignments in LADDER:
            write_set("server", tag, assignments, args.outdir)
    else:
        assert args.tag, "--tag is required without --ladder"
        write_set(args.layer, args.tag, args.set, args.outdir)


if __name__ == "__main__":
    main()
