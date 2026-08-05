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

M0a and M0d are also done:
- **M0a**: the forked CometBFT core (`github.com/cometbft/cometbft`, replaced via `go.mod` to the
  local checkout at `/Users/cuongct090_04/Code/engram-consensus-core`, Phase 7) has real P2P health
  telemetry -- `lp2p/health_monitor.go`'s `HealthMonitor` tracks per-peer connect time/churn/subnet
  from real libp2p data, wired into `lp2p/switch.go`'s peer lifecycle
  (`onPeerConnected`/`onPeerDisconnected` wrap the existing reactor hooks), exposed via
  `Switch.PeerHealthSnapshot()`. Phase 7 finished the wiring into `x/sovereignty`'s P2P sensor:
  `x/sovereignty/keeper/sensors/p2p_health.go`'s `P2PSensor` gained a `P2PHealthSource`
  interface + `SetSource`; `cmd/engramd/main.go`'s `wireP2PSensor` type-asserts
  `node.NewNode(...)`'s returned `n.Switch()` to `*lp2p.Switch` (late-bound after
  `node.NewNode()` constructs it, since `NewEngramApp` runs before that) and adapts
  `lp2p.HealthSnapshot` into `sensors.P2PSnapshot` (the two shapes are already field-compatible).
  `x/sovereignty/sensors_refresh.go`'s `RefreshMetrics` -- previously missing entirely, so
  `PrepareProposal`/`ProcessProposal` always read stale keeper state instead of live sensor data --
  now calls this every block from both handlers, matching "sensors propose, consensus decides."
