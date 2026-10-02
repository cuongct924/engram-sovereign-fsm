package keeper

import (
	"testing"

	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
	"github.com/stretchr/testify/require"
)

// healthyMetrics returns a PeripheralMetrics snapshot that satisfies
// types.IsHealthyCondition under types.DefaultParams().
func healthyMetrics() *types.PeripheralMetrics {
	p := types.DefaultParams()
	return &types.PeripheralMetrics{
		BtcGap:          0,
		DaGap:           0,
		SubnetDiversity: p.MinSubnetDiversity,
		ActiveAnchors:   p.MinAnchorPeers,
		CleanPeers:      p.MinPeers,
		PeerChurnRate:   0,
		AvgPeerTenure:   p.MinAvgTenure,
		PeerLatency:     0,
	}
}

func TestCalculateNextState_AnchoredNeverSkipsSuspicious(t *testing.T) {
	p := types.DefaultParams()
	m := healthyMetrics()
	m.BtcGap = p.SovereignThreshold // critical: btc_gap >= SOVEREIGN_THRESHOLD

	next := CalculateNextState(types.StateAnchored, FSMInput{Metrics: m}, p)
	require.Equal(t, types.StateSovereign, next,
		"ANCHORED must go straight to SOVEREIGN only via the explicit critical branch, matching CalculateNextFSMState's first CASE arm")
}

func TestCalculateNextState_AnchoredAbsorbsWarningBelowDownHysteresisThreshold(t *testing.T) {
	// E5's flapping fix: a single warning-only block must NOT immediately
	// demote ANCHORED -> SUSPICIOUS -- it's absorbed while UnhealthyStreak+1
	// stays below DownHysteresisThreshold.
	p := types.DefaultParams()
	require.Greater(t, p.DownHysteresisThreshold, uint64(1), "test assumes the default threshold grants at least one block of grace")
	m := healthyMetrics()
	m.BtcGap = p.SuspiciousThreshold // warning, not yet critical

	next := CalculateNextState(types.StateAnchored, FSMInput{Metrics: m, UnhealthyStreak: 0}, p)
	require.Equal(t, types.StateAnchored, next, "must absorb a single noisy block, not demote immediately")
}

func TestCalculateNextState_AnchoredToSuspiciousOnceDownHysteresisThresholdReached(t *testing.T) {
	p := types.DefaultParams()
	m := healthyMetrics()
	m.BtcGap = p.SuspiciousThreshold // warning, not yet critical

	next := CalculateNextState(types.StateAnchored, FSMInput{
		Metrics:         m,
		UnhealthyStreak: p.DownHysteresisThreshold - 1,
	}, p)
	require.Equal(t, types.StateSuspicious, next, "must demote once the streak has recurred DownHysteresisThreshold times")
}

func TestCalculateNextState_AnchoredCriticalBypassesDownHysteresis(t *testing.T) {
	// A critical condition must demote SOVEREIGN immediately regardless of
	// UnhealthyStreak -- down-hysteresis only ever softens non-critical noise.
	p := types.DefaultParams()
	m := healthyMetrics()
	m.BtcGap = p.SovereignThreshold // critical

	next := CalculateNextState(types.StateAnchored, FSMInput{Metrics: m, UnhealthyStreak: 0}, p)
	require.Equal(t, types.StateSovereign, next)
}

func TestCalculateNextState_SuspiciousToSovereignOnCritical(t *testing.T) {
	p := types.DefaultParams()
	m := healthyMetrics()
	m.BtcGap = p.SovereignThreshold

	next := CalculateNextState(types.StateSuspicious, FSMInput{Metrics: m}, p)
	require.Equal(t, types.StateSovereign, next)
}

func TestCalculateNextState_SuspiciousAbsorbsHealthyBlockBelowSuspiciousHysteresisWait(t *testing.T) {
	// Gray Failure Arbitrage fix: a single healthy block must NOT immediately
	// exit SUSPICIOUS -> ANCHORED -- it's absorbed while SuspiciousSafeBlocks+1
	// stays below SuspiciousHysteresisWait, so suspicious_duration keeps
	// accumulating instead of resetting for free.
	p := types.DefaultParams()
	require.Greater(t, p.SuspiciousHysteresisWait, uint64(1), "test assumes the default threshold grants at least one block of grace")

	next := CalculateNextState(types.StateSuspicious, FSMInput{Metrics: healthyMetrics(), SuspiciousSafeBlocks: 0}, p)
	require.Equal(t, types.StateSuspicious, next, "must absorb a single healthy block, not exit immediately")
}

