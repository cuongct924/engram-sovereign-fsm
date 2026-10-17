#!/usr/bin/env python3
"""Full recovery SOVEREIGN -> RECOVERING -> ANCHORED on the multi-host cluster (Testbed B), proof-only, with chain-level negative tests.

One cycle per wanted boundary round (1, 2):
  1. prover control (container reanchoring-prover): /control/want_round = N, /control/hold exists
  2. fault: docker pause bitcoin-node01 + all Celestia bridges stopped, until all seven validators are SOVEREIGN
  3. heal: unpause + bridges started, until all seven are RECOVERING
  4. two validators adjacent in the proposer rotation are stopped (the other five are exactly the 2f+1 quorum): a height whose
     round-0 (and round-1) proposer is down commits in round 1 (or 2); the prover only ends a segment at a block of round N
  5. the prover archives the finished proof and holds the submission; the validators are restarted
  6. negative tests on the archived proof, submitted to the chain with `engramd tx-submit-recovery-proof`: each mutation must
     be rejected while the valid proof is still held (state unchanged afterwards)
  7. hold removed -> the prover submits -> all seven ANCHORED; the same proof is submitted again (replay) -> rejected

    ENGRAM_CLUSTER=ssh python3 -u scripts/e10_bitcoin_reorg/live_recovery_rounds.py --engramd /tmp/engramd-local [--deploy]
"""

import argparse
import base64
import csv
import json
import os
import re
import subprocess
import sys
import tempfile
import time

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(HERE, "..", "framework"))
sys.path.insert(0, os.path.join(HERE, "..", "e7_consensus_overhead"))
from cluster import CLUSTER  # noqa: E402
from logger import query_node, _rpc_get  # noqa: E402
import run_cluster_ab as ab  # noqa: E402

RESULTS = os.path.join(HERE, "results_live")
BOUNDARY_RE = re.compile(r"segment (\d+): boundary blocks first=(\d+) \(round (\d+)\) last=(\d+) \(round (\d+)\), (\d+) header")
RETRY_RE = re.compile(r"segment (\d+): boundary at height (\d+) unusable \((.*?)\)")


def sh(host, cmd, timeout=120):
    return CLUSTER.run(host, f"{CLUSTER.sudo(host)}{cmd}", timeout)


def prover_exec(cmd):
    r = sh(CLUSTER.prover, f"docker exec reanchoring-prover sh -c {json.dumps(cmd)}")
    return r.returncode, r.stdout + r.stderr


def prover_log_full():
    r = sh(CLUSTER.prover, "docker logs reanchoring-prover 2>&1", 300)
    return r.stdout + r.stderr


def validator_log_grep(node, pattern):
    h = [x for x in CLUSTER.validators if x.name == node][0]
    r = sh(h, f"docker logs engramd 2>&1 | grep -iE {json.dumps(pattern)} | tail -300", 120)
    return r.stdout.strip().splitlines()


def states():
    out = {}
    for n in CLUSTER.nodes:
        try:
            s = query_node(n)
            out[n] = (s.height, s.fsm_state)
        except Exception:  # noqa: BLE001
            out[n] = (0, "")
    return out


def wait_state(want, timeout, rows, t0, phase, nodes=None, stall=None):
    nodes = nodes or CLUSTER.nodes
    end = time.time() + timeout
    while time.time() < end:
        st = states()
        for n, (h, s) in st.items():
            rows.append(dict(t=round(time.time() - t0, 1), phase=phase, node=n, height=h, fsm_state=s))
        if all(st[n][1] == want for n in nodes):
            return True
        if stall:
            stall()
        time.sleep(5)
    return False


def proposer_order():
    """Validators in the proposer rotation order (equal power: the order of /validators), as node names."""
    node_of = {}
    for n in CLUSTER.nodes:
        st = _rpc_get(CLUSTER.rpc_port(n), "/status", host=CLUSTER.rpc_host(n))
        node_of[st["result"]["validator_info"]["address"]] = n
    vals = _rpc_get(CLUSTER.rpc_port(CLUSTER.nodes[0]), "/validators?per_page=100", host=CLUSTER.rpc_host(CLUSTER.nodes[0]))["result"]["validators"]
    return [node_of[v["address"]] for v in vals]


def commit_round(h):
    r = _rpc_get(CLUSTER.rpc_port(CLUSTER.nodes[-1]), f"/commit?height={h}", timeout=5, host=CLUSTER.rpc_host(CLUSTER.nodes[-1]))
    return int(r["result"]["signed_header"]["commit"]["round"])


