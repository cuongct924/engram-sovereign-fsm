# Multi-host deployment (7 validators across regions)

Deploys the Engram testnet to 7 validator VMs plus 1 infrastructure VM, so the experiments run on
real wide-area links instead of one machine's Docker network. It answers the reviewer question that
7 validators + Bitcoin + Celestia on one physical machine raises.

Bitcoin and Celestia stay a **private regtest Bitcoin node and a single-validator `celestia-app`
chain**, as in the single-host testnet. That is deliberate: the experiments inject Bitcoin and DA
faults, which is impossible on mainnet, and mainnet would cost real BTC/TIA per checkpoint and blob.
Only the validator network is real WAN.

## Layout

```
infra01          Bitcoin regtest node + steady miner, celestia-app, ZK re-anchoring prover
engram-node01..07  engramd + that validator's own Celestia bridge (independent DA observation)
```

Every validator reads Bitcoin from `infra01` over the WAN and publishes/reads DA through its local
bridge, which attaches to `celestia-app` on `infra01`.

```
deploy/
├── terraform/{hetzner,digitalocean,gcp,aws}/   one stack per provider, identical `inventory` output
├── bin/make-inventory.py                       terraform output -> ansible inventory
└── ansible/                                    provider-independent; works on any 8 SSH-reachable Linux hosts
    ├── site.yml  teardown.yml  status.yml
    ├── inventory/example.yml
    ├── group_vars/all.yml                      every tunable, documented in place
    └── roles/{controller,common,infra,bridge,engramd}
```

Terraform is optional. Ansible needs only an inventory, so any cloud, bare metal or an academic
testbed (CloudLab, Chameleon) works: copy `inventory/example.yml` to `inventory/hosts.yml`.

## Prerequisites

On the machine you run this from (the *controller*, which is also where experiments run):

- Go (to build `engramd` and generate genesis), `ansible-core` >= 2.15, `tar`, and Terraform >= 1.5 if
  you use the stacks.
- The `engram-consensus-core` fork checked out next to this repo (`go.mod`'s `replace`, see
  `CLAUDE.md`), or set `cometbft_fork_dir`.
- An SSH key pair. Hosts: Ubuntu 22.04/24.04 or Debian 12, amd64, public IPv4.

Sizing (the numbers the stacks default to):

| Host | vCPU / RAM | Why |
|---|---|---|
| each validator | 2 / 4 GB | engramd and a Celestia bridge |
| `infra01` | 4–8 / 16 GB | Bitcoin, celestia-app, and the prover (bb UltraHonk OOM-kills below ~8 GB). Set `prover_enabled: false` to run without it; RECOVERING then never reaches ANCHORED. |

## Deploy

**1. Provision (skip if you bring your own hosts).** Pick a stack, authenticate as its provider expects
(`HCLOUD_TOKEN`, `DIGITALOCEAN_TOKEN`, `gcloud auth application-default login`, `AWS_*`):

```bash
cd deploy/terraform/hetzner                 # or digitalocean | gcp | aws
cp terraform.tfvars.example terraform.tfvars   # set operator_cidrs and the regions
terraform init && terraform apply
../../bin/make-inventory.py .               # writes ansible/inventory/hosts.yml
```

Each stack places the 7 validators in the regions of its `validator_*` variable (defaults span
Europe, North America and Asia-Pacific) and opens SSH to `operator_cidrs` only. Stack-specific notes:

- **Hetzner** also restricts every service port to the 8 hosts at the cloud firewall.
- **DigitalOcean, GCP, AWS** open the service ports at the cloud layer; the Ansible `common` role then
  restricts each port to the peer IPs with `ufw` on every host.
- **AWS** regions are four fixed slots (`regions`, `validator_slots`) because a provider's region is
  fixed per provider block; point two slots at one region to use fewer.
- **Bring your own hosts:** edit `ansible/inventory/hosts.yml` from `example.yml`. `ansible_host` is the
  address *other* hosts use to reach that machine; `node_index` is 1..N without gaps.

**2. Configure.** Edit `ansible/group_vars/all.yml` or pass `-e`. The required ones:

```bash
export BITCOIN_RPC_PASSWORD='choose-a-long-random-password'
export BITCOIN_RPC_USER=engram_admin                 # optional
# operator_cidrs: who may reach validator RPC (26657) and the infra Bitcoin RPC (18443)
```

Pass `-e '{"operator_cidrs":["198.51.100.7/32"]}'` or set it in `group_vars/all.yml`. Also choose the
system under test with `-e engram_mode=engram|hard-gated|soft` (see `docs/EXPERIMENT.md`, E11).

**3. Deploy.**

```bash
cd deploy/ansible
ansible-playbook site.yml -e '{"operator_cidrs":["198.51.100.7/32"]}'
```

It runs, in order: build `engramd` and generate one genesis and 7 key sets locally, install Docker and
the firewall on every host, start Bitcoin + the miner + `celestia-app`, start the 7 bridges, **fund
each bridge account one at a time**, start the 7 validators, then start the prover. The first run
builds Docker images on each host (several minutes; the prover image also compiles the Noir
circuits). Re-running is safe and resumes after a failure.

Only each validator's own key directory is copied to its host. `deploy/.generated/` on the controller
holds all 7 private keys: keep it private and out of git (it is ignored).

**4. Verify.**