func TestCalculateNextState_SuspiciousToAnchoredOnceSuspiciousHysteresisWaitReached(t *testing.T) {
	p := types.DefaultParams()
	next := CalculateNextState(types.StateSuspicious, FSMInput{
		Metrics:              healthyMetrics(),
		SuspiciousSafeBlocks: p.SuspiciousHysteresisWait - 1,
	}, p)
	require.Equal(t, types.StateAnchored, next, "must exit once the healthy streak has recurred SuspiciousHysteresisWait times")
}

func TestCalculateNextState_SuspiciousCriticalBypassesSuspiciousHysteresis(t *testing.T) {
	// A critical condition must demote SOVEREIGN immediately regardless of
	// SuspiciousSafeBlocks -- up-hysteresis only ever softens a healthy exit.
	p := types.DefaultParams()
	m := healthyMetrics()
	m.BtcGap = p.SovereignThreshold // critical

	next := CalculateNextState(types.StateSuspicious, FSMInput{Metrics: m, SuspiciousSafeBlocks: p.SuspiciousHysteresisWait - 1}, p)
	require.Equal(t, types.StateSovereign, next)
}

func TestCalculateNextState_SovereignToRecoveringOnHealthy(t *testing.T) {
	p := types.DefaultParams()
	next := CalculateNextState(types.StateSovereign, FSMInput{Metrics: healthyMetrics()}, p)
	require.Equal(t, types.StateRecovering, next)
}

func TestCalculateNextState_RecoveringAbsorbsUnhealthyBlockBelowDownHysteresisThreshold(t *testing.T) {
	// E5's flapping fix: a single non-critical unhealthy block must NOT
	// immediately regress RECOVERING -> SOVEREIGN.
	p := types.DefaultParams()
	require.Greater(t, p.DownHysteresisThreshold, uint64(1), "test assumes the default threshold grants at least one block of grace")
	m := healthyMetrics()
	m.SubnetDiversity = 0 // breaks IsP2PQualityHealthy -> not healthy, but not critical

	next := CalculateNextState(types.StateRecovering, FSMInput{
		Metrics:               m,
		SafeBlocks:            p.RecoveryHysteresisWait,
		UnhealthyStreak:       0,
		ReanchoringProofValid: true,
	}, p)
	require.Equal(t, types.StateRecovering, next, "must absorb a single noisy block, not regress immediately")
}

func TestCalculateNextState_RecoveringRegressesToSovereignOnceDownHysteresisThresholdReached(t *testing.T) {
	p := types.DefaultParams()
	m := healthyMetrics()
	m.SubnetDiversity = 0 // breaks IsP2PQualityHealthy -> not healthy, but not critical

	next := CalculateNextState(types.StateRecovering, FSMInput{
		Metrics:               m,
		SafeBlocks:            p.RecoveryHysteresisWait,
		UnhealthyStreak:       p.DownHysteresisThreshold - 1,
		ReanchoringProofValid: true,
	}, p)
	require.Equal(t, types.StateSovereign, next, "must regress once the streak has recurred DownHysteresisThreshold times")
}

func TestCalculateNextState_RecoveringCriticalBypassesDownHysteresis(t *testing.T) {
	// A critical condition must regress RECOVERING -> SOVEREIGN immediately
	// regardless of UnhealthyStreak.
	p := types.DefaultParams()
	m := healthyMetrics()
	m.ActiveAnchors = 0 // complete anchor isolation: critical

	next := CalculateNextState(types.StateRecovering, FSMInput{
		Metrics:               m,
		SafeBlocks:            p.RecoveryHysteresisWait,
		UnhealthyStreak:       0,
		ReanchoringProofValid: true,
	}, p)
	require.Equal(t, types.StateSovereign, next)
}

