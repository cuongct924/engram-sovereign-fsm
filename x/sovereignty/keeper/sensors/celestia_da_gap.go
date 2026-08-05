package sensors

import "context"

// DAAvailabilitySource abstracts a live Celestia availability observer --
// concretely, a real da.Publisher (x/da/publisher.go, wired via SetSource by
// cmd/engramd/main.go) tracking blob submissions against a real
// celestia-bridge. This package intentionally does not depend on any
// particular DA client library (mirrors BTCHeightSource's separation): the
// adapter lives in x/da / cmd/engramd instead.
//
// When a Source is set, RefreshMetrics (x/sovereignty/sensors_refresh.go)
// computes da_gap from spec/core/EngramFSM.tla:87's real formula
// (h_engram_current - h_engram_verified) using this live VerifiedHeight
// reading, instead of GetMetric's static SetAvailable-injected value below.
type DAAvailabilitySource interface {
	VerifiedHeight() (height uint64, ok bool)
	Failed() bool
}

// DASensor defaults to a static, test-controlled availability reading
// (SetAvailable/SetFailureFlags) -- production wiring to a live source is
// SetSource (Phase 7, mirroring BTCSensor).
type DASensor struct {
	available         bool
	dasFailed         bool
	attestationFailed bool
	source            DAAvailabilitySource
}

func NewDASensor() *DASensor {
	return &DASensor{available: true}
}

// SetSource wires a live Celestia availability observer in. Passing nil
// reverts to GetMetric/IsHealthy's static SetAvailable-based reading being
// the only signal.
func (s *DASensor) SetSource(src DAAvailabilitySource) {
	s.source = src
}

// Source returns the currently wired live source, or nil if none is set.
func (s *DASensor) Source() DAAvailabilitySource {
	return s.source
}

// SetAvailable toggles DA receipt availability (used for scenario S3 DA unavailable).
func (s *DASensor) SetAvailable(available bool) {
	s.available = available
}

// SetFailureFlags mirrors is_das_failed / is_attestation_failed in spec/core/EngramFSM.tla.
func (s *DASensor) SetFailureFlags(dasFailed, attestationFailed bool) {
	s.dasFailed = dasFailed
	s.attestationFailed = attestationFailed
}

// DasFailed and AttestationFailed expose the static, test-controlled
// failure flags set via SetFailureFlags -- read by RefreshMetrics
// (x/sovereignty/sensors_refresh.go) when no live Source is wired.
func (s *DASensor) DasFailed() bool         { return s.dasFailed }
func (s *DASensor) AttestationFailed() bool { return s.attestationFailed }

// GetMetric returns the DA gap: 0 when healthy, 1 otherwise (kept for backward
// compatibility with the SensorProvider interface's uint64 metric shape).
func (s *DASensor) GetMetric(ctx context.Context) (uint64, error) {
	if s.available {
		return 0, nil
	}
	return 1, nil
}

// IsHealthy mirrors IsDAHealthy's non-gap conjuncts (~is_das_failed /\ ~is_attestation_failed).
func (s *DASensor) IsHealthy() bool {
	return s.available && !s.dasFailed && !s.attestationFailed
}
