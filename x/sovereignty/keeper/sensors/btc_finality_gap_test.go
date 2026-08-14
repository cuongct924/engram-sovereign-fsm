package sensors

import (
	"context"
	"errors"
	"testing"
)

type stubBTCHeightSource struct {
	height    uint64
	err       error
	reachable bool
}

func (s stubBTCHeightSource) CurrentHeight(ctx context.Context) (uint64, error) {
	return s.height, s.err
}

func (s stubBTCHeightSource) Reachable(ctx context.Context) bool {
	return s.reachable
}

func TestBTCSensor_GetMetricReturnsStaticGapByDefault(t *testing.T) {
	s := NewBTCSensor()
	s.SetGap(42)

	got, err := s.GetMetric(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 42 {
		t.Errorf("got %d, want 42", got)
	}
}

func TestBTCSensor_NewSensorHasZeroGapAndNoSource(t *testing.T) {
	s := NewBTCSensor()

	got, err := s.GetMetric(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 0 {
		t.Errorf("got %d, want 0", got)
	}
	if s.Source() != nil {
		t.Error("expected no source wired by default")
	}
}

func TestBTCSensor_SetSourceIsIndependentOfGetMetric(t *testing.T) {
	// GetMetric always reads the static gap even with a Source wired -- only
	// RefreshMetrics consults Source, so SetGap fault-injection keeps working.
	s := NewBTCSensor()
	s.SetGap(7)
	s.SetSource(stubBTCHeightSource{height: 999})

	got, err := s.GetMetric(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 7 {
		t.Errorf("got %d, want 7 (GetMetric must not read from Source)", got)
	}
}

func TestBTCSensor_SourceRoundTrip(t *testing.T) {
	s := NewBTCSensor()
	if s.Source() != nil {
		t.Fatal("expected nil source before SetSource")
	}

	src := stubBTCHeightSource{height: 123, err: errors.New("rpc down")}
	s.SetSource(src)
	if s.Source() == nil {
		t.Fatal("expected non-nil source after SetSource")
	}
	height, err := s.Source().CurrentHeight(context.Background())
	if err == nil {
		t.Error("expected the wired source's error to propagate")
	}
	if height != 123 {
		t.Errorf("got %d, want 123", height)
	}

	s.SetSource(nil)
	if s.Source() != nil {
		t.Error("expected SetSource(nil) to clear the source")
	}
}
