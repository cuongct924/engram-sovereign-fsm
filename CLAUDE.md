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
app/                -- EngramApp (real BaseApp wiring: codec, single KVStoreKey, ante handler,
                       PrepareProposal/ProcessProposal/PreBlocker); no bank/auth/staking module yet
cmd/engramd/        -- node binary entrypoint: `engramd init [moniker]` + `engramd start [--vanilla]` are
                       real and were verified end-to-end (see "Current status"); `--vanilla` skips the
                       ExtendedProposal ABCI hooks for docs/EXPERIMENT.md's baseline comparison;
                       `engramd testnet init-files --v N` generates a real N-validator shared genesis
                       for a multi-node testnet (single-validator `init` can't bootstrap one)
proto/              -- protobuf sources (buf); generated .pb.go land in x/sovereignty/types/
circuit/reanchoring/ -- Noir ZK circuit for the re-anchoring recovery proof
tests/e2e/          -- in-process fault-injection harness + scenarios (docs/EXPERIMENT.md's E2)
tests/benchmark/, tests/mbt/ -- stubs, not yet implemented
docker/, compose.yml -- multi-node local testnet + Pumba chaos-engineering profiles; `docker compose
                       config` validates cleanly (M6 done) -- actually running the 4-node testnet is
                       still untested (only the M5 single-validator engramd path was run for real)
scripts/            -- Python analysis/plotting for E2-E9 experiments (docs/EXPERIMENT.md); E2/E3/E5/E6/E7/E8/E9
                       are real (consume real tests/e2e or tests/benchmark output, see docs/EXPERIMENT.md's
                       per-section "Đã đo thật" notes); E4 is real detector code against synthetic (not
                       live-network) input, explicitly labeled as such -- no Docker/Pumba chaos injection is
                       available in this environment (M7)
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

Phases 0-4 and milestones M1-M6 are done:
- The module builds and tests clean, with real `PrepareProposal`/`ProcessProposal`/`PreBlocker`
  handlers wired onto a real `*baseapp.BaseApp` (`app/app.go`) and a real `engramd init`/`start`
  CLI (`cmd/engramd/`) -- confirmed by actually running a single-validator node end-to-end
  (`engramd init && engramd start`), which produces real blocks and correctly drives FSM
  transitions through consensus (verified via RPC, not just unit tests).
- The in-process E2 fault-injection harness (`tests/e2e/`) separately produces real experiment
  data without needing a running node (see "Spec fidelity" above for why `BeginBlocker` there is a
  deliberately different code path from the real ABCI++ one).
- `cmd/engramd/main.go`'s `runStart` previously called `cmtcfg.DefaultConfig()` directly, which
  does **not** read `config.toml` from disk at all -- per-node config (RPC/P2P ports, seeds,
  persistent_peers, instrumentation) was silently ignored at `start` time regardless of what
  `engramd init` wrote or what was hand-edited (found by running two nodes with different
  `config.toml` ports side by side: the second failed to bind, still using the hardcoded default).
  Fixed via a `viper`-based `loadConfig` helper (mirrors CometBFT's own `cmd/cometbft/commands`
  bootstrap pattern) -- load-bearing for both the `--vanilla` baseline comparison and the
  multi-node Docker testnet, which both depend on per-node config actually taking effect.
- `docker compose config` (root `compose.yml` + everything under `docker/`) validates cleanly --
  all services are named `engram-nodeNN` (no more `stratium`/hyphen-vs-no-hyphen naming bugs), and
  the root `compose.yml` is the sole owner of shared `networks:`/`volumes:` resources (included
  files reference them by name in `services:` only, never redeclaring them at top level -- Compose's
  `include:` rejects that as "conflicts with imported resource" even when marked `external: true`).
- **Real 4-node consensus over Docker networking was confirmed** (`engramd testnet init-files`, new
  subcommand in `cmd/engramd/main.go`, generates N validator homes sharing one genesis + correct
  `persistent_peers` -- `initCmd`/`initHome` only ever supported a single validator). Found and
  fixed 3 real bugs doing this: (1) `engram-net`'s `172.20.0.0/24` subnet collided with an
  unrelated pre-existing `kind` (Kubernetes-in-Docker) network on the host -- moved to
  `172.28.0.0/24` in `compose.yml`+`docker/engram-monitoring.yml`; (2) the Dockerfile's
  `ENTRYPOINT ["engramd"]` plus each `docker/engram-validator-node0N.yml`'s
  `command: ["engramd", "start"]` doubled up to `engramd engramd start` (crash loop) -- fixed to
  `command: ["start"]`; (3) RPC defaulted to `127.0.0.1`, unreachable through the Docker port
  mapping from the host (containers showed "healthy" because the healthcheck curls from inside the
  same container) -- `testnetInitFiles` now sets `config.RPC.ListenAddress` to `0.0.0.0`. All 4
  `engram-nodeNN` services also switched from named Docker volumes to bind-mounting
  `./testnet-data/engram-nodeNN` (gitignored, generated by the command above) so the shared
  genesis/keys are actually present at container start -- previously there was no init step in the
  container's command at all. **Not yet re-verified after fix (3)**: a Docker Desktop
  storage-engine corruption (triggered by the host disk hitting ~98% full mid-session) is blocking
  a rebuild; the daemon itself is currently down and needs a manual restart. The Bitcoin-regtest/
  Celestia-light satellite services (`vigilante-*`, `celestia-light0N`) were deliberately not
  brought up in this pass -- `DA_ENDPOINT` is set as an env var on the engramd services but never
  actually read by any Go code today, so the 4 validators reaching consensus does not need them.

M0a and M0d are also done, at app/fork-fidelity scope (not full core-consensus scope -- see gaps below):
- **M0a**: the forked CometBFT core (`github.com/cuongct924/engram-consensus-core`, cloned at
  `/Users/cuongct090_04/Code/engram-consensus-core`) now has real P2P health telemetry --
  `lp2p/health_monitor.go`'s `HealthMonitor` tracks per-peer connect time/churn/subnet from real
  libp2p data, wired into `lp2p/switch.go`'s peer lifecycle (`onPeerConnected`/`onPeerDisconnected`
  wrap the existing reactor hooks), exposed via `Switch.PeerHealthSnapshot()`. Not yet done: wiring
  this into `x/sovereignty`'s P2P sensor (still mock-only) -- that needs a `go.mod` `replace`
  pointing at the fork plus a design for how a Cosmos app process reads data out of an in-process
  CometBFT `Switch`, which hasn't been designed yet.
- **M0d**: forced-inclusion censorship resistance is done at the ABCI level in
  `engram-sovereign-fsm` (no fork changes needed) -- `x/sovereignty/types/censorship.go` ports
  `IsCensoring`/`UpdateIgnoredRounds`, keeper state `ForcedTxQueue`/`TxIgnoredRounds` track it,
  `MsgSubmitForcedTx` (new proto RPC) queues a tx, `ProcessProposal` rejects a proposal that still
  omits a forced tx past `MaxIgnoreRounds`, and `PreBlocker` updates the ignore-round counters every
  block. **Documented gap**: the spec's censoring branch also forces an *immediate* round advance
  (`StartRound(p, r+1)`); vanilla ABCI 2.0 gives `ProcessProposal` no lever to shorten CometBFT's
  round timer, so a reject here only yields a nil prevote and the existing local timeout still
  governs when the round actually advances -- closing this needs M0b's fork-level round-skip work.

**Not yet done**, tracked as later milestones (ask the repo owner for the current plan file if
picking this up cold):
- **M0b**: f+1-timeout quorum round-skip (HotStuff/Jolteon-style) in the fork's
  `consensus/state.go` -- confirmed via reading the real code that vanilla round-skip there is
  purely local-timer-driven, so this needs a new P2P message type + reactor broadcast + a real
  state-machine change, not just an ABCI hook.
- **M0c**: FSM-state-dependent fork choice (`CanElect`-style: K-deep BTC anchor vs max-stake-branch)
  in the fork's block-sync/chain-selection logic -- needs a core-to-app bridge to read `fsm_state`,
  the highest-risk remaining fork change.
- **M7**: `scripts/`'s Python orchestration (Pumba chaos injection against a real multi-node
  testnet) is mostly empty stubs, blocked on M5+M6.

