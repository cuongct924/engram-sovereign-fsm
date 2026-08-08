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
  **not** the real `babylonlabs/babylond` vigilante images. `docker/engram-validator-node0N.yml`
  previously scaffolded these as `vigilante-submitter0N`/`reporter0N`/`checkpointing-monitor0N`
  services (plus the `config/{submitter,reporter,monitor}.toml` files backing them, and
  `BABYLOND_VERSION` in `.env`/`.env.example`) -- removed at the repo owner's request to keep the
  prototype minimal, since those daemons expect a real Babylon chain's
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
- **Second real liveness bug, found later by re-deriving `btc_gap` from spec instead of trusting
  the earlier port**: `x/sovereignty/sensors_refresh.go`'s `btcGapMetric` computed
  `btc_gap = h_btc_current - min(h_btc_submitted, h_btc_anchored)`, copying `EngramFSM.tla:95`'s
  ABSTRACT-layer formula verbatim. That formula only works there because `BTCNormalUpdate`/
  `BTCSPVFailure` (`EngramFSM.tla:173-188`) always keep `h_btc_anchored <= h_btc_submitted` in
  lockstep, so the `min()` always resolves to `h_btc_anchored` anyway. The CONCRETE layer
  (`ServerUponProposalInPrecommitNoDecision` Step 3/4, `EngramServer.tla:148,151-159`, which
  `preblock.go` correctly ports) decouples the two: `h_btc_anchored` is written from the real
  committed `btc_receipt` every block regardless of FSM state, while `h_btc_submitted` is written
  ONLY on entering/staying RECOVERING with a claimed ZK proof -- so outside a re-anchoring cycle it
  never leaves its Go zero value, collapsing `min(0, h_btc_anchored)` to 0 and inflating `btc_gap`
  to ~`h_btc_current` regardless of real anchor state. Confirmed live via temporary debug prints:
  this held the FSM in `SOVEREIGN` across this session's entire "successful" BTC pipeline testing
  (the height-8+ block production above was real, but the FSM state driving it was not). Fixed by
  dropping the `min()` and using `h_btc_anchored` alone, matching Step 3 exactly. Also found while
  fixing this: `reanchoring_proof_valid` was never ported at all -- the concrete layer's Step 4
  only ever writes `FALSE` to it (on submission) or leaves it `UNCHANGED`; the only place it's ever
  computed back to `TRUE` is the ABSTRACT layer's `UpdateSensors` environment action
  (`EngramFSM.tla:294-301`), which has no concrete-layer counterpart in `EngramServer.tla`. Fixed
  by adding `refreshReanchoringProofValid` (same file), called from `RefreshMetrics` alongside
  `btcGapMetric` -- mirroring how the abstract action recomputes it every environment tick, since
  the concrete hooks alone never do. Without this, `CalculateNextState`'s `RECOVERING -> ANCHORED`
  transition could never fire via a successful reanchoring proof, in any test or real run to date.
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
  gitignored repo-root `data/`; aligned both to `../data/...`). One more found later, while wiring
  up real blob submission: `celestia-appd init` writes `config.toml` with `[tx_index] indexer =
  "null"` (indexing disabled) by default, and there's no `start`-time CLI flag for it -- `blob.Submit`
  needs tx indexing and fails with "transaction indexing is disabled" without it, so the bootstrap
  script now `sed`s `indexer = "kv"` in right after `init`.
