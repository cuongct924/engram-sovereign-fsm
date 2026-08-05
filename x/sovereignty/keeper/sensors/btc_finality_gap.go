package sensors

import "context"

// BTCHeightSource abstracts a live Bitcoin chain-tip observer -- concretely,
// a real bitcoind RPC client (vigilante.RPCClient, Phase 7) calling
// getblockcount, wired in via SetSource by cmd/engramd/main.go. This package
// intentionally does not depend on any particular RPC library (mirrors
// P2PHealthSource's separation): the adapter lives in x/vigilante /
// cmd/engramd instead.
//
// When a Source is set, RefreshMetrics (x/sovereignty/sensors_refresh.go)
// computes btc_gap from spec/README.md §4.1's real formula
// (H_current - min(H_submitted, H_anchored)) using this live H_current
// reading, instead of GetMetric's static SetGap-injected value below.
type BTCHeightSource interface {
	CurrentHeight(ctx context.Context) (uint64, error)
}

// BTCSensor defaults to a static, test-controlled gap reading (SetGap) --
// production wiring to a live source is SetSource (Phase 7).
type BTCSensor struct {
	gap    uint64
	source BTCHeightSource
}

func NewBTCSensor() *BTCSensor {
	return &BTCSensor{}
}

// SetGap overrides the current BTC finality gap reading (btc_gap in spec
// terms); used by fault-injection scenarios (e.g. S2 BTC congestion) to
// force a schedule. This is independent of Source -- fault injection always
// works, even with a live source wired, since RefreshMetrics only consults
// Source when GetMetric's caller asks it to (see Source()'s doc).
func (s *BTCSensor) SetGap(gap uint64) {
	s.gap = gap
}

func (s *BTCSensor) GetMetric(ctx context.Context) (uint64, error) {
	return s.gap, nil
}

// SetSource wires a live Bitcoin height observer in. Passing nil reverts to
// GetMetric's static SetGap-based reading being the only signal.
func (s *BTCSensor) SetSource(src BTCHeightSource) {
	s.source = src
}

// Source returns the currently wired live source, or nil if none is set.
func (s *BTCSensor) Source() BTCHeightSource {
	return s.source
}
