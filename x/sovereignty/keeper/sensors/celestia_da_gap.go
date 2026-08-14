package sensors

import "context"

// DAAvailabilitySource abstracts a live Celestia availability observer --
// concretely a real da.Publisher (x/da/publisher.go, wired via SetSource by
// cmd/engramd/main.go). This package stays independent of any DA client
// library; the adapter lives in x/da / cmd/engramd.
//
// When set, RefreshMetrics computes da_gap from EngramFSM.tla:87's real
// formula (h_engram_current - h_engram_verified) instead of GetMetric's
// static value.
type DAAvailabilitySource interface {
	VerifiedHeight() (height uint64, ok bool)
	Failed() bool
	// ProbeHealthy is a fresh, bounded, stateless reachability check -- never
	// stale (unlike Failed()), so validators converge on the same DA-down
	// reading within ~1 block.
	ProbeHealthy(ctx context.Context) bool
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

// DasFailed and AttestationFailed expose the static, test-controlled failure
// flags set via SetFailureFlags -- read by RefreshMetrics when no live Source
// is wired.
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
