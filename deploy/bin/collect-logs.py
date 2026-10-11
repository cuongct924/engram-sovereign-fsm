#!/usr/bin/env python3
"""Pulls the container logs of every host of the deployment into a local directory, before the hosts are destroyed.

    deploy/bin/collect-logs.py <output dir>          # uses deploy/ansible/inventory/hosts.yml, or ENGRAM_INVENTORY

Writes <output dir>/<host>/<container>.log (the last 20,000 lines of each container, stdout and stderr). Run it after a failure and
before every `terraform destroy`: the logs are lost with the hosts. Stdlib only.
"""
import json
import os
import subprocess
import sys

INV = os.environ.get("ENGRAM_INVENTORY", os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "ansible", "inventory", "hosts.yml"))
SSH = ["ssh", "-n", "-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=accept-new", "-o", "ConnectTimeout=10"]


def run(addr, cmd, timeout=180):
    return subprocess.run(SSH + [f"ubuntu@{addr}", cmd], capture_output=True, text=True, timeout=timeout)


def main() -> int:
    if len(sys.argv) != 2:
        print(__doc__, file=sys.stderr)
        return 2
    out = sys.argv[1]
    listing = subprocess.run(["ansible-inventory", "-i", INV, "--list"], capture_output=True, text=True, check=True)
    inv = json.loads(listing.stdout)
    hosts = inv["_meta"]["hostvars"]
    errors = 0
    for name, v in sorted(hosts.items()):
        names = run(v["ansible_host"], "sudo docker ps -a --format '{{.Names}}'")
        if names.returncode != 0:
            print(f"{name}: cannot list containers: {names.stderr.strip()[:200]}", file=sys.stderr)
            errors += 1
            continue
        os.makedirs(os.path.join(out, name), exist_ok=True)
        for c in names.stdout.split():
            log = run(v["ansible_host"], f"sudo docker logs --tail 20000 {c} 2>&1")
            with open(os.path.join(out, name, f"{c}.log"), "w") as f:
                f.write(log.stdout)
        print(f"{name}: {len(names.stdout.split())} container logs")
    return 1 if errors else 0


if __name__ == "__main__":
    sys.exit(main())