- **M0d**: forced-inclusion censorship resistance is done at the ABCI level in
  `engram-sovereign-fsm` (no fork changes needed) -- `x/sovereignty/types/censorship.go` ports
  `IsCensoring`/`UpdateIgnoredRounds`, keeper state `ForcedTxQueue`/`TxIgnoredRounds` track it,
  `MsgSubmitForcedTx` (new proto RPC) queues a tx, `ProcessProposal` rejects a proposal that still
  omits a forced tx past `MaxIgnoreRounds`, and `PreBlocker` updates the ignore-round counters every
  block. The spec's censoring branch also forces an *immediate* round advance (`StartRound(p,
  r+1)`); vanilla ABCI 2.0 gives `ProcessProposal` no lever to shorten CometBFT's round timer, so a
  reject here still only yields a nil prevote -- M0b's f+1-quorum round-skip (below) is what
  actually lets the network fast-forward past a stalled/censoring round once enough honest
  validators time out, closing the rest of this gap.

M0b and M0c are done, in the fork repo (`/Users/cuongct090_04/Code/engram-consensus-core`):
- **M0b**: f+1-timeout quorum round-skip (HotStuff/Jolteon-style), replacing the fork's previously
  purely local-timer-driven round advance. `consensus/state.go`'s `handleTimeout`'s
  `RoundStepPrecommitWait` case now broadcasts a `Timeout` attestation for the round being
  requested and fast-forwards immediately once f+1 *distinct, validly-signed* attestations arrive
  (`recordTimeoutSenderAndMaybeAdvance`), falling back to the old unconditional local-timer advance
  (via a new `RoundStepPrecommitWaitFallback` step) only if quorum never arrives -- needed for
  liveness in scenarios with no live peers (e.g. many of CometBFT's own unit tests construct a bare
  `State` with no `Reactor`/`Switch`). The `Timeout` message is a first-class signed type
  (`types.Timeout`, mirroring `Vote`/`Proposal`; `PrivValidator.SignTimeout`, implemented for
  `FilePV`/`MockPV`/`SignerClient`/`RetrySignerClient` including the remote-signer wire protocol) --
  an earlier draft tallied quorum by raw p2p peer ID with no signature at all, which would have let
  any connected peer (not necessarily a validator) forge an f+1 quorum and force premature
  round-skips; `handleTimeoutMessage` now verifies the signature against the current validator set
  before counting a sender. `consensus/round_skip_test.go`'s `TestStateRoundSkipFPlus1Quorum` /
  `TestStateRoundSkipRejectsUnauthenticatedSender` cover both the quorum mechanism itself and that
  fix. Full `go test ./...` across the fork (91 packages) is clean.
- **M0c**: FSM-state-dependent fork choice (`CanElect`, K-deep BTC anchor vs max-stake-branch,
  `spec/core/EngramConsensus.tla:143-149`) turned out to need **no new fork-level code** --
  confirmed by reading `EngramServerRefinement.tla`: `CanElect`/`IsKDeep`/`IsMaxStakeBranch` are
  never called by the concrete layer (`EngramServer.tla`/`EngramTendermint.tla`); they appear only
  inside the `INSTANCE ... WITH` substitution that feeds concrete state into the abstract LiDO layer
  for the *refinement proof*, and `ServerInsertProposal` (the concrete hook mapped to abstract
  `Pull`) never references them at runtime. The concrete counterparts already exist and are already
  tested: `IsKDeep`'s "not lost to reorg" clause is `x/vigilante.VerifySPVProof`'s
  `checkpoint_block_height >= h_btc_anchored` check (M2, wired into `ProcessProposal`;
  `TestVerifySPVProof_RejectsCheckpointBelowAnchored` is now doc-commented as M0c's concrete
  coverage for this branch), and `IsMaxStakeBranch`'s `SumStake >= TOTAL_STAKE/2` is structurally
  implied by CometBFT's own unmodified >=2/3 commit quorum (2/3 > 1/2), so it needs no app-layer
  reimplementation -- consistent with "sensors propose, consensus decides."

**Phase 7 (real BTC anchor pipeline)** is done, confirmed by actually running `engramd start`
against a real `bitcoind` regtest node (`docker/bitcoin-regtest-cluster.yml`'s `bitcoin-node01`)
and watching it commit real blocks, not just unit tests:
- `x/vigilante/rpc.go`'s `RPCClient` is a minimal stdlib-only (no btcsuite/btcd dependency)
  Bitcoin Core JSON-RPC client: `CurrentHeight` (getblockcount), `BlockHashAt`, `SubmitOpReturn`
  (createrawtransaction/fundrawtransaction/signrawtransactionwithwallet/sendrawtransaction),
  `TxConfirmation` (gettransaction), `BlockContainsTag` (getblock verbosity=2 script scan).
- `x/vigilante/anchor.go`'s `AnchorTracker` is this repo's minimal stand-in for Babylon's real
  Vigilante Submitter+Reporter daemons (`github.com/babylonlabs-io/vigilante`) -- deliberately
  **not** the real `babylonlabs/babylond` vigilante images already scaffolded in
  `docker/engram-validator-node0N.yml` (`vigilante-submitter0N`/`reporter0N`/
  `checkpointing-monitor0N`), because those expect a real Babylon chain's
  `x/checkpointing`/`x/btccheckpoint`/`x/btclightclient` modules to source a BLS-aggregated
  epoch checkpoint from, which `engram-node` (only `x/sovereignty` mounted, see `app/app.go`'s
  TODO on staking) does not have and would not function against. `AnchorTracker` instead submits
  a simple `AnchorTag + engramHeight` OP_RETURN marker, tracks its own confirmation depth
  (`MaybeSubmit`/`ConfirmedAnchorHeight`, mutex-protected -- an earlier unguarded version let
  `PrepareProposal` and `ProcessProposal` race and double-submit within the same block), and lets
  ANY validator independently re-verify a peer's claimed anchor height against its own bitcoind
  (`VerifyAnchor`) rather than trusting it -- wired into `x/sovereignty/proposal.go`'s
  `ProcessProposal` as a new check (3b) alongside the existing spec-fidelity `VerifyReceipt`.
- **Real liveness bug found and fixed by actually running this against real, continuously-mined
  Bitcoin** (not visible in any unit test, since `h_btc_current` was never live before Phase 7):
  `h_btc_anchored` was never advanced by any code path in normal operation (`PrepareProposal`
  always just re-echoed the last-committed value) -- fixed by having `PrepareProposal` adopt
  `AnchorTracker.ConfirmedAnchorHeight()` once it exceeds the committed value. That fix alone
  still deadlocked: `vigilante.VerifyReceipt`'s freshness check
  (`checkpoint_height >= h_btc_current - tol`) and `IsKDeep`'s safety check
  (`h_btc_current - checkpoint_height >= kDeepFinality`) are structurally incompatible whenever
  `tol < kDeepFinality` -- and the spec's `BTCTolerance` maxes out at 1, while round was ALSO
  hardcoded to 0 everywhere (`Tolerance` was never even called with a real round), making
  `tol=0` always. Fixed with two changes: (1) a **fork-level ABCI extension**
  (`proto/tendermint/abci/types.proto`'s `RequestPrepareProposal`/`RequestProcessProposal` gained
  a `round` field, threaded from `consensus/state.go`'s `cs.Round` through
  `state/execution.go`'s `CreateProposalBlock`/`ProcessProposal` -- vanilla ABCI 2.0 does not
  expose this at all) so `x/sovereignty/proposal.go` no longer hardcodes round=0; (2)
  `x/vigilante/verify.go`'s `Tolerance` now widens to `kDeepFinality` (not the spec's flat 1)
  once round>=3, a deliberate, documented divergence from the literal spec constant (see its
  doc). `types.Params` gained `KDeepFinality` (default 2, regtest-appropriate). Confirmed via
  `engramd start` reaching height 8+ with real, sustained block production once these landed.
- **Celestia infra (docker/celestia-local-cluster.yml) is now real and healthy** -- `celestia-app`
  (core) and `celestia-bridge` both reach real, sustained `docker compose ps` "healthy" status and
  serve real RPC data (confirmed via `header.NetworkHead` returning a real, advancing header).
  Found and fixed 7 real bugs bringing this up, none visible from just reading the compose file:
  (1) both services' `command:` was a multi-statement shell script handed to the image's default
  ENTRYPOINT (`/opt/entrypoint.sh`), which does `exec <binary> "$@"` unconditionally with no shell
  interpretation -- needed an explicit `entrypoint: ["/bin/sh", "-c"]` override on both; (2)
  `celestia-appd init` has no `--keyring-backend` flag (that belongs to `keys add`) and its default
  `--home` is a subdirectory of the mounted volume, not the volume root; (3) `celestia-appd start`
  needs an explicit `--minimum-gas-prices` (app.toml alone isn't enough) and the real default bond/
  fee denom is `utia`, not `stake`; (4) `celestia-appd init` alone never registers a validator --
  needed the full manual Cosmos SDK bootstrap (`keys add` / `add-genesis-account` / `gentx` /
  `collect-gentxs`) `engramd init` in this repo does automatically but celestia-appd does not,
  without it `start` panics with "validator set is empty after InitGenesis"; (5) both healthchecks
  used the exec-form `CMD` array with a `|`/`||` shell operator baked into one string argument,
  which is never interpreted without a shell, so both silently failed unconditionally regardless of
  real health -- celestia-app's fixed with `wget --spider` (no curl in that image), celestia-bridge's
  needs a real JSON-RPC POST (its RPC is JSON-RPC 2.0 over POST only; a GET always 400s) with
  `--rpc.skip-auth` added to `bridge start` so the healthcheck doesn't also need a JWT; (6)
  `celestia-app`'s `app.toml` ships with `[grpc] enable = false` -- needed explicit
  `--grpc.enable --grpc.address 0.0.0.0:9090` on `start` or celestia-bridge's own start fails
  querying gas price ("connection refused" on 9090); (7) **the actual liveness-blocking bug**:
  `celestia bridge init --p2p.network private` hardcodes an EXPECTED chain-id of the literal string
  `"private"` for its core-node subscription (confirmed the flag only accepts the enum
  `celestia|mocha-4|arabica-11` for real networks -- "private" is a separate, special-cased local-
  devnet keyword, not a slot for an arbitrary custom chain-id) -- celestia-app's chain-id must
  therefore also literally be `"private"` (not e.g. `engram-dev-1`) or bridge crash-loops forever
  with `FATAL core/listener: invalid subscription` / `unexpected chain ID: expected private,
  received <whatever>`. Also found and fixed: `[Header] TrustedHash` is never auto-populated by
  `--core.ip` at init in this celestia-node version (no CLI flag for it either) -- the bootstrap
  script now fetches block 1's real hash from celestia-app's RPC and `sed`s it into
  config.toml, the same fix Celestia's own troubleshooting docs describe for other versions/
  scenarios; and a `$VAR`/`$(...)` escaping bug (needed `$$` throughout the compose `command:` block
  so Compose passes literals through to the container's shell instead of trying to interpolate them
  itself from the host environment first). Also fixed: a port conflict (`celestia-app` redundantly
  mapped bridge's `26658`) and a relative-volume-path inconsistency (`./data/...` in this file
  resolves relative to `docker/celestia-local-cluster.yml`'s own location, landing in
  `docker/data/`, unlike `bitcoin-regtest-cluster.yml`'s `../data/...` which correctly reaches the
  gitignored repo-root `data/`; aligned both to `../data/...`).
- **Not yet done**: none of the above is wired into `x/da` yet -- `DASensor` is still on its static
  mock (no live source), and there is no Go client analogous to `x/vigilante/rpc.go`/`anchor.go` for
  submitting blobs to celestia-app or querying DAS-style availability from celestia-bridge. That
  wiring (plus a genuine celestia-light node sampling from the bridge, per the repo owner's
  preference over trusting the bridge's full data directly) is the next piece of work.

**Not yet done**, tracked as later milestones (ask the repo owner for the current plan file if
picking this up cold):
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
