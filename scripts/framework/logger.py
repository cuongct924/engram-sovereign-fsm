"""Live-Docker data-collection framework, the data source for E2/E3/E4/E7/E9: reads from the
REAL running docker testnet, not tests/e2e's in-process mock-sensor Harness. Every function
talks to real RPC/ABCI-query interfaces -- nothing mocks or recomputes FSM logic, it only
OBSERVES committed state.
"""

import base64
import json
import os
import subprocess
import time
import urllib.request
import urllib.error
from dataclasses import dataclass, asdict
from typing import List, Optional

from cluster import CLUSTER

# The node list comes from CLUSTER: ENGRAM_VALIDATOR_COUNT (default 7) for the local Docker
# testnet -- must match whichever `make testnet-up N=4|7` topology is running; a too-high count
# doesn't crash (query_node() returns error-flagged samples for unreachable nodes), it silently
# hangs any wait_for_state() check -- or the deploy/ inventory under ENGRAM_CLUSTER=ssh.
VALIDATOR_COUNT = len(CLUSTER.nodes)

# engram-node01..NN's RPC ports (Docker: node01=26657, +100 per node; ssh: 26657 on each host,
# addressed by CLUSTER.rpc_host). Some E2-E10 scripts still hardcode their own node list for
# topology reasons (a named byzantine/eclipse target) -- this only sets sample_all_nodes()'s default.
NODE_RPC_PORTS = {n: CLUSTER.rpc_port(n) for n in CLUSTER.nodes}

# Each validator's own independent Celestia bridge -- a real cluster-wide DA
# outage means stopping ALL of them, since AggregateHealthCertificates'
# power-weighted majority would otherwise keep reporting DA-healthy from the rest.
CELESTIA_BRIDGES = [f"celestia-bridge-node{i:02d}" for i in range(1, VALIDATOR_COUNT + 1)]


def stop_all_da_bridges(timeout: float = 30.0) -> None:
    CLUSTER.da_stop_all()


def start_all_da_bridges(timeout: float = 30.0) -> None:
    CLUSTER.da_start_all()

QUERY_STATE_PATH = "/engram.sovereignty.v1.Query/State"


def _rpc_get(port: int, path: str, timeout: float = 3.0, host: str = "localhost") -> dict:
    url = f"http://{host}:{port}{path}"
    with urllib.request.urlopen(url, timeout=timeout) as resp:
        return json.loads(resp.read())


def _decode_query_state(value_b64: str) -> dict:
    """Hand-decodes QueryStateResponse's protobuf wire format -- a minimal decoder for this one
    message's tag/wire-type combinations, avoiding a generated Python stub.
    """
    raw = base64.b64decode(value_b64) if value_b64 else b""
    out = {
        "fsm_state": "",
        "safe_blocks": 0,
        "suspicious_duration": 0,
        "reanchoring_proof_valid": False,
    }
    i = 0
    while i < len(raw):
        tag = raw[i]
        i += 1
        field_num = tag >> 3
        wire_type = tag & 0x7
        if wire_type == 2:  # length-delimited (string or embedded message)
            length = raw[i]
            i += 1
            payload = raw[i : i + length]
            i += length
            if field_num == 1:
                out["fsm_state"] = payload.decode("utf-8")
            # field 5 (metrics) deliberately not decoded: PreBlocker never
            # writes k.Metrics into committed state, so it's always stale.
        elif wire_type == 0:  # varint
            value = 0
            shift = 0
            while True:
                b = raw[i]
                i += 1
                value |= (b & 0x7F) << shift
                if not (b & 0x80):
                    break
                shift += 7
            if field_num == 2:
                out["safe_blocks"] = value
            elif field_num == 3:
                out["suspicious_duration"] = value
            elif field_num == 4:
                out["reanchoring_proof_valid"] = bool(value)
        else:
            raise ValueError(f"unexpected wire type {wire_type} for field {field_num}")
    return out


@dataclass
class NodeSample:
    timestamp: float
    node: str
    height: int
    app_hash: str
    catching_up: bool
    fsm_state: str
    safe_blocks: int
    suspicious_duration: int
    reanchoring_proof_valid: bool
    error: str = ""


