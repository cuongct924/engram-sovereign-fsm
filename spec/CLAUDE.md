# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this repository is

A TLA+ formal specification and TLC model-checking harness for **Engram Hybrid Adaptive Consensus** — a modular blockchain consensus that treats peripheral network health (Bitcoin settlement finality, Celestia data availability, P2P health) as a first-class consensus variable, degrading gracefully into a local-PoS "Sovereign" fallback instead of halting when those layers fail.

There is no application source code here: the deliverable is the spec files (`.tla`), their model-checking configs (`.cfg`), and `README.md` — the full paper describing the design and proofs. Read `README.md` before non-trivial spec changes; it is authoritative on semantics, invariants, and liveness properties. Keep the corresponding README section (invariant statement, transition table, sensor formula) in sync with any `core/*.tla` change.

## Running the verification

Requires Java JDK 11+ and `tla2tools.jar` (from the [TLA+ releases page](https://github.com/tlaplus/tlaplus/releases)) at a known path. Run TLC from the repo root (`spec/`), pointing at a model-checking (`MC_*`) file and its matching `.cfg`:

```bash
java -cp /path/to/tla2tools.jar tlc2.TLC -workers 8 \
  -config core/MC_ServerRefinementSafety.cfg \
  core/MC_ServerRefinementSafety.tla
```

No separate `mc/` directory, no symlinks — specs and every `MC_*` driver (TLC and Apalache alike) live directly in `core/`. Deliberate: both tools resolve `EXTENDS` relative to the file being checked, and Apalache has no search-path flag. A prior symlinked `core/`+`mc/` split was dropped because a symlink silently becomes a disconnected copy under `git config core.symlinks false` or certain editors.

The four spec layers each have their own `MC_*` model-checking pair, all living directly in `core/` — swap the filename below for the layer under test:

| Layer | Spec under test | Safety model | Liveness model |
|---|---|---|---|
| FSM (sovereign fallback / circuit breaker) | `core/EngramFSM.tla` | `core/MC_FSMSafety.{tla,cfg}` | `core/MC_FSMLiveness.{tla,cfg}` |
| Tendermint (consensus engine) | `core/EngramTendermint.tla` | `core/MC_TendermintSafety.{tla,cfg}` | — |
| Abstract consensus (LiDO ADO model) | `core/EngramConsensus.tla` | `core/MC_ConsensusSafety.{tla,cfg}` | `core/MC_ConsensusLiveness.{tla,cfg}` |
| Server (full refinement bridge, all layers integrated) | `core/EngramServer.tla` + `core/EngramServerRefinement.tla` | `core/MC_ServerRefinementSafety.{tla,cfg}` | `core/MC_ServerRefinementLiveness.{tla,cfg}` |

Full-server checks are the expensive ones; FSM/consensus/tendermint-layer checks are much cheaper and are the right target when iterating on a single layer instead of the whole hybrid protocol. For current benchmark numbers (states generated, depth, time) across several parameter scales, see `README.md`'s §9.2 Formal Verification Stress Test table — that table is the single source of truth for these figures, so they aren't duplicated here.

To iterate on a single invariant/property without editing the checked-in `.cfg`, comment/uncomment entries under `INVARIANTS` / `PROPERTIES` in the relevant `.cfg` file (see the commented-out `Sanity_*` lines in `core/MC_ServerRefinementSafety.cfg`/`core/MC_ServerRefinementLiveness.cfg` for the existing pattern) rather than adding a new config file.

VS Code with the TLA+ extension works too: open an `MC_*.tla` file, run "TLA+: Check model with TLC" from the command palette, and select the matching `.cfg`.

**Apalache** is a complementary bounded-safety checker (SMT-based, via Z3) for the same state-safety invariants — it does not replace TLC for refinement (`RefinementSafety`/`RefinementLiveness`) or for `~>`+fairness liveness properties, which remain TLC-only. Each layer's Apalache entry point is `core/MC_<Layer>Safety_Apalache.tla` (e.g. `core/MC_FSMSafety_Apalache.tla`), `EXTENDS`-ing the corresponding TLC driver and defining an `ApalacheCInit` operator that mirrors the `.cfg`'s `CONSTANTS` by hand — keep the two in sync manually. See `README.md`'s "Apalache — Complementary Bounded Safety Checking" section for install steps and example commands.

## Architecture: the four-layer refinement hierarchy

The spec is built as a refinement stack, each layer `EXTENDS` the one below it, so a property proved abstractly is mechanically inherited by the concrete layer above:

```
core/EngramVars.tla            -- shared CONSTANTS/VARIABLES declarations only, extended by FSM & Tendermint
core/EngramConsensus.tla        (Layer 1 - Abstract Core: LiDO Atomic Distributed Object model,
                                  fork-choice rule `canElect`, K-Deep finality, max-stake-branch rule)
        ^ instantiated by (not extended by) the refinement bridge
core/EngramFSM.tla       core/EngramTendermint.tla   (Layer 3 - Concrete Implementations)
  (adaptive circuit           (CometBFT-style Propose -> Prevote -> Precommit -> Commit
   breaker: sensors,           engine; extended Proposal carries fsm_state/da_receipt/
   hysteresis, ZK re-anchor    btc_receipt/zk_proof_ref; Byzantine actions for data
   proof validity flag)        withholding, censorship, timeout flooding)
        \___________________________/
                    |
core/EngramServer.tla            (Layer 2 - Refinement Bridge: EXTENDS EngramFSM, EngramTendermint;
                                   intercepts Tendermint events via 4 server hooks and builds the
                                   abstract LiDO certificate tree)
core/EngramServerRefinement.tla  (EXTENDS EngramServer; defines `AbstractConsensus == INSTANCE
                                   EngramConsensus WITH ...` mapping concrete state to abstract
                                   state, e.g. mapped_tree, mapped_fsm_state, mapped_local_times;
                                   this is what makes refinement-based proof possible)
```

Key implication for editing: **`EngramConsensus.tla` is never `EXTENDS`-ed directly by the concrete layers** — it is pulled in only via the `INSTANCE ... WITH` mapping in `EngramServerRefinement.tla`. If you change a variable name or operator signature in `EngramConsensus.tla`, the fix-up point is the `WITH` substitution list in `EngramServerRefinement.tla`, not `EngramServer.tla`.

The four server hooks in `EngramServer.tla` are the translation points between the concrete Tendermint pipeline and abstract LiDO operations — `Server_InsertProposal` → Pull (E_QC), `Server_ProposerVotes` → Invoke (M_QC), `Server_UponProposalInPrecommitNoDecision` → Push (C_QC, and this is also where FSM state sync happens: `CalculateNextFSMState` is proposed pre-commit, `ExecuteFSMTransition` writes it after commit), `Server_UponTimeoutCert` → Timeout (T_QC).

## Working within the FSM layer (`core/EngramFSM.tla`)

The FSM has 4 states (`ANCHORED`, `SUSPICIOUS`, `SOVEREIGN`, `RECOVERING`) with a strict adjacency graph enforced by `StrictFSMTransitionSafety` — transitions are never a direct jump, e.g. `RECOVERING` can only go to `ANCHORED` or back to `SOVEREIGN`, never straight to `SUSPICIOUS`. State transitions are computed as a pure function `CalculateNextFSMState` (sensor readings → target state) and written by the action `ExecuteFSMTransition`; a validator only prevotes for a proposal whose embedded `fsm_state` matches its own locally-computed `CalculateNextFSMState`. If you touch transition logic, update it in `CalculateNextFSMState`'s CASE expression, not by adding a parallel action — this keeps the "sensors propose, consensus decides" separation that `FSMStateConsistency` (in `EngramServer.tla`) depends on.

The three sensor categories (Bitcoin finality gap, DA gap, P2P/tri-interface health) each resolve to a boolean feeding `IsWarningCondition` / `IsCriticalCondition` / `IsHealthyCondition` — these three predicates are the only integration surface between sensors and the FSM transition function. `safe_blocks` is the hysteresis counter gating `RECOVERING -> ANCHORED`; any deterioration into `IsCriticalCondition` resets it to 0, per `HysteresisSafety`.

## Naming conventions to preserve

- Invariants checked by TLC are named `<Noun>Safety` (e.g. `CircuitBreakerSafety`, `HysteresisSafety`, `MonotonicitySafety`); temporal liveness properties use `~>` ("leads to") and are named `<Noun>Liveness` (e.g. `CircuitBreakerLiveness`, `RecoveryAttemptLiveness`).
- `MC_*` files are TLC-specific instantiations (small constant substitutions like `MC_Nodes`, `MC_Byzantine`, bounded numeric ranges) — never add real protocol logic there; new behavior belongs in `core/`.
- `Sanity_*` invariants (see `core/MC_ServerRefinementSafety.cfg`/`core/MC_ServerRefinementLiveness.cfg`, mostly commented out) are meant to fail — they're negative/vacuity checks confirming the model can actually reach the states it's supposed to be verifying (e.g. `Sanity_NeverSovereign` should be violated, proving SOVEREIGN is reachable). Don't treat a `Sanity_*` violation as a bug.