func TestCalculateNextState_RecoveringStaysWhenHysteresisNotYetSatisfied(t *testing.T) {
	p := types.DefaultParams()
	require.Greater(t, p.RecoveryHysteresisWait, uint64(0), "test assumes RecoveryHysteresisWait > 0 so safe_blocks < RecoveryHysteresisWait is reachable")

	next := CalculateNextState(types.StateRecovering, FSMInput{
		Metrics:               healthyMetrics(),
		SafeBlocks:            0, // < RecoveryHysteresisWait
		ReanchoringProofValid: true,
		DABackfillValid:       true,
	}, p)
	require.Equal(t, types.StateRecovering, next, "must not exit RECOVERING before safe_blocks reaches RecoveryHysteresisWait")
}

func TestCalculateNextState_RecoveringStaysWhenProofNotYetValid(t *testing.T) {
	p := types.DefaultParams()

	next := CalculateNextState(types.StateRecovering, FSMInput{
		Metrics:               healthyMetrics(),
		SafeBlocks:            p.RecoveryHysteresisWait,
		ReanchoringProofValid: false,
		DABackfillValid:       true,
	}, p)
	require.Equal(t, types.StateRecovering, next, "must not exit RECOVERING before the reanchoring proof is valid, even if hysteresis is satisfied")
}

// A recovery proof shows the SOVEREIGN interval is valid, not that its data is
// available (EngramFSM.tla:444-446): the DA backfill is a separate exit gate.
func TestCalculateNextState_RecoveringStaysWhenDABackfillPending(t *testing.T) {
	p := types.DefaultParams()

	next := CalculateNextState(types.StateRecovering, FSMInput{
		Metrics:               healthyMetrics(),
		SafeBlocks:            p.RecoveryHysteresisWait,
		ReanchoringProofValid: true,
		DABackfillValid:       false,
	}, p)
	require.Equal(t, types.StateRecovering, next, "must not exit RECOVERING before the SOVEREIGN interval's data is published to DA")
}

func TestCalculateNextState_RecoveringToAnchoredWhenAllConditionsMet(t *testing.T) {
	p := types.DefaultParams()

	next := CalculateNextState(types.StateRecovering, FSMInput{
		Metrics:               healthyMetrics(),
		SafeBlocks:            p.RecoveryHysteresisWait,
		ReanchoringProofValid: true,
		DABackfillValid:       true,
	}, p)
	require.Equal(t, types.StateAnchored, next)
}

// The gate only affects the RECOVERING->ANCHORED edge: a critical reading
// still falls back to SOVEREIGN regardless of backfill progress.
func TestCalculateNextState_RecoveringCriticalFallsBackEvenWithBackfillPending(t *testing.T) {
	p := types.DefaultParams()
	m := healthyMetrics()
	m.BtcGap = p.SovereignThreshold

	next := CalculateNextState(types.StateRecovering, FSMInput{
		Metrics:         m,
		DABackfillValid: false,
	}, p)
	require.Equal(t, types.StateSovereign, next)
}

func TestDABackfillValid(t *testing.T) {
	tests := []struct {
		name                  string
		contiguous, entryHigh uint64
		want                  bool
	}{
		{"no recovering episode recorded imposes no bound", 0, 0, true},
		{"nothing published yet", 0, 10, false},
		{"one short of the last interval height", 8, 10, false},
		{"exactly covers the interval (entry-1)", 9, 10, true},
		{"covers beyond the interval", 25, 10, true},
		{"entry at height 1 has an empty interval", 0, 1, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, DABackfillValid(tt.contiguous, tt.entryHigh))
		})
	}
}

func TestNextSafeBlocks_IncrementsWhileStayingRecoveringAndCapsAtHysteresisWait(t *testing.T) {
	p := types.DefaultParams()
	p.RecoveryHysteresisWait = 3

	got := NextSafeBlocks(types.StateRecovering, types.StateRecovering, p.RecoveryHysteresisWait, true, p)
	require.Equal(t, p.RecoveryHysteresisWait, got, "must cap at RecoveryHysteresisWait, not overflow past it")

	got = NextSafeBlocks(types.StateRecovering, types.StateRecovering, 0, true, p)
	require.Equal(t, uint64(1), got)
}

