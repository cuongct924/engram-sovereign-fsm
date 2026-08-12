# TLA+ operator → Go port map

Index of every `spec/core/*.tla` operator ported into `x/sovereignty`, `x/da`, `x/anchor`, built
by grepping each file's own citation comment. Treat this as a starting point for "does this
predicate already exist", not the source of truth — the citation comment on the Go function itself
is authoritative; re-grep before relying on a row here if the function may have moved.

| TLA+ operator | Spec location | Go function | Go file |
|---|---|---|---|
| `CalculateNextFSMState` | `EngramFSM.tla:309-329` | `CalculateNextState` | `x/sovereignty/keeper/circuit_breaker.go` |
| `safe_blocks` update | `EngramFSM.tla:343-346` | `NextSafeBlocks` | `x/sovereignty/keeper/circuit_breaker.go` |
| `suspicious_duration` update | `EngramFSM.tla:337-340` | `NextSuspiciousDuration` | `x/sovereignty/keeper/circuit_breaker.go` |
| `WithdrawLocked` | `EngramFSM.tla` | `WithdrawLocked` | `x/sovereignty/types/state.go` |
| initial state | `EngramFSM.tla:143-165` | `DefaultGenesis` | `x/sovereignty/types/genesis.go` |
| `is_das_failed` / `is_attestation_failed` | `EngramFSM.tla` | `(*DASensor).SetFailureFlags` | `x/sovereignty/keeper/sensors/celestia_da_gap.go` |
| `da_gap` formula | `EngramFSM.tla:87` | `daGapMetric` | `x/sovereignty/sensors_refresh.go` |
| `h_btc_current`/`h_btc_anchored` gap | `EngramFSM.tla:95` | `btcGapMetric` | `x/sovereignty/sensors_refresh.go` |
| `IsCensoring` | `EngramTendermint.tla:310-315` | `IsCensoring` | `x/sovereignty/types/censorship.go` |
| `UpdateIgnoredRounds` | `EngramTendermint.tla:493-503` | `NextIgnoredRounds` | `x/sovereignty/types/censorship.go` |
| `UpdateIgnoredRounds` (call site) | `EngramTendermint.tla:493-503` | `updateForcedTxTracking` | `x/sovereignty/preblock.go` |
| `ServerInsertProposal` | `EngramServer.tla:52-102` | `NewPrepareProposalHandler` | `x/sovereignty/proposal.go` |
| `IsValidProposal` | `EngramTendermint.tla:281-307` | `NewProcessProposalHandler` | `x/sovereignty/proposal.go` |
| `VerifyZkProof` | `EngramTendermint.tla:257-260` | `verifyZkProofFlag` | `x/sovereignty/proposal.go` |
| `SubmitToCelestiaDA` | `EngramTendermint.tla:886-892` | `(*MsgServerImpl).SubmitForcedTx` | `x/sovereignty/keeper/msg_server.go` |
| `ServerUponProposalInPrecommitNoDecision` | `EngramServer.tla:135-189` | `NewPreBlocker` / `CommitFSMTransition` | `x/sovereignty/preblock.go` |
| `DATolerance` | `EngramTendermint.tla:237-241` | `Tolerance` | `x/da/verify.go` |
| `IsValidProposal`'s DA-pipeline check | `EngramTendermint.tla:290-294` | `VerifyReceipt` | `x/da/verify.go` |
| `DANormalUpdate`/`DAFailure` | `EngramFSM.tla:196-212` | `(*Publisher).MaybePublish` | `x/da/publisher.go` |
| `ExpectedBlockHash` | `EngramTendermint.tla:266` | `ExpectedBlockHash` / `BlockHash` | `x/anchor/types.go` |
| `BTCTolerance` | `EngramTendermint.tla:243-246` | `Tolerance` | `x/anchor/verify.go` |
| `VerifySPVProof` | `EngramTendermint.tla:271-275` | `VerifySPVProof` | `x/anchor/verify.go` |
| `IsValidProposal`'s BTC-settlement check | `EngramTendermint.tla:296-298` | `VerifyReceipt` | `x/anchor/verify.go` |

## Regenerating this table

```bash
grep -rn "spec/core/.*\.tla:" x/sovereignty x/da x/anchor --include="*.go" | grep -v "_test.go"
```

Re-run after any change that adds, renames, or moves a ported function, and update the row rather
than letting it drift — a stale mapping here is worse than no mapping, since rule 3 (reuse, don't
recompute) depends on this being trustworthy enough to grep-and-trust at a glance.
