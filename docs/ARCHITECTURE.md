# Infrastructure Architecture

This document describes the REAL, currently-running Docker Compose topology and the ABCI++
mechanism actually implemented -- not an aspirational design. See root `CLAUDE.md`'s "Current
status" section for the authoritative, continuously-updated log of what's built vs. not; this
file is a structural snapshot, refreshed whenever the topology changes materially (last verified
against the live 4-node testnet, `docker/*.yml`, and `compose.yml` directly).

## IP Addressing Scheme

```
Network: 172.28.0.0/24 (engram-net) - Main Validator Network
├── Gateway: 172.28.0.1
├── Monitoring: Prometheus 172.28.0.20, Grafana 172.28.0.21
├── Validators:
│   ├── engram-node01: 172.28.0.100
│   ├── engram-node02: 172.28.0.110
│   ├── engram-node03: 172.28.0.120
│   └── engram-node04: 172.28.0.130
├── Attacker-swarm (profile-gated, docker/attacker-peer-swarm.yml -- E4/E8's A1/A2 live tests):
│   ├── attacker-a1-01..10: 172.28.0.201-210 (A1 leg, peer-slot-exhaustion)
│   └── attacker-a2-*: dynamic (A2 leg -- see "A known Docker quirk" below for why these
│       still show up on 172.28.0.0/24 despite also being attached to their own
│       attacker-subnet-a/b/c/d)
└── Duplicate-key harness (profile-gated, docker/engram-validator-node04-duplicate.yml --
    E8's Double-signing test): 172.28.0.220

Network: 172.21.0.0/24 (bitcoin-net) - Bitcoin Regtest Network [isolated]
├── Gateway: 172.21.0.1
├── bitcoin-node01: 172.21.0.10
├── bitcoin-node02: 172.21.0.11
└── Validators (joined so BITCOIN_HOST=bitcoin-node01 resolves -- see "Why validators are
    multi-homed" below): engram-node01: 172.21.0.100, node02: 172.21.0.110,
    node03: 172.21.0.120, node04: 172.21.0.130

Network: 172.22.0.0/24 (celestia-net) - Celestia DA Layer [isolated]
├── Gateway: 172.22.0.1
├── celestia-app: 172.22.0.50
├── celestia-bridge: 172.22.0.51
└── Validators (joined so CELESTIA_BRIDGE_URL resolves): engram-node01: 172.22.0.100,
    node02: 172.22.0.110, node03: 172.22.0.120, node04: 172.22.0.130

Networks: 172.30-33.0.0/24 (attacker-subnet-a/b/c/d) - E4/E8's A2 live test only
[profile-gated, only exist when docker/attacker-peer-swarm.yml's attacker-swarm-a2 profile
is active -- see "A known Docker quirk" below]
```

