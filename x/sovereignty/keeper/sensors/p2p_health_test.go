package sensors

import (
	"context"
	"testing"
)

type stubP2PHealthSource struct {
	snapshot P2PSnapshot
}

func (s stubP2PHealthSource) PeerHealthSnapshot() P2PSnapshot { return s.snapshot }

func TestP2PSensor_NewSensorHasZeroSnapshotAndNoSource(t *testing.T) {
	s := NewP2PSensor()

	got, err := s.GetSnapshot(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != (P2PSnapshot{}) {
		t.Errorf("got %+v, want zero value", got)
	}
}

func TestP2PSensor_GetSnapshotReadsStaticByDefault(t *testing.T) {
	s := NewP2PSensor()
	want := P2PSnapshot{ActiveAnchors: 3, CleanPeers: 5, SubnetDiversity: 2, ChurnRate: 1, AvgTenure: 300, Latency: 20}
	s.SetSnapshot(want)

	got, err := s.GetSnapshot(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestP2PSensor_GetSnapshotPrefersLiveSourceWhenSet(t *testing.T) {
	s := NewP2PSensor()
	s.SetSnapshot(P2PSnapshot{ActiveAnchors: 1})
	live := P2PSnapshot{ActiveAnchors: 9, CleanPeers: 9}
	s.SetSource(stubP2PHealthSource{snapshot: live})

	got, err := s.GetSnapshot(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != live {
		t.Errorf("got %+v, want live source's %+v", got, live)
	}
}

// Regression for SetSnapshot's override semantics: an explicit fault-injection
// override must win even after a live Source was wired in.
func TestP2PSensor_SetSnapshotAlwaysWinsOverSource(t *testing.T) {
	s := NewP2PSensor()
	s.SetSource(stubP2PHealthSource{snapshot: P2PSnapshot{ActiveAnchors: 9}})

	override := P2PSnapshot{ActiveAnchors: 0, CleanPeers: 0}
	s.SetSnapshot(override)

	got, err := s.GetSnapshot(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != override {
		t.Errorf("got %+v, want override %+v -- SetSnapshot must clear any wired Source", got, override)
	}
}

func TestP2PSensor_SetSourceNilRevertsToStaticSnapshot(t *testing.T) {
	s := NewP2PSensor()
	static := P2PSnapshot{ActiveAnchors: 4}
	s.SetSnapshot(static)
	s.SetSource(stubP2PHealthSource{snapshot: P2PSnapshot{ActiveAnchors: 99}})
	s.SetSource(nil)

	got, err := s.GetSnapshot(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != static {
		t.Errorf("got %+v, want static %+v after SetSource(nil)", got, static)
	}
}
