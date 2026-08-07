package keeper

import (
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
)

// FSMInput bundles the FSM-internal variables that CalculateNextState needs
// beyond the raw sensor snapshot, mirroring safe_blocks/suspicious_duration/
// reanchoring_proof_valid in spec/core/EngramFSM.tla.
type FSMInput struct {
	Metrics               *types.PeripheralMetrics
	SafeBlocks            uint64
	SuspiciousDuration    uint64
	ReanchoringProofValid bool
}

// CalculateNextState ports CalculateNextFSMState (spec/core/EngramFSM.tla:309-329)
// branch-for-branch, in the same order. Do NOT reintroduce a "jump directly to
// SOVEREIGN from any state" shortcut: StrictFSMTransitionSafety in the spec
// forbids skipping SUSPICIOUS on the way down from ANCHORED, and this exact
// bug was the reason the previous version of this function was rewritten.
func CalculateNextState(currentState string, in FSMInput, p types.Params) string {
	critical := types.IsCriticalCondition(in.Metrics, p, in.SuspiciousDuration)
	warning := types.IsWarningCondition(in.Metrics, p)
	healthy := types.IsHealthyCondition(in.Metrics, p)

	switch currentState {
	case types.StateAnchored:
		if critical {
			return types.StateSovereign
		}
		if warning {
			return types.StateSuspicious
		}

	case types.StateSuspicious:
		if critical {
			return types.StateSovereign
		}
		if healthy {
			return types.StateAnchored
		}

	case types.StateSovereign:
		if healthy {
			return types.StateRecovering
		}

	case types.StateRecovering:
		if !healthy {
			return types.StateSovereign
		}
		if in.SafeBlocks == p.HysteresisWait && in.ReanchoringProofValid {
			return types.StateAnchored
		}
		// healthy but hysteresis not yet satisfied or proof still pending: stay RECOVERING
		return types.StateRecovering
	}

	// No transition condition matched: hold current state.
	return currentState
}

// NextSafeBlocks mirrors ExecuteFSMTransition's safe_blocks' update
// (spec/core/EngramFSM.tla:343-346): increments, capped at HysteresisWait,
// only while staying in RECOVERING between consecutive blocks; resets to 0
// on any other transition (including entering RECOVERING for the first time).
func NextSafeBlocks(currentState, targetState string, safeBlocks uint64, p types.Params) uint64 {
	if targetState == types.StateRecovering && currentState == types.StateRecovering {
		if safeBlocks+1 > p.HysteresisWait {
			return p.HysteresisWait
		}
		return safeBlocks + 1
	}
	return 0
}

// NextSuspiciousDuration mirrors the suspicious_duration' update
// (spec/core/EngramFSM.tla:337-340): increments, capped at MaxSuspiciousTime+1,
// whenever the TARGET state is SUSPICIOUS -- including the very first block
// entering SUSPICIOUS from ANCHORED, not just while already staying in
// SUSPICIOUS. The spec's formula only guards on target_state, not on
// currentState also being SUSPICIOUS (unlike NextSafeBlocks, which
// deliberately does guard on both -- see its own doc); an earlier version of
// this function copied that two-state guard here too, which made the
// counter lag the spec by exactly one block for the entire SUSPICIOUS dwell,
// delaying the MAX_SUSPICIOUS_TIME gray-failure escalation to SOVEREIGN by
// one block. currentState is unused now but kept in the signature to match
// NextSafeBlocks' call shape.
func NextSuspiciousDuration(currentState, targetState string, duration uint64, p types.Params) uint64 {
	_ = currentState
	if targetState != types.StateSuspicious {
		return 0
	}
	cap := p.MaxSuspiciousTime + 1
	if duration+1 > cap {
		return cap
	}
	return duration + 1
}