def container(node, action):
    h = [x for x in CLUSTER.validators if x.name == node][0]
    sh(h, f"docker {action} engramd", 120)


def fetch_archive(workdir):
    """The archived proof with the lowest last height: every segment is archived and held, and segment k+1 only submits after
    segment k, so the lowest one is the proof the chain can take next (a later one fails the rt_last binding whatever is mutated).
    Returns (base, proof, public_inputs, da_height) copied to workdir, or None."""
    rc, out = prover_exec("ls /control/archive 2>/dev/null")
    found = [(int(m.group(1)), x[: -len("_proof")]) for x in out.split() for m in [re.search(r"_h(\d+)_proof$", x)] if m]
    if not found:
        return None
    base = min(found)[1]
    files = {}
    for suffix in ("_proof", "_public_inputs", "_da_height"):
        r = sh(CLUSTER.prover, f"docker exec reanchoring-prover base64 /control/archive/{base}{suffix}")
        files[suffix] = base64.b64decode(r.stdout)
        with open(os.path.join(workdir, base + suffix), "wb") as f:
            f.write(files[suffix])
    return base, files["_proof"], files["_public_inputs"], int(files["_da_height"] or b"0")


def counters(plog):
    """The last `submitted N, failed M` of the prover's check lines."""
    m = re.findall(r"submitted (\d+), failed (\d+)", plog)
    return (int(m[-1][0]), int(m[-1][1])) if m else None


