package sovereignty

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper/sensors"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
	"github.com/cuongct220020/engram-sovereign-fsm/x/vigilante"
)

// Sensors bundles the three per-node background sensors (each running
// independently, continuously observing its peripheral network) that
// RefreshMetrics snapshots into a single PeripheralMetrics every block. This
// is the real-ABCI-path counterpart of tests/e2e/harness.go's Advance:
// harness.go builds this same snapshot directly for the in-process test
// harness; RefreshMetrics does it for the real PrepareProposal/
// ProcessProposal handlers (previously missing entirely -- both handlers
// read whatever PeripheralMetrics happened to already be sitting in keeper
// state, e.g. from genesis or MsgInjectFaultRequest, never refreshed live).
//
// DA still reads from DASensor's static, test-controlled fields -- real
// Celestia light-node wiring is a follow-up. BTC and P2P's Source can be
// (and now are) wired to real observers: BTC to vigilante.RPCClient's real
// bitcoind getblockcount (x/vigilante/rpc.go, Phase 7), P2P to the CometBFT
// fork's live lp2p.Switch.PeerHealthSnapshot() (cmd/engramd/main.go, Phase 7).
//
// Anchor, when set, is this node's own vigilante.AnchorTracker -- the real
// checkpoint-submission-and-confirmation pipeline that gives h_btc_anchored
// somewhere to actually come from (see btcGapMetric's doc for the liveness
// bug this closes: without it, h_btc_anchored never advances while
// h_btc_current does, so VerifyReceipt's monotonicity check eventually
// rejects every proposal). nil means h_btc_anchored stays static, matching
// pre-Phase-7 behavior (fine for tests/fault-injection that don't wire BTC's
// live Source either).
type Sensors struct {
	BTC    *sensors.BTCSensor
	DA     *sensors.DASensor
	P2P    *sensors.P2PSensor
	Anchor *vigilante.AnchorTracker
}

// RefreshMetrics snapshots s's sensors and writes the result into k.Metrics,
// so the FSM-state computation that follows (currentFSMInput,
// CalculateNextState) sees this block's live readings rather than a stale
// prior value. Called by both PrepareProposal (the leader proposing a
// target_state) and ProcessProposal (every other validator cross-checking
// it against their OWN local sensors) -- matching "sensors propose,
// consensus decides" (CLAUDE.md): each node's target_state computation is
// always driven by that node's own current readings, never a value another
// node reported.
func RefreshMetrics(ctx sdk.Context, k *keeper.Keeper, s *Sensors) error {
	if s == nil {
		return nil
	}

	btcGap, err := btcGapMetric(ctx, k, s.BTC)
	if err != nil {
		return err
	}
	if s.Anchor != nil {
		// Idempotent per pending submission: only actually broadcasts a new
		// tx once the previous one has resolved, so calling this from both
		// PrepareProposal and ProcessProposal in the same block never
		// double-submits (see AnchorTracker.MaybeSubmit's doc).
		if err := s.Anchor.MaybeSubmit(ctx, uint64(ctx.BlockHeight())); err != nil {
			return err
		}
	}
	daHealthy := s.DA.IsHealthy()
	p2pSnap, err := s.P2P.GetSnapshot(ctx)
	if err != nil {
		return err
	}

	metrics := &types.PeripheralMetrics{
		BtcGap:              btcGap,
		DaGap:                daGapMetric(daHealthy, k.Params.DAThreshold),
		IsDasFailed:         false,
		IsAttestationFailed: false,
		SubnetDiversity:     p2pSnap.SubnetDiversity,
		ActiveAnchors:       p2pSnap.ActiveAnchors,
		CleanPeers:          p2pSnap.CleanPeers,
		PeerChurnRate:       p2pSnap.ChurnRate,
		AvgPeerTenure:       p2pSnap.AvgTenure,
		PeerLatency:         p2pSnap.Latency,
	}
	return k.Metrics.Set(ctx, metrics)
}

// daGapMetric converts DASensor's coarse healthy/unhealthy reading into the
// raw da_gap scalar CalculateNextState's predicates expect, mirroring
// tests/e2e/harness.go's daGapMetric but against the keeper's actually
// configured DAThreshold rather than a hardcoded default.
func daGapMetric(healthy bool, daThreshold uint64) uint64 {
	if healthy {
		return 0
	}
	return daThreshold // >= DAThreshold => not healthy
}

// btcGapMetric computes btc_gap. With no live Source wired, it's exactly
// btc.GetMetric's static SetGap-injected value (fault-injection tests,
// unchanged). With a live Source (vigilante.RPCClient, Phase 7), it instead
// applies spec/README.md §4.1's real formula:
//
//	Δ_BTC = H_current - min(H_submitted, H_anchored)
//
// using the Source's live chain-tip reading for H_current (also persisting
// it to k.HBtcCurrent, the canonical location other code already reads it
// from) and the keeper's tracked H_submitted/H_anchored. A submitted-but-
// not-yet-anchored checkpoint is counted toward the gap, matching the
// README's rationale for the min().
func btcGapMetric(ctx sdk.Context, k *keeper.Keeper, btc *sensors.BTCSensor) (uint64, error) {
	src := btc.Source()
	if src == nil {
		return btc.GetMetric(ctx)
	}

	hCurrent, err := src.CurrentHeight(ctx)
	if err != nil {
		return 0, err
	}
	if err := k.HBtcCurrent.Set(ctx, hCurrent); err != nil {
		return 0, err
	}

	hSubmitted, _ := k.HBtcSubmitted.Get(ctx)
	hAnchored, _ := k.HBtcAnchored.Get(ctx)
	baseline := min(hSubmitted, hAnchored)
	if hCurrent <= baseline {
		return 0, nil
	}
	return hCurrent - baseline, nil
}
