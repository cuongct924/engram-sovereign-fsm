package types

// Ports the health-condition predicates in spec/core/EngramFSM.tla (76-125)
// verbatim -- the only integration surface between sensors and
// CalculateNextState. Keep branch structure identical to the spec.

// IsBTCGapSuspicious mirrors IsBTCGapSuspicious: SUSPICIOUS_THRESHOLD <= btc_gap < SOVEREIGN_THRESHOLD.
func IsBTCGapSuspicious(m *PeripheralMetrics, p Params) bool {
	return p.SuspiciousThreshold <= m.BtcGap && m.BtcGap < p.SovereignThreshold
}

// IsBTCGapSovereign mirrors IsBTCGapSovereign: btc_gap >= SOVEREIGN_THRESHOLD.
func IsBTCGapSovereign(m *PeripheralMetrics, p Params) bool {
	return m.BtcGap >= p.SovereignThreshold
}

// IsBTCHealthy mirrors IsBTCHealthy (spec/core/EngramFSM.tla): gap within
// both thresholds and no SPV failure -- gates anchor.VerifyReceipt like
// IsDAHealthy gates da.VerifyReceipt.
func IsBTCHealthy(m *PeripheralMetrics, p Params) bool {
	return !IsBTCGapSuspicious(m, p) && !IsBTCGapSovereign(m, p) && !m.IsBtcSpvFailed
}

// IsDAHealthy mirrors IsDAHealthy: (da_gap < DA_THRESHOLD) /\ ~is_das_failed /\ ~is_attestation_failed.
func IsDAHealthy(m *PeripheralMetrics, p Params) bool {
	return m.DaGap < p.DAThreshold && !m.IsDasFailed && !m.IsAttestationFailed
}

// IsP2PQualityHealthy mirrors IsP2PQualityHealthy (structural, behavioral,
// latency conjuncts) -- not peer-count-only (E4's inferior baseline).
func IsP2PQualityHealthy(m *PeripheralMetrics, p Params) bool {
	return m.SubnetDiversity >= p.MinSubnetDiversity &&
		m.ActiveAnchors >= p.MinAnchorPeers &&
		m.CleanPeers >= p.MinPeers &&
		m.PeerChurnRate <= p.MaxChurnRate &&
		m.AvgPeerTenure >= p.MinAvgTenure &&
		m.PeerLatency <= p.MaxPeerLatency
}

// IsCriticalCondition mirrors IsCriticalCondition: BTC gap/SPV failure, total
// anchor-peer loss (eclipse), or a SUSPICIOUS gray-failure timeout.
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
