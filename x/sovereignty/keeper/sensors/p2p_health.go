package sensors

import "context"

// P2PSnapshot mirrors the inputs to IsP2PQualityHealthy in spec/core/EngramFSM.tla
// (SubnetDiversity, ActiveAnchors, CleanPeers, peer_churn_rate, avg_peer_tenure, peer_latency).
type P2PSnapshot struct {
	ActiveAnchors  uint64
	CleanPeers     uint64
	SubnetDiversity uint64
	ChurnRate      uint64
	AvgTenure      uint64
	Latency        uint64
}

// P2PHealthSource abstracts a live, continuously-updated P2P telemetry
// provider -- concretely, the CometBFT fork's *lp2p.Switch.PeerHealthSnapshot()
// (github.com/cuongct090_04/engram-consensus-core, M0a), wired in via SetSource
// by cmd/engramd/main.go once node.NewNode() has constructed the real Switch.
// This package intentionally does not import the fork directly (x/sovereignty
// is otherwise agnostic to the specific CometBFT P2P implementation) -- the
// adapter that converts the fork's lp2p.HealthSnapshot into a P2PSnapshot
// lives at the cmd/engramd wiring layer instead.
type P2PHealthSource interface {
	PeerHealthSnapshot() P2PSnapshot
}

// P2PSensor defaults to a static, test-controlled reading (SetSnapshot) --
// production wiring to a live source is SetSource (Phase 7).
type P2PSensor struct {
	snapshot P2PSnapshot
	source   P2PHealthSource
}

func NewP2PSensor() *P2PSensor {
	return &P2PSensor{}
}

// SetSnapshot overrides the current P2P health reading; used by fault-injection
// scenarios (e.g. S4 partial eclipse, S5 anchor isolation) to force a specific
// state. An explicit override always wins over a live Source, mirroring
// MsgInjectFaultRequest's role as a debug/experiment-only escape hatch.
func (s *P2PSensor) SetSnapshot(snap P2PSnapshot) {
	s.snapshot = snap
	s.source = nil
}

// SetSource wires a live telemetry provider in; GetSnapshot reads from it on
// every call once set. Passing nil reverts to the static SetSnapshot-based
// reading.
func (s *P2PSensor) SetSource(src P2PHealthSource) {
	s.source = src
}

func (s *P2PSensor) GetSnapshot(ctx context.Context) (P2PSnapshot, error) {
	if s.source != nil {
		return s.source.PeerHealthSnapshot(), nil
	}
	return s.snapshot, nil
}
