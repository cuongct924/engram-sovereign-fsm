#!/usr/bin/env python3
"""LIVE f=2 collusion suite on the 7-validator Docker testnet -- docs/EXPERIMENT.md's E8 B1-B9.

The threat model is n=7, f=2 (quorum 5). Two (B9: up to three) validators are Byzantine at once,
set through docker/engram-byzantine.yml (ENGRAM_BYZANTINE_BEHAVIOR_NODE0X), and every case is
judged on more than AppHash agreement:

  1. safety        honest nodes' AppHash never diverges at the same height;
  2. semantics     `engramd debug replay-fsm` re-derives every committed transition from the
                   block's own bytes and reports illegal ones (state the protocol would not
                   compute, aggregate outside the honest validators' observed range, withdrawal
                   committed while locked, committed state != replayed state);
  3. liveness      the chain keeps committing blocks during the attack window;
  4. case-specific the honest validators' own REJECT reasons, evidence logs, state transitions.

Cases:  b1 coordinated false observations        b2 + Byzantine leader biasing the certificate set
        b3a/b3b certificate omission (=quorum / below)    b4 coordinated double-signing
        b5 withdrawal attempt while SOVEREIGN             b6 stale + forged receipts
        b7a/b7b false reports + 2 / 3 honest nodes really eclipsed
        b8 pure honest split view (no Byzantine), 3/7 then 4/7 eclipsed
        b9 sweep of the number of Byzantine validators b=0..3 (b=3=f+1 is the negative control)

Run on a FRESH 7-node deploy of an image built from this tree (the replay reads block 1 onwards,
and the Byzantine hooks must exist in the image):  make testnet-up N=7

Usage:
    python3 -u scripts/e8_attack_resilience/live_collusion_suite.py --list
    python3 -u scripts/e8_attack_resilience/live_collusion_suite.py b1 b2 b3a b3b
    python3 -u scripts/e8_attack_resilience/live_collusion_suite.py --all --trials 3
"""

import argparse
import collections
import csv
import json
import os
import re
import shutil
import subprocess
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "framework"))
from logger import (  # noqa: E402
    NODE_RPC_PORTS,
    sample_all_nodes,
    start_all_da_bridges,
    stop_all_da_bridges,
)

REPO_ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
RESULTS_DIR = os.path.join(os.path.dirname(__file__), "results_live")
COMPOSE = ["docker", "compose", "-f", "docker-compose.yml", "-f", "docker/engram-byzantine.yml"]
BYZ_CANDIDATES = ["engram-node04", "engram-node05", "engram-node06"]
NETEM_IMAGE = "nicolaka/netshoot"
POLL_S = 3.0

# What a Byzantine reporter signs. Proto field names (x/sovereignty/byzantine.go).
LIE_DEGRADED = json.dumps(
    {
        "btc_gap": "100",
        "da_gap": "1000",
        "is_das_failed": True,
        "is_attestation_failed": True,
        "is_anchor_reverification_failed": True,
    }
)
LIE_HEALTHY = json.dumps(
    {"da_gap": "0", "is_das_failed": False, "is_attestation_failed": False, "btc_gap": "0"}
)
FALSE_DEGRADED = f"false_observation:{LIE_DEGRADED}"
FALSE_HEALTHY = f"false_observation:{LIE_HEALTHY}"

REJECT_RE = re.compile(r"ProcessProposal REJECT height=(\d+) round=(\d+) reason=(\w+)")
BYZ_PROPOSAL_RE = re.compile(r"BYZANTINE PrepareProposal height=(\d+)")
EVIDENCE_RE = re.compile(r"SLASHABLE EVIDENCE DETECTED type=(\S+) validator=(\S+) offense_height=(\d+) detected_at_height=(\d+)")
VIOLATION_RE = re.compile(r"^\s+height=(\d+) kind=(\S+)", re.M)


def now() -> str:
    return time.strftime("%H:%M:%S", time.gmtime())


def run(cmd, **kw):
    kw.setdefault("capture_output", True)
    kw.setdefault("text", True)
    kw.setdefault("timeout", 120)
    return subprocess.run(cmd, cwd=REPO_ROOT, **kw)


