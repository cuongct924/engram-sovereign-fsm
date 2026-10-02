"""Where the live experiments' fault actions, logs and queries go: the local Docker testnet
(default) or the multi-host deployment under deploy/, selected with ENGRAM_CLUSTER.

    ENGRAM_CLUSTER=docker   (default) containers on this machine, exactly as before
    ENGRAM_CLUSTER=ssh      the hosts in deploy/ansible/inventory/hosts.yml, driven over SSH
    ENGRAM_INVENTORY=...    override the inventory path (ssh mode)

Both backends expose the same fault actions, so a script written against CLUSTER runs on either.
SSH mode needs `ansible-inventory` (installed with ansible-core) to read the inventory, key-based
SSH to every host, and passwordless sudo for non-root users. RPC goes straight to each validator's
public address, so `operator_cidrs` (deploy/README.md) must include the machine running the script.

SSH-mode fault mapping (the Docker backend keeps the Pumba profiles it always used):
  da_stop / da_start    docker stop/start the validator's celestia-bridge container
  anchor_pause          touch/rm the checkpoint-submission pause file in the engramd container
  btc_delay             tc netem on the infra host, only for traffic from Bitcoin's RPC port
  p2p_loss              tc netem on the chosen validators, only for CometBFT P2P (26656) traffic
  wan_*                 no-op: the links are already a real WAN
"""

import json
import os
import shlex
import subprocess
from concurrent.futures import ThreadPoolExecutor
from dataclasses import dataclass
from typing import List, Optional, Sequence

REPO_ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
DEFAULT_INVENTORY = os.path.join(REPO_ROOT, "deploy", "ansible", "inventory", "hosts.yml")
ANCHOR_PAUSE_FILE = "/tmp/anchor_submission_paused"  # ANCHOR_SUBMISSION_PAUSED_FILE in both deployments
P2P_PORT = 26656
BITCOIN_RPC_PORT = 18443


def env_from(proc: subprocess.CompletedProcess, node: str, var: str) -> str:
    """`printenv VAR` exits 1 with no output when VAR is unset; any other failure (unreachable
    host, missing container) must not read as "unset", or a dead node would pass for a default."""
    if proc.returncode != 0 and proc.stderr.strip():
        raise RuntimeError(f"cannot read {var} on {node}: {proc.stderr.strip()}")
    return proc.stdout.strip()


class DockerBackend:
    """The single-machine Docker testnet: docker CLI against local containers, Pumba for netem."""

    name = "docker"

    def __init__(self):
        count = int(os.environ.get("ENGRAM_VALIDATOR_COUNT", "7"))
        self.nodes: List[str] = [f"engram-node{i:02d}" for i in range(1, count + 1)]

    def rpc_host(self, node: str) -> str:
        return "localhost"

    def rpc_port(self, node: str) -> int:
        # engram-node01..NN host-mapped RPC ports: node01=26657, +100 per node.
        return 26657 + (self.nodes.index(node)) * 100

    def exec_in_node(self, node: str, argv: Sequence[str], timeout: float = 30.0) -> subprocess.CompletedProcess:
        return subprocess.run(["docker", "exec", node, *argv], capture_output=True, text=True, timeout=timeout)

    def container_env(self, node: str, var: str) -> str:
        return env_from(self.exec_in_node(node, ["printenv", var], timeout=15), node, var)

    def logs(self, node: str, timeout: float = 30.0) -> str:
        proc = subprocess.run(["docker", "logs", node], capture_output=True, text=True, timeout=timeout)
        return proc.stdout + proc.stderr  # engramd logs to stderr

    def _bridge(self, node: str) -> str:
        return f"celestia-bridge-node{self.nodes.index(node) + 1:02d}"

    def da_stop(self, node: str) -> None:
        subprocess.run(["docker", "stop", self._bridge(node)], capture_output=True, text=True, timeout=30)

    def da_start(self, node: str) -> None:
        subprocess.run(["docker", "start", self._bridge(node)], capture_output=True, text=True, timeout=30)

    def da_stop_all(self) -> None:
        for n in self.nodes:
            self.da_stop(n)

    def da_start_all(self) -> None:
        for n in self.nodes:
            self.da_start(n)

    def anchor_pause(self, on: bool) -> None:
        for n in self.nodes:
            self.exec_in_node(n, ["touch", ANCHOR_PAUSE_FILE] if on else ["rm", "-f", ANCHOR_PAUSE_FILE])

    def btc_delay(self, on: bool) -> None:
        import injector

        if on:
            injector.wait_for_no_active_netem()
            injector.start_pumba_profile("chaos-btc-delay")
        else:
            injector.cleanup_profile("chaos-btc-delay")

    def p2p_loss(self, on: bool, nodes: Optional[Sequence[str]] = None) -> None:
        import injector

        # chaos-loss is a fixed profile (5% on node01/node02), so `nodes` is only honored over SSH.
        if on:
            injector.wait_for_no_active_netem()
            injector.start_pumba_profile("chaos-loss")
        else:
            injector.cleanup_profile("chaos-loss")

    def wan_start(self, profile: str) -> None:
        import injector

        injector.start_pumba_wan_profile(profile)

    def wan_stop(self, profile: str) -> None:
        import injector

        injector.cleanup_wan_profile(profile)

    def wan_resume(self, profile: str) -> None:
        import injector

        injector.wait_for_no_active_netem()
        injector.start_pumba_wan_profile(profile)


