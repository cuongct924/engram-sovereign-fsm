package sensors

import "context"

// DASensor is mock-controlled for tests/fault-injection: production wiring to a
// real Celestia light client / DAS is TODO(Phase 5).
type DASensor struct {
	available          bool
	dasFailed          bool
	attestationFailed  bool
}

func NewDASensor() *DASensor {
	return &DASensor{available: true}
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