def env_var(node: str) -> str:
    return f"ENGRAM_BYZANTINE_BEHAVIOR_NODE{node[-2:]}"


# --- Byzantine assignment ---------------------------------------------------


class Assignment:
    """Tracks which validators currently run a Byzantine behavior and recreates only the changed ones."""

    def __init__(self):
        self.current: dict = {}

    def set(self, wanted: dict) -> None:
        changed = [n for n in set(self.current) | set(wanted) if self.current.get(n, "") != wanted.get(n, "")]
        if not changed:
            return
        env = dict(os.environ)
        for n in wanted:
            env[env_var(n)] = wanted[n]
        for n in changed:
            if n not in wanted:
                env.pop(env_var(n), None)
        print(f"[{now()}] >>> recreating {sorted(changed)} with {wanted or 'honest'}")
        run(COMPOSE + ["up", "-d", "--no-deps", "--force-recreate", *sorted(changed)], env=env, check=True, timeout=300)
        self.current = dict(wanted)
        # A node missing from docker/engram-byzantine.yml silently stays honest and would invalidate the
        # case (a control once ran with only 2 of its 5 intended faulty validators): read the env back.
        for n in changed:
            got = run(["docker", "exec", n, "printenv", "ENGRAM_BYZANTINE_BEHAVIOR"], check=False).stdout.strip()
            if got != wanted.get(n, ""):
                raise RuntimeError(f"{n} runs ENGRAM_BYZANTINE_BEHAVIOR={got!r}, wanted {wanted.get(n, '')!r}")

    def clear(self) -> None:
        self.set({})


# --- Real faults ------------------------------------------------------------


def eclipse(nodes, on: bool) -> None:
    """Really cut the validators' P2P links (every 172.40.* validator-link interface) of the nodes,
    leaving their Bitcoin/Celestia interfaces alone: the node sees degraded peers, not a dead BTC/DA."""
    tc = "tc qdisc replace dev $i root netem loss 100%" if on else "tc qdisc del dev $i root 2>/dev/null || true"
    script = "for i in $(ip -o -4 addr show | awk '/ 172\\.40\\./{print $2}' | sed 's/@.*//'); do " + tc + "; done"
    for n in nodes:
        run(["docker", "run", "--rm", "--net", f"container:{n}", "--cap-add", "NET_ADMIN", NETEM_IMAGE, "sh", "-c", script])


def wait_all(nodes, state: str, timeout_s: float) -> bool:
    deadline = time.time() + timeout_s
    while time.time() < deadline:
        samples = {s.node: s.fsm_state for s in sample_all_nodes(list(nodes))}
        if samples and all(v == state for v in samples.values()):
            return True
        time.sleep(POLL_S)
    return False


# --- Observation ------------------------------------------------------------


class Tracker:
    def __init__(self, honest):
        self.honest = list(honest)
        self.start = time.time()
        self.divergence = []
        self.states = collections.OrderedDict()  # node -> ordered list of distinct states
        self.first_state_t = {}  # (node, state) -> seconds since start
        self.heights = {}
        self.first_height = {}

    def poll(self, seconds: float, label: str, stop_when=None) -> bool:
        deadline = time.time() + seconds
        while time.time() < deadline:
            samples = sample_all_nodes()
            t = time.time() - self.start
            by_height = collections.defaultdict(dict)
            for s in samples:
                if s.node in self.honest and s.height > 0:
                    by_height[s.height][s.node] = s.app_hash
                    self.heights[s.node] = s.height
                    self.first_height.setdefault(s.node, s.height)
                    seen = self.states.setdefault(s.node, [])
                    if not seen or seen[-1] != s.fsm_state:
                        seen.append(s.fsm_state)
                        self.first_state_t.setdefault((s.node, s.fsm_state), t)
            for h, hashes in by_height.items():
                if len(set(hashes.values())) > 1:
                    self.divergence.append((round(t), label, h, dict(hashes)))
                    print(f"  *** [{label}] SAFETY VIOLATION height={h}: {hashes}")
            print(f"[{t:6.0f}s][{label}] " + " ".join(f"{s.node[-2:]}:{s.height}/{s.fsm_state[:4]}" for s in samples))
            if stop_when and stop_when(samples):
                return True
            time.sleep(POLL_S)
        return False

    def reset_window(self):
        self.states = collections.OrderedDict()
        self.first_state_t = {}
        self.first_height = {}

    def blocks_advanced(self) -> int:
        return max((self.heights.get(n, 0) - self.first_height.get(n, 0) for n in self.honest), default=0)

    def transitions(self):
        """Honest-node state changes after the first observation, i.e. real FSM moves in the window."""
        out = set()
        for seen in self.states.values():
            out.update(seen[1:])
        return sorted(out)


