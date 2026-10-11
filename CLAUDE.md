# CLAUDE.md

Guidance for Claude Code when working in this repository.

## What this repository is

A Cosmos SDK + CometBFT prototype implementing **Engram Hybrid Adaptive Consensus** — a
blockchain consensus protocol that treats peripheral network health (Bitcoin settlement finality,
Celestia data availability, peripheral P2P health) as a first-class consensus variable, degrading gracefully
into a local-PoS "Sovereign" fallback instead of halting when those layers fail.

The formal TLA+ specification and its TLC/Apalache proofs live in `spec/` (see `spec/CLAUDE.md`).
**This root-level Go code is the reference implementation of that spec**: every non-trivial piece
of logic is a direct port of a named operator in `spec/core/*.tla`, cited in a comment. That
traceability is load-bearing — see "Spec fidelity" below.

## Repository layout

```
x/sovereignty/      -- FSM engine (Cosmos SDK module): state machine, sensors, circuit breaker,
                        ABCI++ hooks (PrepareProposal/ProcessProposal/PreBlocker)
x/da/                -- Celestia DA receipt type + verification
x/anchor/         -- Bitcoin settlement receipt type + anchor re-verification
app/                 -- EngramApp: BaseApp wiring (codec, KVStoreKey, ante handler, ABCI++ hooks);
                        no bank/auth/staking module mounted
cmd/engramd/         -- node binary: `engramd init`, `start [--vanilla]`, `testnet init-files --v N`
proto/               -- protobuf sources (buf); generated .pb.go land in x/sovereignty/types/
circuit/reanchoring/ -- Noir ZK circuit for the re-anchoring recovery proof
tests/e2e/           -- in-process fault-injection harness (docs/EXPERIMENT.md's E2)
docker/, docker-compose.yml -- multi-node local testnet + Pumba chaos-engineering profiles
scripts/             -- Python analysis/plotting for the E1-E9 experiments
docs/EXPERIMENT.md   -- the experiment design this codebase satisfies (E1-E9); read before adding
                        experiment-facing code
docs/DEVELOPMENT.md  -- full build/test/deploy workflow, including the Docker multi-node sequence
```