func TestNextSafeBlocks_ResetsOnAnyOtherTransition(t *testing.T) {
	p := types.DefaultParams()

	require.Equal(t, uint64(0), NextSafeBlocks(types.StateSovereign, types.StateRecovering, 0, true, p),
		"entering RECOVERING for the first time must start safe_blocks at 0")
	require.Equal(t, uint64(0), NextSafeBlocks(types.StateRecovering, types.StateSovereign, p.RecoveryHysteresisWait, false, p),
		"regressing out of RECOVERING must reset safe_blocks")
	require.Equal(t, uint64(0), NextSafeBlocks(types.StateRecovering, types.StateAnchored, p.RecoveryHysteresisWait, true, p),
		"completing recovery must reset safe_blocks")
}

func TestNextSafeBlocks_LeaksInsteadOfHardResettingWhenAbsorbingNoise(t *testing.T) {
	// E5's flapping fix: staying in RECOVERING on a non-critical unhealthy
	// (absorbed) block must leak safe_blocks by 1, not hard-reset to 0.
	p := types.DefaultParams()

	got := NextSafeBlocks(types.StateRecovering, types.StateRecovering, 3, false, p)
	require.Equal(t, uint64(2), got, "must leak by 1, not reset to 0")

	got = NextSafeBlocks(types.StateRecovering, types.StateRecovering, 0, false, p)
	require.Equal(t, uint64(0), got, "must floor at 0, not underflow")
}

func TestNextSuspiciousSafeBlocks_IncrementsWhileStayingSuspiciousAndCapsAtSuspiciousHysteresisWait(t *testing.T) {
	p := types.DefaultParams()
	p.SuspiciousHysteresisWait = 3

	got := NextSuspiciousSafeBlocks(types.StateSuspicious, types.StateSuspicious, p.SuspiciousHysteresisWait, true, p)
	require.Equal(t, p.SuspiciousHysteresisWait, got, "must cap at SuspiciousHysteresisWait, not overflow past it")

	got = NextSuspiciousSafeBlocks(types.StateSuspicious, types.StateSuspicious, 0, true, p)
	require.Equal(t, uint64(1), got)
}

func TestNextSuspiciousSafeBlocks_ResetsOnAnyOtherTransition(t *testing.T) {
	p := types.DefaultParams()

	require.Equal(t, uint64(0), NextSuspiciousSafeBlocks(types.StateAnchored, types.StateSuspicious, 0, false, p),
		"entering SUSPICIOUS for the first time must start suspicious_safe_blocks at 0")
	require.Equal(t, uint64(0), NextSuspiciousSafeBlocks(types.StateSuspicious, types.StateSovereign, p.SuspiciousHysteresisWait-1, false, p),
		"escalating out of SUSPICIOUS must reset suspicious_safe_blocks")
	require.Equal(t, uint64(0), NextSuspiciousSafeBlocks(types.StateSuspicious, types.StateAnchored, p.SuspiciousHysteresisWait-1, true, p),
		"completing the exit must reset suspicious_safe_blocks")
}

func TestNextSuspiciousSafeBlocks_LeaksInsteadOfHardResettingWhenAbsorbingNoise(t *testing.T) {
	// Gray Failure Arbitrage fix: staying in SUSPICIOUS on a non-healthy
	// (absorbed) block must leak suspicious_safe_blocks by 1, not hard-reset to 0.
	p := types.DefaultParams()

	got := NextSuspiciousSafeBlocks(types.StateSuspicious, types.StateSuspicious, 2, false, p)
	require.Equal(t, uint64(1), got, "must leak by 1, not reset to 0")

	got = NextSuspiciousSafeBlocks(types.StateSuspicious, types.StateSuspicious, 0, false, p)
	require.Equal(t, uint64(0), got, "must floor at 0, not underflow")
}