def docker_logs_since(node: str, since: float) -> str:
    p = run(["docker", "logs", "--since", str(int(since)), node], timeout=900)
    return p.stdout + p.stderr


def collect_rejects(honest, since: float) -> dict:
    reasons = collections.Counter()
    for n in honest:
        for _, _, reason in REJECT_RE.findall(docker_logs_since(n, since)):
            reasons[reason] += 1
    return dict(reasons)


def byzantine_proposed_heights(byz, since: float) -> set:
    heights = set()
    for n in byz:
        heights.update(int(h) for h in BYZ_PROPOSAL_RE.findall(docker_logs_since(n, since)))
    return heights


def replay(byz, report_from: int) -> dict:
    """Run the semantic replay with the host binary (build/engramd) against an honest node's RPC.
    Not `docker exec`: the validators sit at ~80% of their 512 MiB limit, and an OOM kill of
    the replay's host container would take the validator down with it."""
    honest0 = next(n for n in NODE_RPC_PORTS if n not in byz)
    binary = os.path.join(REPO_ROOT, "build", "engramd")
    out_csv = os.path.join(RESULTS_DIR, "replay_violations_latest.csv")
    args = [binary, "debug", "replay-fsm", "--rpc", f"http://localhost:{NODE_RPC_PORTS[honest0]}",
            "--out", out_csv, "--report-from", str(report_from)]
    # No Byzantine node (b8, b9's b=0): every validator is honest, so the range check still applies.
    args += ["--byzantine", ",".join(byz)] if byz else ["--honest", ",".join(NODE_RPC_PORTS)]
    p = run(args, timeout=3600)
    out = p.stdout + p.stderr
    kinds = collections.Counter(k for _, k in VIOLATION_RE.findall(out))
    m = re.search(r"heights (\d+)\.\.(\d+),.*final state (\S+), (\d+) violations", out)
    if p.returncode != 0 or not m:
        return {"ok": False, "error": out.strip()[-400:], "violations": None, "kinds": {}, "replayed": 0}
    return {"ok": True, "violations": int(m.group(4)), "kinds": dict(kinds), "replayed": int(m.group(2)) - int(m.group(1)) + 1,
            "final_state": m.group(3)}


# --- Cases ------------------------------------------------------------------


def nodes_of(n_byz: int):
    return BYZ_CANDIDATES[:n_byz]


