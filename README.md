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
x/vigilante/           Bitcoin settlement receipt type + SPV verification
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

## System Architecture

![System Architecture](docs/architecture.png?v=1)

See [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) for the full technical breakdown: real network
topology, sensor wiring, and the `PrepareProposal` / `ProcessProposal` / `PreBlocker` consensus
flow (with diagrams).

## Quick Start

```bash
go build ./... && go test ./...
docker compose up -d --build   # 4-node testnet + Bitcoin regtest + Celestia
```

The full build/test/lint workflow, the real multi-node Docker deploy sequence (including the
operational ordering that actually matters), and the ZK re-anchoring pipeline are documented in
[`docs/DEVELOPMENT.md`](docs/DEVELOPMENT.md) — written so anyone can run and review this from a
clean checkout.

## Experiment Results

Real measurements from the live 4-node Docker testnet, the in-process fault-injection harness, and
the real Noir/Barretenberg proving pipeline -- not synthetic placeholders. Full methodology, raw
tables, and which numbers are real vs. synthetic (always labeled) are in
[`docs/EXPERIMENT.md`](docs/EXPERIMENT.md); this is just the visual summary. Click a thumbnail for
the full-resolution vector PDF.

> **If you regenerate any figure below and keep the same filename**, GitHub/browsers will keep
> serving the old cached image at that URL. Bump the `?v=N` suffix on that image's line (both the
> thumbnail and its PDF link) by 1 -- that's the only way a same-filename update reliably shows up.

### E1 — Formal Verification (TLA+ / Apalache stress & ablation)
Table only -- see [`docs/EXPERIMENT.md`](docs/EXPERIMENT.md).

### E2 — Fault-Injection End-to-End (S1-S7 scenarios, live)
[![Figure 3](scripts/e2_fault_injection/results/figure3_state_timelines_live.png?v=1)](scripts/e2_fault_injection/results/figure3_state_timelines_live.pdf?v=1)

### E3 — Failure Matrix
Table only -- see [`docs/EXPERIMENT.md`](docs/EXPERIMENT.md).

### E4 — P2P Eclipse / Sybil Detection
Table only -- see [`docs/EXPERIMENT.md`](docs/EXPERIMENT.md).

### E5 — Hysteresis Sensitivity (live)
[![Figure 4](scripts/e5_hysteresis_flapping/results/figure4_hysteresis_live.png?v=1)](scripts/e5_hysteresis_flapping/results/figure4_hysteresis_live.pdf?v=1)

### E6 — ZK Re-Anchoring Proof Scaling (Noir + UltraHonk)
[![Figure 6](scripts/e6_zk_reanchoring_benchmark/results/figure6_scaling.png?v=1)](scripts/e6_zk_reanchoring_benchmark/results/figure6_scaling.pdf?v=1)

### E7 — Consensus Overhead of the Extended Proposal
Table only -- see [`docs/EXPERIMENT.md`](docs/EXPERIMENT.md).

### E8 — Attack Resilience (A1-A8 + double-signing)
Table only -- see [`docs/EXPERIMENT.md`](docs/EXPERIMENT.md).

### E9 — Trace-Driven Combined-Failure Stress Test (live)
[![Figure 2](scripts/e9_trace_driven/results/figure2_trace_timeline_live.png?v=1)](scripts/e9_trace_driven/results/figure2_trace_timeline_live.pdf?v=1)