func TestNextSuspiciousDuration_IncrementsWhileStayingSuspiciousAndFeedsCriticalCondition(t *testing.T) {
	p := types.DefaultParams()
	p.MaxSuspiciousTime = 2

	d := uint64(0)
	d = NextSuspiciousDuration(types.StateSuspicious, types.StateSuspicious, d, p)
	require.Equal(t, uint64(1), d)
	d = NextSuspiciousDuration(types.StateSuspicious, types.StateSuspicious, d, p)
	require.Equal(t, uint64(2), d)

	// Once suspicious_duration >= MaxSuspiciousTime, IsCriticalCondition must fire
	// (gray-failure timeout), forcing SUSPICIOUS -> SOVEREIGN even with otherwise
	// healthy sensor readings.
	next := CalculateNextState(types.StateSuspicious, FSMInput{
		Metrics:            healthyMetrics(),
		SuspiciousDuration: d,
	}, p)
	require.Equal(t, types.StateSovereign, next)
}

func TestNextSuspiciousDuration_ResetsOnLeavingSuspicious(t *testing.T) {
	p := types.DefaultParams()
	require.Equal(t, uint64(0), NextSuspiciousDuration(types.StateSuspicious, types.StateAnchored, 5, p))
}

func TestNextSuspiciousDuration_IncrementsOnFirstEntryFromAnchored(t *testing.T) {
	// spec/core/EngramFSM.tla:337-340's suspicious_duration' formula guards
	// only on target_state = "SUSPICIOUS", not on current state also being
	// SUSPICIOUS -- so the very first block entering SUSPICIOUS from ANCHORED
	// counts as 1, not 0 (a prior version also required currentState ==
	// SUSPICIOUS, lagging the spec's counter by one block).
	p := types.DefaultParams()
	got := NextSuspiciousDuration(types.StateAnchored, types.StateSuspicious, 0, p)
	require.Equal(t, uint64(1), got)
}

func TestNextUnhealthyStreak_IncrementsWhileStayingUnhealthy(t *testing.T) {
	got := NextUnhealthyStreak(types.StateAnchored, types.StateAnchored, 0, false)
	require.Equal(t, uint64(1), got, "staying ANCHORED on an unhealthy (absorbed) block must increment")

	got = NextUnhealthyStreak(types.StateRecovering, types.StateRecovering, 1, false)
	require.Equal(t, uint64(2), got, "staying RECOVERING on an unhealthy (absorbed) block must increment")
}

func TestNextUnhealthyStreak_ResetsOnHealthyBlock(t *testing.T) {
	require.Equal(t, uint64(0), NextUnhealthyStreak(types.StateAnchored, types.StateAnchored, 3, true),
		"a healthy block must reset the streak even while staying in the same state")
}

func TestNextUnhealthyStreak_ResetsOnRealTransition(t *testing.T) {
	require.Equal(t, uint64(0), NextUnhealthyStreak(types.StateAnchored, types.StateSuspicious, 1, false),
		"a real demotion must reset the streak, not carry it into the new state")
	require.Equal(t, uint64(0), NextUnhealthyStreak(types.StateRecovering, types.StateSovereign, 1, false),
		"a real regression must reset the streak")
	require.Equal(t, uint64(0), NextUnhealthyStreak(types.StateRecovering, types.StateAnchored, 0, true),
		"completing recovery must reset the streak")
}

func TestNextUnhealthyStreak_IgnoresOtherStates(t *testing.T) {
	// SUSPICIOUS/SOVEREIGN have their own hysteresis mechanisms
	// (suspicious_duration, healthy-gated recovery) -- unhealthy_streak is
	// only ever meaningful for ANCHORED/RECOVERING's down-hysteresis.
	require.Equal(t, uint64(0), NextUnhealthyStreak(types.StateSuspicious, types.StateSuspicious, 2, false))
	require.Equal(t, uint64(0), NextUnhealthyStreak(types.StateSovereign, types.StateSovereign, 2, false))
}

func TestEffectiveDownHysteresisThreshold_DoublesPerFailedAttemptAndCaps(t *testing.T) {
	p := types.DefaultParams()
	p.DownHysteresisThreshold = 2
	p.MaxDownHysteresisThreshold = 8

	require.Equal(t, uint64(2), EffectiveDownHysteresisThreshold(0, p), "no prior failed attempts: plain threshold")
	require.Equal(t, uint64(4), EffectiveDownHysteresisThreshold(1, p), "1 failed attempt: doubled once")
	require.Equal(t, uint64(8), EffectiveDownHysteresisThreshold(2, p), "2 failed attempts: doubled twice, exactly at the cap")
	require.Equal(t, uint64(8), EffectiveDownHysteresisThreshold(3, p), "3+ failed attempts: still capped, not doubled further")
	require.Equal(t, uint64(8), EffectiveDownHysteresisThreshold(10, p), "many failed attempts: capped, no overflow")
}

