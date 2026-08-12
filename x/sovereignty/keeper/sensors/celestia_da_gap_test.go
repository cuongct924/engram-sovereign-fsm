package sensors

import (
	"context"
	"testing"
)

type stubDAAvailabilitySource struct {
	height  uint64
	ok      bool
	failed  bool
	healthy bool
}

func (s stubDAAvailabilitySource) VerifiedHeight() (uint64, bool)        { return s.height, s.ok }
func (s stubDAAvailabilitySource) Failed() bool                          { return s.failed }
func (s stubDAAvailabilitySource) ProbeHealthy(ctx context.Context) bool { return s.healthy }

func TestDASensor_NewSensorDefaultsToAvailable(t *testing.T) {
	s := NewDASensor()

	got, err := s.GetMetric(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 0 {
		t.Errorf("got %d, want 0 (available)", got)
	}
	if !s.IsHealthy() {
		t.Error("expected healthy by default")
	}
}

func TestDASensor_SetAvailableTogglesGetMetric(t *testing.T) {
	s := NewDASensor()

	s.SetAvailable(false)
	got, err := s.GetMetric(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 1 {
		t.Errorf("got %d, want 1 (unavailable)", got)
	}

	s.SetAvailable(true)
	got, err = s.GetMetric(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 0 {
		t.Errorf("got %d, want 0 (available)", got)
	}
}

// spec/core/EngramFSM.tla:87's IsDAHealthy non-gap conjuncts:
// ~is_das_failed /\ ~is_attestation_failed -- each flag must independently
// break IsHealthy even while available.
func TestDASensor_IsHealthy(t *testing.T) {
	cases := []struct {
		name              string
		available         bool
		dasFailed         bool
		attestationFailed bool
		want              bool
	}{
		{"available, no failures", true, false, false, true},
		{"unavailable alone breaks healthy", false, false, false, false},
		{"das failed alone breaks healthy", true, true, false, false},
		{"attestation failed alone breaks healthy", true, false, true, false},
		{"both failures", true, true, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := NewDASensor()
			s.SetAvailable(tc.available)
			s.SetFailureFlags(tc.dasFailed, tc.attestationFailed)
			if got := s.IsHealthy(); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
			if s.DasFailed() != tc.dasFailed {
				t.Errorf("DasFailed(): got %v, want %v", s.DasFailed(), tc.dasFailed)
			}
			if s.AttestationFailed() != tc.attestationFailed {
				t.Errorf("AttestationFailed(): got %v, want %v", s.AttestationFailed(), tc.attestationFailed)
			}
		})
	}
}

func TestDASensor_SourceRoundTrip(t *testing.T) {
	s := NewDASensor()
	if s.Source() != nil {
		t.Fatal("expected nil source before SetSource")
	}

	src := stubDAAvailabilitySource{height: 50, ok: true, failed: true}
	s.SetSource(src)
	if s.Source() == nil {
		t.Fatal("expected non-nil source after SetSource")
	}
	height, ok := s.Source().VerifiedHeight()
	if height != 50 || !ok {
		t.Errorf("got (%d, %v), want (50, true)", height, ok)
	}
	if !s.Source().Failed() {
		t.Error("expected the wired source's Failed() to propagate")
	}

	s.SetSource(nil)
	if s.Source() != nil {
		t.Error("expected SetSource(nil) to clear the source")
	}
}
