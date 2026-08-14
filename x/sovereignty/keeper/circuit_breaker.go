package keeper

import (
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
)

// FSMInput bundles the FSM-internal variables CalculateNextState needs
// beyond the raw sensor snapshot, mirroring safe_blocks/suspicious_duration/
// unhealthy_streak/failed_recovery_attempts/reanchoring_proof_valid in
// spec/core/EngramFSM.tla.
type FSMInput struct {
	Metrics                *types.PeripheralMetrics
	SafeBlocks             uint64
	SuspiciousDuration     uint64
	SuspiciousSafeBlocks   uint64
	UnhealthyStreak        uint64
	FailedRecoveryAttempts uint64
	ReanchoringProofValid  bool
}

// EffectiveDownHysteresisThreshold mirrors the TLA+ operator of the same name
// (spec/core/EngramFSM.tla): RECOVERING's down-hysteresis grace doubles per
// RECOVERING->SOVEREIGN regression, capped at MaxDownHysteresisThreshold --
// flapping attacks face a harder bar each cycle.
func EffectiveDownHysteresisThreshold(failedRecoveryAttempts uint64, p types.Params) uint64 {
	effective := p.DownHysteresisThreshold
	for i := uint64(0); i < failedRecoveryAttempts; i++ {
		if effective >= p.MaxDownHysteresisThreshold {
			return p.MaxDownHysteresisThreshold
		}
		effective *= 2
	}
	if effective > p.MaxDownHysteresisThreshold {
		return p.MaxDownHysteresisThreshold
	}
	return effective
}

// CalculateNextState ports CalculateNextFSMState (spec/core/EngramFSM.tla:318-358)
// branch-for-branch, in the same order. No direct-to-SOVEREIGN shortcut from
// any state -- StrictFSMTransitionSafety forbids skipping SUSPICIOUS.
//
// Down-hysteresis (E5's flapping fix): non-critical noise only demotes
// ANCHORED/RECOVERING after UnhealthyStreak+1 >= DownHysteresisThreshold
// consecutive times; critical conditions always demote immediately.
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
			if in.UnhealthyStreak+1 >= p.DownHysteresisThreshold {
				return types.StateSuspicious
			}
			return types.StateAnchored // absorb: streak not yet exhausted
		}

	case types.StateSuspicious:
		if critical {
			return types.StateSovereign
		}
		if healthy {
			// Up-hysteresis on the exit edge (Gray Failure Arbitrage fix): a
			// short healthy blip must not buy a free SuspiciousDuration reset --
			// require SuspiciousSafeBlocks+1 consecutive healthy blocks.
			if in.SuspiciousSafeBlocks+1 >= p.SuspiciousHysteresisWait {
				return types.StateAnchored
			}
			return types.StateSuspicious // absorb: streak not yet exhausted
		}

	case types.StateSovereign:
		if healthy {
			return types.StateRecovering
		}

	case types.StateRecovering:
		if critical {
			return types.StateSovereign
		}
		if !healthy {
			if in.UnhealthyStreak+1 >= EffectiveDownHysteresisThreshold(in.FailedRecoveryAttempts, p) {
				return types.StateSovereign
			}
			return types.StateRecovering // absorb: streak not yet exhausted
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
// (spec/core/EngramFSM.tla:383-394): staying in RECOVERING increments on
// healthy (cap HysteresisWait) and leaks -1 on absorbed noise (E5's flapping
// fix); any other transition resets to 0.
func NextSafeBlocks(currentState, targetState string, safeBlocks uint64, healthy bool, p types.Params) uint64 {
	if targetState != types.StateRecovering || currentState != types.StateRecovering {
		return 0
	}
	if healthy {
		if safeBlocks+1 > p.HysteresisWait {
			return p.HysteresisWait
		}
		return safeBlocks + 1
	}
	if safeBlocks == 0 {
		return 0
	}
	return safeBlocks - 1
}

// NextSuspiciousSafeBlocks mirrors ExecuteFSMTransition's
// suspicious_safe_blocks' update (spec/core/EngramFSM.tla): staying in
// SUSPICIOUS increments on healthy (cap SuspiciousHysteresisWait), leaks -1
// on absorbed noise (Gray Failure Arbitrage fix); any other transition resets
// to 0.
func NextSuspiciousSafeBlocks(currentState, targetState string, suspiciousSafeBlocks uint64, healthy bool, p types.Params) uint64 {
	if targetState != types.StateSuspicious || currentState != types.StateSuspicious {
		return 0
	}
	if healthy {
		if suspiciousSafeBlocks+1 > p.SuspiciousHysteresisWait {
			return p.SuspiciousHysteresisWait
		}
		return suspiciousSafeBlocks + 1
	}
	if suspiciousSafeBlocks == 0 {
		return 0
	}
	return suspiciousSafeBlocks - 1
}

// NextUnhealthyStreak mirrors ExecuteFSMTransition's unhealthy_streak'
// update (spec/core/EngramFSM.tla:371-381): increments while absorbing a
// non-critical unhealthy reading; resets to 0 on a real transition or a
// healthy block.
//
// The spec's "warning /\ ~critical" / "~healthy /\ ~critical" collapse to one
// healthy bool (staying implies ~critical; under ~critical, warning <=> ~healthy)
// so healthy can be the already-agreed ExtendedProposal.Healthy, not a live
// local sensor read (CLAUDE.md's "never write live local sensor reads into
// committed state" rule).
func NextUnhealthyStreak(currentState, targetState string, streak uint64, healthy bool) uint64 {
	stayed := currentState == targetState &&
		(currentState == types.StateAnchored || currentState == types.StateRecovering)
	if stayed && !healthy {
		return streak + 1
	}
	return 0
}

// NextFailedRecoveryAttempts mirrors ExecuteFSMTransition's
// failed_recovery_attempts' update (spec/core/EngramFSM.tla): +1 per real
// RECOVERING->SOVEREIGN regression (saturating once the capped effective
// threshold stops growing), 0 on RECOVERING->ANCHORED, unchanged otherwise.
func NextFailedRecoveryAttempts(currentState, targetState string, attempts uint64, p types.Params) uint64 {
	switch {
	case currentState == types.StateRecovering && targetState == types.StateSovereign:
		if EffectiveDownHysteresisThreshold(attempts, p) >= p.MaxDownHysteresisThreshold {
			return attempts
		}
		return attempts + 1
	case currentState == types.StateRecovering && targetState == types.StateAnchored:
		return 0
	default:
		return attempts
	}
}

// NextSuspiciousDuration mirrors the suspicious_duration' update
// (spec/core/EngramFSM.tla:337-340): increments (cap MaxSuspiciousTime+1)
// whenever the TARGET is SUSPICIOUS -- counting the entry block, unlike
// NextSafeBlocks which guards both states. currentState is kept only to match
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
