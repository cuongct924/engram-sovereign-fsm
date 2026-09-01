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

	// Diagnostic-only, never committed -- this node's own local sensor
	// reads, printed for E9's live figure to scrape via `docker logs`
	// (docs/EXPERIMENT.md's E9), and for E4 A3/A4 live-test debugging
	// (churn_rate/avg_tenure/peer_latency added after a first A3 run showed
	// a real SUSPICIOUS transition with BTC/DA/anchor-count all healthy,
	// leaving the P2P-quality fields as the only unlogged, unconfirmed
	// candidate cause). Deliberately height-keyed, not timestamped, matching
	// this package's existing engramd: diagnostic convention (preblock.go's
	// recordDetectedEvidence, reanchor.go, x/anchor/anchor.go) -- stdout
	// only, no k.*.Set call, so it cannot affect AppHash or determinism
	// regardless of per-validator disagreement.
	fmt.Printf(
		"engramd: sensor_snapshot height=%d btc_gap=%d btc_spv_failed=%t da_gap=%d da_failed=%t attestation_failed=%t active_anchors=%d clean_peers=%d churn_rate=%d avg_tenure=%d peer_latency=%d\n",
		ctx.BlockHeight(), metrics.BtcGap, metrics.IsBtcSpvFailed, metrics.DaGap, metrics.IsDasFailed, metrics.IsAttestationFailed, metrics.ActiveAnchors, metrics.CleanPeers, metrics.PeerChurnRate, metrics.AvgPeerTenure, metrics.PeerLatency,
	)

	return k.Metrics.Set(ctx, metrics)
}

// daGapMetric computes da_gap (EngramFSM.tla:87, h_engram_current -
// h_engram_verified) plus is_das_failed/is_attestation_failed. Without a live
// Source it mirrors DASensor's static flags; with one (da.Publisher) it
// measures the real gap vs this validator's own chain height, persisted to
// k.HEngramCurrent.
//
// Simplification: Source.Failed() stands in for both failure flags --
// da.Publisher can't separate "sampling failed" from "attestation failed".
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
		// No DA attestation has EVER completed yet (fresh chain, or
		// Publisher lost all state) -- report the maximal gap, matching
		// btcGapMetric's own fail-safe convention (return
		// SovereignThreshold on an unreachable/unconfirmed source), not
		// hCurrent-0. The latter reads as a SMALL, healthy gap at low
		// heights even though zero real Celestia activity has happened,
		// which forced IsDAHealthy=true before Publisher.MaybePublish had
		// completed even one submit+confirm cycle -- da.VerifyReceipt then
		// required a real Attestation that couldn't exist yet, deadlocking
		// every proposal at genesis bootstrap (reproduced live on a fresh
		// 4-node deploy). DA health absent any real attestation must fail
		// toward degraded, not toward "coincidentally low height".
		return k.Params.DAThreshold, failed, failed, nil
	}
	if hCurrent <= hVerified {
		return 0, failed, failed, nil
	}
	return hCurrent - hVerified, failed, failed, nil
}

// btcGapMetric computes btc_gap. Without a live Source it uses
// btc.GetMetric's static SetGap; with one it uses h_btc_anchored, written
// each block from the committed proposal's btc_receipt
// (spec/core/EngramServer.tla:148) -- not the abstract
// min(H_submitted, H_anchored) (EngramFSM.tla:95), which would collapse to
// ~h_btc_current since h_btc_submitted is 0 outside a re-anchoring cycle
// (EngramServer.tla:151-159).
//
// spvFailed re-verifies the agreed h_btc_anchored against our own bitcoind
// (VerifyAnchor's OP_RETURN scan, mirroring is_btc_spv_failed,
// EngramFSM.tla:184) so a forged/reorged anchor isn't trusted until btc_gap
// crosses SOVEREIGN_THRESHOLD.
//
// hCurrent is this validator's own live reading of the current BTC height,
// returned (not committed here) so RefreshMetrics can carry it into
// PeripheralMetrics.BtcCurrentHeight for quorum certification -- see that
// field's doc for why writing it directly to committed state from here
// (the pre-vote-extension design) silently stopped working.
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

