# Finding: Liveness Deadlock in `MC_ServerRefinementLiveness` (checked-in, unmodified)

**Status:** confirmed real, root-caused, fix implemented (Hướng A+B, §4 below), re-verification in progress. Discovered 2026-08-03 while re-running stress-test configs (C1) after this session's Safety-bugfix pass. A second, related bug was found during re-verification of the fix -- see §7.

**Severity:** breaks the checked-in Liveness claim. `Theorem 8.1 (Autonomous Liveness under Degradation)` and the properties `ServerEventualDecisionLiveness`, `EventualDecisionUnderGSTLiveness`, `RefinementLiveness` are currently **false** for the spec as checked in, not merely unverified.

---

## 1. How it was found

While re-running the §9.2 stress-test table (config C1, smallest parameters) on `core/MC_ServerRefinementLiveness.tla`/`.cfg`, TLC reported a violation at a suspiciously shallow depth (3). Four escalating tests were run to rule out a parameter/bound artifact before concluding this is real:

| Run | `MAX_TIMESTAMP` | `MAX_ROUND` | `MAX_BTC_HEIGHT`/`MAX_ENGRAM_HEIGHT` | Result |
|---|---|---|---|---|
| C1 v1 | 4 | 2 | 2 / 2 | **Violated**, depth 3 |
| C1 v2 | 12 | 2 | 2 / 2 | **Violated**, depth 3 (ruled out timestamp bound) |
| C1 v3 | 12 | 4 | 2 / 2 | **Violated**, depth 3 (ruled out round bound) |
| C1 v4 | 12 | 4 | 3 / 3 (= canonical) | **Violated**, depth 3 (config now identical to checked-in) |
| **Canonical** `MC_ServerRefinementLiveness.cfg`/`.tla`, no copy, no edits | 12 | 4 | 3 / 3 | **Violated**, depth 3, confirms it's real |

Canonical run command (fully reproducible, zero modifications to any checked-in file):
```bash
java -cp tla2tools.jar tlc2.TLC -workers 4 \
  -config core/MC_ServerRefinementLiveness.cfg \
  core/MC_ServerRefinementLiveness.tla
```

**Result:** `Error: Temporal properties ServerEventualDecisionLiveness, EventualDecisionUnderGSTLiveness, and RefinementLiveness were violated.` Counterexample: 173,071 states generated, 9,487 distinct, depth of complete state graph search = 3, 2min 9s.

Trace and full log archived at `spec/traces/liveness_deadlock_canonical.trace.tla` and `spec/traces/liveness_deadlock_canonical.log.txt`.

---

## 2. The counterexample, state by state

```
State 1: <Initial predicate>              round=0 (all honest), real_time=0, step=PROPOSE
State 2: <ServerByzantinePull>             quorum_certs' = quorum_certs U {E_QC for round 0, caller n1}
                                           msgs_propose[0] STAYS EMPTY (n1 never actually proposes)
State 3: Stuttering                        round=0, real_time=0 -- TLC's notation for
                                           "the system can loop here forever"
```

`n1` (the fixed Byzantine node, proposer of round 0 in the checked-in schedule) never broadcasts a real proposal. `ServerByzantinePull` still lets it register an abstract `E_QC` for round 0 directly in `quorum_certs`, modeling "the leader silently starts but never really proposes." From that point on the system never advances, at any depth, regardless of how large the bounds are made.

---

## 3. Root cause, traced through the code

### 3.1 The abstract pacemaker (`core/EngramConsensus.tla`)

```tla
Elapse ==
    /\ ~\E c \in tree : c.cert_round = round /\ c.type \in {"E", "M"}   \* blocks once Pull/Invoke started
    /\ rem_time > 0
    /\ rem_time' = rem_time - 1
```
Once an `E`-cache (Pull) or `M`-cache (Invoke) exists for the round, the abstract countdown timer `rem_time` freezes. The only ways out are `Push` (forms a `C`-cache, letting `EarlyStartNext` advance `round`) or `rem_time` already having reached 0 before the Pull happened.

### 3.2 The concrete mirror (`core/EngramServer.tla`, added by this session's task #33)