func adaptiveParams() types.Params {
	p := types.DefaultParams()
	p.DownHysteresisThreshold = 2
	p.MaxDownHysteresisThreshold = 8 // saturates at flap_level=2 (2*2^2=8)
	p.StableWindow = 3
	return p
}

func TestNextFlapLevel_IncrementsOnNoiseDrivenDemotions(t *testing.T) {
	p := adaptiveParams()

	require.Equal(t, uint64(1), NextFlapLevel(types.StateAnchored, types.StateSuspicious, 0, 0, false, false, p),
		"ANCHORED->SUSPICIOUS is warning-driven by construction and must increment")
	require.Equal(t, uint64(1), NextFlapLevel(types.StateRecovering, types.StateSovereign, 0, 0, false, false, p),
		"a non-critical RECOVERING->SOVEREIGN (down-hysteresis exhausted) must increment")
	require.Equal(t, uint64(2), NextFlapLevel(types.StateRecovering, types.StateSovereign, 1, 0, false, false, p),
		"consecutive flaps keep incrementing")
}

func TestNextFlapLevel_CriticalDemotionIsNotAFlap(t *testing.T) {
	p := adaptiveParams()

	require.Equal(t, uint64(1), NextFlapLevel(types.StateRecovering, types.StateSovereign, 1, 0, false, true, p),
		"a critical RECOVERING->SOVEREIGN is a genuine fault and must not raise the level")
	require.Equal(t, uint64(0), NextFlapLevel(types.StateAnchored, types.StateSovereign, 0, 0, false, true, p),
		"ANCHORED->SOVEREIGN (critical) is never a flap")
	require.Equal(t, uint64(0), NextFlapLevel(types.StateSuspicious, types.StateSovereign, 0, 0, false, true, p),
		"SUSPICIOUS->SOVEREIGN is never a flap")
}

func TestNextFlapLevel_SaturatesOnceEffectiveThresholdCaps(t *testing.T) {
	// Past the cap a higher level has no effect and would be an unbounded Nat,
	// breaking TLC's finite-state assumption (FSMTypeOK's bound).
	p := adaptiveParams()

	require.Equal(t, uint64(2), NextFlapLevel(types.StateAnchored, types.StateSuspicious, 2, 0, false, false, p),
		"must saturate once the effective threshold already equals the cap")
	require.Equal(t, uint64(2), NextFlapLevel(types.StateRecovering, types.StateSovereign, 2, 0, false, false, p))
}

func TestNextFlapLevel_SuccessfulRecoveryDoesNotReset(t *testing.T) {
	p := adaptiveParams()
	require.Equal(t, uint64(2), NextFlapLevel(types.StateRecovering, types.StateAnchored, 2, 0, true, false, p),
		"forgiveness is earned by stability in ANCHORED, not by one successful recovery")
}

func TestNextFlapLevel_DecaysAfterStableWindowHealthyAnchoredBlocks(t *testing.T) {
	p := adaptiveParams() // StableWindow = 3

	require.Equal(t, uint64(2), NextFlapLevel(types.StateAnchored, types.StateAnchored, 2, 1, true, false, p),
		"stable_blocks+1 < StableWindow: no decay yet")
	require.Equal(t, uint64(1), NextFlapLevel(types.StateAnchored, types.StateAnchored, 2, 2, true, false, p),
		"stable_blocks+1 >= StableWindow: decays by exactly 1")
	require.Equal(t, uint64(0), NextFlapLevel(types.StateAnchored, types.StateAnchored, 0, 2, true, false, p),
		"flap_level is floored at 0")
	require.Equal(t, uint64(2), NextFlapLevel(types.StateAnchored, types.StateAnchored, 2, 2, false, false, p),
		"an absorbed warning block never decays the level")
	require.Equal(t, uint64(2), NextFlapLevel(types.StateSuspicious, types.StateSuspicious, 2, 2, true, false, p),
		"only ANCHORED blocks count toward decay")
}

