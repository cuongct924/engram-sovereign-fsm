package types

import "fmt"

// BaselineMode selects a same-binary comparison system for docs/EXPERIMENT.md's
// RQ2. It is an experiment configuration, not a protocol change: no spec/core
// operator is altered, and BaselineEngram (the zero value) is the real protocol.
type BaselineMode string

const (
	BaselineEngram BaselineMode = ""
	// BaselineHardGated pins the FSM to ANCHORED, so IsValidProposal's BTC/DA
	// receipt checks (EngramTendermint.tla:290-312) apply at every height.
	BaselineHardGated BaselineMode = "hard-gated"
	// BaselineSoft runs the real FSM but never locks withdrawals, so liveness
	// is kept at the cost of the circuit breaker's safety guarantee.
	BaselineSoft BaselineMode = "soft"
)

// ParseBaselineMode accepts "", "engram", "hard-gated" and "soft".
func ParseBaselineMode(s string) (BaselineMode, error) {
	switch s {
	case "", "engram":
		return BaselineEngram, nil
	case string(BaselineHardGated):
		return BaselineHardGated, nil
	case string(BaselineSoft):
		return BaselineSoft, nil
	}
	return BaselineEngram, fmt.Errorf("unknown baseline mode %q (want engram, hard-gated or soft)", s)
}

func (m BaselineMode) HardGated() bool { return m == BaselineHardGated }

// SkipsCircuitBreaker reports whether withdrawals stay admissible in
// SOVEREIGN/RECOVERING (the soft baseline's defining difference).
func (m BaselineMode) SkipsCircuitBreaker() bool { return m == BaselineSoft }

// hardGatedBTCRoundCap bounds the round fed to the BTC tolerance for the
// hard-gated baseline. anchor.Tolerance grows with round without limit, so
// an uncapped gate would eventually re-admit a stale checkpoint and only
// slow down instead of stalling (da.Tolerance already caps at round 3).
const hardGatedBTCRoundCap = 3

// BTCToleranceRound returns the round to pass to anchor.VerifyReceipt.
func (m BaselineMode) BTCToleranceRound(round uint64) uint64 {
	if m.HardGated() && round > hardGatedBTCRoundCap {
		return hardGatedBTCRoundCap
	}
	return round
}
