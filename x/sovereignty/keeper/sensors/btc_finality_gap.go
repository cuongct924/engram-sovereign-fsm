package sensors

import "context"

// BTCSensor is mock-controlled for tests/fault-injection: production wiring to a
// real Bitcoin RPC / vigilante-reporter is TODO(Phase 5).
type BTCSensor struct {
	gap uint64
}

func NewBTCSensor() *BTCSensor {
	return &BTCSensor{}
}

// SetGap overrides the current BTC finality gap reading (btc_gap in spec terms);
// used by fault-injection scenarios (e.g. S2 BTC congestion) to force a schedule.
func (s *BTCSensor) SetGap(gap uint64) {
	s.gap = gap
}

func (s *BTCSensor) GetMetric(ctx context.Context) (uint64, error) {
	return s.gap, nil
}