```tla
ServerAdvanceRealTime ==
    /\ AdvanceRealTime
    /\ LET current_max_round == Max({round[p] : p \in HonestNodes}) IN
       ~\E qc \in quorum_certs : qc.type \in {"E_QC", "M_QC"} /\ qc.round = current_max_round
```
This guard is a **correct** mirror of `Elapse` — added specifically because the previous (narrower, Byzantine-only) version let the concrete clock advance past states the abstract model forbids, which is an actual Safety/refinement bug that task #33 fixed (confirmed via `RefinementSafety` at the time).

### 3.3 The part nobody had traced through: `AdvanceRealTime` is not just the shared clock

```tla
AdvanceRealTime ==
    /\ real_time' = real_time + 1
    /\ local_clock' = [p \in HonestNodes |-> local_clock[p] + 1]
    /\ local_rem_time' = [p \in HonestNodes |-> IF local_rem_time[p] > 0 /\ ... THEN local_rem_time[p]-1 ELSE local_rem_time[p]]
```
`local_rem_time[p]` is each honest node's **own** countdown to detecting a local timeout (`OnLocalTimerExpire(p)` fires when `local_rem_time[p] = 0`, which then broadcasts a real `TIMEOUT` message). It is decremented **only** by this same action.

Once `ServerByzantinePull` creates the round-0 `E_QC`, `ServerAdvanceRealTime`'s guard blocks — and since it's one atomic action, **every honest node's `local_rem_time` freezes too**. No honest node can ever locally time out. No `TIMEOUT` message is ever broadcast. `UponfPlusOneTimeoutsAny` (the f+1 fast-forward mechanism — the paper's own headline liveness feature) needs `f+1` real `TIMEOUT` messages to fire, so it never becomes enabled either. The system is stuck at round 0 forever, by construction, the moment a Byzantine leader silently Pulls and never completes.

### 3.4 Why the obvious quick fixes don't work

- **"Just decouple `local_rem_time` from the guard."** Checked `core/EngramServerRefinement.tla`:
  ```tla
  MIN_REM_TIME == Min({ local_rem_time[p] : p \in CurrentNodes })
  ...
  rem_time <- MIN_REM_TIME
  ```
  `local_rem_time` **is** the variable mapped to the abstract `rem_time`. Letting it decrement freely while `Elapse` is blocked reintroduces exactly the Safety violation task #33 fixed.

- **"Just add a Byzantine action that completes the M_QC/Push."** Checked `MappedCCaches`:
  ```tla
  MappedCCaches == ... msgs_precommit[r] reaching a REAL quorum (Cardinality >= THRESHOLD2) ...
  ```
  A `C`-cache (Push) is only ever derived from a genuine Tendermint precommit quorum — never synthesized abstractly the way `E_QC`/`M_QC` are. Since `msgs_propose[0]` never receives a real message for this round, no real prevote/precommit quorum can ever form. There is no way to reach Push for this specific round, concretely or abstractly.

### 3.5 The actual gap

The abstract `Liveness` formula:
```tla
Liveness == ... /\ \A n \in Nodes : WF_vars(Pull(n)) /\ WF_vars(Push(n)) ...
```
applies weak fairness to `Push(n)` for **every** node, Byzantine included — i.e. the abstract LiDO model's liveness proof implicitly assumes even a Byzantine leader eventually completes what it starts. `ServerByzantinePull` explicitly models the opposite (permanent silence), which is exactly the scenario the paper's f+1 fast-forward mechanism is supposed to handle. The abstract model has no formal representation of "round advances because f+1 honest nodes gave up," only "round advances because someone pushed." This is a genuine gap in the abstract model, not an implementation bug in the concrete bridge.

---

## 4. Proposed fix (agreed direction: A + B combined)

