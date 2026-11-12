# Engram Sovereign FSM

A Cosmos SDK + CometBFT prototype implementing **Engram Hybrid Adaptive Consensus** — a modular
consensus protocol that treats peripheral network health (Bitcoin settlement finality, Celestia
data availability, peripheral P2P health) as a first-class consensus variable, degrading gracefully into a
local-PoS "Sovereign" fallback mode instead of halting when those layers fail.

This is the reference implementation of the TLA+ specification in [`spec/core`](spec/core), model-checked
with TLC and Apalache; [`spec/README.md`](spec/README.md) §7 and §9 list which properties are checked and
which are still open. Consensus itself runs on a forked CometBFT core:
**[cuongct924/engram-consensus-core](https://github.com/cuongct924/engram-consensus-core)**.

## Repository Structure

```
x/sovereignty/         FSM engine: state machine, sensors, circuit breaker, ABCI++ hooks
x/da/                  Celestia DA receipt type + verification
x/anchor/              Bitcoin settlement receipt type + anchor re-verification
app/                   EngramApp (real BaseApp wiring)
cmd/engramd/           Node binary -- init/start, testnet bootstrap, CLI tooling
internal/              Re-anchoring prover pipeline; cross-file invariant tests
circuit/               Noir ZK circuits for the re-anchoring recovery proof (leaf, chunk aggregator)
proto/                 Protobuf sources
docker/, docker-compose.yml   Single-host multi-node testnet + Pumba chaos-engineering profiles
deploy/                Terraform + Ansible multi-host (wide-area) testnet
scripts/               Python live-experiment framework (E2-E11)
tests/e2e/             In-process fault-injection harness
spec/                  TLA+ formal specification + TLC/Apalache model-checking configs and results
docs/                  Architecture, development, and experiment documentation
```

## Running the experiments (E1-E11)

`docs/EXPERIMENT.md` is authoritative for what each experiment measures and its real-vs-synthetic
status. This section is the command reference; run `make help` for one-line descriptions of each
target.

**E1 (formal verification)** — TLA+/TLC, no Docker needed:

```bash
make e1-fsm-safety TLA_JAR=/path/to/tla2tools.jar   # or e1-fsm-liveness, e1-tendermint-safety,
                                                     # e1-server-{safety,liveness}
```

`spec/CLAUDE.md` has the per-layer detail and the ablation matrix; results are in `spec/README.md` §9.

**E2-E11 (live Docker testnet):**

1. `cp .env.example .env`, fill in `BITCOIN_RPC_USER`/`BITCOIN_RPC_PASSWORD`.
2. `make testnet-up`.
3. For any `RECOVERING -> ANCHORED` scenario (E2 S7, E3's recovery legs, E9), run
   `make prover-watch` in a separate terminal first.

| Experiment | `make` targets |
|---|---|
| E2 | `e2`, `e2-figures`, `e2-timeline` |
| E3 | `e3-lifecycle`, `e3-p2p-lifecycle`, `e3-recovery`, `e3-latency` |
| E4 | `e4-sybil LEG=a1\|a2`, `e4-churn`, `e4-relay-latency`, `e4-figures` |
| E5 | `e5-spot-check RECOVERY_HYSTERESIS_WAIT=... ENV=...`, `e5-absorb-sweep` |
| E6 | `e6-prover-bench`, `e6-plonky3-bench`, `e6-stats`, `e6-table6c` |
| E7 | `e7-overhead-scan`, `e7-throughput`, `e7-sensor-mismatch`, `e7-vanilla-comparison` |
| E8 | `e8-byzantine ATTACK=...`, `e8-censorship`, `e8-combined`, `e8-double-signing`, `e8-timeout-flood`, `e8-withdrawal` |
| E9 | `e9-trace`, `e9-figures` |
| E10 | `e10-reorg DEPTH=shallow\|deep`, `e10-figures` |
| E11 | `e11-trial MODE=hard-gated\|engram\|soft FAULT=btc\|da\|both`, `e11-matrix`, `e11-summary` |

Two things `make help` won't tell you: E5's spot-check targets need `ENGRAM_PARAM_*` set in
`.env` and a matching `make testnet-up` redeploy *before* running (see the script's own docstring
for the exact value); E9 should run last, after an E2 and an E4 leg are independently validated.

The same experiments run on a multi-host wide-area testnet (7 validator VMs across 3 regions) with
[`deploy/README.md`](deploy/README.md).

## Documentation

- [`spec/README.md`](spec/README.md) — formal spec: protocol design, FSM logic, refinement hierarchy, TLC/Apalache results and open checks.
- [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) — Docker Compose network topology, port allocation, diagrams.
- [`docs/DEVELOPMENT.md`](docs/DEVELOPMENT.md) — build/test/lint, ABCI++ consensus mechanics, multi-node deploy sequence, ZK pipeline.
- [`docs/EXPERIMENT.md`](docs/EXPERIMENT.md) — experiment methodology and results (E1-E11), real vs. synthetic always labeled.
- [`docs/REPRODUCE.md`](docs/REPRODUCE.md) — pinned inputs (consensus fork, embedded verifier) and how to rebuild the node and images.
- [`deploy/README.md`](deploy/README.md) — multi-host deployment on AWS or your own hosts.
- [`circuit/README.md`](circuit/README.md) — ZK re-anchoring circuits: `Proof of Recovery` definition, dynamic padding, opt-in chunk aggregation, benchmarks.

## License

Apache License 2.0, see [`LICENSE`](LICENSE).