def case_plan(case: str, args):
    """Returns dict(byz=<node->behavior>, attack_s, eclipse=<nodes>, sovereign=<bool>, ...)."""
    a = args.attack_s
    n4, n5, n6 = BYZ_CANDIDATES
    plans = {
        # Control: node04 is recreated (as every Byzantine container is when an attack starts) but stays honest, so
        # the honest transitions seen in the other cases can be compared with the churn of the restart itself.
        "c0": dict(byz={}, restart=[n4], attack_s=a, expect_states=set()),
        "b1": dict(byz={n4: FALSE_DEGRADED, n5: FALSE_DEGRADED}, attack_s=a, expect_states=set()),
        "b2": dict(byz={n4: FALSE_DEGRADED, n5: FALSE_DEGRADED + ";bias_certificate"}, attack_s=a, expect_states=set()),
        "b3a": dict(byz={n4: "omit_certs:5"}, attack_s=a, expect_states=set()),
        "b3b": dict(byz={n4: "omit_certs:2"}, attack_s=a, expect_states=set(), expect_reject="sub_quorum_certificate"),
        # NEXT_SESSION_PLAN item 14: A3/A4 re-run counting the honest validators' REJECT reasons, to show
        # the DA / BTC receipt checks (not something incidental) blocked the proposal.
        # a3 claims a DA attestation one height ahead, inside da.VerifyReceipt's tolerance: honest validators cannot
        # refute it, so no rejection is expected (a claim of this size is accepted by design; see the paper's DA-trust limit).
        "a3": dict(byz={n4: "false_da_attestation"}, attack_s=a, expect_states=set()),
        "a4": dict(byz={n4: "forge_btc_hash"}, attack_s=a, expect_states=set(), expect_reject="btc_verify_receipt"),
        "b5": dict(byz={n4: "include_withdrawal", n5: "include_withdrawal"}, attack_s=a, sovereign=True,
                   expect_reject="withdrawal_locked"),
        "b6": dict(byz={n4: "stale_receipt", n5: "forge_btc_hash"}, attack_s=a, expect_states=set(),
                   expect_reject=("da_verify_receipt", "btc_verify_receipt")),
        "b7a": dict(byz={n4: FALSE_DEGRADED, n5: FALSE_DEGRADED}, attack_s=a, eclipse=["engram-node06", "engram-node07"]),
        "b7b": dict(byz={n4: FALSE_DEGRADED, n5: FALSE_DEGRADED}, attack_s=a,
                    eclipse=["engram-node03", "engram-node06", "engram-node07"]),
    }
    return plans[case]


def verdict(case, plan, tr, rejects, replay_res, extra) -> dict:
    checks = {
        "safety_held": not tr.divergence,
        "liveness": tr.blocks_advanced() > 0,
        "replay_clean": bool(replay_res["ok"]) and replay_res["violations"] == 0,
    }
    if "expect_states" in plan:
        checks["no_honest_transition"] = getattr(tr, "attack_transitions", tr.transitions()) == []
        checks["no_honest_transition_whole_run"] = tr.transitions() == []
    expect_reject = plan.get("expect_reject")
    if expect_reject:
        wanted = (expect_reject,) if isinstance(expect_reject, str) else expect_reject
        checks["byzantine_proposal_rejected"] = any(rejects.get(r, 0) > 0 for r in wanted)
    checks.update(extra)
    return checks


def run_case(case: str, trial: int, args, assignment: Assignment) -> dict:
    print(f"\n=== {case} trial {trial} ===")
    if case == "b4":
        return run_double_sign(trial, args, assignment)
    if case == "b8":
        return run_b8(trial, args, assignment)
    if case == "b9":
        return run_b9(trial, args, assignment)
    if case in ("h4a", "h4i"):
        return run_hide_k4(case, trial, args, assignment)
    return run_standard(case, trial, args, assignment)


def start_of_window(honest) -> int:
    return max((s.height for s in sample_all_nodes(list(honest)) if s.height > 0), default=1)


def finish(case, trial, plan, tr, byz, since, report_from, extra=None, honest_extra=None) -> dict:
    honest = [n for n in NODE_RPC_PORTS if n not in byz]
    rejects = collect_rejects(honest, since)
    proposed = getattr(tr, "proposed_heights", None)
    if proposed is None:
        proposed = byzantine_proposed_heights(byz, since)
    replay_res = replay(byz, report_from)
    end_h = max(tr.heights.values(), default=report_from)
    total_heights = max(end_h - report_from + 1, 1)
    row = {
        "case": case, "trial": trial, "byzantine": ",".join(byz), "blocks_advanced": tr.blocks_advanced(),
        "transitions": ",".join(tr.transitions()), "divergence_events": len(tr.divergence),
        "rejects": json.dumps(rejects, sort_keys=True), "byz_proposed_heights": len(proposed),
        "byz_proposer_share": round(len(proposed) / total_heights, 4),
        "replayed_blocks": replay_res["replayed"], "replay_violations": replay_res["violations"],
        "replay_kinds": json.dumps(replay_res["kinds"], sort_keys=True),
    }
    checks = verdict(case, plan, tr, rejects, replay_res, honest_extra or {})
    row["checks"] = json.dumps(checks, sort_keys=True)
    row["pass"] = all(v for k, v in checks.items() if not k.endswith("_whole_run"))
    row.update(extra or {})
    print(f"VERDICT {case}#{trial}: pass={row['pass']} checks={checks} rejects={rejects} replay={replay_res}")
    return row