def query_node(node: str, port: Optional[int] = None) -> NodeSample:
    """Queries one real node's /status (CometBFT RPC) and Query.State
    (routed through /abci_query, the same path engramd's CLI uses -- see
    cmd/engramd/reanchor_cli.go's query-recovery-headers) -- both real,
    committed state, not recomputed here.
    """
    port = port or NODE_RPC_PORTS[node]
    host = CLUSTER.rpc_host(node) if node in CLUSTER.nodes else "localhost"
    ts = time.time()
    try:
        status = _rpc_get(port, "/status", host=host)
        sync_info = status["result"]["sync_info"]
        height = int(sync_info["latest_block_height"])
        app_hash = sync_info["latest_app_hash"]
        catching_up = bool(sync_info["catching_up"])

        abci = _rpc_get(port, f"/abci_query?path=%22{QUERY_STATE_PATH}%22&data=0x", host=host)
        resp = abci["result"]["response"]
        if resp.get("code", 0) != 0:
            raise RuntimeError(f"abci_query error: {resp.get('log')}")
        state = _decode_query_state(resp.get("value", ""))

        return NodeSample(
            timestamp=ts,
            node=node,
            height=height,
            app_hash=app_hash,
            catching_up=catching_up,
            fsm_state=state["fsm_state"],
            safe_blocks=state["safe_blocks"],
            suspicious_duration=state["suspicious_duration"],
            reanchoring_proof_valid=state["reanchoring_proof_valid"],
        )
    except (
        urllib.error.URLError,
        TimeoutError,
        KeyError,
        ValueError,
        RuntimeError,
        OSError,  # ConnectionReset/Refused etc. from a mid-poll container
        # recreate -- raised from the socket layer, not wrapped in URLError.
    ) as e:
        return NodeSample(
            timestamp=ts,
            node=node,
            height=-1,
            app_hash="",
            catching_up=False,
            fsm_state="",
            safe_blocks=0,
            suspicious_duration=0,
            reanchoring_proof_valid=False,
            error=str(e),
        )


def sample_all_nodes(nodes: Optional[List[str]] = None) -> List[NodeSample]:
    nodes = nodes or list(NODE_RPC_PORTS.keys())
    return [query_node(n) for n in nodes]


def poll_timeline(
    duration_s: float,
    interval_s: float,
    nodes: Optional[List[str]] = None,
    on_sample=None,
) -> List[NodeSample]:
    """Samples all nodes every interval_s for duration_s wall-clock seconds.
    on_sample(list[NodeSample]), if given, is called after each round --
    used by callers that want to print live progress or trigger chaos
    partway through the window.
    """
    nodes = nodes or list(NODE_RPC_PORTS.keys())
    samples: List[NodeSample] = []
    deadline = time.time() + duration_s
    while time.time() < deadline:
        round_samples = sample_all_nodes(nodes)
        samples.extend(round_samples)
        if on_sample:
            on_sample(round_samples)
        time.sleep(interval_s)
    return samples


def write_csv(samples: List[NodeSample], path: str) -> None:
    import csv

    fields = (
        list(asdict(samples[0]).keys())
        if samples
        else [
            "timestamp",
            "node",
            "height",
            "app_hash",
            "catching_up",
            "fsm_state",
            "safe_blocks",
            "suspicious_duration",
            "reanchoring_proof_valid",
            "error",
        ]
    )
    with open(path, "w", newline="") as f:
        w = csv.DictWriter(f, fieldnames=fields)
        w.writeheader()
        for s in samples:
            w.writerow(asdict(s))


def net_info(node: str, port: Optional[int] = None) -> dict:
    """Real CometBFT /net_info RPC -- connected peer list with remote IPs,
    independent of the app-layer Query.State gap (PeripheralMetrics.
    CleanPeers/SubnetDiversity/ActiveAnchors are never written into
    committed state -- see _decode_query_state's comment on field 5). Reads
    the SAME real p2p.Switch.Peers() data vanillaP2PHealthAdapter and
    x/sovereignty/keeper/peer_filter.go's FilterPeerByAddr use, via
    CometBFT's own /net_info endpoint.
    """
    port = port or NODE_RPC_PORTS[node]
    return _rpc_get(port, "/net_info")


def dump_consensus_state(node: str, port: Optional[int] = None) -> dict:
    """Real CometBFT /dump_consensus_state RPC -- includes the current round's per-validator
    prevotes/precommits ("nil-Vote" or "Vote{...}" strings), used by E7's nil-prevote live test.
    """
    port = port or NODE_RPC_PORTS[node]
    return _rpc_get(port, "/dump_consensus_state")