**Corrected from an earlier draft of this file**: engram-net was `172.20.0.0/24` originally,
moved to `172.28.0.0/24` after a real collision with a pre-existing `kind` (Kubernetes-in-Docker)
network on the development host (see `CLAUDE.md`'s M6 notes) -- this file previously still said
`172.20.0.0/24`, now fixed. `celestia-light-N` services (previously listed at `172.22.0.100-103`)
were removed entirely at the repo owner's request -- `x/da`'s `Publisher`/`RPCClient` talk to
`celestia-bridge` directly, no light-client hop exists in this deployment. `vigilante-*` sidecar
containers (Submitter/Reporter/checkpointing-monitor) were likewise removed -- the real BTC
anchor mechanism is `x/vigilante.AnchorTracker`, a Go package linked directly into `engramd`
itself (see §3 below), not a separate container.

### Why validators are multi-homed, and a real Docker gateway-priority quirk found live

Each `engram-nodeNN` container joins all three networks above so its hostname-based env vars
(`BITCOIN_HOST=bitcoin-node01`, `CELESTIA_BRIDGE_URL=http://celestia-bridge:26658`) resolve via
Docker's embedded DNS -- `bitcoin-node01`/`celestia-bridge` only exist on `bitcoin-net`/
`celestia-net` respectively, not on `engram-net`.

**A real, confirmed-live Docker behavior worth knowing before debugging P2P connectivity or
subnet-based logic** (`x/sovereignty/keeper/peer_filter.go`'s `FilterPeerByAddr`,
`vanillaP2PHealthAdapter`'s `SubnetDiversity`): for a multi-homed container with no explicit
`gw_priority` set, Docker's default outbound route consistently prefers the network declared
**second** in that service's `networks:` block, not the first. Confirmed via `docker inspect`/
CometBFT's own `/net_info`: the 4 real validators (engram-net declared first, bitcoin-net
second) show up to each other via their `172.21.0.0/24` (bitcoin-net) addresses, not
`172.28.0.0/24` -- despite engram-net being the intended shared P2P subnet. The same pattern
recurred with the A2 attacker-swarm test (own subnet declared first, engram-net added second so
they could reach `engram-node01` at all): they all land on `172.28.0.0/24` too, defeating the
test's original multi-subnet-diversity goal. A partial fix via Compose's `gw_priority` field was
attempted and did not change the observed routing within the time available to debug further --
see `scripts/e4_p2p_eclipse_detection/results_live/sybil_attack_live_run_20260808_summary.md`
for the full writeup. This is a real environmental quirk of this Docker Desktop setup, not a
`FilterPeerByAddr` bug -- the filter correctly enforces its subnet cap regardless of which
subnet peers actually arrive from (demonstrated live: it held at exactly `MaxPeersPerSubnet=8`
in both the A1 and A2 runs).

## Port Allocation

Each validator uses offset ports to avoid host collisions (all containers still listen on the
same in-network ports; only the **host-mapped** ports below differ):

```
engram-node01 (docker/engram-validator-node01.yml):
├── Engram RPC:      26657  (host-mapped, exposed)
├── Cosmos REST API:   1317   (host-mapped, exposed)
└── Prometheus:        26660  (host-mapped, exposed)

engram-node02 (docker/engram-validator-node02.yml)  [offset +100]:
├── Engram RPC:      26757
├── Cosmos REST API:   1417
└── Prometheus:        26760

engram-node03 (docker/engram-validator-node03.yml)  [offset +200]:
├── Engram RPC:      26857
├── Cosmos REST API:   1517
└── Prometheus:        26860

engram-node04 (docker/engram-validator-node04.yml)  [offset +300]:
├── Engram RPC:      26957
├── Cosmos REST API:   1617
└── Prometheus:        26960

Bitcoin regtest (docker/bitcoin-regtest-cluster.yml, isolated on bitcoin-net):
├── bitcoin-node01 RPC: 18443 (host-mapped)
└── bitcoin-node02 RPC: 19443 (host-mapped)

Celestia (docker/celestia-local-cluster.yml, isolated on celestia-net):
├── celestia-app RPC:    26658 (host-mapped)
├── celestia-app gRPC:   9090  (host-mapped)
└── celestia-bridge RPC: 26658 (host-mapped, distinct container from celestia-app despite the
                                 same in-network port number)
```

There is no "Celestia Light RPC" port per validator (an earlier draft of this file listed one)
-- no light-client hop exists in this deployment, see the removal note above.

## 1. The Sensor Architecture (real, not mock -- as of Phase 7 and this session's work)

An earlier draft of this document described sensors as "mock-controlled for testing" and
external infrastructure as "not yet wired". **That is no longer accurate.** As of Phase 7
(documented extensively in `CLAUDE.md`'s "Current status") and this session's Part 1b:

* **BTC** (`x/vigilante`): `AnchorTracker`, a Go package linked directly into `engramd`
  (`cmd/engramd/main.go`'s `wireBTCSensor`), talks to a real `bitcoind` regtest node via JSON-RPC
  (`x/vigilante/rpc.go`) -- submits real OP_RETURN checkpoint transactions, tracks real
  confirmation depth. No separate "Vigilante Submitter/Reporter" container exists; that was
  scaffolding removed at the repo owner's request (this app has no staking module to source a
  real Babylon-style BLS-aggregated checkpoint from).
* **DA** (`x/da`): `Publisher`, likewise linked directly into `engramd` (`wireDASensor`), talks
  to a real `celestia-bridge` via JSON-RPC 2.0 (`x/da/rpc.go`) -- submits real blobs, confirms
  real retrievability via `blob.GetAll`.
* **P2P** (`cmd/engramd/main.go`'s `vanillaP2PHealthAdapter`): reads real, live data from the
  actually-running `*p2p.Switch` (`SubnetDiversity`, `ActiveAnchors`, `CleanPeers`, `ChurnRate`,
  `AvgTenure`) plus, as of this session's Part 1b, real per-peer RTT -- piggybacked on
  `MConnection`'s existing `PacketPing`/`PacketPong` keep-alive exchange
  (`engram-consensus-core`'s `p2p/conn/connection.go`), not a new reactor.
* **Ingress filter** (`x/sovereignty/keeper/peer_filter.go`'s `FilterPeerByAddr`, this session's
  Part 1a): an ACTIVE defense, not just passive detection -- rejects a new peer connection
  outright (via `baseapp.SetAddrPeerFilter`, a stock Cosmos SDK ABCI hook, no fork changes
  needed) if admitting it would push its `/24` (or `/48` for IPv6) subnet's connected-peer count
  past `Params.MaxPeersPerSubnet`. Demonstrated live against a real 10-22-container attacker
  swarm, see the results file cited above.
* **Verification layer** (`x/da`, `x/vigilante`'s `VerifyReceipt` functions): stateless
  verification packages, not full Cosmos SDK modules with their own keeper/KVStore -- they port
  `IsValidProposal`'s DA-pipeline and BTC-settlement checks
  (`spec/core/EngramTendermint.tla:290-298`) directly, called from `x/sovereignty/proposal.go`'s
  `ProcessProposal` handler.

## 2. Consensus-Layer Integration (as implemented -- NOT ABCI++ Vote Extensions)

An earlier draft of this document described `ExtendVote`/`VerifyVoteExtension` wiring. **That was
never built and is not the current design.** The actual mechanism (`x/sovereignty/proposal.go`,
`preblock.go`):

* **`PrepareProposal`:** the leader computes `target_state` via `keeper.CalculateNextState`
  (mirrors `ServerInsertProposal`, `spec/core/EngramServer.tla:52-102`), builds
  `da_receipt`/`btc_receipt`/`zk_proof_ref`, JSON-encodes them as an `ExtendedProposal`, and
  prepends it as `Txs[0]` of the block -- a leading pseudo-transaction, not a vote extension.
  `zk_proof_ref` is a hash commitment (`rt_new`, the accepted proof's attested state root) as of
  this session's Part 3, not a bare bool -- an audit-traceability refinement of
  `spec/core/EngramTendermint.tla:150`'s abstract `BOOLEAN`, safe because no proved invariant
  depends on the proof's identity, only its presence (see `spec/README.md` §6.1's refinement
  note).
* **`ProcessProposal`:** every validator decodes `Txs[0]` and re-validates it against its own
  local `CalculateNextState` + `x/da.VerifyReceipt` + `x/vigilante.VerifyReceipt` + the
  withdrawal circuit breaker (mirrors `IsValidProposal`, `spec/core/EngramTendermint.tla:281-307`),
  rejecting the whole proposal on any mismatch. If `ENGRAM_BYZANTINE_BEHAVIOR` is set on a
  validator (this session's Part 4a, `docker/engram-validator-node04-byzantine.yml` -- never set
  on a real validator), `PrepareProposal` deliberately corrupts its own claim (fake `fsm_state`,
  forged BTC hash, false DA attestation, or censored tx) to exercise this rejection path live,
  for docs/EXPERIMENT.md's E8 attack-resilience rows.
* **`PreBlocker`:** once the block is decided, this is the only place `FSMState`/`safe_blocks`/
  `suspicious_duration`/the tracked heights are actually written, using the *agreed-upon* `Txs[0]`
  (mirrors `ServerUponProposalInPrecommitNoDecision`, `spec/core/EngramServer.tla:135-189`) --
  never recomputed locally at this point. As of this session's Part 4c, it also reads
  `RequestFinalizeBlock.Misbehavior` (CometBFT's own, stock evidence-pool report of detected
  double-signing) and commits it to queryable state (`Keeper.DetectedEvidenceCount`/
  `LastDetectedEvidence`) -- safe to commit because `Misbehavior` is part of the already-agreed
  block request itself, deterministic and identical across every honest validator, unlike a
  fresh local sensor read (see `x/sovereignty/preblock.go`'s own doc for the real
  AppHash-divergence bug that lesson came from, and why raw P2P/BTC/DA sensor VALUES are
  deliberately never committed the same way -- `Query.State`'s metrics field stays the
  last-committed value, not a live re-read).

This mechanism was a deliberate simplification to avoid needing a full TxConfig/signing pipeline
for leader-computed system data. See `x/sovereignty/proposal.go`'s package comment for the
tradeoff and what would be needed to move to real Vote Extensions.

## 3. Live-Docker Attack/Fault-Injection Infrastructure (docs/EXPERIMENT.md's E2-E9)

Beyond the 4-validator + bitcoin + celestia core, several profile-gated compose services exist
purely for live experiment data collection -- never started by a plain `docker compose up`:

* **Pumba chaos profiles** (`compose.yml`'s `pumba-*` services): `chaos-delay`/`chaos-loss`/
  `chaos-crash`/`chaos-eclipse` (network delay/loss/kill/isolate on `engram-node*`), plus
  `chaos-btc-delay` (delay on `bitcoin-node01`, added this session for E2's S2/E9).
* **`docker/attacker-peer-swarm.yml`** (this session's Part 2, E4/E8's A1/A2): real, non-validator
  `engramd` full-node containers whose only job is to dial a target validator aggressively.
* **`docker/engram-validator-node04-byzantine.yml`** (Part 4a, E8's A3/A4/A6/A7): an override
  applied via `docker compose -f compose.yml -f docker/engram-validator-node04-byzantine.yml`
  (never included by default) that adds `ENGRAM_BYZANTINE_BEHAVIOR` to node04's real environment.
* **`docker/engram-validator-node04-duplicate.yml`** (Part 4c, E8's Double-signing): a second
  process holding node04's real signing key but a deliberately separate
  `priv_validator_state.json`, to produce a genuine double-sign for CometBFT's stock evidence
  pool to detect.

`scripts/framework/{logger,injector}.py` are the shared Python primitives every live-data
collection script in `scripts/e*/` builds on. See `docs/EXPERIMENT.md` for what each experiment
group actually measures and `scripts/*/results_live/` for real, dated output.
