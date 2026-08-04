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

// P2PSensor is mock-controlled for tests/fault-injection: production wiring to a
// real CometBFT p2p.Switch is TODO(Phase 5).
type P2PSensor struct {
	snapshot P2PSnapshot
}

func NewP2PSensor() *P2PSensor {
	return &P2PSensor{}
}

// SetSnapshot overrides the current P2P health reading; used by fault-injection
// scenarios (e.g. S4 partial eclipse, S5 anchor isolation) to force a specific state.
func (s *P2PSensor) SetSnapshot(snap P2PSnapshot) {
	s.snapshot = snap
}

func (s *P2PSensor) GetSnapshot(ctx context.Context) (P2PSnapshot, error) {
	return s.snapshot, nil
}