func TestNextStableBlocks_IncrementsDecaysLeaksAndResets(t *testing.T) {
	p := adaptiveParams() // StableWindow = 3

	require.Equal(t, uint64(2), NextStableBlocks(types.StateAnchored, types.StateAnchored, 1, 1, true, p),
		"healthy ANCHORED block with flap_level>0 increments")
	require.Equal(t, uint64(0), NextStableBlocks(types.StateAnchored, types.StateAnchored, 2, 1, true, p),
		"reaching StableWindow restarts the counter (the level just decayed)")
	require.Equal(t, uint64(1), NextStableBlocks(types.StateAnchored, types.StateAnchored, 2, 1, false, p),
		"an absorbed warning block leaks 1 instead of hard-resetting")
	require.Equal(t, uint64(0), NextStableBlocks(types.StateAnchored, types.StateAnchored, 0, 1, false, p),
		"leak is floored at 0")
	require.Equal(t, uint64(0), NextStableBlocks(types.StateAnchored, types.StateAnchored, 2, 0, true, p),
		"nothing to forgive at flap_level=0: stays 0")
	require.Equal(t, uint64(0), NextStableBlocks(types.StateAnchored, types.StateSuspicious, 2, 1, false, p),
		"leaving ANCHORED resets")
	require.Equal(t, uint64(0), NextStableBlocks(types.StateRecovering, types.StateAnchored, 2, 1, true, p),
		"entering ANCHORED starts from 0")
}

func TestCalculateNextState_AnchoredBackoffRequiresMoreConsecutiveNoiseAfterAFlap(t *testing.T) {
	// ANCHORED->SUSPICIOUS now uses the adaptive threshold: the same streak
	// that demoted at flap_level=0 must be absorbed at flap_level=1.
	p := adaptiveParams()
	warn := healthyMetrics()
	warn.BtcGap = p.SuspiciousThreshold // warning band, below SovereignThreshold
	in := FSMInput{Metrics: warn, UnhealthyStreak: 1, FlapLevel: 0}
	require.Equal(t, types.StateSuspicious, CalculateNextState(types.StateAnchored, in, p),
		"streak 1+1 >= threshold 2: demote")

	in.FlapLevel = 1
	require.Equal(t, types.StateAnchored, CalculateNextState(types.StateAnchored, in, p),
		"same streak at flap_level=1 (threshold 4): absorbed")

	in.UnhealthyStreak = 3
	require.Equal(t, types.StateSuspicious, CalculateNextState(types.StateAnchored, in, p),
		"streak 3+1 >= threshold 4: demote")
}

func TestCalculateNextState_RecoveringBackoffRequiresMoreConsecutiveNoiseAfterAFailedAttempt(t *testing.T) {
	// The core flapping-attack scenario: after one failed recovery attempt,
	// the SAME UnhealthyStreak that used to trigger a regression must now be
	// absorbed instead, since EffectiveDownHysteresisThreshold has doubled.
	p := types.DefaultParams()
	p.DownHysteresisThreshold = 2
	p.MaxDownHysteresisThreshold = 8
	m := healthyMetrics()
	m.SubnetDiversity = 0 // unhealthy, not critical

	// With 0 prior failed attempts: UnhealthyStreak = threshold-1 = 1 triggers regression.
	next := CalculateNextState(types.StateRecovering, FSMInput{
		Metrics:               m,
		SafeBlocks:            p.RecoveryHysteresisWait,
		UnhealthyStreak:       1,
		FlapLevel:             0,
		ReanchoringProofValid: true,
	}, p)
	require.Equal(t, types.StateSovereign, next, "baseline: threshold-1 streak regresses with no backoff")

	// With 1 prior failed attempt: the effective threshold is now 4, so the
	// same UnhealthyStreak=1 must be absorbed instead.
	next = CalculateNextState(types.StateRecovering, FSMInput{
		Metrics:               m,
		SafeBlocks:            p.RecoveryHysteresisWait,
		UnhealthyStreak:       1,
		FlapLevel:             1,
		ReanchoringProofValid: true,
	}, p)
	require.Equal(t, types.StateRecovering, next, "after 1 failed attempt: the same streak must now be absorbed, harder to exploit")
}
