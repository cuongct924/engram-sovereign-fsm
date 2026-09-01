package sovereignty

import (
	"fmt"

	"github.com/cuongct220020/engram-sovereign-fsm/x/anchor"
	"github.com/cuongct220020/engram-sovereign-fsm/x/da"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper/sensors"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Sensors bundles the per-node background sensors RefreshMetrics snapshots
// into a PeripheralMetrics each block (real-ABCI counterpart of
// tests/e2e/harness.go's Advance). nil Anchor/DAPublisher keeps
// h_btc_anchored/h_engram_verified static -- fine for tests without a
// live Source.
type Sensors struct {
	BTC         *sensors.BTCSensor
	DA          *sensors.DASensor
	P2P         *sensors.P2PSensor
	Anchor      *anchor.AnchorTracker
	DAPublisher *da.Publisher
}

// RefreshMetrics snapshots sensors into k.Metrics so the FSM computation
// reads this block's live readings. Called from both proposal handlers so
// each node's target_state comes from its own readings, never another
// node's ("sensors propose, consensus decides").
func RefreshMetrics(ctx sdk.Context, k *keeper.Keeper, s *Sensors) error {
	if s == nil {
		return nil
	}

	btcGap, btcSpvFailed, btcCurrentHeight, err := btcGapMetric(ctx, k, s.BTC, s.Anchor)
	if err != nil {
		return err
	}
	if s.Anchor != nil {
		// Idempotent per pending submission -- never double-submits across
		// the PrepareProposal/ProcessProposal pair in a block (see
		// AnchorTracker.MaybeSubmit's doc).
		if err := s.Anchor.MaybeSubmit(ctx, uint64(ctx.BlockHeight())); err != nil {
			return err
		}
	}
	if s.DAPublisher != nil {
		// Same idempotence as Anchor.MaybeSubmit above (see
		// da.Publisher.MaybePublish's doc).
		if err := s.DAPublisher.MaybePublish(ctx, uint64(ctx.BlockHeight()), da.HeightMarker(uint64(ctx.BlockHeight()))); err != nil {
			return err
		}
	}
	daGap, dasFailed, attestationFailed, err := daGapMetric(ctx, k, s.DA)
	if err != nil {
		return err
	}
	p2pSnap, err := s.P2P.GetSnapshot(ctx)
	if err != nil {
		return err
	}

	// confirmedAnchorHeight/daVerifiedHeight feed NewPrepareProposalHandler/
	// NewProcessProposalHandler's h_btc_anchored/h_engram_verified advancement
	// (formerly a direct live read inside PrepareProposal itself) via the
	// quorum-certified median in AggregateHealthCertificates -- this is now
	// the only place any validator calls these sensor methods.
	var confirmedAnchorHeight uint64
	if s.Anchor != nil {
		if confirmed, ok := s.Anchor.ConfirmedAnchorHeight(); ok {
			confirmedAnchorHeight = confirmed
		}
	}
	var daVerifiedHeight uint64
	if s.DAPublisher != nil {
		if verified, ok := s.DAPublisher.VerifiedHeight(); ok {
			daVerifiedHeight = verified
		}
	}

	metrics := &types.PeripheralMetrics{
		BtcGap:                btcGap,
		IsBtcSpvFailed:        btcSpvFailed,
		DaGap:                 daGap,
		IsDasFailed:           dasFailed,
		IsAttestationFailed:   attestationFailed,
		SubnetDiversity:       p2pSnap.SubnetDiversity,
		ActiveAnchors:         p2pSnap.ActiveAnchors,
		CleanPeers:            p2pSnap.CleanPeers,
		PeerChurnRate:         p2pSnap.ChurnRate,
		AvgPeerTenure:         p2pSnap.AvgTenure,
		PeerLatency:           p2pSnap.Latency,
		ConfirmedAnchorHeight: confirmedAnchorHeight,
		DaVerifiedHeight:      daVerifiedHeight,
		BtcCurrentHeight:      btcCurrentHeight,
	}

	// Diagnostic-only, never committed -- this node's own local sensor reads,
	// scraped from `docker logs` by E9's live figure and E4's A3/A4 tests.
	// stdout only, no k.*.Set, so it can't affect AppHash or determinism.
	fmt.Printf(
		"engramd: sensor_snapshot height=%d btc_gap=%d btc_spv_failed=%t da_gap=%d da_failed=%t attestation_failed=%t active_anchors=%d clean_peers=%d churn_rate=%d avg_tenure=%d peer_latency=%d\n",
		ctx.BlockHeight(), metrics.BtcGap, metrics.IsBtcSpvFailed, metrics.DaGap, metrics.IsDasFailed, metrics.IsAttestationFailed, metrics.ActiveAnchors, metrics.CleanPeers, metrics.PeerChurnRate, metrics.AvgPeerTenure, metrics.PeerLatency,
	)

	return k.Metrics.Set(ctx, metrics)
}

// daGapMetric computes da_gap (EngramFSM.tla:87). Without a live Source it
// mirrors DASensor's static flags; with one it measures the real gap vs
// this validator's own chain height. Simplification: Source.Failed()
// stands in for both failure flags -- da.Publisher can't separate them.
func daGapMetric(ctx sdk.Context, k *keeper.Keeper, sensor *sensors.DASensor) (gap uint64, dasFailed, attestationFailed bool, err error) {
	src := sensor.Source()
	if src == nil {
		if sensor.IsHealthy() {
			return 0, sensor.DasFailed(), sensor.AttestationFailed(), nil
		}
		return k.Params.DAThreshold, sensor.DasFailed(), sensor.AttestationFailed(), nil
	}

	hCurrent := uint64(ctx.BlockHeight())
	if err := k.HEngramCurrent.Set(ctx, hCurrent); err != nil {
		return 0, false, false, err
	}

	// Failed() can lag an in-flight Submit by up to its own timeout, so
	// different validators see different stale values and disagree on Healthy
	// -- OR in ProbeHealthy's fresh, stateless check (see ProbeHealthy's doc).
	failed := src.Failed() || !src.ProbeHealthy(ctx)
	hVerified, ok := src.VerifiedHeight()
	if !ok {
		// No DA attestation has EVER completed yet -- report the maximal gap,
		// not hCurrent-0 (reads as healthy at low heights and deadlocked every
		// proposal at genesis bootstrap, reproduced live).
		return k.Params.DAThreshold, failed, failed, nil
	}
	if hCurrent <= hVerified {
		return 0, failed, failed, nil
	}
	return hCurrent - hVerified, failed, failed, nil
}

// btcGapMetric computes btc_gap from h_btc_anchored (spec/core/
// EngramServer.tla:148), not the abstract min(H_submitted, H_anchored)
// (EngramFSM.tla:95). spvFailed re-verifies the agreed anchor against our
// own bitcoind (is_btc_spv_failed, EngramFSM.tla:184). hCurrent is
// returned, not committed here, for quorum certification via
// PeripheralMetrics.BtcCurrentHeight.
func btcGapMetric(ctx sdk.Context, k *keeper.Keeper, btc *sensors.BTCSensor, anchorTracker *anchor.AnchorTracker) (gap uint64, spvFailed bool, hCurrent uint64, err error) {
	src := btc.Source()
	if src == nil {
		gap, err = btc.GetMetric(ctx)
		return gap, false, 0, err
	}

	// Reachable first: a stale pooled connection can succeed on one validator
	// and fail on another at the same instant, disagreeing on btc_gap/Healthy
	// while bitcoind is uniformly down. The fresh, stateless TCP check keeps
	// the down/up determination consistent across validators.
	if !src.Reachable(ctx) {
		return k.Params.SovereignThreshold, false, 0, nil
	}

	hCurrent, err = src.CurrentHeight(ctx)
	if err != nil {
		// An outage degrades through btc_gap, not a failed proposal: report
		// the maximum gap (matching daGapMetric's fallback) -- zero
		// visibility is at least as severe as any gap this threshold catches.
		return k.Params.SovereignThreshold, false, 0, nil
	}

	hAnchored, _ := k.HBtcAnchored.Get(ctx)
	if anchorTracker != nil && hAnchored > 0 {
		// A verification-RPC error is treated as failed, not swallowed like
		// MaybeSubmit's RPC errors: trusting an unverifiable anchor is the
		// exploitation window this check closes.
		ok, verr := anchorTracker.VerifyAnchor(ctx, hAnchored)
		spvFailed = verr != nil || !ok
	}

	if hCurrent <= hAnchored {
		return 0, spvFailed, hCurrent, nil
	}
	return hCurrent - hAnchored, spvFailed, hCurrent, nil
}