- **`x/da`'s real celestia-bridge wiring is now done**, confirmed end-to-end via `engramd start`
  against the real `celestia-app`/`celestia-bridge` pair above (not just unit tests): `x/da/rpc.go`'s
  `RPCClient` is a minimal stdlib-only celestia-node JSON-RPC 2.0 client (`Submit` = `blob.Submit`,
  `Available` = `blob.GetAll`-backed retrievability check); `x/da/publisher.go`'s `Publisher` is the
  DA-side counterpart of `AnchorTracker` -- **deliberately no K-deep-style confirmation depth**
  (unlike BTC): `DANormalUpdate`/`DAFailure` (`spec/core/EngramFSM.tla:196-212`) have no depth term,
  so once a submission is confirmed retrievable, `h_engram_verified` is set EQUAL to that height
  immediately, not just advanced past it. Wired into `Sensors.DAPublisher` / `DASensor.SetSource`
  (mirrors `BTCSensor`'s pattern) and `cmd/engramd/main.go`'s `wireDASensor` (reads
  `CELESTIA_BRIDGE_URL`/`CELESTIA_BRIDGE_AUTH_TOKEN`/`CELESTIA_NAMESPACE_ID`). Also fixed the same
  liveness-bug class as BTC's: `h_engram_verified` was frozen at its last-committed value in
  `PrepareProposal` with no path to advance -- same fix, adopt `Publisher.VerifiedHeight()` when
  ahead. Separately, `RefreshMetrics` was silently hardcoding `IsDasFailed`/`IsAttestationFailed` to
  `false` regardless of what `DASensor.SetFailureFlags` had set (pre-existing bug, unrelated to this
  wiring but found and fixed while touching the same function).
  - **Real liveness bug found and fixed by actually running this against a real celestia-bridge**
    (not visible in any unit test, since blob submission was never live before this wiring): a real
    celestia-node's `blob.Submit` blocks until the blob is actually **included in a Celestia block**
    (~12s at default block time) before returning -- calling it synchronously from `RefreshMetrics`
    (invoked by both `PrepareProposal` and `ProcessProposal`, both on the hot consensus path) made
    every `PrepareProposal` call take ~9-12s against CometBFT round timeouts of only 3-4.5s, so the
    leader's own proposal was never ready before M0b's f+1-timeout quorum round-skipped -- confirmed
    by running a live node and watching it round-skip forever at height 1 despite being the sole
    validator (100% of the stake), with `signed proposal` consistently landing 9-12s after `entering
    propose step` in the logs. Fixed by running `Publisher`'s `Submit` call in a background
    goroutine (mutex-protected, at most one in-flight submission at a time) so `MaybePublish` always
    returns near-instantly regardless of Celestia's block time -- `AnchorTracker`'s `SubmitOpReturn`
    never had this problem since bitcoind's RPC returns as soon as a tx is broadcast, not confirmed.
    Confirmed via `engramd start` reaching height 14+ with real, sustained block production, and via
    direct `blob.GetAll` queries against celestia-bridge showing the exact `HeightMarker(height)`
    payload landing at increasing Celestia heights as Engram height advanced.
  - **Documented simplification**: the blob payload is `HeightMarker(engramHeight)` (an 8-byte tag +
    height marker, mirroring `AnchorTracker`'s `AnchorTag + height`), not the block's actual
    transaction data -- `RefreshMetrics` (where `MaybePublish` is called) only has `sdk.Context`, not
    `req.Txs`; publishing the real block data needs wiring at `PrepareProposal` itself, where `Txs`
    are available. The availability MECHANISM being exercised (a real `blob.Submit`/`blob.GetAll`
    round-trip against live Celestia) is real; only the blob's CONTENT is currently a placeholder.
  - **Not yet done**: a genuine `celestia-light` node service sampling from `celestia-bridge` (per
    the repo owner's preference over trusting the bridge's full data directly) is not in
    `docker/celestia-local-cluster.yml` yet -- `Publisher`/`RPCClient` talk to the bridge directly.

**Real ZK re-anchoring (spec/README.md's §Re-anchoring via ZK-Proof of Recovery)** is now wired
end-to-end, confirmed by actually running the pipeline against a real node (not just unit tests) --
previously `x/sovereignty/keeper/reanchor.go`'s `VerifyZKProof` was a `return true` stub with nothing
upstream or downstream of it real:
- **`x/sovereignty/types/recovery_header.go`**'s `RecoveryHeader` + keeper's new
  `HeaderHistory`/`LastAnchoredRoot` collections (`x/sovereignty/keeper/keeper.go`) track the real
  per-block witness data the circuit needs, populated in `preblock.go`'s `CommitFSMTransition` only
  while `fsm_state ∈ {SOVEREIGN, RECOVERING}` (pruned on return to ANCHORED). `state_root` is
  CometBFT's real per-block `AppHash` (one-block-lagged, the standard ABCI lag already documented
  elsewhere in this repo), NOT the keeper's `Tree` (SMT) -- that field is unrelated dead code (see
  below), and this prototype has no `x/bank`/account state to put in an SMT leaf yet regardless.
  Both `state_root` and `rt_last` are reduced into the circuit's BN254 scalar field
  (`types.ReduceToField`) before storage -- a raw 32-byte hash has roughly a 1-in-16 chance of
  exceeding the field modulus if used verbatim, which Noir/bb reject outright ("non-canonical...
  value >= field modulus") rather than silently wrapping.
- **`circuit/reanchoring_witness/`** is a new, separate Noir crate (its own `Nargo.toml`) that links
  real raw header data into a real `prev_hash` chain via the same `hash_header`
  (Pedersen) function `circuit/reanchoring/src/main.nr` uses -- deliberately NOT a Go-side hash
  reimplementation (a real, avoidable correctness risk) and deliberately NOT touching `main.nr`
  itself, so the already-collected E6 benchmark numbers (Table 6A/6B/Figure 6) stay valid. Mirrors
  `main.nr`'s own `dump_chain` test technique, just fed real chain data instead of synthetic.
- **`VerifyZKProof`** now really verifies: `x/sovereignty/keeper/zk_assets/vk` is a git-committed copy
  of the compiled N=4 circuit's verification key (`go:embed`bed into the binary -- `go:embed` can't
  reach `circuit/reanchoring/target/`, which is gitignored and outside this package's directory
  tree), and the function shells out to a pinned `bb verify` against temp-file copies of the
  submitted proof/inputs, fail-closed (returns false, never panics) on any error or missing binary.
  Confirmed against the real, previously-generated N=4 proof on disk: verifies `true` for the valid
  proof, `false` for a single-byte-flipped tamper. `bb`/`nargo` are now required, pinned runtime
  dependencies on every validator for this determinism to hold (verification runs inside `DeliverTx`,
  executed identically by all validators).
- **Real safety bug found and fixed**: `keeper.MsgServerImpl.SubmitRecoveryProof` used to set
  `FSMState = ANCHORED` directly and unconditionally on proof-math validity alone -- bypassing
  `StrictFSMTransitionSafety` (e.g. a direct SOVEREIGN -> ANCHORED jump) and the hysteresis dwell
  time entirely, via a second, unguarded FSM-state writer parallel to the real consensus-driven one.
  Fixed by having it latch `RealProofSubmittedHeight` instead (the height of the header it proved up
  to) -- consumed by `sensors_refresh.go`'s `refreshReanchoringProofValid` (OR'd with the existing
  BTC-anchor heuristic), which is what `CalculateNextState`'s existing, already-correct
  `SafeBlocks == HysteresisWait && ReanchoringProofValid` guard reads. No parallel FSM-state-writing
  path exists anymore. Also added: cross-checking the proof's public inputs (`rt_last`/`rt_new`)
  against `LastAnchoredRoot`/`HeaderHistory`'s real tracked tip, closing a replay gap (proof math
  alone only proves "some self-consistent chain links some rt_last to some rt_new" -- without this,
  the proof already checked into `circuit/reanchoring/target/proof/` could be replayed against any
  deployment).
