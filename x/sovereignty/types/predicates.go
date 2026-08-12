package types

// This file ports the health-condition predicates in spec/core/EngramFSM.tla
// (lines 76-125) verbatim. Keep the branch structure identical to the spec
// when editing -- these predicates are the only integration surface between
// sensors and CalculateNextState (see keeper/circuit_breaker.go).

// IsBTCGapSuspicious mirrors IsBTCGapSuspicious: SUSPICIOUS_THRESHOLD <= btc_gap < SOVEREIGN_THRESHOLD.
func IsBTCGapSuspicious(m *PeripheralMetrics, p Params) bool {
	return p.SuspiciousThreshold <= m.BtcGap && m.BtcGap < p.SovereignThreshold
}

// IsBTCGapSovereign mirrors IsBTCGapSovereign: btc_gap >= SOVEREIGN_THRESHOLD.
func IsBTCGapSovereign(m *PeripheralMetrics, p Params) bool {
	return m.BtcGap >= p.SovereignThreshold
}

// IsDAHealthy mirrors IsDAHealthy: (da_gap < DA_THRESHOLD) /\ ~is_das_failed /\ ~is_attestation_failed.
func IsDAHealthy(m *PeripheralMetrics, p Params) bool {
	return m.DaGap < p.DAThreshold && !m.IsDasFailed && !m.IsAttestationFailed
}

// IsP2PQualityHealthy mirrors IsP2PQualityHealthy (tri-interface profiler: structural,
// behavioral and latency conjuncts) -- NOT peer-count-only, which spec/docs/EXPERIMENT.md's
// E4 uses as the inferior baseline detector.
func IsP2PQualityHealthy(m *PeripheralMetrics, p Params) bool {
	return m.SubnetDiversity >= p.MinSubnetDiversity &&
		m.ActiveAnchors >= p.MinAnchorPeers &&
		m.CleanPeers >= p.MinPeers &&
		m.PeerChurnRate <= p.MaxChurnRate &&
		m.AvgPeerTenure >= p.MinAvgTenure &&
		m.PeerLatency <= p.MaxPeerLatency
}

// IsCriticalCondition mirrors IsCriticalCondition: hard failure via BTC gap,
// BTC SPV/header verification failure (the anchor height itself is
// untrustworthy, not merely stale -- same severity class as
// IsBTCGapSovereign, unlike is_das_failed/is_attestation_failed which only
// feed IsWarningCondition), total anchor-peer loss (complete eclipse), or a
// SUSPICIOUS gray-failure timeout.
func IsCriticalCondition(m *PeripheralMetrics, p Params, suspiciousDuration uint64) bool {
	return IsBTCGapSovereign(m, p) ||
		m.IsBtcSpvFailed ||
		m.ActiveAnchors == 0 ||
		suspiciousDuration >= p.MaxSuspiciousTime
}

// IsWarningCondition mirrors IsWarningCondition: soft warning from an elevated
// BTC gap, or DA/P2P degradation.
func IsWarningCondition(m *PeripheralMetrics, p Params) bool {
	return IsBTCGapSuspicious(m, p) || !IsDAHealthy(m, p) || !IsP2PQualityHealthy(m, p)
}

// IsHealthyCondition mirrors IsHealthyCondition: all sensors green and thresholds satisfied.
func IsHealthyCondition(m *PeripheralMetrics, p Params) bool {
	return !IsBTCGapSovereign(m, p) && !IsBTCGapSuspicious(m, p) && !m.IsBtcSpvFailed && IsDAHealthy(m, p) && IsP2PQualityHealthy(m, p)
}
