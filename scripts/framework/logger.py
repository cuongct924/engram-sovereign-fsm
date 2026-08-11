"""Live-Docker data-collection framework, replacing tests/e2e's in-process
Harness as the data source for E2/E3/E4/E7/E9 -- the user explicitly asked
for these scripts to read from the REAL running docker testnet
(engram-node01..04, bitcoin-node01, celestia-bridge), not from
tests/e2e/results/*.csv (in-process, mock-sensor Go harness).

Every function here talks to real, already-running services over their real
RPC/ABCI-query interfaces (the same endpoints `engramd query-*` and
CometBFT's own RPC use) -- nothing here mocks or recomputes FSM logic
locally; it only OBSERVES the real chain's already-committed state.
"""

import base64
import json
import subprocess
import time
import urllib.request
import urllib.error
from dataclasses import dataclass, asdict
from typing import List, Optional

# engram-node01..04's host-mapped RPC ports (docker/engram-validator-node0N.yml).
NODE_RPC_PORTS = {
    "engram-node01": 26657,
    "engram-node02": 26757,
    "engram-node03": 26857,
    "engram-node04": 26957,
}

QUERY_STATE_PATH = "/engram.sovereignty.v1.Query/State"


def _rpc_get(port: int, path: str, timeout: float = 3.0) -> dict:
    url = f"http://localhost:{port}{path}"
    with urllib.request.urlopen(url, timeout=timeout) as resp:
        return json.loads(resp.read())


def _decode_query_state(value_b64: str) -> dict:
    """Hand-decodes QueryStateResponse's protobuf wire format (fsm_state:
    field 1 string, safe_blocks: field 2 varint, suspicious_duration: field 3
    varint, reanchoring_proof_valid: field 4 bool, metrics: field 5 message).
    Avoids depending on a generated Python protobuf stub for one small
    message -- this is a deliberately minimal decoder, not a general
    protobuf parser; it only handles the tag/wire-type combinations
    QueryStateResponse actually uses.
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
            # field 5 (metrics) is an embedded message -- deliberately not
            # decoded further: PreBlocker never writes k.Metrics into
            # committed state (documented dead field, see
            # x/sovereignty/preblock.go's NewPreBlocker doc), so its content
            # is always stale/empty regardless.
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
    (routed through /abci_query, the same path engramd's own CLI commands
    use -- see cmd/engramd/reanchor_cli.go's query-recovery-headers for the
    precedent) -- both real, already-committed state, not recomputed here.
    """
    port = port or NODE_RPC_PORTS[node]
    ts = time.time()
    try:
        status = _rpc_get(port, "/status")
        sync_info = status["result"]["sync_info"]
        height = int(sync_info["latest_block_height"])
        app_hash = sync_info["latest_app_hash"]
        catching_up = bool(sync_info["catching_up"])

        abci = _rpc_get(port, f"/abci_query?path=%22{QUERY_STATE_PATH}%22&data=0x")
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
        OSError,  # covers ConnectionResetError/ConnectionRefusedError etc. --
        # a mid-response reset while docker compose force-recreates the node
        # being polled raises these directly from the socket layer, NOT
        # wrapped in urllib.error.URLError the way a failed connection
        # attempt is -- without this, they escape the except clause and
        # crash every long-running poll loop in this framework whenever a
        # container is recreated mid-poll.
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
    independent of the app-layer Query.State gap (PeripheralMetrics.CleanPeers/
    SubnetDiversity/ActiveAnchors are never written into committed state --
    see _decode_query_state's own comment on field 5). This reads the SAME
    real p2p.Switch.Peers() data vanillaP2PHealthAdapter and
    x/sovereignty/keeper/peer_filter.go's FilterPeerByAddr both use, just via
    CometBFT's own /net_info endpoint instead of an app-level ABCI query.
    """
    port = port or NODE_RPC_PORTS[node]
    return _rpc_get(port, "/net_info")


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
    the real ingress filter's view of the world (e.g. for E4/E8's A1/A2
    attacker-swarm experiments) without needing app-level Query.State access.
    """
    info = net_info(node, port)
    counts: dict = {}
    for p in info.get("result", {}).get("peers", []):
        ip = p.get("remote_ip", "")
        if not ip:
            continue
        subnet = _subnet_of(ip)
        counts[subnet] = counts.get(subnet, 0) + 1
    return counts


def bitcoin_cli(*args: str) -> str:
    """Runs bitcoin-cli inside the real bitcoin-node01 container -- used to
    independently cross-check anchor state (OP_RETURN scan) against what a
    manual bitcoin-cli query would show.
    """
    cmd = [
        "docker",
        "exec",
        "bitcoin-node01",
        "bitcoin-cli",
        "-regtest",
        "-rpcuser=cuongct",
        "-rpcpassword=cuongct123",
        *args,
    ]
    result = subprocess.run(cmd, capture_output=True, text=True, timeout=10)
    if result.returncode != 0:
        raise RuntimeError(f"bitcoin-cli {args}: {result.stderr}")
    return result.stdout.strip()


def bitcoin_height() -> int:
    return int(bitcoin_cli("getblockcount"))
