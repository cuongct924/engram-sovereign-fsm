package types

// Params mirrors the CONSTANTS declared in spec/core/EngramFSM.tla.
type Params struct {
	SuspiciousThreshold uint64 // BTC gap threshold for Gray Failure warning
	SovereignThreshold  uint64 // BTC gap threshold for Hard Failure (circuit-break)
	DAThreshold         uint64 // da_gap threshold for DA health
	HysteresisWait      uint64 // safe_blocks required before RECOVERING -> ANCHORED
	MinPeers            uint64 // minimum clean (non-blacklisted) peers required
	MinSubnetDiversity  uint64 // minimum distinct subnets required
	MinAnchorPeers      uint64 // minimum active anchor/bootstrap peers required
	MaxChurnRate        uint64 // maximum allowed peer disconnects/reconnects per epoch
	MinAvgTenure        uint64 // minimum average peer tenure
	MaxPeerLatency      uint64 // maximum allowed peer latency
	MaxSuspiciousTime   uint64 // max blocks tolerated in SUSPICIOUS before escalating to SOVEREIGN
	MaxIgnoreRounds     uint64 // rounds a forced tx can be ignored before IsCensoring trips
}

// DefaultParams returns the exact FSM THRESHOLDS block from
// spec/core/MC_StressC1Safety.cfg -- the smallest configuration formally
// verified (Safety + Liveness + RefinementSafety/Liveness) by TLC. Using
// these as the prototype default keeps E2 fault-injection results directly
// comparable to the formal verification numbers in README.md's stress-test
// table, rather than picking arbitrary thresholds.
func DefaultParams() Params {
	return Params{
		SuspiciousThreshold: 1,
		SovereignThreshold:  2,
		DAThreshold:         1,
		HysteresisWait:      1,
		MinPeers:            2,
		MinSubnetDiversity:  2,
		MinAnchorPeers:      1,
		MaxChurnRate:        1,
		MinAvgTenure:        1,
		MaxPeerLatency:      1,
		MaxSuspiciousTime:   1,
		MaxIgnoreRounds:     1,
	}
}