**Level1** (ZK backend benchmark, lower priority) is done for the mandatory half:
`scripts/e6_zk_reanchoring_benchmark/benchmark_prover.sh` runs a real Noir (1.0.0-beta.22) +
Barretenberg (`bb`, UltraHonk) `compile`/`gates`/`execute`/`prove`/`verify` pipeline against
`circuit/reanchoring/src/main.nr` at N=4..256 sovereign blocks, and `stats_collector.py` turns the
raw CSV into `docs/EXPERIMENT.md`'s Table 6A/6B and Figure 6 -- all real measured numbers, not
placeholders (see `docs/EXPERIMENT.md`'s E6 section for the headline results). The circuit itself
is a simplified stand-in for the design doc's original sketch (Pedersen-hash header-chain
continuity instead of a real SMT inclusion/update proof -- documented in the circuit's header
comment); toolchain note: the repo's `nargo`/`bb` were upgraded from a stale 0.31.0-era pairing to
current stable during this work (`noirup -v 1.0.0-beta.22` + matching `bbup`), and this version of
Noir needs explicit parens around chained `==`/`|` comparisons (`(a == 2) | (a == 3)`, not
`a == 2 || a == 3`) due to operator precedence -- not a spec-fidelity issue, just this Noir
version's grammar.
**Not done**: Table 6C / Figure 7 (Plonky3 backend comparison) -- explicitly the optional half of
E6 ("tùy chọn nếu còn thời gian").

Do not assume any of the above exists just because a directory or file for it is present.

## Vietnamese comments

Existing code mixes Vietnamese and English comments (the repo owner is a Vietnamese speaker). Match
whichever a file already uses; don't force a wholesale translation pass.
