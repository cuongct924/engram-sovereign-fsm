---
name: go-spec-fidelity
description: Conventions for writing/editing Go code in x/sovereignty, x/da, x/vigilante that ports logic from spec/core/*.tla. Use whenever adding or modifying FSM state-transition logic, sensor predicates, receipt verification, or ABCI hooks in this repo (not spec/, which has its own readme-style skill).
---

# Porting spec/core/*.tla logic to Go

Code in `x/sovereignty`, `x/da`, `x/vigilante` is a **reference implementation** of
`spec/core/*.tla` — a formally verified TLA+ spec (see `spec/README.md`, `spec/CLAUDE.md`). The
value of this codebase depends on that traceability staying exact. A Go function that "improves
on" or diverges from its spec operator without saying so is a defect, not a stylistic choice — this
has been the actual root cause of real bugs found this session (a prior `CalculateNextState` had a
branch — "jump directly to SOVEREIGN from any state" — that doesn't exist in `CalculateNextFSMState`,
silently violating `StrictFSMTransitionSafety`).

## Hard rules

1. **Read the cited TLA+ operator before touching the Go function that ports it.** Every ported
   function's doc comment names the spec operator and usually a line range
   (e.g. `// IsBTCGapSuspicious mirrors IsBTCGapSuspicious ... spec/core/EngramFSM.tla:97-100`).
   Open that file at that range first. If the comment is missing a citation, add one when you touch
   the function — don't perpetuate an uncited port.

2. **Branch order and structure must match the spec's CASE/switch exactly**, including branches
   that look redundant. TLA+'s `CASE` picks the *first* matching arm; Go's `switch`/`if`-chain must
   preserve that same evaluation order. Do not collapse two spec branches into one "equivalent"
   condition, and do not add a branch the spec doesn't have, even if it seems like a harmless
   optimization or a sensible-looking fallback.

3. **Reuse the existing ported function; never recompute the same predicate two ways.** Before
   writing a new `Is*`/`Verify*`/`CalculateNext*` function, grep for whether the spec operator it
   would port already exists in `x/sovereignty/types/predicates.go`, `x/sovereignty/keeper/circuit_breaker.go`,
   `x/da/verify.go`, or `x/vigilante/verify.go`. If a new call site needs the same logic, import and
   call the existing function — do not paste/adapt its body.

4. **`*PeripheralMetrics` is always a pointer**, never a value, in signatures, struct fields, and
   `collections.Item[...]` type parameters. It's a proto3-generated type
   (`x/sovereignty/types/state.pb.go`) embedding a `sync.Mutex` via `protoimpl.MessageState`; passing
   it by value trips `go vet`'s lock-copy check. `go vet ./...` must stay clean — treat any new
   warning here as a real bug, not noise to suppress.

5. **Proto field casing follows protoc-gen-go, not the TLA+ variable name.** `btc_gap` in
   `.proto` becomes `BtcGap` in Go (not `BTCGap`) — protoc-gen-go capitalizes each snake_case
   segment independently, it does not preserve acronym casing. Verify the actual generated field
   name in the `.pb.go` file rather than guessing from the TLA+ spelling.

6. **"Sensors propose, consensus decides" is not optional.** Any code that writes `FSMState` (or
   the `safe_blocks`/`suspicious_duration` counters) outside of `CommitFSMTransition`
   (`x/sovereignty/preblock.go`) or the explicitly-separate `BeginBlocker` test-harness path
   (`x/sovereignty/abci.go`) is very likely wrong — re-read `EngramServer.tla`'s four server hooks
   (`ServerInsertProposal`, `ServerProposerVotes`, `ServerUponProposalInPrecommitNoDecision`,
   `ServerUponTimeoutCert`) before adding a new state-writing path.

7. **Document simplifications where the Go port can't yet match the spec exactly**, and say why
   (e.g., "vanilla ABCI 2.0 doesn't expose consensus round to PrepareProposal/ProcessProposal, so
   round=0/no-tolerance-widening is used here"). Silently returning a stricter-than-spec result is
   an acceptable simplification; silently returning a looser one is a safety bug.

## Before submitting a change to ported logic

- Re-open the cited `spec/core/*.tla` lines and diff the Go branch structure against them
  mentally, arm by arm.
- Run `go build ./... && go vet ./... && go test ./...` — all three must be clean. New FSM/predicate
  logic needs a table-driven test covering each spec branch, matching the style already in
  `x/sovereignty/keeper/circuit_breaker_test.go` (one test per CASE arm, named after the transition
  it covers, e.g. `TestCalculateNextState_RecoveringStaysWhenHysteresisNotYetSatisfied`).
- If you added a new ABCI-adjacent handler (anything under `x/sovereignty/proposal.go`,
  `preblock.go`), check whether `app/app.go` (M5, not yet wired) needs a matching TODO note — the
  handler *logic* lives in `x/sovereignty`, but *registration* onto a real `BaseApp` happens there.