def run_standard(case, trial, args, assignment) -> dict:
    plan = case_plan(case, args)
    byz = list(plan["byz"])
    honest = [n for n in NODE_RPC_PORTS if n not in byz]
    # A case that does not itself drive the chain to SOVEREIGN starts from ANCHORED on every validator: b7a/b7b once
    # started in the SOVEREIGN left by the previous case, which changes what an eclipse or a lie can do.
    if not plan.get("sovereign") and not wait_all(list(NODE_RPC_PORTS), "ANCHORED", 900):
        if not fresh_cluster(assignment):
            raise RuntimeError(f"{case}: no ANCHORED start available")
    tr = Tracker(honest)
    assignment.clear()
    tr.poll(args.baseline_s, "baseline")
    if plan.get("sovereign"):
        print(f"[{now()}] >>> stopping every DA bridge to reach SOVEREIGN")
        stop_all_da_bridges()
        if not wait_all(honest, "SOVEREIGN", 600):
            print("  could not reach SOVEREIGN within 600s")
    report_from = start_of_window(honest)
    since = time.time()
    tr.reset_window()
    assignment.set(plan["byz"])
    if plan.get("restart"):
        print(f"[{now()}] >>> recreating {plan['restart']} as honest (control)")
        run(COMPOSE + ["up", "-d", "--no-deps", "--force-recreate", *plan["restart"]], check=True, timeout=300)
    if plan.get("eclipse"):
        print(f"[{now()}] >>> eclipsing {plan['eclipse']}")
        eclipse(plan["eclipse"], True)
    tr.poll(plan["attack_s"], "attack")
    # Read the Byzantine proposals now: recreating the containers below discards their logs.
    tr.proposed_heights = byzantine_proposed_heights(byz, since)
    # The recovery phase below recreates the Byzantine containers as honest ones, whose rejoin makes the others dip to
    # SUSPICIOUS for a few blocks: harness churn, not the attack. no_honest_transition judges the attack window only.
    tr.attack_transitions = tr.transitions()
    if plan.get("eclipse"):
        eclipse(plan["eclipse"], False)
    assignment.clear()
    if plan.get("sovereign"):
        start_all_da_bridges()
    tr.poll(args.recovery_s, "recovery")
    extra = {}
    if plan.get("sovereign"):
        extra["recovered_to_anchored"] = wait_all(honest, "ANCHORED", 900)
    return finish(case, trial, plan, tr, byz, since, report_from, extra=extra)


def run_b8(trial, args, assignment) -> dict:
    """Pure honest split view: no Byzantine; 3/7 then 4/7 honest eclipsed for b8_minutes each."""
    plan = {}
    honest = list(NODE_RPC_PORTS)
    tr = Tracker(honest)
    assignment.clear()
    tr.poll(args.baseline_s, "baseline")
    report_from = start_of_window(honest)
    since = time.time()
    tr.reset_window()
    for label, group in (("eclipse3", ["engram-node05", "engram-node06", "engram-node07"]),
                         ("eclipse4", ["engram-node04", "engram-node05", "engram-node06", "engram-node07"])):
        eclipse(group, True)
        tr.poll(args.b8_minutes * 60, label)
        eclipse(group, False)
        tr.poll(args.recovery_s, f"{label}_heal")
    row = finish("b8", trial, plan, tr, [], since, report_from)
    row["note"] = "transitions are measured, not asserted: a majority really seeing degradation may legitimately move the FSM"
    return row


def fresh_cluster(assignment) -> bool:
    """Wipe and redeploy the whole testnet, then wait for ANCHORED. Repeated DA outages on one chain make every
    recovery longer (a first b9 run started its fourth trial permanently in RECOVERING), so a run must not inherit
    the previous one's state."""
    print(f"[{now()}] >>> fresh deployment")
    run([os.path.join(REPO_ROOT, "scripts", "testnet_redeploy_clean.sh")], check=False, timeout=3600)
    assignment.current = {}
    return wait_all(list(NODE_RPC_PORTS), "ANCHORED", 1800)