### Part A — abstract model addition (`core/EngramConsensus.tla`)
Add a new escape valve keyed on the existing `T`-cache (Timeout), mirroring `EarlyStartNext`'s existing `C`-cache-keyed one:
```tla
TimeoutSkipNext ==
    /\ \E c \in tree : c.type = "T" /\ c.cert_round = round
    /\ round' = round + 1
    /\ rem_time' = RESET_TIME
    /\ UNCHANGED <<tree, local_times>>
    /\ UNCHANGED <<fsm_state, h_btc_current, h_btc_anchored>>
```
Add `\/ TimeoutSkipNext` to `Next`, `WF_vars(TimeoutSkipNext)` to `Liveness`. This gives the abstract model a formal counterpart to "f+1 honest nodes gave up" that doesn't require Push.

### Part B — concrete bridge (`core/EngramServer.tla`)
`Timeout(n)` (abstract) is **not** gated by `rem_time`/`Elapse` — it can form at any time. The concrete `ServerUponTimeoutCert(p)` **is** gated (needs real `TIMEOUT` messages, which need the frozen clock). Adding a new concrete action mirroring the existing `ServerByzantinePull`/`ServerByzantineDataWithholding` pattern (direct abstract-cache synthesis, bypassing real message flow) breaks the bootstrapping cycle:
```tla
ServerHonestRoundSkip ==
    \E r \in Rounds :
        /\ Proposer[r] \in ByzantineNodes
        /\ \E eqc \in quorum_certs : eqc.type = "E_QC" /\ eqc.round = r /\ eqc.caller = Proposer[r]
        /\ ~\E q \in quorum_certs : q.type \in {"M_QC"} /\ q.round = r
        /\ ~\E tqc \in timeout_certs : tqc.round = r
        /\ LET new_TQC == [type |-> "T_QC", round |-> r, caller |-> CHOOSE p \in HonestNodes : TRUE, btc_anchored |-> h_btc_current]
           IN timeout_certs' = timeout_certs \cup {new_TQC}
        /\ UNCHANGED <<...>>
```
Plus `WF_serverVars(ServerHonestRoundSkip)`, plus widen `ServerAdvanceRealTime`'s guard to also unblock once a `T_QC` exists for the round (mirrors Part A).

### Verification plan once implemented
1. `L1`: re-verify `EngramConsensus.tla` standalone (Safety **and** Liveness) with the `TimeoutSkipNext` addition — must not introduce new Safety violations.
2. Re-run `RefinementSafety` (full `MC_ServerRefinementSafety`) — the new concrete action changes `ServerNext`, so this must be re-checked from scratch, not assumed.
3. Re-run `RefinementLiveness`/`MC_ServerRefinementLiveness` — must now show `NoError` where it previously found this counterexample.
4. Re-run the C1 stress config (this is what surfaced the bug) to confirm the fix at small scale first.

---

## 5. Options considered and rejected/deferred

- **Narrow `ServerAdvanceRealTime`'s guard back to Byzantine-only.** Rejected: this is exactly the pre-task-33 guard, already proven to violate `RefinementSafety` (lets the clock race past a legitimate honest in-flight round).
- **Scope Liveness to exclude the permanently-silent-leader scenario.** Considered, rejected: this would make the Liveness claim weaker than what the paper's own f+1 fast-forward narrative asserts — verifying a less interesting property than the one the paper needs.

---

## 6. Numbers to date (for reference)

- C1-Safety stress config (N=4, T=1, MAX_ROUND=2, MAX_BTC_HEIGHT=2, MAX_ENGRAM_HEIGHT=2, MAX_TIMESTAMP=6): first attempt ran ~8.5h (77.8M states, 850K distinct, depth 9) before finding the second bug described in §7 below.
- C1-Liveness stress config (post-fix, smallest bounds 2,2,2,4): running for several hours post-fix without a violation, depth 6+ and climbing — no regression of the original deadlock observed so far. Still in progress as of this writing.

---

## 7. Second bug found during re-verification: `ServerByzantinePull` guard gap

While re-running `MC_StressC1Safety` (Safety, unaffected by the Liveness fix in principle) to confirm Hướng A+B didn't regress Safety, TLC found a **real, independent** `RefinementSafety` violation after ~8.5 hours (77.8M states, depth 9) -- unrelated to `TimeoutSkipNext`/`ServerHonestRoundSkip` directly, though the widened `ServerAdvanceRealTime` guard likely made the triggering state combination reachable sooner.