- **Second real bug found by actually running the full pipeline against a live node, not by
  inspection**: a proof submitted while N headers were tracked must stop counting the moment a NEW
  header is appended before RECOVERING is reached -- the interval keeps growing, and a proof only
  covers what it was built against. A flat bool latch can't express this; `RealProofSubmittedHeight`
  stores the proven height instead, and `refreshReanchoringProofValid` requires it to still equal the
  CURRENT tracked tip's height, not merely be nonzero. `TestSubmitRecoveryProof_StaleProofRejectedAfterIntervalGrows`
  (`x/sovereignty/keeper/msg_server_test.go`) covers the primitive this depends on.
- **`app/app.go` had three separate, previously-undiscovered wiring gaps**, all found by actually
  trying to build+broadcast a real tx against a running node (every prior test only ever called
  `MsgServerImpl`/`QueryServerImpl` methods directly in Go, never through real ABCI tx routing):
  (1) no `module.Manager`/`module.Configurator` exists in this app (`InitChain` is a hand-rolled
  `InitChainer`, not `module.Manager`'s `InitGenesis`), so `x/sovereignty/module.go`'s
  `AppModule.RegisterServices` was NEVER called -- every `Msg` (`MsgInjectFaultRequest`,
  `MsgSubmitForcedTxRequest`, `MsgSubmitRecoveryProofRequest`) and the `Query.State` RPC (defined in
  the proto with generated types since early on, but never implemented OR registered) were
  unroutable via any real submitted tx, "no message handler found" being the exact live symptom.
  Fixed by registering directly against `bApp.MsgServiceRouter()`/`bApp.GRPCQueryRouter()` in
  `NewEngramApp`, matching this app's existing no-module-manager architecture rather than
  introducing one just for this. `x/sovereignty/keeper/grpc_query.go`'s `QueryServerImpl` is the
  first real `QueryServer` implementation this module has ever had (also implements `Query.State`,
  previously unimplemented on top of being unregistered). (2) No address codec was configured on the
  `InterfaceRegistry` (bare `codectypes.NewInterfaceRegistry()`) -- any message with a
  `cosmos.msg.v1.signer`-annotated field (like `MsgSubmitRecoveryProofRequest`'s `authority`) has its
  `GetSigners()` auto-derived via bech32 decoding, which BaseApp invokes even with no
  `SigVerificationDecorator` to cryptographically check it; live symptom was CheckTx failing with
  "InterfaceRegistry requires a proper address codec implementation." Fixed via
  `codectypes.NewInterfaceRegistryWithOptions` with a real `"engram"`-HRP Bech32 codec (this app
  never had ANY bech32 prefix configured before, having no `x/auth`/`x/bank`). (3) Given (2)'s fix
  plus no `x/auth`, `cmd/engramd/reanchor_cli.go`'s new `tx-submit-recovery-proof` command builds the
  minimal structurally-valid `sdk.Tx` envelope BaseApp's `TxDecoder` accepts (one `SignerInfo`, one
  empty signature, empty `Fee`) rather than a real signed tx -- nothing in the ante chain
  (`CircuitBreakerDecorator` only) checks a signature, so this is deliberate, not an oversight; a
  real `AccountKeeper`-backed signing flow would need `x/auth` mounted, which this prototype doesn't
  have (see this file's `app/` layout note).
- **New CLI**: `engramd query-recovery-headers` (dumps the real tracked interval + `rt_last` as
  plain lines, via the real ABCI-routed gRPC query -- no separate gRPC server needed, CometBFT's
  `/abci_query` already routes to `bApp.GRPCQueryRouter()`) and `engramd tx-submit-recovery-proof
  --proof <file> --public-inputs <file>` (`cmd/engramd/reanchor_cli.go`). Both confirmed working
  against a real running node: query returns real per-height data with correct Field encoding
  (`fsm_state`: 2=SOVEREIGN/3=RECOVERING, matching `main.nr`'s comment), and tx submission correctly
  rejects a garbage proof end-to-end (real CheckTx -> mempool -> DeliverTx -> `SubmitRecoveryProof`
  -> `VerifyZKProof` -> `ErrInvalidZKProof`).
- **`scripts/reanchoring_prover/prove_and_submit.sh`** wires all of the above into one real pipeline
  (query -> witness-helper crate -> real `main.nr` `Prover.toml` -> `nargo execute` + `bb prove`,
  using the SAME embedded VK the Go node checks against -> submit), mirroring
  `scripts/e6_zk_reanchoring_benchmark/benchmark_prover.sh`'s style. Every stage confirmed working
  with 100% real components against a real running node (real query, real Noir/bb proving, real
  submission). **Known, documented, inherent limitation** (see the script's own header comment): `N`
  is fixed at compile time (currently 4), so this only works when exactly `N` headers are tracked;
  and because querying-then-proving-then-submitting takes real wall-clock time (tens to low hundreds
  of ms of actual proving per the E6 numbers, plus RPC/CLI overhead) while a still-unhealthy
  SOVEREIGN/RECOVERING interval keeps growing underneath it, a proof can legitimately go stale
  between query and submission -- confirmed by repeated real end-to-end runs against a continuously-
  producing test node being correctly rejected this way. This is the same anti-replay protection
  working as designed (see `RealProofSubmittedHeight` above), just observed from the submission side
  instead of the already-latched side; a real deployment needs to submit while the interval is
  genuinely stable, not race a continuously-growing one.
- **Not done / explicitly out of scope for this pass**: the keeper's `Tree` (SMT,
  `iden3/go-merkletree-sql`) and `x/sovereignty/keeper/smt_storage.go`'s `BadgerStorage` remain
  completely unwired dead code (confirmed via exhaustive grep -- `Tree` is constructed in
  `NewKeeper` and never read/written anywhere else; `BadgerStorage` isn't even passed as `NewKeeper`'s
  `smtStore` argument, which uses an in-memory store instead). Deliberately NOT pressed into service
  for `state_root` here: this prototype has no real account/balance state to put in SMT leaves yet,
  and forcing fabricated leaves in would violate this repo's own "don't fabricate data" convention.
  Reviving this SMT for a real future purpose (or removing it) is separate, unstarted work. Also not
  done: replacing the app's KVStore backend (`cosmos-db`, currently GoLevelDB) with BadgerDB was
  investigated and rejected -- `cosmos-db` only implements goleveldb/memdb/pebbledb/rocksdb, and
  Badger's WiscKey (value-separated) architecture is designed for large values, not IAVL's
  small-node/random-access pattern; no ecosystem precedent either. Variable-length/recursive re-
  anchoring proofs (removing the fixed-N limitation) are also out of scope.

**Real 4-node Docker testnet (M6's remaining verification)** is now done, confirmed by actually
running 4 `engram-nodeNN` containers to 20+ blocks with matching `AppHash` at every height,
including a real safety bug found and fixed along the way -- not just `docker compose config`
validating cleanly (that was already true before this pass):
- **Build/compose plumbing fixes**, none visible from just reading the compose files: (1)
  `.dockerignore` was missing `data`/`testnet-data` entries, sending 4.48GB+ of runtime chain data
  as Docker build context on every build for zero benefit -- fixed. (2) the root `Dockerfile` was
  rewritten to use BuildKit cache mounts (`--mount=type=cache` for `/go/pkg/mod`,
  `/root/.cache/go-build`, `/var/cache/apk`) plus a multi-stage build, cutting rebuild time
  substantially. (3) `go.mod`'s `replace github.com/cometbft/cometbft => ...` pointed at an
  absolute host path (`/Users/.../engram-consensus-core`), unbuildable inside any container --
  fixed to a relative path (`../engram-consensus-core`) plus each `docker/engram-validator-node0N.yml`
  gained `build.additional_contexts: [cometbft-fork=../../engram-consensus-core]` (BuildKit
  multi-context build) so the Docker build can see the sibling fork repo despite the relative
  replace pointing outside the primary build context. (4) three unrelated port/path conflicts:
  `docker/engram-monitoring.yml`'s bind mounts used `./config/...` (resolves relative to the
  compose file's own directory, wrong) instead of `../config/...`; prometheus's host port 9090
  collided with celestia-app's gRPC port; celestia-app's host ports 26656/26657 collided with
  engram-node01's CometBFT defaults -- all remapped.
- **`celestia-lightNN` services removed entirely** (all 4, plus their named volumes) from
  `docker/engram-validator-node0N.yml`, per the repo owner's explicit request -- confirmed via grep
  that no Go code anywhere reads `CELESTIA_LIGHT_*` or connects to a light node; `x/da`'s
  `Publisher`/`RPCClient` talk to `celestia-bridge` directly (documented limitation, see Phase 7's DA
  section above).
- **`HealthMonitor.SetAnchorPeers`** (`engram-consensus-core/lp2p/health_monitor.go`) was referenced
  only in a doc comment ("anchor peers start empty and are configured via SetAnchorPeers once
  bootstrap peers are known") but the method never existed -- implemented it, wired into
  `Switch.OnStart()` from `s.host.BootstrapPeers()`. This fix is real and passes `go test ./lp2p/...`,
  but turned out to be **inert in this testnet**: the real generated `config.toml` has
  `[p2p.libp2p] enabled = false` (vanilla CometBFT's own `p2p.Switch` is what's actually running),
  so `lp2p.Switch`/`HealthMonitor` is entirely dormant in every real deployment to date. Kept for
  whenever libp2p transport is actually enabled.
- **Real P2P telemetry for the vanilla `p2p.Switch`** (`cmd/engramd/main.go`'s
  `vanillaP2PHealthAdapter`): since libp2p is disabled, `wireP2PSensor` now type-switches on
  `n.Switch()`'s concrete type and, for the vanilla `*p2p.Switch` case, builds a small local adapter
  that derives `SubnetDiversity`/`ActiveAnchors`/`CleanPeers`/`ChurnRate`/`AvgTenure` from
  `sw.Peers()` (real connected peer IPs/persistence/uptime) -- confirmed via temporary debug prints
  to compute real, correct numbers matching each node's actual peer set. Chosen over enabling real
  libp2p transport (the "more architecturally complete" option) specifically because it carries
  zero risk to the already-working vanilla consensus data plane -- lower-risk option preferred over
  completeness for this piece.
- **Consensus stuck at height 1 forever (infinite round-skipping)** had three independent causes,
  found by iterating through each in turn: (1) `bitcoin-net` and `engram-net` are separate Docker
  bridge networks; `BITCOIN_HOST=bitcoin-node01` was unresolvable from any `engram-nodeNN` container
  until all 4 were explicitly joined to `bitcoin-net` too (with distinct IPs) -- every
  `PrepareProposal` was silently failing its BTC anchor submission with a DNS lookup error. (2)
  bitcoind's regtest wallet was never created/funded ("No wallet is loaded" on every
  `fundrawtransaction` RPC call from `AnchorTracker.SubmitOpReturn`) -- fixed via `createwallet` +
  `generatetoaddress` to fund it. (3) `priv_validator_state.json` round-regression: CometBFT's
  `FilePV` refuses to sign a vote for a round lower than its last-signed round, and restarting a
  container (fresh process) against a *persisted* `priv_validator_state.json` without a matching
  fresh WAL replay reliably triggers this -- the only reliable fix found was wiping `testnet-data/`
  and regenerating fresh genesis (`engramd testnet init-files --v 4`) before every redeploy, now the
  standing operational practice for this testnet.
- **Bitcoin settlement checks rejecting every proposal after height ~1**: `vigilante.Tolerance`
  widens to `KDeepFinality` (default 2) once round>=3 (see Phase 7's liveness-bug note above), so a
  checkpoint confirmed at height H becomes permanently unverifiable the moment
  `h_btc_current - H > KDeepFinality` -- irregular *burst* mining (`bitcoin-cli generatetoaddress 5`
  repeatedly, done manually while debugging) pushed `h_btc_current` past a just-confirmed
  checkpoint's tolerance window before a later consensus round could use it, observed live as
  `hBtcCurrent=120 receiptHeight=116 kDeep=2` (window `[118,120]`, 116 rejected). Fixed by replacing
  manual burst-mining with `scripts/bitcoin_miner_loop.sh`, a steady 1-block-per-20s regtest miner --
  not a code fix, an operational one, but worth recording since it's exactly the kind of gap between
  the spec's idealized "Bitcoin block arrives roughly on schedule" assumption and an
  operator-controlled regtest chain's actual mining cadence.
- **A real, consensus-safety bug found (and fixed by reverting) by running 4 real nodes, invisible
  in any single-node test or unit test**: to make `Query.State` show live BTC/DA/P2P telemetry
  instead of the genesis-default zero value it had shown forever (root cause: `PrepareProposal`/
  `ProcessProposal` only ever write `k.Metrics` to their own throwaway `prepareProposalState`/
  `processProposalState` branches -- BaseApp's ABCI 2.0 state separation -- so those writes never
  reach the committed store `FinalizeBlock`/queries read from), an earlier version of this fix added
  a `RefreshMetrics` call to `x/sovereignty/preblock.go`'s `NewPreBlocker`, writing live sensor data
  into **committed** state. This looked like it worked: `Query.State` started returning real,
  non-zero, matching numbers on 3 of the 4 nodes. It was actually a state-machine-determinism bug:
  `RefreshMetrics` reads each node's own **local** P2P peer snapshot and live bitcoind height --
  writing that into `PreBlocker`'s committed state makes it part of `AppHash`, so any two validators
  whose local sensor view differs even slightly compute a **different** `AppHash` for the identical
  agreed block. 3 of the 4 nodes happened to have near-identical peer views and coincidentally
  matched; the 4th (`engram-node03`) diverged and hit
  `CONSENSUS FAILURE!!! precommit step; +2/3 prevoted for an invalid block: wrong Block.Header.AppHash`
  at height 2 -- its consensus engine's `receiveRoutine` goroutine panicked and died permanently
  (the container kept running and answering RPC, since only that one goroutine crashed, but it could
  never produce another block again). This is exactly the failure mode "sensors propose, consensus
  decides" (this file's top-level Spec Fidelity section) exists to prevent, and this fix violated it
  by routing a live local sensor read into the hashed state tree. **Fixed by reverting**:
  `NewPreBlocker` no longer takes a `*Sensors` or calls `RefreshMetrics` at all -- it commits only
  the fields already deterministically embedded in the *agreed* proposal
  (`ext.BTCReceipt`/`ext.DAReceipt`, via `CommitFSMTransition`, unchanged), never a fresh local
  re-read. Confirmed via a clean redeploy (fresh genesis, rebuilt images): all 4 nodes, including the
  previously-crashed node03, reached 20+ blocks with **identical AppHash** at every height and zero
  further `CONSENSUS FAILURE` occurrences in any container's logs. The FSM state decision itself
  (`fsm_state`) was never wrong or non-deterministic through any of this -- it's computed from live
  sensors only within the safe, throwaway `PrepareProposal`/`ProcessProposal` branches, exactly as
  before; only the raw `PeripheralMetrics` values visible via `Query.State` reverted to being stale
  (the last value genesis or a rare RECOVERING->ANCHORED sync wrote), which is the correct, safe
  tradeoff -- real-time query observability of live per-node sensor data was never actually
  achievable through the committed/hashed state tree without embedding those readings into the
  agreed proposal itself (not done; `ExtendedProposal` carries only `fsm_state`/receipts/
  `zk_proof_ref`, no raw metric values), or exposing them through a side channel outside the state
  machine entirely (e.g. an in-memory, non-committed field with its own query path) -- neither
  attempted here, tracked as unstarted future work if live observability is ever wanted.
- **Missing `timestamp` field, found by checking `ExtendedProposal`
  (`x/sovereignty/proposal.go`) against README section 6.1's "Extended Proposal Structure"**: the
  README documents `value`, `timestamp`, `round`, `fsm_state`, `da_receipt`, `btc_receipt`,
  `zk_proof_ref` (mirroring `EngramTendermint.tla:281-307`'s `prop.timestamp \in
  MIN_TIMESTAMP..MAX_TIMESTAMP` check). `ExtendedProposal` today has only `FSMState`/`DAReceipt`/
  `BTCReceipt`/`ZKProofRef` -- `value` and `round` are implicit (the tx's own block height/round, not
  carried redundantly inside the payload, a reasonable simplification), but `timestamp` has no
  representation anywhere and `req.Time` (available on both `RequestPrepareProposal` and
  `RequestProcessProposal`) is never read by `proposal.go`. This is a real, previously-undocumented
  gap against the spec -- not flagged as a deliberate simplification anywhere in the code, unlike
  this file's other documented gaps (round=0 tolerance, no `is_btc_spv_failed` field, etc.). Left
  unfixed this pass; worth closing before treating `ExtendedProposal` as spec-complete.

**M7 is now done** -- see "E2-E9 live-Docker completion" below for the full real-data story
(previously blocked on M5+M6, both since completed).

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
**Table 6C / Figure 7 (Plonky3 backend comparison) is also now done** (was previously the optional
half of E6, "tùy chọn nếu còn thời gian") -- `scripts/e6_zk_reanchoring_benchmark/benchmark_plonky3.sh`
+ `table6c_collector.py` produce real measured Plonky3 numbers alongside the Noir/UltraHonk ones,
see `docs/EXPERIMENT.md`'s E6 section.

## E2-E9 live-Docker completion + real Sybil/eclipse P2P defense

Everything below happened across several sessions after the section above was last written --
read this before assuming E2-E9 are still "mostly in-process/synthetic" as some of the language
above implies. `docs/EXPERIMENT.md` is the authoritative, continuously-updated record of exactly
which numbers are real live-Docker data vs. in-process vs. synthetic per experiment/row -- this
section is a narrative summary of how that state was reached, not a duplicate of it.

**Real active Sybil/eclipse ingress defense** (previously only passive detection existed, found by
re-reading this project's own early design notes in the fork repo, `lp2p/note02.md`/
`p2p/note_develop.md`, which sketched an active filter + real RTT that was never implemented):
- **Ingress filter**: `x/sovereignty/keeper/peer_filter.go`'s `FilterPeerByAddr`, wired via
  `baseapp.SetAddrPeerFilter` (a stock Cosmos SDK ABCI hook the fork already exposes -- no fork
  changes needed) -- rejects a new peer connection outright if admitting it would push its `/24`
  (or `/48` IPv6) subnet's connected-peer count past `Params.MaxPeersPerSubnet` (default 8).
  `cmd/engramd/main.go`'s `wirePeerFilter` late-binds a live `PeerFilterSource` after
  `node.NewNode()` constructs the real `*p2p.Switch`, mirroring `wireP2PSensor`'s existing
  late-binding pattern for the same ordering constraint.
- **Real per-peer RTT**: piggybacked on the fork's EXISTING `PacketPing`/`PacketPong` keep-alive
  (`p2p/conn/connection.go`'s `MConnection`, new `RTT()` method) instead of building a whole new
  Reactor/protobuf message type as originally planned -- this closed `vanillaP2PHealthAdapter`'s
  previously-hardcoded-0 `Latency` field with real data.
- Confirmed live against a real 10-22-container attacker swarm (`docker/attacker-peer-swarm.yml`,
  new, profile-gated `attacker-swarm-a1`/`attacker-swarm-a2`): the filter correctly capped both
  legs at exactly `MaxPeersPerSubnet=8` peers, real 4-node cluster stayed completely safe
  (matching AppHash, normal height progression) throughout both attacks. **Real Docker
  gateway-priority quirk found live and left honestly unresolved**: a multi-homed container with
  no explicit `gw_priority` consistently routes via the network declared SECOND in its
  `networks:` block, not first -- this defeated leg A2's genuine multi-subnet-diversity goal (all
  attackers landed on the same subnet as the real validators regardless of which
  `attacker-subnet-a/b/c/d` they were assigned) -- see
  `scripts/e4_p2p_eclipse_detection/results_live/sybil_attack_live_run_20260808_summary.md` for
  the full writeup. Not a `FilterPeerByAddr` bug -- it held its cap regardless of which subnet
  peers actually arrived from.
- **Real double-signing detection, closed cheaper than planned**: `RequestFinalizeBlock.Misbehavior`
  already carries real `DuplicateVoteEvidence` from CometBFT's own stock evidence pool -- no
  `x/evidence` (Cosmos SDK) module wiring needed at all, just read the field directly in
  `PreBlocker` (`x/sovereignty/preblock.go`'s `recordDetectedEvidence`, new
  `Keeper.DetectedEvidenceCount`/`LastDetectedEvidence`, `x/sovereignty/types/evidence.go`). Safe
  to commit (unlike a fresh local sensor read) because `Misbehavior` is part of the already-agreed
  block request, deterministic and identical across every honest validator.
- **CI was genuinely broken and got fixed twice**: first attempt used `actions/checkout@v4`'s
  `path: ../engram-consensus-core` to pull in the fork sibling repo for the local
  `go.mod replace` directive -- confirmed broken by real CI failure output (`checkout`'s sandbox
  disallows writing outside the initial workspace). Corrected to a plain `git clone` in a `run:`
  step, pinned to a specific fork commit. Also fixed real `black` formatting debt across
  `scripts/` (`black --check` checks the WHOLE tree per invocation, not just changed files).
  `.claude/skills/github-actions-ci/SKILL.md` updated with both lessons.

**E8 full A1-A8 + Double-signing matrix, now 100% real live-Docker (previously in-process only)**:
every row -- Eclipse, Sybil, Data Withholding, Forged BTC Receipt, Withdrawal-During-SOVEREIGN,
Malicious Proposer, Censorship, Combined Attack, and Double-signing -- now has a real pass against
the live 4-node cluster (`scripts/e8_attack_resilience/live_*.py`), driven by a new
`ENGRAM_BYZANTINE_BEHAVIOR` env var (`x/sovereignty/proposal.go`'s `applyByzantineBehavior`, never
set on a real validator) that makes `PrepareProposal` deliberately lie about its own proposal
(`fake_fsm_state:<STATE>`, `forge_btc_hash`, `false_da_attestation`, `censor_tx:<hex>`) so the
OTHER (honest) validators' real `ProcessProposal` rejection path gets exercised live, not just
unit-tested. Double-signing itself is real equivocation, not simulated: a second `engramd`
process holds node04's real signing key but its own separate `priv_validator_state.json`
(`docker/engram-validator-node04-duplicate.yml`) -- since FilePV's own state file is CometBFT's
built-in anti-double-sign safety net, NOT sharing it is what lets the second process actually
equivocate. Confirmed live: all 3 honest validators detected real `DuplicateVoteEvidence` with
1-block detection latency, reproduced twice independently in the same run.

Getting this live-clean surfaced **6 real bugs, none visible from reading the code, all found by
actually running it**:
1. **Withdrawal-tx permanent liveness deadlock**: `ProcessProposal`'s check #4 correctly rejected
   a withdrawal tx while SOVEREIGN, but `PrepareProposal` never filtered it out of ITS OWN
   proposal -- the tx just sat in mempool and every subsequent leader kept re-proposing it,
   getting rejected forever, real round-skip stall observed for dozens of rounds. Fixed:
   `PrepareProposal` now filters withdrawal-marked txs out of its own proposal while
   `WithdrawLocked`, not just relying on check #4 as the only guard (`x/sovereignty/proposal.go`).
2. **`ForcedTxQueue` never dequeued after inclusion -- a second, worse permanent deadlock**:
   `updateForcedTxTracking` only ever reset `ignoredRounds` to 0 on inclusion, never removed the
   entry -- but a tx can only ever be included ONCE (consumed from mempool on commit), so every
   round after that had `included[tx]==false` forever, tripping `IsCensoring` permanently, on
   EVERY validator, even after reverting the byzantine one back to honest. Fixed: dequeue the
   entry from `ForcedTxQueue`/`TxIgnoredRounds` entirely once included, not just reset the counter
   (`x/sovereignty/preblock.go`).
3. **`SubmitForcedTx` accepted content that could never satisfy inclusion -- an unbounded,
   no-privilege-required DoS vector**: root cause of a THIRD instance of the same deadlock class --
   choosing "another `MsgSubmitForcedTxRequest`" as a test's forced-tx target meant broadcasting
   it re-triggered `SubmitForcedTx`'s OWN handler, queuing its inner payload (a bare string, never
   a valid raw tx) as an unsatisfiable entry. Real implication: ANY account submitting
   `MsgSubmitForcedTx` with undecodable content can halt the entire network permanently, no
   validator privilege needed. Fixed at the source: `SubmitForcedTx` now rejects content that
   doesn't decode as a real tx via a new optional `Keeper.TxDecoder` (nil-safe, wired from
   `app.go`'s `txConfig.TxDecoder()`, same pattern as `peerFilterSrc`) --
   `x/sovereignty/keeper/msg_server.go`, `keeper.go`, `app/app.go`.
4. **Compose project-identity collision**: `docker/engram-validator-node04-byzantine.yml` had its
   own top-level `name:` -- Compose merges/forks project identity from the LAST file's `name:`
   when given multiple `-f` flags, so swapping in this override forked a separate Compose project
   from the real cluster's, causing a real "container name already in use" crash the first time
   it was actually invoked. Fixed by removing `name:` from both this file and
   `engram-validator-node04-duplicate.yml` (which had the same latent bug, non-fatal there only
   because it creates a new container rather than swapping an existing one).
5. **Docker Desktop (macOS virtiofs) cannot mount a file whose target path resolves inside a
   directory that is itself already a bind-mount source** -- `docker/engram-validator-node04-duplicate.yml`'s
   nested `priv_validator_key.json`/`genesis.json` mounts failed with "mountpoint ... is outside
   of rootfs", a real environment limitation, not a YAML mistake. Fixed by having the caller
   (`scripts/e8_attack_resilience/live_double_signing_test.py`'s `stage_duplicate_identity`) copy
   both files onto the host BEFORE `docker compose up`, landing inside the container for free via
   the single top-level home-dir mount.
6. **Missing `priv_validator_state.json` bootstrap for the duplicate-key harness**: `engramd init`
   bails out early once genesis/key files already exist (staged by fix #5 above), so it never
   reached the step that creates FilePV's state file -- `engramd start` crashed real
   ("no such file or directory") the first time this harness actually got far enough to run.
   Fixed: `stage_duplicate_identity` writes a fresh `{"height":"0","round":0,"step":0}` itself --
   correct by design, since the whole point of this harness is a validator that has never signed
   anything, not a copy of the real node04's already-advanced state.

**Prometheus/Grafana monitoring stack removed** (`docker/engram-monitoring.yml`,
`config/prometheus.yml`, `config/grafana/`, plus a dead `fetch_prometheus_metric` helper in
`scripts/utils.py`) -- confirmed via grep that no E2-E9 experiment script ever reads the
Prometheus HTTP API; every real number in this repo comes from `scripts/framework/logger.py`
polling CometBFT RPC/ABCI-query directly. Each `engram-nodeNN`'s own built-in CometBFT
Prometheus-format `/metrics` endpoint is unrelated and was left in place (free, always-on, just
unscraped).

**E9 live combined trace -- includes a real self-correction worth remembering**: Phases 1-6 (BTC
congestion + DA outage + P2P churn, layered not sequential) passed cleanly against the live
cluster; Phase 7 (wait for real ANCHORED via the ZK pipeline) legitimately timed out after 600s.
The FIRST write-up of this in `docs/EXPERIMENT.md` attributed the timeout to "proof rejected by
the chain, staleness race" -- copied from `watch_and_prove.sh`'s own generic log message on any
non-zero exit, **never independently verified**. When directly challenged on whether this
explanation was trustworthy, re-running `prove_and_submit.sh` by hand with `bash -x` showed it
was dying at Step 1/4 with **SIGPIPE (exit 141)**, never even reaching proof submission:
`HEADER_LINES=$(echo "$ALL_HEADER_LINES" | head -n "$EXPECTED_N")` breaks under `set -o pipefail`
once the tracked interval is large enough that `head` reads its N lines and exits before `echo`
finishes writing the rest. Fixed with a here-string (`head -n N <<< "$ALL_HEADER_LINES"`, no live
pipe to race) -- re-ran and got a real accepted proof submission with a real checkpoint advance.
**The original explanation was retracted and corrected in `docs/EXPERIMENT.md`** rather than left
standing. Lesson: a log message explaining a failure is a claim to verify, not a fact to repeat,
especially one`s own tooling's generic catch-all message. Also found and fixed while chasing this:
`scripts/framework/injector.py`'s `cleanup_profile` (shared by every chaos script) only ever
called `docker compose rm -f` without `stop` first -- invisible everywhere else because prior
callers always ran it AFTER a Pumba profile's own `--duration` had already elapsed naturally, but
E9's churn-burst phase deliberately cuts a profile short, leaving it stuck "Up" forever and
correctly triggering `wait_for_no_active_netem`'s refusal to layer a second profile on top.

**Figure regeneration for real academic-paper proportions**: `scripts/utils.py` gained
`figsize_single`/`figsize_multi_panel`/`figsize_row`/`figsize_grid`/`savefig_academic` --
IEEE/ACM two-column standard widths (3.5in single-column, 7.16in double-column), golden-ratio
height, 300 DPI PNG (up from a stale 150 DPI) + vector PDF as the primary format. Applied across
every figure-generating script (E2/E5/E6/E9). New live-data figure builders
(`scripts/e{2,5,9}_*/live_figure_builder.py`) replaced the 3 figures that were still sourced from
in-process/synthetic data with real live-Docker equivalents. Found and fixed while building these,
each only visible by actually looking at the rendered PNGs, not just checking exit codes:
- E2's `representative_rows()` originally picked the alphabetically-first node as representative --
  but S4/S5 specifically isolate `engram-node01`, so its own samples were almost all RPC-error
  sentinels, collapsing those panels to seconds instead of minutes. Fixed to pick whichever node
  has the most valid samples in that scenario.
- Multiple label/title-overlap and suptitle-clipping layout bugs (labels too long for a 3-panel
  row at double-column width; rotated y-axis labels clipped against the canvas edge) -- fixed with
  shorter label text, taller figures, and explicit `tight_layout(rect=...)` margins per figure.
- Redeploying fresh for E5's `HysteresisWait=10` combo hit the SAME operational bugs documented
  elsewhere in this file (Bitcoin wallet unloaded after a container restart; separately,
  `bitcoin_miner_loop.sh` had silently stopped running, so a freshly-submitted anchor tx could
  never accumulate `kDeepFinality` confirmations) -- real, recurring evidence for why
  `docs/DEVELOPMENT.md` insists the Bitcoin wallet must be funded/mature AND continuously mining
  before AND throughout every `engramd` run, not just at first bootstrap.

**Documentation overhaul**: `docs/DEVELOPMENT.md` (previously a single narrow section on manual
Bitcoin regtest fork/reorg testing) rewritten to cover the full real build/test/lint workflow, the
real multi-node Docker deploy sequence with its load-bearing ordering (Bitcoin/Celestia funded and
mining BEFORE `engramd`, never burst-mine live, always wipe `testnet-data/` before redeploy, never
bare `docker compose down`), and the ZK re-anchoring pipeline -- with mermaid diagrams. Root
`docs/ARCHITECTURE.md` corrected (stale `172.20.0.0/24` IP range, stale celestia-light/vigilante
sidecar references, stale "mock-controlled" sensor description) and given 2 real mermaid diagrams
(network topology; the `PrepareProposal`/`ProcessProposal`/`PreBlocker` consensus flow). Root
`README.md` rewritten in English as a concise, image-driven entry point (repo structure,
architecture diagram slot, quick start, and a real E1-E9 results gallery linking each figure's
full-resolution PDF) -- deliberately does NOT link this file, which is written for AI-agent
session continuity, not human onboarding.

Do not assume any of the above exists just because a directory or file for it is present.

## Vietnamese comments

Existing code mixes Vietnamese and English comments (the repo owner is a Vietnamese speaker). Match
whichever a file already uses; don't force a wholesale translation pass.
