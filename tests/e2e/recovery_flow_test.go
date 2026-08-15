package e2e

import (
	"testing"

	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
	"github.com/stretchr/testify/require"
)

// TestRecoveryFlow_RegressesToSovereignOnSustainedDeterioration exercises the
// RECOVERING -> SOVEREIGN edge end-to-end through the real BeginBlocker
// (S1-S7 never drive it: S7 only goes SOVEREIGN -> RECOVERING -> ANCHORED).
//
// It also pins a real spec/README.md diagram mismatch: the mermaid diagram
// labels this edge "IsCriticalCondition", but spec/core/EngramFSM.tla guards
// it with the strictly broader "~IsHealthyCondition". IsCriticalCondition
// fires only on BTC gap >= SOVEREIGN_THRESHOLD, total anchor loss, or the
// suspicious-duration timeout -- a DA-only outage satisfies none. This test
// regresses with DA alone to confirm CalculateNextState follows the TLA+ CASE
// expression, not the diagram's label.
//
// E5's flapping fix means a single DA-only blip no longer regresses: down-
// hysteresis absorbs it (safe_blocks leaks by 1, not reset to 0), and only a
// SUSTAINED DownHysteresisThreshold-block outage regresses to SOVEREIGN.
// Both halves are driven explicitly.
func TestRecoveryFlow_RegressesToSovereignOnSustainedDeterioration(t *testing.T) {
	h := NewHarness(t)
	p := types.DefaultParams()
	require.GreaterOrEqual(t, p.DownHysteresisThreshold, uint64(2),
		"test assumes at least one block is absorbed before a real regression")

	// Reach SOVEREIGN, then heal BTC alone to enter RECOVERING.
	h.BTC.SetGap(p.SovereignThreshold)
	h.Advance()
	require.Equal(t, types.StateSovereign, h.State())

	h.BTC.SetGap(0)
	h.Advance()
	require.Equal(t, types.StateRecovering, h.State(), "healed sensors must move SOVEREIGN -> RECOVERING")

	// safe_blocks only increments from the SECOND consecutive RECOVERING block
	// (NextSafeBlocks needs currentState == targetState == RECOVERING) -- advance
	// one more so there's real progress to lose on regression.
	h.Advance()
	require.Equal(t, types.StateRecovering, h.State())
	beforeOutage := h.Timeline()[len(h.Timeline())-1]
	require.Greater(t, beforeOutage.SafeBlocks, uint64(0), "must have accumulated some hysteresis progress before the outage")

	// DA outage: never satisfies IsCriticalCondition (no DA disjunct), only
	// ~IsHealthyCondition. The first DownHysteresisThreshold-1 blocks must be
	// ABSORBED (stay RECOVERING, safe_blocks leaking by 1, not hard-reset).
	h.DA.SetAvailable(false)
	for i := uint64(0); i < p.DownHysteresisThreshold-1; i++ {
		h.Advance()
		require.Equal(t, types.StateRecovering, h.State(),
			"a DA-only outage must be absorbed by down-hysteresis, not regress on the very first bad block")
	}
	absorbed := h.Timeline()[len(h.Timeline())-1]
	require.Less(t, absorbed.SafeBlocks, beforeOutage.SafeBlocks,
		"an absorbed outage block must leak safe_blocks by 1, not leave it untouched or hard-reset it to 0")

	// After DownHysteresisThreshold recurrences, it must finally regress.
	h.Advance()
	require.Equal(t, types.StateSovereign, h.State(),
		"a SUSTAINED DA-only outage must eventually regress RECOVERING -> SOVEREIGN once down-hysteresis is exhausted")

	afterRegression := h.Timeline()[len(h.Timeline())-1]
	require.Equal(t, uint64(0), afterRegression.SafeBlocks, "regressing out of RECOVERING must reset safe_blocks to 0 (HysteresisSafety)")
	require.True(t, afterRegression.WithdrawLocked, "withdrawals must remain locked across the regression (SOVEREIGN is also WithdrawLocked)")

	m := h.ComputeMetrics()
	h.WriteCSV("recovery_flow_regression")
	t.Log(fmtMetrics("RecoveryFlow-Regression", m))
}

// TestRecoveryFlow_ReRecoveryRestartsHysteresisFromScratch covers
// HysteresisSafety's "restarting the recovery process from the beginning"
// clause: after a RECOVERING attempt regresses to SOVEREIGN, a second attempt
// must NOT inherit safe_blocks -- it accumulates HysteresisWait from zero.
func TestRecoveryFlow_ReRecoveryRestartsHysteresisFromScratch(t *testing.T) {
	h := NewHarness(t)
	p := types.DefaultParams()

	// First attempt: reach SOVEREIGN, heal, fully satisfy hysteresis (no proof),
	// then regress on a fresh genuine critical failure.
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

	// Second attempt: heal and recover from scratch. If safe_blocks carried over,
	// HysteresisWait would already be satisfied on the first RECOVERING block
	// of this attempt -- assert it is NOT.
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