def run_b9(trial, args, assignment) -> list:
    rows = []
    if trial > 1 and not fresh_cluster(assignment):
        raise RuntimeError("fresh deployment did not reach ANCHORED")
    for target in ("force_sovereign", "hide_outage"):
        for b in range(4):
            # Every run starts from ANCHORED on all validators; if the chain has not recovered by itself in 15 min,
            # redeploy rather than measure from a degraded start.
            if not wait_all(list(NODE_RPC_PORTS), "ANCHORED", 900):
                if not fresh_cluster(assignment):
                    raise RuntimeError("no ANCHORED start available for b9")
            byz = nodes_of(b)
            behaviors = {n: (FALSE_DEGRADED if target == "force_sovereign" else FALSE_HEALTHY) + (
                ";bias_certificate" if target == "force_sovereign" else ";bias_certificate:best") for n in byz}
            honest = [n for n in NODE_RPC_PORTS if n not in byz]
            tr = Tracker(honest)
            assignment.clear()
            tr.poll(args.baseline_s, "baseline")
            report_from = start_of_window(honest)
            since = time.time()
            tr.reset_window()
            assignment.set(behaviors)
            if target == "hide_outage":
                stop_all_da_bridges()
            t0 = time.time() - tr.start
            tr.poll(args.attack_s, f"b9_{target}_b{b}")
            if target == "hide_outage":
                start_all_da_bridges()
            assignment.clear()
            tr.poll(args.recovery_s, "recovery")
            if target == "hide_outage":
                wait_all(honest, "ANCHORED", 900)
            plan = {}
            detected = [tr.first_state_t[(n, s)] - t0 for n in honest for s in ("SUSPICIOUS", "SOVEREIGN")
                        if (n, s) in tr.first_state_t]
            extra = {
                "target": target, "b": b,
                "forced_transition": bool(set(tr.transitions()) & {"SUSPICIOUS", "SOVEREIGN"}) if target == "force_sovereign" else None,
                "outage_masked": (not detected) if target == "hide_outage" else None,
                "outage_detect_s": round(min(detected), 1) if detected else None,
            }
            # b <= f must not be exploitable; b = f+1 is expected to be (negative control).
            exploited = bool(extra["forced_transition"]) if target == "force_sovereign" else bool(extra["outage_masked"])
            # A transition is not evidence of exploitation: the c0 control and a b=0 run both show honest
            # transitions from environment noise alone. The noise-independent property is Lemma 1: for b <= f the
            # committed aggregate stays inside the range of the honest observations (the replay's
            # aggregate_outside_honest_range count is 0); b = f+1 is the negative control. `exploited` stays recorded.
            checks = {}
            row = finish(f"b9_{target}_b{b}", trial, plan, tr, byz, since, report_from, extra=extra, honest_extra=checks)
            row["exploited"] = exploited
            row["replay_outside_range"] = json.loads(row["replay_kinds"]).get("aggregate_outside_honest_range", 0)
            row["start_anchored"] = True
            row["lemma1_matches_threshold"] = (row["replay_outside_range"] == 0) if b <= 2 else True
            row["pass"] = bool(row["pass"]) and row["lemma1_matches_threshold"]
            rows.append(row)
    return rows


def proposer_cycle() -> list:
    """The 7-slot proposer rotation of the running chain, as node names in order (it changes with every fresh genesis:
    the validator keys are regenerated)."""
    import urllib.request
    addr = {}
    for n in NODE_RPC_PORTS:
        with open(os.path.join(REPO_ROOT, "testnet-data", n, "config", "priv_validator_key.json")) as f:
            addr[json.load(f)["address"].upper()] = n
    port = NODE_RPC_PORTS[next(iter(NODE_RPC_PORTS))]
    top = int(json.load(urllib.request.urlopen(f"http://127.0.0.1:{port}/status", timeout=10))["result"]["sync_info"]["latest_block_height"])
    seq = []
    for h in range(top - 27, top + 1):
        hdr = json.load(urllib.request.urlopen(f"http://127.0.0.1:{port}/block?height={h}", timeout=10))["result"]["block"]["header"]
        seq.append(addr[hdr["proposer_address"].upper()])
    cycle = seq[:7]
    if seq[7:14] != cycle or seq[14:21] != cycle:
        raise RuntimeError(f"proposer order is not a stable 7-cycle: {seq}")
    return cycle