**Trace (parsed from the full `_TETrace`, not the abbreviated `MyTraceView`):**
```
State 6-7: n2, n3 reach a prevote quorum on round 0 with an all-NIL vote
           (timeout-driven -- no real proposal was ever broadcast for round 0)
State 8:   n2 reaches a precommit quorum -> COMMITS round 0 (decides NIL) ->
           round[n2] advances 0 -> 1, step[n2] resets to PROPOSE
State 9:   ServerByzantinePull STILL fires, synthesizing an E_QC for round 0 --
           a round n2 has already closed and moved past
```

**Root cause:** `ServerByzantinePull`'s guard only checks `msgs_propose[r] = {}` (no real proposal message) and the absence of a duplicate E_QC. It never checks whether any honest node has already moved past round `r`. A round can close via an all-NIL, timeout-driven precommit quorum without `msgs_propose[r]` ever receiving a message, so the guard doesn't exclude synthesizing a Pull for an already-closed round -- which the abstract model has no way to justify once the round has advanced (there's no abstract action for "Pull into a round that's already over").

**Fix applied** (`core/EngramServer.tla`, `ServerByzantinePull`):
```tla
/\ \A p \in HonestNodes : round[p] <= r
```
added as a new conjunct -- only allows the Byzantine Pull while no honest node has exited round `r` yet.

---

## 8. Third bug found: the original Hướng B was itself unsound, redesigned into two steps

Re-running `MC_StressC1Safety` with the §7 fix applied found a **third** violation, this time directly in the original single-action `ServerHonestRoundSkip` from §4 -- at depth 5, in 3 minutes.

**Root cause:** the original fix made two mistakes:
1. It widened `ServerAdvanceRealTime`'s guard to also unblock on a T_QC existing. But `ServerAdvanceRealTime` maps to the abstract `Elapse`, whose guard only ever checks for E/M-type tree entries -- it has **no** T-cache escape. Letting a T_QC unblock this action has no abstract justification and is exactly what `RefinementSafety` caught.
2. It conflated two abstract actions into one concrete step. The abstract model advances past a stalled round in two separate actions: `Timeout(n)` creates a T-cache (unconstrained by `rem_time`), and a *later*, separate `TimeoutSkipNext` consumes an *already-existing* T-cache to advance `round` (requiring `tree` itself to stay `UNCHANGED`). The original `ServerHonestRoundSkip` did both in one step, which cannot correspond to either abstract action alone.

**Redesign** (`core/EngramServer.tla`):
- `ServerAdvanceRealTime`'s guard reverted to the original, `Elapse`-only condition (§3.2) -- no T_QC widening.
- `ServerHonestRoundSkip` split into two actions matching the abstract model's two steps exactly:
  - **`ServerHonestTimeout`** -- creates the T_QC directly (same guard/shape as the original fix), leaving `round` untouched. Maps to `Timeout(n)`.
  - **`ServerHonestRoundSkip(p)`** -- given a T_QC *already* exists for the round honest node `p` is stuck at, advances `p` via `StartRound(p, round[p]+1)` (the same per-node round-advance `UponfPlusOneTimeoutsAny(p)` already uses), leaving `quorum_certs`/`timeout_certs` `UNCHANGED`. Maps to `TimeoutSkipNext`.
- `MC_ServerFairness` (`core/MC_ServerRefinementLiveness.tla`) updated to `WF_serverVars(ServerHonestTimeout)` plus `WF_serverVars(ServerHonestRoundSkip(p))` for each honest `p`.

Both fixes have their own dedicated fairness guarantee, so the bootstrap sequence (Byzantine Pull → `ServerHonestTimeout` → `ServerHonestRoundSkip(p)` for each stuck honest `p` → clock naturally unblocks for the new round under the unmodified `Elapse`-only guard) is fully weakly-fair-driven, with no guard-widening needed anywhere.

Re-verification of the two-step redesign immediately surfaced a fourth, purely mechanical bug: `ServerHonestRoundSkip(p)`'s itemized `UNCHANGED` list (mirroring `UponfPlusOneTimeoutsAny(p)`'s style) omitted `action` (part of `traceVars`, bundled into `bookkeepingVars` -- `ServerByzantinePull` avoids this because it uses the `bookkeepingVars` bundle directly rather than itemizing). TLC caught this immediately as "Successor state is not completely specified ... action" in both the Safety and Liveness configs, at depth 4-5. Fixed by adding `action` to the `UNCHANGED` tuple.

