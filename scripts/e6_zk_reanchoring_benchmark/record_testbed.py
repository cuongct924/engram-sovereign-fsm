#!/usr/bin/env python3
"""Records the multi-host testbed (deploy/): hosts, regions, CPU and memory, clock offsets and the round-trip times between the hosts.

ICMP is not open on the hosts, so a round trip is the TCP connect time (best of five) to a port the target accepts from the source: the
P2P port between validators, the Bitcoin RPC port from validators to infra01, the RPC port from the prover host to node01 and infra01, SSH
from the prover host to its workers. Writes the report to the file given as the first argument.

    ENGRAM_INVENTORY=deploy/ansible/inventory/hosts.yml python3 scripts/e6_zk_reanchoring_benchmark/record_testbed.py <report file>
"""
import json
import os
import subprocess
import sys
import time

INV = os.environ.get("ENGRAM_INVENTORY", os.path.join(os.path.dirname(__file__), "..", "..", "deploy", "ansible", "inventory", "hosts.yml"))
PROBE = ("import socket,sys,time\nb=1e9\nfor _ in range(5):\n s=socket.socket();s.settimeout(3);t=time.time()\n"
         " try:\n  s.connect((sys.argv[1],int(sys.argv[2])));b=min(b,(time.time()-t)*1000)\n except Exception:\n  pass\n s.close()\n"
         "print('%.0f'%b if b<1e9 else 'x')")


def ssh(addr, cmd, timeout=60):
    p = subprocess.run(["ssh", "-n", "-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=accept-new", "-o", "ConnectTimeout=10",
                        f"ubuntu@{addr}", cmd], capture_output=True, text=True, timeout=timeout)
    return p.stdout.strip()


def main():
    out = sys.argv[1]
    inv = json.loads(subprocess.run(["ansible-inventory", "-i", INV, "--list"], capture_output=True, text=True, check=True).stdout)
    hv = inv["_meta"]["hostvars"]
    hosts = []
    for g in ("infra", "prover", "prover_workers", "validators"):
        for h in inv.get(g, {}).get("hosts", []):
            hosts.append({"name": h, "ip": hv[h]["ansible_host"], "pip": hv[h].get("private_ip", ""), "region": hv[h].get("region", ""), "group": g})
    by = {h["name"]: h for h in hosts}
    lines = [f"# Testbed B, recorded {time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime())}", "", "## Hosts"]
    for h in hosts:
        cpu = ssh(h["ip"], "echo $(nproc) vCPU, $(free -g | awk '/Mem:/{print $2}') GB, $(lscpu | grep 'Model name' | sed 's/.*: *//')")
        off = ssh(h["ip"], "chronyc tracking 2>/dev/null | grep 'System time' | sed 's/System time *: *//'")
        lines.append(f"{h['name']:16} {h['region']:15} {h['group']:15} {cpu}; clock {off}")

    def rtt(src, dst, port):
        addr = by[dst]["pip"] if by[src]["region"] == by[dst]["region"] and by[dst]["pip"] else by[dst]["ip"]
        return ssh(by[src]["ip"], f"python3 -c \"{PROBE}\" {addr} {port}") or "?"

    pairs = []
    vals = [h["name"] for h in hosts if h["group"] == "validators"]
    for a in vals:
        for b in vals:
            if a != b:
                pairs.append((a, b, 26656, "validator P2P"))
        pairs.append((a, "infra01", 18443, "Bitcoin RPC"))
    pairs += [("prover01", "engram-node01", 26657, "node01 RPC"), ("prover01", "infra01", 26657, "celestia-app RPC")]
    pairs += [("prover01", w["name"], 22, "worker SSH") for w in hosts if w["group"] == "prover_workers"]
    lines += ["", "## Round trips, ms (TCP connect, best of 5)", "from -> to (port, what): ms"]
    for a, b, port, what in pairs:
        lines.append(f"{a} -> {b} ({port}, {what}): {rtt(a, b, port)}")
    with open(out, "w") as f:
        f.write("\n".join(lines) + "\n")
    print(f"wrote {out}")


if __name__ == "__main__":
    main()