def run_hide_k4(case, trial, args, assignment) -> dict:
    """Three Byzantine validators hide a real DA outage on a chain deployed with DownHysteresisThreshold = MaxDown = 4.
    h4a: three consecutive proposer slots (the honest proposers form one run of four). h4i: slots 0, 2, 4 of the cycle
    (honest runs of at most two). The simulation predicts the outage is detected with the adjacent placement and hidden
    with the interleaved one; this is the live check."""
    if trial > 1 and not fresh_cluster(assignment):  # a new genesis also gives a new proposer rotation
        raise RuntimeError(f"{case}: fresh deployment did not reach ANCHORED")
    if not wait_all(list(NODE_RPC_PORTS), "ANCHORED", 900):
        if not fresh_cluster(assignment):
            raise RuntimeError(f"{case}: no ANCHORED start available")
    cycle = proposer_cycle()
    slots = [0, 1, 2] if case == "h4a" else [0, 2, 4]
    byz = [cycle[i] for i in slots]
    behaviors = {n: FALSE_HEALTHY + ";bias_certificate:best" for n in byz}
    honest = [n for n in NODE_RPC_PORTS if n not in byz]
    print(f"proposer cycle {[c[-2:] for c in cycle]}; Byzantine {[b[-2:] for b in byz]} ({case})")
    tr = Tracker(honest)
    assignment.clear()
    tr.poll(args.baseline_s, "baseline")
    report_from = start_of_window(honest)
    since = time.time()
    tr.reset_window()
    assignment.set(behaviors)
    stop_all_da_bridges()
    tr.poll(args.attack_s, f"{case}_attack")
    tr.attack_transitions = tr.transitions()
    tr.proposed_heights = byzantine_proposed_heights(byz, since)
    start_all_da_bridges()
    assignment.clear()
    tr.poll(args.recovery_s, "recovery")
    left = sorted(set(tr.attack_transitions))
    extra = {"placement": case, "proposer_cycle": ",".join(c[-2:] for c in cycle), "byzantine_nodes": ",".join(b[-2:] for b in byz),
             "left_anchored_during_attack": bool(left), "reached_sovereign_during_attack": "SOVEREIGN" in left}
    plan = {}
    return finish(case, trial, plan, tr, byz, since, report_from, extra=extra)


def run_double_sign(trial, args, assignment) -> dict:
    """B4: node04 AND node05 are impersonated by independent processes holding their real keys."""
    assignment.clear()
    byz = ["engram-node04", "engram-node05"]
    honest = [n for n in NODE_RPC_PORTS if n not in byz]
    keys, addrs = {}, {}
    for n in byz:
        keys[n] = os.path.join(REPO_ROOT, "testnet-data", n, "config", "priv_validator_key.json")
        addrs[n] = json.load(open(keys[n]))["address"].upper()
    genesis = os.path.join(REPO_ROOT, "testnet-data", honest[0], "config", "genesis.json")
    ports = {n: NODE_RPC_PORTS[n] for n in honest}
    ids = {}
    for n in honest:
        out = run(["curl", "-s", f"http://localhost:{ports[n]}/status"]).stdout
        ids[n] = json.loads(out)["result"]["node_info"]["id"]
    peers = ",".join(f"{ids[n]}@{n}:26656" for n in honest)
    for n in byz:
        home = os.path.join(REPO_ROOT, "testnet-data", f"{n}-double-sign")
        shutil.rmtree(os.path.join(home, "config"), ignore_errors=True)
        shutil.rmtree(os.path.join(home, "data"), ignore_errors=True)
        os.makedirs(os.path.join(home, "config"))
        os.makedirs(os.path.join(home, "data"))
        shutil.copy(keys[n], os.path.join(home, "config", "priv_validator_key.json"))
        shutil.copy(genesis, os.path.join(home, "config", "genesis.json"))
        with open(os.path.join(home, "data", "priv_validator_state.json"), "w") as f:
            f.write('{"height":"0","round":0,"step":0}')
    tr = Tracker(honest)
    tr.poll(args.baseline_s, "baseline")
    report_from = start_of_window(honest)
    since = time.time()
    tr.reset_window()
    env = dict(os.environ, DUPLICATE_PERSISTENT_PEERS=peers)
    services = [f"{n}-double-sign" for n in byz]
    run(["docker", "compose", "--profile", "double-sign-harness", "up", "-d", "--build", *services], env=env, check=True, timeout=600)
    deadline = time.time() + args.evidence_timeout_s
    found = {}  # (witness, address) -> (offense_h, detected_h)
    while time.time() < deadline and len(found) < len(honest) * len(byz):
        tr.poll(POLL_S, "double_signing")
        for w in honest:
            for _, val, off, det in EVIDENCE_RE.findall(docker_logs_since(w, since)):
                for n in byz:
                    if val.upper() == addrs[n]:
                        found.setdefault((w, n), (int(off), int(det)))
    run(["docker", "compose", "--profile", "double-sign-harness", "stop", *services])
    run(["docker", "compose", "--profile", "double-sign-harness", "rm", "-f", *services])
    tr.poll(args.recovery_s, "recovery")
    latencies = [d - o for o, d in found.values()]
    extra = {
        "evidence_pairs": len(found), "evidence_expected": len(honest) * len(byz),
        "evidence_latency_blocks_max": max(latencies) if latencies else None,
    }
    row = finish("b4", trial, {}, tr, byz, since, report_from, extra=extra,
                 honest_extra={"evidence_for_both_validators": len(found) == len(honest) * len(byz)})
    return row