`x/sovereignty/keeper/zk_assets/barretenberg.wasm.gz` (the embedded ZK verifier) is built by the
sibling [engram-barretenberg-wasm](https://github.com/cuongct924/engram-barretenberg-wasm) repo —
provenance only: `zk_assets/ASSETS.lock` records each asset's SHA-256 and source, `scripts/verify_zk_assets.sh` checks them.

## Build, test, lint

```bash
go build ./...
go vet ./...
go test ./...              # or: make test
golangci-lint run          # or: make lint (config: .golangci.yml, schema v2 -- needs golangci-lint v2+)
make proto-gen              # requires buf, protoc-gen-go, protoc-gen-go-grpc on PATH
```

`go vet` must stay clean.

## Code style / spec fidelity

Every operator ported from `spec/core/*.tla` keeps its TLA+ name and cites the source file+line
range in a comment. Branch/CASE order and structure must match the spec exactly — do not
"improve" or simplify it (a previous bug: an added branch not in `CalculateNextFSMState` violated
`StrictFSMTransitionSafety`). See the `go-spec-fidelity` skill for the full rule set (reuse vs.
reimplement, pointer semantics, documenting simplifications) when touching
`x/sovereignty`/`x/da`/`x/anchor`.

Comments: cite the spec operator + line range for ported logic; otherwise comment only the
non-obvious "why" (a hidden constraint, a workaround, an invariant), never the "what". English
only, 2-3 lines per comment unless a genuine divergence from the spec needs justifying.

## Critical architecture constraints

- **"Sensors observe, quorum certifies, the state machine decides" is not optional.** Each
  validator signs its own sensor snapshot as a vote extension (`x/sovereignty/vote_extension.go`'s
  `ExtendVote` — the only ABCI++ hook allowed to touch live sensors/RPC). `PrepareProposal`
  aggregates ≥2f+1 voting power's worth of signed extensions into a `HealthCertificate`
  (`AggregateHealthCertificates`, a pure deterministic function — no RPC) and prepends it as
  `Txs[0]`; `ProcessProposal` only re-verifies and re-runs that same aggregation, never touches a
  live sensor. FSM state is written only by `CommitFSMTransition` (`x/sovereignty/preblock.go`),
  from that already-agreed certificate — never recomputed from a node's own live sensors inside
  committed state. (`x/sovereignty/abci.go`'s `BeginBlocker` is a separate, simplified path used
  only by the in-process `tests/e2e` harness.)
- **Never write live local sensor reads into committed/hashed state.** A `PreBlocker` that calls
  `RefreshMetrics` makes each validator's local P2P/BTC view part of `AppHash`, and validators with
  even slightly different local readings diverge — this caused a real consensus failure. Only
  `ext.BTCReceipt`/`ext.DAReceipt` (already-agreed proposal data) are safe to commit.
- **`*PeripheralMetrics` is always a pointer**, never a value, in signatures, struct fields, or
  `collections.Item[...]` — it's a proto3 type embedding `sync.Mutex`; passing by value trips
  `go vet`'s lock-copy check.
- **`go.mod` replaces `github.com/cometbft/cometbft` with the public `engram-consensus-core` fork,
  pinned to a commit (pseudo-version).** Nothing needs checking out beside this repo. To develop
  the fork locally, use a `go.work` file (`go work init . ../engram-consensus-core`), never edit
  the `replace` line for it; when the fork changes, move the pin (`go get
  github.com/cuongct924/engram-consensus-core@<commit>`, then update the `replace` line).
- **Docker multi-node testnet**: always wipe `testnet-data/` and regenerate fresh genesis
  (`engramd testnet init-files --v N`) before redeploying. A persisted `priv_validator_state.json`
  without a matching fresh WAL replay trips FilePV's anti-double-sign guard.
- **Bitcoin regtest must mine continuously** (`scripts/bitcoin_miner_loop.sh`, steady cadence) —
  burst-mining pushes `h_btc_current` past a checkpoint's tolerance window and breaks BTC
  settlement checks.
- **`ENGRAM_BYZANTINE_BEHAVIOR`** (`x/sovereignty/proposal.go`) is a deliberate-misbehavior test
  hook for `docs/EXPERIMENT.md`'s E8 attack rows — must never be set on a real validator.
- This app has no `x/auth`/`x/bank`/`x/staking` mounted: transactions are unsigned envelopes, and
  the validator set comes only from genesis (no staking-driven rotation).

## Troubleshooting

Full detail and fixes: `docs/DEVELOPMENT.md` §5.5 (gotchas) and §10 (quick reference). The
non-obvious ones, found live while doing a from-scratch redeploy:

- `data/bitcoin-node0N/` and `data/celestia-*/` persist across `testnet-up` redeploys, unlike
  `testnet-data/`. A fresh genesis against old, far-advanced chain data desyncs `btc_gap`/`da_gap`
  from their near-zero baseline — wipe both together for a genuinely clean trial, not just
  `testnet-data/`.
- Wiping `celestia-app` and its bridges in the same cycle races the bridge's one-time
  `TrustedHash` fetch; a bridge that loses that race crash-loops permanently and needs its own data
  dir wiped again (after `celestia-app` is stable) to re-fetch.
- A celestia bridge's blob-submission account needs its own TIA funding, or every `blob.Submit`
  fails with "account ... not found" — `make testnet-up` now runs
  `scripts/testnet_fund_celestia_bridge.sh` for each bridge automatically, so a normal deploy
  shouldn't hit this. If it's ever unfunded anyway: this isn't just a silent audit-trail gap — it
  feeds `is_das_failed`/`is_attestation_failed=true` into the real per-block DA sensor, which keeps
  the FSM stuck in `SOVEREIGN` forever (`IsHealthyCondition` requires `IsDAHealthy`), with no log
  line at the FSM layer pointing back to the cause.

## Current status

The full E1-E10 experiment suite (`docs/EXPERIMENT.md`) are done, with real (not
synthetic) data measured against a live 4-node Docker testnet plus real Bitcoin regtest and
Celestia infrastructure. Real active Sybil/eclipse P2P ingress defense, double-signing detection,
and the ZK re-anchoring pipeline (Noir/Barretenberg, max-N=256 circuit) are wired end-to-end.
`docs/EXPERIMENT.md` is the authoritative, continuously-updated record of exactly which numbers
are real live-Docker data vs. in-process vs. synthetic, per experiment row — check there, not
this file, for current measurement status. Don't assume a feature exists just because a directory
or file for it is present; grep for the real wiring.

## Vietnamese comments

Existing code mixes Vietnamese and English comments (the repo owner is a Vietnamese speaker). Match
whichever a file already uses; don't force a wholesale translation pass unless asked.
