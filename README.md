# Engram Sovereign FSM

A Cosmos SDK + CometBFT prototype implementing **Engram Hybrid Adaptive Consensus** — a modular
consensus protocol that treats peripheral network health (Bitcoin settlement finality, Celestia
data availability, P2P health) as a first-class consensus variable, degrading gracefully into a
local-PoS "Sovereign" fallback mode instead of halting when those layers fail.

This is the reference implementation of the formal specification in [`spec/`](spec/) (TLA+, with
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
scripts/               Python live-experiment framework (E2-E9)
tests/e2e/             In-process fault-injection harness
spec/                  TLA+ formal specification + model-checking proofs
docs/                  Architecture, development, and experiment documentation
```

## Documentation

- [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) — system architecture: real network topology,
  sensor wiring, the `PrepareProposal` / `ProcessProposal` / `PreBlocker` consensus flow, diagrams.
- [`docs/DEVELOPMENT.md`](docs/DEVELOPMENT.md) — build/test/lint workflow, the real multi-node
  Docker deploy sequence (including the operational ordering that actually matters), the ZK
  re-anchoring pipeline — written so anyone can run and review this from a clean checkout.
- [`docs/EXPERIMENT.md`](docs/EXPERIMENT.md) — full experiment methodology, raw results, and
  figures (E1-E9): real measurements from the live 4-node Docker testnet, the in-process
  fault-injection harness, and the real Noir/Barretenberg proving pipeline, with which numbers are
  real vs. synthetic always labeled.
