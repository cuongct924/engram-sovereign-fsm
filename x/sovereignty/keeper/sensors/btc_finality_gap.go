package sensors

import "context"

// BTCHeightSource abstracts a live Bitcoin chain-tip observer (concretely
// anchor.RPCClient's getblockcount, wired via SetSource by
// cmd/engramd/main.go) so this package stays independent of any RPC library
// -- mirrors P2PHealthSource's separation.
//
// When set, sensors_refresh.go's btcGapMetric uses it to compute btc_gap
// from live h_btc_current, instead of GetMetric's static SetGap value below.
type BTCHeightSource interface {
	CurrentHeight(ctx context.Context) (uint64, error)
}

// BTCSensor defaults to a static, test-controlled gap reading (SetGap);
// production wiring to a live source is SetSource.
type BTCSensor struct {
	gap    uint64
	source BTCHeightSource
}

func NewBTCSensor() *BTCSensor {
	return &BTCSensor{}
}

// SetGap overrides the current btc_gap reading, e.g. for fault-injection
// scenarios (S2 BTC congestion). Independent of Source: the caller decides
// whether to consult Source or this static value (see btcGapMetric).
func (s *BTCSensor) SetGap(gap uint64) {
	s.gap = gap
}

func (s *BTCSensor) GetMetric(ctx context.Context) (uint64, error) {
	return s.gap, nil
}

// SetSource wires a live Bitcoin height observer in. nil reverts to
// GetMetric's static SetGap reading being the only signal.
func (s *BTCSensor) SetSource(src BTCHeightSource) {
	s.source = src
}

// Source returns the currently wired live source, or nil if none is set.
func (s *BTCSensor) Source() BTCHeightSource {
	return s.source
}
