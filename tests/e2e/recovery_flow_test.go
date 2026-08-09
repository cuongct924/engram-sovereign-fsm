package e2e

import (
	"testing"

	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
	"github.com/stretchr/testify/require"
)

// TestRecoveryFlow_RegressesToSovereignOnDeterioration exercises the
// RECOVERING -> SOVEREIGN edge of spec/README.md's top-level FSM diagram
// end-to-end, through the real BeginBlocker across multiple simulated
// blocks -- fault_injection_test.go's S1-S7 scenarios never drive this
// specific edge (S7 only ever goes SOVEREIGN -> RECOVERING -> ANCHORED, with
// no regression along the way).
//
// It also pins down a real mismatch between spec/README.md's mermaid diagram
// and the TLA+ CASE expression it's meant to summarize: the diagram labels
// this edge "IsCriticalCondition", but spec/core/EngramFSM.tla:321 actually
// guards it with the strictly broader "~IsHealthyCondition"
// (state = "RECOVERING" /\ ~IsHealthyCondition -> "SOVEREIGN"). IsHealthyCondition
// requires ALL of {BTC gap clean, DA healthy, P2P healthy}; IsCriticalCondition
// only fires on BTC gap >= SOVEREIGN_THRESHOLD, total anchor loss, or the
// suspicious-duration timeout -- none of which a DA-only outage trips. This
// test regresses RECOVERING -> SOVEREIGN using DA alone (never satisfying
// IsCriticalCondition) to confirm keeper.CalculateNextState (and hence the
// real chain) follows the TLA+ CASE expression, not the diagram's label.
func TestRecoveryFlow_RegressesToSovereignOnDeterioration(t *testing.T) {
	h := NewHarness(t)
	p := types.DefaultParams()

	// Reach SOVEREIGN, then heal BTC alone to enter RECOVERING.
	h.BTC.SetGap(p.SovereignThreshold)
	h.Advance()
	require.Equal(t, types.StateSovereign, h.State())

	h.BTC.SetGap(0)
	h.Advance()
	require.Equal(t, types.StateRecovering, h.State(), "healed sensors must move SOVEREIGN -> RECOVERING")

	// safe_blocks only starts incrementing from the SECOND consecutive block
	// spent in RECOVERING (NextSafeBlocks requires currentState == targetState
	// == RECOVERING) -- advance one more healthy block so there is real
	// hysteresis progress to lose when we regress below.
	h.Advance()
	require.Equal(t, types.StateRecovering, h.State())
	last := h.Timeline()[len(h.Timeline())-1]
	require.Greater(t, last.SafeBlocks, uint64(0), "must have accumulated some hysteresis progress before regressing")

	// Regress via DA alone: never satisfies IsCriticalCondition (no DA
	// disjunct there), only ~IsHealthyCondition.
	h.DA.SetAvailable(false)
	h.Advance()
	require.Equal(t, types.StateSovereign, h.State(),
		"RECOVERING must regress to SOVEREIGN on ANY health deterioration (~IsHealthyCondition per EngramFSM.tla:321), "+
			"not only on IsCriticalCondition as spec/README.md's diagram label suggests -- a DA-only outage alone must be enough")

	afterRegression := h.Timeline()[len(h.Timeline())-1]
	require.Equal(t, uint64(0), afterRegression.SafeBlocks, "regressing out of RECOVERING must reset safe_blocks to 0 (HysteresisSafety)")
	require.True(t, afterRegression.WithdrawLocked, "withdrawals must remain locked across the regression (SOVEREIGN is also WithdrawLocked)")

	m := h.ComputeMetrics()
	h.WriteCSV("recovery_flow_regression")
	t.Log(fmtMetrics("RecoveryFlow-Regression", m))
}

// TestRecoveryFlow_ReRecoveryRestartsHysteresisFromScratch covers
// HysteresisSafety's "restarting the recovery process from the beginning"
// clause (spec/README.md's Hysteresis Mechanism section): once a RECOVERING
// attempt regresses back to SOVEREIGN, a second, later recovery attempt must
// NOT inherit any safe_blocks progress from the first -- it has to
// accumulate HysteresisWait again from zero.
func TestRecoveryFlow_ReRecoveryRestartsHysteresisFromScratch(t *testing.T) {
	h := NewHarness(t)
	p := types.DefaultParams()

	// First attempt: reach SOVEREIGN, heal, fully satisfy hysteresis in
	// RECOVERING (but never submit a proof), then regress on a fresh, genuine
	// critical failure.
	h.BTC.SetGap(p.SovereignThreshold)
	h.Advance()
	h.BTC.SetGap(0)
	h.Advance()
	require.Equal(t, types.StateRecovering, h.State())

	for i := uint64(0); i < p.HysteresisWait; i++ {
		h.Advance()
	}
	require.Equal(t, types.StateRecovering, h.State(), "must not exit RECOVERING without a proof, even with hysteresis satisfied")
	beforeRegression := h.Timeline()[len(h.Timeline())-1]
	require.Equal(t, p.HysteresisWait, beforeRegression.SafeBlocks, "must have fully accumulated hysteresis before regressing")

	h.BTC.SetGap(p.SovereignThreshold) // genuinely critical this time
	h.Advance()
	require.Equal(t, types.StateSovereign, h.State())

	// Second attempt: heal again and recover from scratch. If safe_blocks had
	// carried over from the first attempt, HysteresisWait would already be
	// satisfied on the very first RECOVERING block of this second attempt --
	// assert it is NOT.
	h.BTC.SetGap(0)
	h.Advance()
	require.Equal(t, types.StateRecovering, h.State())
	freshEntry := h.Timeline()[len(h.Timeline())-1]
	require.Equal(t, uint64(0), freshEntry.SafeBlocks,
		"a fresh RECOVERING interval must start safe_blocks at 0, not carry over progress from a prior aborted attempt")

	for i := uint64(0); i < p.HysteresisWait; i++ {
		h.Advance()
	}
	require.Equal(t, types.StateRecovering, h.State(), "must not exit RECOVERING before a proof is submitted, even with hysteresis satisfied")

	h.SetReanchoringProofValid(true)
	h.Advance()
	require.Equal(t, types.StateAnchored, h.State(), "RECOVERING -> ANCHORED once both hysteresis and proof are satisfied on the second attempt")

	m := h.ComputeMetrics()
	h.WriteCSV("recovery_flow_re_recovery")
	t.Log(fmtMetrics("RecoveryFlow-ReRecovery", m))
}
