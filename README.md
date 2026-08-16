# Engram Sovereign FSM

A Cosmos SDK + CometBFT prototype implementing **Engram Hybrid Adaptive Consensus** — a modular
consensus protocol that treats peripheral network health (Bitcoin settlement finality, Celestia
data availability, P2P health) as a first-class consensus variable, degrading gracefully into a
local-PoS "Sovereign" fallback mode instead of halting when those layers fail.

This is the reference implementation of the formal specification in [`spec/core`](spec/core) (TLA+, with
TLC/Apalache model-checked safety and liveness proofs). Consensus itself runs on a forked
CometBFT core: **[cuongct924/engram-consensus-core](https://github.com/cuongct924/engram-consensus-core)**.

## Repository Structure

```
x/sovereignty/         FSM engine: state machine, sensors, circuit breaker, ABCI++ hooks
x/da/                  Celestia DA receipt type + verification
x/anchor/              Bitcoin settlement receipt type + SPV verification
app/                   EngramApp (real BaseApp wiring)
cmd/engramd/           Node binary -- init/start, testnet bootstrap, CLI tooling
circuit/               Noir ZK circuit for the re-anchoring recovery proof
proto/                 Protobuf sources
docker/, compose.yml   Multi-node local testnet + Pumba chaos-engineering profiles
scripts/               Python live-experiment framework (E2-E10)
tests/e2e/             In-process fault-injection harness
spec/                  TLA+ formal specification + model-checking proofs
docs/                  Architecture, development, and experiment documentation
```

## Documentation

- [`spec/README.md`](spec/README.md) — formal spec: protocol design, FSM logic, refinement hierarchy, TLC/Apalache proofs.
- [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) — Docker Compose network topology, port allocation, diagrams.
- [`docs/DEVELOPMENT.md`](docs/DEVELOPMENT.md) — build/test/lint, ABCI++ consensus mechanics, multi-node deploy sequence, ZK pipeline.
- [`docs/EXPERIMENT.md`](docs/EXPERIMENT.md) — experiment methodology and results (E1-E10), real vs. synthetic always labeled.
- [`circuit/README.md`](circuit/README.md) — ZK re-anchoring circuit: `Proof of Recovery` definition, dynamic-padding design, aggregation spike results.