```bash
ansible-playbook status.yml      # height, catching_up and peer count per validator; Bitcoin height
```

Healthy: every validator at the same, advancing height with 6 peers, `catching_up=false`. Then check
that the FSM reaches ANCHORED (engram/soft modes):

```bash
curl -s "http://<validator-ip>:26657/abci_query?path=%22/engram.sovereignty.v1.Query/State%22&data=0x" | jq .
```

or use `scripts/framework/logger.py`'s `query_node`.

## Redeploying and tearing down

A fresh chain always needs a fresh genesis: a persisted `priv_validator_state.json` without a matching
WAL trips FilePV's anti-double-sign guard (`CLAUDE.md`).

```bash
ansible-playbook teardown.yml -e wipe_generated=true     # stops containers, deletes volumes, node homes and genesis
ansible-playbook site.yml ...                            # fresh deploy
```

`terraform destroy` removes the VMs. Stop paying for idle VMs between experiment sessions.

## WAN specifics that change results

- **P2P latency bound.** `ENGRAM_PARAM_MAX_PEER_LATENCY` defaults to 200 ms, below real inter-continent
  RTT, so a WAN cluster would report unhealthy P2P and never stay ANCHORED. `group_vars/all.yml` sets
  500 ms in `engram_params` (baked into genesis). Report this value beside any WAN result; it is a
  deployment difference from the single-host numbers.
- **Bitcoin RPC timeout.** `x/anchor` gives each Bitcoin RPC call 800 ms (`rpcTimeout`), sized for
  regtest on one machine. Validators call `infra01` across the WAN, so place `infra01` in the most
  central region and keep every validator's RTT to it well under 800 ms, or the BTC sensor reports
  failures that are artefacts of the placement.
- **Clock sync.** `chrony` is enabled on every host; block times are medians of validator clocks.
- **Infra is a single point.** One host serves Bitcoin and `celestia-app` to all validators, as in the
  single-host testnet; the per-validator bridges keep DA observations independent, Bitcoin's are not.

## Running the experiments on this cluster

The fault-injection scripts talk to the cluster through `scripts/framework/cluster.py`. Select the
deployment with environment variables; without them everything runs on the local Docker testnet as before.

```bash
export ENGRAM_CLUSTER=ssh                       # default: docker
# export ENGRAM_INVENTORY=...                   # default: deploy/ansible/inventory/hosts.yml
python3 -u scripts/e9_composite_fault/live_combined_trace.py     # the composite fault
make e11-trial MODE=engram FAULT=da TRIALS=3                     # RQ2 baseline comparison
```

Needs on the controller: `ansible-inventory` (ansible-core), key-based SSH to every host (the
inventory's `ansible_ssh_private_key_file`), passwordless sudo for non-root users, and an
`operator_cidrs` entry for this machine (RPC goes straight to each validator's public address).
For `make e11-trial`, `MODE` must match the `engram_mode` the cluster was deployed with; the script
checks and refuses otherwise.

| Fault | SSH-mode implementation |
|---|---|
| DA outage | `docker stop celestia-bridge` on all validators at once |
| Checkpoint-submission pause | `touch` the pause file in each `engramd` container |
| BTC congestion (`btc_delay`) | `tc netem delay 500 ms ± 100` on `infra01`, only for traffic from Bitcoin's RPC port |
| P2P churn (`p2p_loss`) | `tc netem loss 5%` on node01/node02, only for CometBFT P2P (26656) traffic |
| WAN-realism profile | skipped: the links are already a real WAN (`--wan-profile` defaults to `none`) |

Scripts that are not yet routed through `CLUSTER` (E2, E3, E4, E5, E7, E8, E10 and the sensor-mismatch
and throughput scripts) still use `docker`/Pumba directly and only work on the local testnet. E9 and
E11 are the ones the multi-host question needs.

## What was verified

The Ansible playbooks pass `--syntax-check`; the controller role was run end to end (builds `engramd`,
generates the 7-validator genesis with the public addresses and 500 ms latency bound, archives the
sources); both compose templates render and pass `docker compose config`; all four Terraform stacks
pass `terraform validate`. **Nothing has been applied to a real cloud account or run on real hosts**,
so expect to fix provider- or distribution-specific details on the first live deploy. The SSH adapter's
command construction, inventory parsing and failure handling are tested against the example inventory
with a stubbed `ssh`; no real remote command has been executed.

## Troubleshooting

| Symptom | Cause and fix |
|---|---|
| Validators stuck in SOVEREIGN right after deploy | A bridge account is unfunded or `celestia-app` is unreachable: `docker logs celestia-bridge` on the host, rerun `ansible-playbook site.yml --tags fund`. |
| A bridge crash-loops on first start | It lost the one-time `TrustedHash` race (`CLAUDE.md`): `docker compose down -v` in `/opt/engram/node` on that host, then `site.yml --tags bridge,fund,engramd`. |
| `engramd` exits with a double-sign guard error | Stale `priv_validator_state.json` after a partial redeploy: run `teardown.yml -e wipe_generated=true`. |
| Peer count below 6 | Firewall: `ufw status` on both ends, and the cloud firewall; `ansible_host` must be the address peers dial. |
| Heights advance but FSM leaves ANCHORED | A validator's RTT to `infra01` or to its peers exceeds the bounds above. |
