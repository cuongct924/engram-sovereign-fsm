package keeper

import (
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
)

// FSMInput bundles the FSM-internal variables that CalculateNextState needs
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

// EffectiveDownHysteresisThreshold mirrors the TLA+ operator of the same
// name (spec/core/EngramFSM.tla): RECOVERING's down-hysteresis grace period
// doubles per consecutive RECOVERING->SOVEREIGN regression since the last
// successful recovery, capped at MaxDownHysteresisThreshold -- a repeated,
// precisely-timed flapping attacker (docs/EXPERIMENT.md) faces a
// progressively harder bar each cycle, instead of paying the same fixed
// DownHysteresisThreshold cost every time. A single genuine network fault
// (failedRecoveryAttempts = 0) still only pays the plain
// DownHysteresisThreshold, unaffected.
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
// branch-for-branch, in the same order. Do not add a direct-to-SOVEREIGN
// shortcut from any state -- StrictFSMTransitionSafety forbids skipping
// SUSPICIOUS on the way down from ANCHORED.
//
// Down-hysteresis (E5's flapping fix, docs/EXPERIMENT.md): a warning-only or
// merely-not-yet-healthy reading -- never a critical one -- only demotes
// ANCHORED/RECOVERING once it has recurred UnhealthyStreak+1 >=
// DownHysteresisThreshold times in a row, instead of on the very first noisy
// block. Critical conditions are never softened this way; they demote
// immediately, exactly as before.
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
			// single healthy block used to exit immediately, hard-resetting
			// SuspiciousDuration to 0 and letting a precisely-timed attacker
			// restart the MaxSuspiciousTime clock forever. Now requires
			// SuspiciousSafeBlocks+1 consecutive healthy blocks while still
			// SUSPICIOUS -- SuspiciousDuration keeps accumulating throughout
			// (NextSuspiciousDuration increments whenever the target state is
			// still SUSPICIOUS), so a short healthy blip no longer buys a free reset.
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
// (spec/core/EngramFSM.tla:383-394): while staying in RECOVERING, a healthy
// block increments (capped at HysteresisWait) as before; a non-critical
// unhealthy block being absorbed by down-hysteresis LEAKS one unit instead
// of hard-resetting to 0 (E5's flapping fix -- keeps partial progress
// through sporadic noise). Any other transition resets to 0.
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
// suspicious_safe_blocks' update (spec/core/EngramFSM.tla): while staying in
// SUSPICIOUS, a healthy block increments (capped at SuspiciousHysteresisWait);
// a non-healthy block being absorbed LEAKS one unit instead of hard-resetting
// to 0, mirroring NextSafeBlocks' own leak semantics applied to SUSPICIOUS's
// exit edge (Gray Failure Arbitrage fix). Any other transition resets to 0.
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
// update (spec/core/EngramFSM.tla:371-381): increments only while absorbing
// a non-critical warning (ANCHORED) or unhealthy (RECOVERING) reading
// without yet demoting; resets to 0 the instant a real transition fires
// (demotion or a fully healthy reading).
//
// Simplified from the spec's literal "warning /\ ~critical" (ANCHORED) /
// "~healthy /\ ~critical" (RECOVERING) form to a single healthy bool:
// whenever currentState = targetState (we stayed rather than demoted),
// critical must already be false -- CalculateNextState's critical branch is
// checked first and always leaves the state, so staying implies ~critical
// held. Under ~critical, warning <=> ~healthy (IsWarningCondition and
// IsHealthyCondition agree on every non-critical BTC/DA/P2P conjunct), so
// both branches collapse to the same "stayed /\ ~healthy" test. This is not
// just style: it lets healthy be threaded in as a single already-agreed
// value (ExtendedProposal.Healthy) rather than requiring the full sensor
// snapshot this function would otherwise need to recompute -- callers at
// commit time (preblock.go) must not recompute IsHealthyCondition from
// their own live local sensors (CLAUDE.md's "never write live local sensor
// reads into committed state" rule).
func NextUnhealthyStreak(currentState, targetState string, streak uint64, healthy bool) uint64 {
	stayed := currentState == targetState &&
		(currentState == types.StateAnchored || currentState == types.StateRecovering)
	if stayed && !healthy {
		return streak + 1
	}
	return 0
}

// NextFailedRecoveryAttempts mirrors ExecuteFSMTransition's
// failed_recovery_attempts' update (spec/core/EngramFSM.tla): +1 on every
// real RECOVERING -> SOVEREIGN regression (critical or down-hysteresis-
// exhausted alike -- both are a failed attempt), reset to 0 on a successful
// RECOVERING -> ANCHORED, saturating once growing further wouldn't change
// the capped effective threshold. Unchanged for every other transition.
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
// (spec/core/EngramFSM.tla:337-340): increments, capped at
// MaxSuspiciousTime+1, whenever the TARGET state is SUSPICIOUS -- including
// the block that enters it, unlike NextSafeBlocks which guards on both
// current and target state. currentState is unused but kept to match
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