@dataclass(frozen=True)
class Host:
    name: str
    address: str
    user: str
    key: Optional[str]


class SshBackend:
    """The deploy/ cluster: every action is `ssh <host> docker ...` or `tc ...` on the right host."""

    name = "ssh"

    def __init__(self, inventory: str):
        self.validators: List[Host]
        self.infra: Host
        self.validators, self.infra = self._load(inventory)
        self.nodes: List[str] = [h.name for h in self.validators]
        self._by_name = {h.name: h for h in self.validators}
        self._iface: dict = {}

    @staticmethod
    def _load(inventory: str):
        if not os.path.exists(inventory):
            raise RuntimeError(f"inventory not found: {inventory} (run deploy/bin/make-inventory.py or copy inventory/example.yml)")
        out = subprocess.run(["ansible-inventory", "-i", inventory, "--list"], capture_output=True, text=True)
        if out.returncode != 0:
            raise RuntimeError(f"ansible-inventory failed (is ansible-core installed?): {out.stderr.strip()}")
        return SshBackend.parse_inventory(json.loads(out.stdout))

    @staticmethod
    def parse_inventory(inv: dict):
        hv = inv["_meta"]["hostvars"]

        def host(name: str) -> Host:
            v = hv[name]
            key = v.get("ansible_ssh_private_key_file")
            return Host(name, v["ansible_host"], v.get("ansible_user", "root"), os.path.expanduser(key) if key else None)

        validators = sorted(inv["validators"]["hosts"], key=lambda n: int(hv[n]["node_index"]))
        infra = inv["infra"]["hosts"]
        if len(infra) != 1:
            raise RuntimeError("the inventory needs exactly one infra host")
        return [host(n) for n in validators], host(infra[0])

    # -- plumbing ---------------------------------------------------------------------------

    def ssh_argv(self, host: Host, remote_cmd: str) -> List[str]:
        argv = [
            "ssh", "-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=accept-new", "-o", "ConnectTimeout=10",
            "-o", "ControlMaster=auto", "-o", "ControlPath=/tmp/engram-ssh-%C", "-o", "ControlPersist=120s",
        ]
        if host.key:
            argv += ["-i", host.key]
        return argv + [f"{host.user}@{host.address}", "sh -c " + shlex.quote(remote_cmd)]

    def run(self, host: Host, remote_cmd: str, timeout: float = 60.0) -> subprocess.CompletedProcess:
        return subprocess.run(self.ssh_argv(host, remote_cmd), capture_output=True, text=True, timeout=timeout)

    @staticmethod
    def sudo(host: Host) -> str:
        return "" if host.user == "root" else "sudo "

    def rpc_host(self, node: str) -> str:
        return self._by_name[node].address

    def rpc_port(self, node: str) -> int:
        return 26657

    # -- the interface shared with DockerBackend ----------------------------------------------

    def exec_in_node(self, node: str, argv: Sequence[str], timeout: float = 30.0) -> subprocess.CompletedProcess:
        h = self._by_name[node]
        return self.run(h, f"{self.sudo(h)}docker exec engramd {shlex.join(argv)}", timeout)

    def container_env(self, node: str, var: str) -> str:
        return env_from(self.exec_in_node(node, ["printenv", var], timeout=20), node, var)

    def logs(self, node: str, timeout: float = 180.0) -> str:
        h = self._by_name[node]
        proc = self.run(h, f"{self.sudo(h)}docker logs engramd 2>&1", timeout)
        return proc.stdout

    def da_stop(self, node: str) -> None:
        h = self._by_name[node]
        self.run(h, f"{self.sudo(h)}docker stop celestia-bridge", 60)

    def da_start(self, node: str) -> None:
        h = self._by_name[node]
        self.run(h, f"{self.sudo(h)}docker start celestia-bridge", 60)

    def _parallel(self, fn, nodes: Sequence[str]) -> None:
        # Near-simultaneous: a DA outage that staggers over seconds is a different fault.
        with ThreadPoolExecutor(max_workers=len(nodes)) as pool:
            list(pool.map(fn, nodes))

    def da_stop_all(self) -> None:
        self._parallel(self.da_stop, self.nodes)

    def da_start_all(self) -> None:
        self._parallel(self.da_start, self.nodes)

    def anchor_pause(self, on: bool) -> None:
        argv = ["touch", ANCHOR_PAUSE_FILE] if on else ["rm", "-f", ANCHOR_PAUSE_FILE]
        self._parallel(lambda n: self.exec_in_node(n, argv), self.nodes)

    # -- netem ------------------------------------------------------------------------------

    def iface(self, host: Host) -> str:
        if host.name not in self._iface:
            out = self.run(host, "ip -o route get 1.1.1.1 | sed -n 's/.* dev \\([^ ]*\\).*/\\1/p'", 20).stdout.strip()
            if not out:
                raise RuntimeError(f"could not detect {host.name}'s default network interface")
            self._iface[host.name] = out
        return self._iface[host.name]

    @staticmethod
    def netem_script(iface: str, sudo: str, netem: str, matches: Sequence[tuple]) -> str:
        """prio qdisc with band 3 delayed/lossy by netem; only flows matching `matches` (a list of
        ("sport"|"dport", port)) are steered into it, everything else (SSH included) is untouched."""
        lines = [
            f"{sudo}tc qdisc del dev {iface} root 2>/dev/null || true",
            f"{sudo}tc qdisc add dev {iface} root handle 1: prio",
            f"{sudo}tc qdisc add dev {iface} parent 1:3 handle 30: netem {netem}",
        ]
        for kind, port in matches:
            lines.append(
                f"{sudo}tc filter add dev {iface} protocol ip parent 1:0 prio 3 u32 match ip {kind} {port} 0xffff flowid 1:3"
            )
        return " && ".join(lines)

    def _netem(self, host: Host, on: bool, netem: str = "", matches: Sequence[tuple] = ()) -> None:
        iface, sudo = self.iface(host), self.sudo(host)
        if on:
            proc = self.run(host, self.netem_script(iface, sudo, netem, matches), 30)
            if proc.returncode != 0:
                raise RuntimeError(f"netem on {host.name} failed: {proc.stderr.strip()}")
        else:
            self.run(host, f"{sudo}tc qdisc del dev {iface} root 2>/dev/null || true", 30)

    def btc_delay(self, on: bool, delay_ms: int = 500, jitter_ms: int = 100) -> None:
        # Bitcoin's responses leave the infra host from its RPC port.
        self._netem(self.infra, on, f"delay {delay_ms}ms {jitter_ms}ms", [("sport", BITCOIN_RPC_PORT)])

    def p2p_loss(self, on: bool, nodes: Optional[Sequence[str]] = None, pct: int = 5) -> None:
        nodes = nodes or self.nodes[:2]  # DockerBackend's chaos-loss targets node01 and node02
        matches = [("sport", P2P_PORT), ("dport", P2P_PORT)]
        self._parallel(lambda n: self._netem(self._by_name[n], on, f"loss {pct}%", matches), list(nodes))

    def wan_start(self, profile: str) -> None:
        print(f"[cluster] ssh backend: skipping {profile}, the links are already a real WAN")

    def wan_stop(self, profile: str) -> None:
        pass

    def wan_resume(self, profile: str) -> None:
        pass


def _make():
    mode = os.environ.get("ENGRAM_CLUSTER", "docker")
    if mode == "docker":
        return DockerBackend()
    if mode == "ssh":
        return SshBackend(os.environ.get("ENGRAM_INVENTORY", DEFAULT_INVENTORY))
    raise RuntimeError(f"ENGRAM_CLUSTER must be docker or ssh, got {mode!r}")


CLUSTER = _make()
