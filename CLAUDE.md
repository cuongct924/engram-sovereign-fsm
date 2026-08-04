# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this repository is

A Cosmos SDK + CometBFT prototype implementing **Engram Hybrid Adaptive Consensus** — a modular blockchain consensus protocol that treats peripheral network health (Bitcoin settlement finality, Celestia data availability, P2P health) as a first-class consensus variable, degrading gracefully into a local-PoS "Sovereign" fallback mode instead of halting when those layers fail.

The formal specification and its TLC/Apalache model-checking proofs live in `spec/` (see `spec/CLAUDE.md` for that subtree's own guidance — it is a separate, self-contained deliverable). **This root-level code is the reference implementation of that spec**: every non-trivial piece of Go logic here is a direct port of a named operator in `spec/core/*.tla`, and that traceability is load-bearing, not decorative — see "Spec fidelity" below.

## Repository layout

```
x/sovereignty/     -- the FSM engine (Cosmos SDK module): FSM state machine, sensors,
                       circuit breaker, ABCI++ hooks (PrepareProposal/ProcessProposal/PreBlocker)
x/da/               -- Celestia DA receipt type + verification (IsValidProposal's DA pipeline check)
x/vigilante/        -- Bitcoin settlement receipt type + SPV verification
app/                -- EngramApp (BaseApp wiring) -- minimal today, see "Current status" below
cmd/engramd/        -- node binary entrypoint
proto/              -- protobuf sources (buf); generated .pb.go land in x/sovereignty/types/
circuit/reanchoring/ -- Noir ZK circuit for the re-anchoring recovery proof
tests/e2e/          -- in-process fault-injection harness + scenarios (docs/EXPERIMENT.md's E2)
tests/benchmark/, tests/mbt/ -- stubs, not yet implemented
docker/, compose.yml -- multi-node local testnet + Pumba chaos-engineering profiles (has known bugs, see M6)
scripts/            -- Python orchestration for E2-E9 experiments (docs/EXPERIMENT.md); mostly stubs today
docs/EXPERIMENT.md  -- the experiment design this codebase is built to satisfy (E1-E9); read this
                       before adding any new experiment-facing code
docs/ARCHITECTURE.md -- STALE, do not use as a design source (confirmed by the repo owner)
```

## Spec fidelity (the most important convention in this repo)

Every operator ported from `spec/core/*.tla` keeps its TLA+ name (or the closest Go-idiomatic
variant) and cites the source file+line range in a comment, e.g.:

```go
// IsBTCGapSuspicious mirrors IsBTCGapSuspicious: SUSPICIOUS_THRESHOLD <= btc_gap < SOVEREIGN_THRESHOLD.
func IsBTCGapSuspicious(m *PeripheralMetrics, p Params) bool { ... }
```

When touching FSM/consensus logic:
- **Read the cited spec lines first.** Do not "improve" or simplify a branch structure relative to the spec — a CASE/switch's branch *order* is often safety-relevant (see `x/sovereignty/keeper/circuit_breaker.go`'s `CalculateNextState`, which was previously buggy exactly because an earlier version added a branch not present in `CalculateNextFSMState`, violating `StrictFSMTransitionSafety`).
- **Reuse ported functions, don't reimplement.** `keeper.CalculateNextState`, `types.Is*Condition` (`x/sovereignty/types/predicates.go`), `da.VerifyReceipt`, `vigilante.VerifyReceipt` are the single source of truth for their respective spec operators — every ABCI hook (`PrepareProposal`/`ProcessProposal`/`PreBlocker` in `x/sovereignty/proposal.go` and `preblock.go`) calls into them rather than recomputing.
- **"Sensors propose, consensus decides."** FSM state is never written based purely on a node's own local sensor reading. The leader computes a target state in `PrepareProposal`, other validators cross-check it in `ProcessProposal` against their own local computation (`IsValidProposal`, `EngramTendermint.tla:281-307`), and only `PreBlocker`/`CommitFSMTransition` (mirroring `ServerUponProposalInPrecommitNoDecision`, `EngramServer.tla:135-189`) actually persists the agreed-upon state. `x/sovereignty/abci.go`'s `BeginBlocker` is a *separate*, simplified path used only by the in-process E2 test harness (`tests/e2e/`) — it recomputes state from local sensors directly, which is intentionally NOT how the real ABCI++ path works.
- **Simplifications are commented, not silent.** Where this Go port deliberately falls short of full spec fidelity (e.g. `x/sovereignty/proposal.go` uses round=0 for DA/BTC tolerance since vanilla ABCI 2.0 doesn't expose consensus round to `PrepareProposal`/`ProcessProposal`), the comment says so and why, and states what would remove the gap.

## Build, test, lint

```bash
go build ./...
go vet ./...
go test ./...              # or: make test
golangci-lint run          # or: make lint (config: .golangci.yml, schema v2 -- needs golangci-lint v2+)
```

`go vet` must stay clean. Proto-generated types (`x/sovereignty/types/*.pb.go`) embed a `sync.Mutex` via
`protoimpl.MessageState` — always pass `*PeripheralMetrics` (pointer), never `PeripheralMetrics` (value),
through function signatures, struct fields, or `collections.Item[...]`.

### Regenerating protobuf code

```bash
make proto-gen
```

Requires `buf`, `protoc-gen-go`, `protoc-gen-go-grpc` on `PATH`. The proto package
(`engram.sovereignty.v1`, under `proto/engram/sovereignty/v1/`) intentionally differs from the Go
package (`x/sovereignty/types`) — `make proto-gen` generates into a `.tmp-proto-gen/` staging
directory (mirroring the proto package path) and copies the output into place; do not hand-edit
`buf.gen.yaml`'s `out:`/`paths:` without preserving that copy step (see the Makefile recipe's
comment for why `paths=import` alone doesn't work here).

## Current status (read before assuming something is wired up)

Phases 0-4 and milestones M1-M4 are done: the module builds and tests clean, includes real
`PrepareProposal`/`ProcessProposal`/`PreBlocker` handler *logic*, and the in-process E2
fault-injection harness (`tests/e2e/`) produces real experiment data. **Not yet done**, tracked as
later milestones (ask the repo owner for the current plan file if picking this up cold):
- **M5**: `app/app.go` is a minimal placeholder (no real `BaseApp` construction, no `SetPrepareProposal`/
  `SetProcessProposal`/`SetPreBlocker` registration yet) -- the M4 handlers exist but aren't wired to
  a runnable node.
- **M6**: `docker/`'s multi-node testnet has known bugs (missing `docker/monitoring-services.yml`,
  container name mismatches like `stratium-node01` vs `stratium-node-01` referenced elsewhere).
- **M0a-d**: a forked CometBFT core (`github.com/cuongct924/engram-consensus-core`, cloned at
  `/Users/cuongct090_04/Code/engram-consensus-core`) needs real changes for P2P health telemetry,
  f+1-timeout quorum round-skip, FSM-state-dependent fork choice, and forced-inclusion censorship
  resistance -- none of vanilla ABCI 2.0 exposes what these need.
- **M7**: `scripts/`'s Python orchestration (Pumba chaos injection against a real multi-node
  testnet) is mostly empty stubs, blocked on M5+M6.
- **Level1** (lower priority): benchmark Noir+Barretenberg/Honk vs Plonky3 backends for
  `circuit/reanchoring/` per `docs/EXPERIMENT.md`'s E6.

Do not assume any of the above exists just because a directory or file for it is present.

## Vietnamese comments

Existing code mixes Vietnamese and English comments (the repo owner is a Vietnamese speaker). Match
whichever a file already uses; don't force a wholesale translation pass.