CASES = ["h4a", "h4i", "c0", "a3", "a4", "b1", "b2", "b3a", "b3b", "b4", "b5", "b6", "b7a", "b7b", "b8", "b9"]


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("cases", nargs="*", choices=CASES)
    ap.add_argument("--all", action="store_true")
    ap.add_argument("--list", action="store_true")
    ap.add_argument("--trials", type=int, default=3)
    ap.add_argument("--attack-s", type=float, default=180.0)
    ap.add_argument("--baseline-s", type=float, default=20.0)
    ap.add_argument("--recovery-s", type=float, default=30.0)
    ap.add_argument("--b8-minutes", type=float, default=10.0)
    ap.add_argument("--evidence-timeout-s", type=float, default=300.0)
    args = ap.parse_args()
    if args.list:
        print("\n".join(CASES))
        return
    selected = CASES if args.all else args.cases
    if not selected:
        ap.error("name cases or pass --all")
    if len(NODE_RPC_PORTS) < 7:
        sys.exit(f"needs the 7-validator testnet (ENGRAM_VALIDATOR_COUNT=7); found {len(NODE_RPC_PORTS)}")

    os.makedirs(RESULTS_DIR, exist_ok=True)
    assignment = Assignment()
    rows = []
    path = os.path.join(RESULTS_DIR, f"collusion_suite_{time.strftime('%Y%m%dT%H%M%S')}.csv")

    def write_rows():
        fields = sorted({k for r in rows for k in r}, key=lambda k: (k not in ("case", "trial", "pass"), k))
        with open(path, "w", newline="") as f:
            w = csv.DictWriter(f, fieldnames=fields)
            w.writeheader()
            w.writerows(rows)

    try:
        for case in selected:
            for trial in range(1, args.trials + 1):
                res = run_case(case, trial, args, assignment)
                rows.extend(res if isinstance(res, list) else [res])
                # After every run: a crash hours in (a docker-logs timeout once lost 14 b9 runs) keeps what finished.
                write_rows()
    finally:
        assignment.clear()
        eclipse(list(NODE_RPC_PORTS), False)
        start_all_da_bridges()

    write_rows()
    print(f"\nwrote {len(rows)} rows to {path}")
    for r in rows:
        print(f"  {r['case']:<24} trial={r['trial']} pass={r['pass']} replay_violations={r['replay_violations']}")
    # b9's b=3 rows are the negative control: a failing exploit check there is the expected finding.
    unexpected = [r for r in rows if not r["pass"] and not r["case"].endswith("_b3")]
    sys.exit(1 if unexpected else 0)


if __name__ == "__main__":
    main()