MUTATIONS = {
    "wrong_rt_last": lambda p, pi: (p, flip(pi, 0)),
    "wrong_first_block_hash": lambda p, pi: (p, flip(pi, 128)),
    "wrong_last_block_hash": lambda p, pi: (p, flip(pi, 160)),
    "wrong_validator_set_hash": lambda p, pi: (p, flip(pi, 192)),
    "wrong_count": lambda p, pi: (p, flip(pi, 95)),
    "wrong_first_height": lambda p, pi: (p, flip(pi, 127)),
    "tampered_proof_byte": lambda p, pi: (flip(p, len(p) // 2), pi),
}


def flip(b, i):
    b = bytearray(b)
    b[i] ^= 0x01
    return bytes(b)


def submit(engramd, node, proof, pub, workdir, tag, da_height=0):
    pp, ip = os.path.join(workdir, tag + "_proof"), os.path.join(workdir, tag + "_public_inputs")
    open(pp, "wb").write(proof)
    open(ip, "wb").write(pub)
    t = time.time()
    r = subprocess.run([engramd, "tx-submit-recovery-proof", "--node", f"http://{CLUSTER.rpc_host(node)}:26657", "--proof", pp, "--public-inputs", ip, "--da-height", str(da_height)],
                       capture_output=True, text=True, timeout=120)
    return dict(test=tag, accepted=r.returncode == 0, seconds=round(time.time() - t, 2), message=(r.stdout + r.stderr).strip()[:300])


def cycle(args, want_round, rows, t0, workdir):
    res = dict(want_round=want_round)
    ts = int(time.time())
    prover_exec("mkdir -p /control && rm -rf /control/archive /control/want_round && touch /control/hold")
    if want_round:
        prover_exec(f"echo {want_round} > /control/want_round")
    print(f"== cycle round {want_round}: fault (bitcoind paused, Celestia bridges stopped)", flush=True)
    sh(CLUSTER.infra, "docker pause bitcoin-node01")
    CLUSTER.da_stop_all()
    res["sovereign"] = wait_state("SOVEREIGN", 900, rows, t0, f"r{want_round}_fault")
    res["t_sovereign_s"] = round(time.time() - t0, 1)
    print(f"   all SOVEREIGN: {res['sovereign']}", flush=True)
    if not res["sovereign"]:
        return res
    print("== heal", flush=True)
    sh(CLUSTER.infra, "docker unpause bitcoin-node01")
    CLUSTER.da_start_all()
    res["recovering"] = wait_state("RECOVERING", 900, rows, t0, f"r{want_round}_heal")
    print(f"   all RECOVERING: {res['recovering']}", flush=True)
    if not res["recovering"]:
        return res
    order = proposer_order()
    pair = (order[args.pair_offset % 7], order[(args.pair_offset + 1) % 7]) if want_round else ()
    res["stopped_validators"] = pair
    print(f"== stopping adjacent proposers {pair} (rotation {order})", flush=True)
    for n in pair:
        container(n, "stop")
    t_stop = time.time()
    archive, deadline = None, time.time() + args.round_timeout_s
    live = [n for n in CLUSTER.nodes if n not in pair]
    while time.time() < deadline and not archive:
        st = states()
        for n, (h, s) in st.items():
            rows.append(dict(t=round(time.time() - t0, 1), phase=f"r{want_round}_prove", node=n, height=h, fsm_state=s))
        archive = fetch_archive(workdir)
        time.sleep(10)
    for n in pair:
        container(n, "start")
    res["restart_s"] = round(time.time() - t_stop, 1)
    logs = prover_log_full()
    res["boundary_lines"] = [m.group(0) for m in BOUNDARY_RE.finditer(logs)][-8:]
    res["retry_reasons"] = [m.group(3)[:100] for m in RETRY_RE.finditer(logs)][-8:]
    res["archived"] = bool(archive)
    if not archive:
        print("   no proof at the wanted round within the time box; cause in boundary_lines/retry_reasons", flush=True)
        prover_exec("rm -f /control/want_round /control/hold")  # let the prover finish a normal recovery so the cluster is usable
        return res
    base, proof, pub, da_height = archive
    seg = re.match(r"seg(\d+)_h(\d+)", base)
    seg_pat = re.compile(rf"segment {seg.group(1)}\b")
    res["archive"] = base
    res["public_inputs_len"] = len(pub)
    first_h, count = int.from_bytes(pub[120:128], "big"), int.from_bytes(pub[92:96], "big")
    last_h = int(seg.group(2))
    res["proof_covers"] = dict(first_height=first_h, last_height=last_h, count=count, da_celestia_height=da_height)
    res["boundary_rounds_from_chain_rpc"] = dict(first=commit_round(first_h), last=commit_round(last_h))
    print(f"   proof covers {first_h}-{last_h}, DA height {da_height}, rounds from the chain (first, last): {res['boundary_rounds_from_chain_rpc']}", flush=True)
    wait_state("RECOVERING", 120, rows, t0, f"r{want_round}_restart", nodes=live)
    node = live[0]
    print(f"== negative tests on {base} via {node}", flush=True)
    res["negatives"] = [dict(submit(args.engramd, node, *MUT(proof, pub), workdir, name), mutation=name) for name, MUT in MUTATIONS.items()]
    for r in res["negatives"]:
        print(f"   {r['mutation']}: accepted={r['accepted']} {r['message'][:110]}", flush=True)
    res["rejecting_checks_in_order"] = [l.split("check=")[-1].strip() for l in validator_log_grep(node, "recovery proof rejected")][-len(MUTATIONS):]
    if want_round == 0:
        # Small spam bound check: many proofs that pass the cheap bindings but fail UltraHonk, submitted at once. At most
        # maxProofVerifiesPerBlock (8) are verified per block; the rest are refused by the cap without being verified.
        from concurrent.futures import ThreadPoolExecutor
        before = len(validator_log_grep(node, "recovery proof rejected"))
        bad_proof = flip(proof, len(proof) // 2)
        with ThreadPoolExecutor(max_workers=args.cap_burst) as ex:
            outs = list(ex.map(lambda i: submit(args.engramd, node, flip(bad_proof, i % 64), pub, workdir, f"burst{i}"), range(args.cap_burst)))
        lines = validator_log_grep(node, "recovery proof rejected")
        checks = [l.split("check=")[-1].strip() for l in lines][-args.cap_burst:]
        res["cap_test"] = dict(submitted=args.cap_burst, accepted=sum(o["accepted"] for o in outs),
                               checks={c: checks.count(c) for c in sorted(set(checks))}, log_lines_before=before)
        print(f"   cap test: {res['cap_test']}", flush=True)
    res["state_after_negatives"] = sorted({s for _, s in states().values()})
    print("== releasing the valid proof", flush=True)
    res["held_proof_segment"] = base
    res["held_boundary_line"] = next((l for l in reversed(logs.splitlines()) if "boundary blocks" in l and seg_pat.search(l)), None)
    res["counters_before_release"] = counters(prover_log_full())
    prover_exec("rm -f /control/hold")
    time.sleep(args.submit_wait_s)
    plog = prover_log_full()
    res["counters_after_release"] = counters(plog)
    res["held_proof_outcome"] = [l for l in plog.splitlines() if seg_pat.search(l) and ("failed" in l or "rejected" in l)][-3:]
    acc = [re.sub(r"\x1b\[[0-9;]*m", "", l) for l in validator_log_grep(node, "recovery proof accepted")]
    res["accepted_log_lines"] = acc[-6:]
    res["held_proof_accepted_in_log"] = any(re.search(rf"last_height=\D*{last_h}\b", l) for l in acc)
    print(f"   held proof (last height {last_h}) logged as accepted: {res['held_proof_accepted_in_log']}", flush=True)
    res["validator_log_rejection"] = validator_log_grep(node, "recovery proof rejected|SubmitRecoveryProof")[-30:]
    prover_exec("rm -f /control/want_round")
    res["anchored"] = wait_state("ANCHORED", 900, rows, t0, f"r{want_round}_anchor")
    res["t_anchored_s"] = round(time.time() - t0, 1)
    print(f"   all ANCHORED: {res['anchored']}", flush=True)
    res["replay"] = submit(args.engramd, node, proof, pub, workdir, "replay_valid_proof", da_height=987654321)
    res["replay"]["rejecting_check"] = [l.split("check=")[-1].strip() for l in validator_log_grep(node, "recovery proof rejected")][-1:]
    print(f"   replay: accepted={res['replay']['accepted']} {res['replay']['message'][:110]}", flush=True)
    res["prover_log_since_fault"] = [l for l in prover_log_full().splitlines() if re.search(r"boundary|backfill|publish|DA|submitted|proof for heights|recover", l)][-40:]
    return res


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--engramd", default="/tmp/engramd-local")
    ap.add_argument("--rounds", default="0,1,2", help="0 = control cycle: proof held and released, no validator stopped")
    ap.add_argument("--deploy", action="store_true")
    ap.add_argument("--pair-offset", type=int, default=0, help="which adjacent proposer pair is stopped")
    ap.add_argument("--submit-wait-s", type=float, default=60, help="wait after releasing the hold, to read the outcome of the held proof")
    ap.add_argument("--round-timeout-s", type=float, default=1500)
    ap.add_argument("--tag", default="b3")
    ap.add_argument("--cap-burst", type=int, default=24, help="proofs submitted at once in the spam bound check (control cycle)")
    args = ap.parse_args()
    if CLUSTER.name != "ssh":
        sys.exit("run with ENGRAM_CLUSTER=ssh")
    if args.deploy:
        for var in ("DOCKERHUB_USER", "ENGRAM_IMAGE_TAG", "BITCOIN_RPC_PASSWORD"):
            if not os.environ.get(var):
                sys.exit(f"{var} is not set")
        ab.deploy("snapshot", extra={"prover_env": {"PROVER_CONTROL_DIR": "/control"}})
    ab.check_config("snapshot")
    for n in CLUSTER.nodes:  # the parameters are written into the genesis at init, not kept in the container's environment
        g = CLUSTER.exec_in_node(n, ["sh", "-c", "grep -E 'proof_only_recovery|max_unproven_tail_blocks' /root/.engramd/config/genesis.json"]).stdout
        if '"proof_only_recovery": true' not in g:
            sys.exit(f"{n}: proof_only_recovery is not true in the genesis; this experiment is proof-only ({g.strip()})")
    if not ab.wait_anchored(1800):
        sys.exit("the chain is not ANCHORED on all validators")
    rows, results, t0 = [], [], time.time()
    workdir = tempfile.mkdtemp(prefix="recovery_")
    try:
        for r in args.rounds.split(","):
            results.append(cycle(args, int(r), rows, t0, workdir))
            if not results[-1].get("anchored"):
                break
    finally:
        sh(CLUSTER.infra, "docker unpause bitcoin-node01")
        CLUSTER.da_start_all()
        for n in CLUSTER.nodes:
            container(n, "start")
    stamp = time.strftime("%Y%m%dT%H%M%S")
    os.makedirs(RESULTS, exist_ok=True)
    base = os.path.join(RESULTS, f"recovery_rounds_{args.tag}_{stamp}")
    with open(base + ".csv", "w", newline="") as f:
        w = csv.DictWriter(f, fieldnames=["t", "phase", "node", "height", "fsm_state"])
        w.writeheader()
        w.writerows(rows)
    json.dump(dict(image_tag=os.environ.get("ENGRAM_IMAGE_TAG"), cycles=results), open(base + "_summary.json", "w"), indent=2)
    print(json.dumps(results, indent=2)[:6000])
    print("wrote", base + "_summary.json")


if __name__ == "__main__":
    main()