def block_proposer(node: str, height: int, port: Optional[int] = None) -> str:
    """proposer_address for a specific COMMITTED height, from CometBFT's
    real /block RPC (result.block.header.proposer_address) -- which
    validator actually proposed that block. Used to determine the
    round-robin proposer schedule (E8's live_censorship_test.py) rather
    than assuming a target validator gets a turn within some fixed window.
    """
    port = port or NODE_RPC_PORTS[node]
    block = _rpc_get(port, f"/block?height={height}")
    return block["result"]["block"]["header"]["proposer_address"]


def own_validator_address(node: str, port: Optional[int] = None) -> str:
    """Real validator consensus address for node, from its own /status --
    dump_consensus_state's prevotes/precommits arrays are ordered by index
    into the shared validators array, not by name, so this is needed to find
    which index is "this node's own vote."
    """
    port = port or NODE_RPC_PORTS[node]
    status = _rpc_get(port, "/status")
    return status["result"]["validator_info"]["address"]


def own_precommit_status(dump_state: dict, target_address: str):
    """(committed_height, precommit_str) for target_address's precommit in the most recently
    COMMITTED block (round_state.last_commit), or None if no commit yet or address not found.

    Deliberately NOT the in-progress round's live `votes[].prevotes` array: at this cluster's
    ~1s block time, fixed-interval polling of that array mostly catches "not yet received"
    (indistinguishable from a real nil vote) rather than a settled outcome. last_commit is
    SETTLED for an already-committed height, immune to this.
    """
    rs = dump_state["result"]["round_state"]
    validators = rs["validators"]["validators"]
    index = next((i for i, v in enumerate(validators) if v["address"] == target_address), None)
    if index is None:
        return None
    last_commit = rs.get("last_commit")
    if not last_commit or not last_commit.get("votes"):
        return None
    votes = last_commit["votes"]
    if index >= len(votes):
        return None
    committed_height = int(rs["height"]) - 1
    return committed_height, votes[index]


def _subnet_of(ip: str) -> str:
    """Mirrors x/sovereignty/types.SubnetOf's masking (IPv4 /24, IPv6 /48) in
    Python -- must stay in lockstep with that Go function, or this script's
    subnet counts won't match what the real ingress filter/sensor computed.
    """
    import ipaddress

    addr = ipaddress.ip_address(ip)
    prefix = 24 if addr.version == 4 else 48
    network = ipaddress.ip_network(f"{ip}/{prefix}", strict=False)
    return str(network.network_address)


def peer_subnet_counts(node: str, port: Optional[int] = None) -> dict:
    """Real per-subnet connected-peer counts for node, computed the same way
    FilterPeerByAddr/vanillaP2PHealthAdapter do -- lets a live script confirm
    the real ingress filter's view (e.g. E4/E8's A1/A2 attacker swarms)
    without app-level Query.State access."""
    info = net_info(node, port)
    counts: dict = {}
    for p in info.get("result", {}).get("peers", []):
        ip = p.get("remote_ip", "")
        if not ip:
            continue
        subnet = _subnet_of(ip)
        counts[subnet] = counts.get(subnet, 0) + 1
    return counts


def bitcoin_cli(*args: str, node: str = "bitcoin-node01") -> str:
    """Runs bitcoin-cli inside a real bitcoin node container -- cross-checks anchor state or
    drives a real reorg via bitcoin-node02 (E10). Credentials read from
    BITCOIN_RPC_USER/PASSWORD (matching .env), not hardcoded.
    """
    user = os.environ.get("BITCOIN_RPC_USER", "engram_admin")
    password = os.environ.get("BITCOIN_RPC_PASSWORD", "secure_password_123")
    cmd = [
        "docker",
        "exec",
        node,
        "bitcoin-cli",
        "-regtest",
        f"-rpcuser={user}",
        f"-rpcpassword={password}",
        *args,
    ]
    result = subprocess.run(cmd, capture_output=True, text=True, timeout=10)
    if result.returncode != 0:
        raise RuntimeError(f"bitcoin-cli {args}: {result.stderr}")
    return result.stdout.strip()


def bitcoin_height(node: str = "bitcoin-node01") -> int:
    return int(bitcoin_cli("getblockcount", node=node))