## 9. Fifth bug: same class as §7, in `ServerByzantineDataWithholding`

With the §8 fix applied, `MC_StressC1Safety` ran clean past the earlier failure points and found a **fifth** violation at depth 6 (12 min, 2.14M states) -- same `RefinementSafety` obligation, this time triggered by `ServerByzantineDataWithholding`.

**Root cause:** identical in shape to §7's `ServerByzantinePull` bug. `ServerByzantineDataWithholding` only checks that an `E_QC` already exists for the target round `r`; it never checks whether honest nodes have already exited `r`. Since a round can close via an all-NIL timeout-driven precommit quorum without `msgs_propose[r]` ever receiving anything, a late Byzantine proposal (and the `M_QC` synthesized from it) can target an already-closed round -- an abstract `Invoke(M)` with no justification once the round has moved on.

**Fix applied** (`core/EngramServer.tla`, `ServerByzantineDataWithholding`): same conjunct as §7,
```tla
/\ \A p \in HonestNodes : round[p] <= r
```

## 10. Sixth bug: `ServerHonestRoundSkip` inflating `tx_ignored_rounds` via `StartRound`

`MC_StressC1Safety` v5 (with §9's fix applied) ran clean for 3h12min (35.5M states, depth 8) -- the deepest run yet -- before finding a sixth violation, triggered by `UponProposalInPropose`'s pre-existing (untouched by this session) censorship branch, which itself calls `StartRound(p, r+1)` and self-triggers an unmapped round advance.

**Trace:** honest node `n2` reaches round 1 via `ServerHonestRoundSkip` (the bootstrap fix from §8), successfully proposes via `InsertProposal`, then -- while processing its own proposal via `UponProposalInPropose` -- evaluates `IsCensoring(n2, prop)` as `TRUE` and self-advances to round 2, an action with no corresponding `Next` disjunct once mapped through the refinement.

**Root cause:** `IsCensoring(p, prop)` checks `tx_ignored_rounds[p][tx] >= MAX_IGNORE_ROUNDS` for some forced tx. `tx_ignored_rounds` is updated by `UpdateIgnoredRounds(p)` (called from inside `StartRound`), which reads `msgs_propose[round[p]]` and increments the ignored-count for every forced tx not found there. `ServerHonestRoundSkip(p)` called `StartRound(p, round[p]+1)` to advance past round 0 -- but round 0's `msgs_propose[0]` is *empty by construction* (that's exactly why the bootstrap mechanism exists: `ServerByzantinePull` never broadcasts a real proposal). `UpdateIgnoredRounds` has no way to distinguish "the round was empty because a real proposer genuinely ignored the forced tx" from "the round was empty because this session's bootstrap fix synthesized a round-skip around a silent Byzantine leader" -- it counts both as ignored rounds. With `MAX_IGNORE_ROUNDS = 1` (C1's smallest config), a single synthetic skip was enough to spuriously trip `IsCensoring`, which then advanced the round a *second* time with no abstract justification.

**Fix applied** (`core/EngramServer.tla`, `ServerHonestRoundSkip(p)`): stopped calling `StartRound(p, round[p]+1)` directly. Replicated its other five effects (`round'`, `step'`, `begin_round'`, `last_begin_round'`, `local_rem_time'`) inline, explicitly, and added `UNCHANGED <<tx_ignored_rounds>>` in place of the `UpdateIgnoredRounds(p)` call it used to inherit. The bootstrap round-skip no longer counts toward forced-tx censorship tracking, since it isn't a real instance of a proposer ignoring a forced transaction.

Re-verification (`MC_StressC1Safety` v6, `MC_StressC1Liveness` v5fix) in progress as of this writing.
