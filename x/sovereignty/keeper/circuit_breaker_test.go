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

func TestCalculateNextState_AnchoredToSuspiciousOnWarning(t *testing.T) {
	p := types.DefaultParams()
	m := healthyMetrics()
	m.BtcGap = p.SuspiciousThreshold // warning, not yet critical

	next := CalculateNextState(types.StateAnchored, FSMInput{Metrics: m}, p)
	require.Equal(t, types.StateSuspicious, next)
}

func TestCalculateNextState_SuspiciousToSovereignOnCritical(t *testing.T) {
	p := types.DefaultParams()
	m := healthyMetrics()
	m.BtcGap = p.SovereignThreshold

	next := CalculateNextState(types.StateSuspicious, FSMInput{Metrics: m}, p)
	require.Equal(t, types.StateSovereign, next)
}

func TestCalculateNextState_SuspiciousToAnchoredOnHealthy(t *testing.T) {
	p := types.DefaultParams()
	next := CalculateNextState(types.StateSuspicious, FSMInput{Metrics: healthyMetrics()}, p)
	require.Equal(t, types.StateAnchored, next)
}

func TestCalculateNextState_SovereignToRecoveringOnHealthy(t *testing.T) {
	p := types.DefaultParams()
	next := CalculateNextState(types.StateSovereign, FSMInput{Metrics: healthyMetrics()}, p)
	require.Equal(t, types.StateRecovering, next)
}

func TestCalculateNextState_RecoveringRegressesToSovereignWhenUnhealthy(t *testing.T) {
	p := types.DefaultParams()
	m := healthyMetrics()
	m.SubnetDiversity = 0 // breaks IsP2PQualityHealthy -> not healthy

	next := CalculateNextState(types.StateRecovering, FSMInput{
		Metrics:               m,
		SafeBlocks:            p.HysteresisWait,
		ReanchoringProofValid: true,
	}, p)
	require.Equal(t, types.StateSovereign, next)
}

func TestCalculateNextState_RecoveringStaysWhenHysteresisNotYetSatisfied(t *testing.T) {
	p := types.DefaultParams()
	require.Greater(t, p.HysteresisWait, uint64(0), "test assumes HysteresisWait > 0 so safe_blocks < HysteresisWait is reachable")

	next := CalculateNextState(types.StateRecovering, FSMInput{
		Metrics:               healthyMetrics(),
		SafeBlocks:            0, // < HysteresisWait
		ReanchoringProofValid: true,
	}, p)
	require.Equal(t, types.StateRecovering, next, "must not exit RECOVERING before safe_blocks reaches HysteresisWait")
}

func TestCalculateNextState_RecoveringStaysWhenProofNotYetValid(t *testing.T) {
	p := types.DefaultParams()

	next := CalculateNextState(types.StateRecovering, FSMInput{
		Metrics:               healthyMetrics(),
		SafeBlocks:            p.HysteresisWait,
		ReanchoringProofValid: false,
	}, p)
	require.Equal(t, types.StateRecovering, next, "must not exit RECOVERING before the reanchoring proof is valid, even if hysteresis is satisfied")
}

func TestCalculateNextState_RecoveringToAnchoredWhenBothConditionsMet(t *testing.T) {
	p := types.DefaultParams()

	next := CalculateNextState(types.StateRecovering, FSMInput{
		Metrics:               healthyMetrics(),
		SafeBlocks:            p.HysteresisWait,
		ReanchoringProofValid: true,
	}, p)
	require.Equal(t, types.StateAnchored, next)
}

func TestNextSafeBlocks_IncrementsWhileStayingRecoveringAndCapsAtHysteresisWait(t *testing.T) {
	p := types.DefaultParams()
	p.HysteresisWait = 3

	got := NextSafeBlocks(types.StateRecovering, types.StateRecovering, p.HysteresisWait, p)
	require.Equal(t, p.HysteresisWait, got, "must cap at HysteresisWait, not overflow past it")

	got = NextSafeBlocks(types.StateRecovering, types.StateRecovering, 0, p)
	require.Equal(t, uint64(1), got)
}

func TestNextSafeBlocks_ResetsOnAnyOtherTransition(t *testing.T) {
	p := types.DefaultParams()

	require.Equal(t, uint64(0), NextSafeBlocks(types.StateSovereign, types.StateRecovering, 0, p),
		"entering RECOVERING for the first time must start safe_blocks at 0")
	require.Equal(t, uint64(0), NextSafeBlocks(types.StateRecovering, types.StateSovereign, p.HysteresisWait, p),
		"regressing out of RECOVERING must reset safe_blocks")
	require.Equal(t, uint64(0), NextSafeBlocks(types.StateRecovering, types.StateAnchored, p.HysteresisWait, p),
		"completing recovery must reset safe_blocks")
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
	// only on target_state = "SUSPICIOUS", not on the current state also
	// being SUSPICIOUS -- so the very first block entering SUSPICIOUS from
	// ANCHORED must already count as 1, not 0 (a prior version of this
	// function required currentState == SUSPICIOUS too, lagging the spec's
	// counter by one block for the entire SUSPICIOUS dwell).
	p := types.DefaultParams()
	got := NextSuspiciousDuration(types.StateAnchored, types.StateSuspicious, 0, p)
	require.Equal(t, uint64(1), got)
}
