package sensors

import "context"

// P2PSnapshot mirrors the inputs to IsP2PQualityHealthy in spec/core/EngramFSM.tla
// (SubnetDiversity, ActiveAnchors, CleanPeers, peer_churn_rate, avg_peer_tenure, peer_latency).
type P2PSnapshot struct {
	ActiveAnchors   uint64
	CleanPeers      uint64
	SubnetDiversity uint64
	ChurnRate       uint64
	AvgTenure       uint64
	Latency         uint64
}

// P2PHealthSource abstracts a live P2P telemetry provider -- concretely the
// CometBFT fork's *lp2p.Switch.PeerHealthSnapshot(), wired by cmd/engramd --
// keeping x/sovereignty agnostic to the P2P implementation.
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

// SetSnapshot overrides the P2P reading for fault-injection (S4/S5) and
// always beats a live Source (same escape-hatch role as MsgInjectFaultRequest).
func (s *P2PSensor) SetSnapshot(snap P2PSnapshot) {
	s.snapshot = snap
	s.source = nil
}

// SetSource wires a live telemetry provider; once set, GetSnapshot reads from
// it every call. nil reverts to the static SetSnapshot reading.
func (s *P2PSensor) SetSource(src P2PHealthSource) {
	s.source = src
}

func (s *P2PSensor) GetSnapshot(ctx context.Context) (P2PSnapshot, error) {
	if s.source != nil {
		return s.source.PeerHealthSnapshot(), nil
	}
	return s.snapshot, nil
}
