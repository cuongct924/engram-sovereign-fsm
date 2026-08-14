package sovereignty

import (
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

	btcGap, btcSpvFailed, err := btcGapMetric(ctx, k, s.BTC, s.Anchor)
	if err != nil {
		return err
	}
	if err := refreshReanchoringProofValid(ctx, k); err != nil {
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

	metrics := &types.PeripheralMetrics{
		BtcGap:              btcGap,
		IsBtcSpvFailed:      btcSpvFailed,
		DaGap:               daGap,
		IsDasFailed:         dasFailed,
		IsAttestationFailed: attestationFailed,
		SubnetDiversity:     p2pSnap.SubnetDiversity,
		ActiveAnchors:       p2pSnap.ActiveAnchors,
		CleanPeers:          p2pSnap.CleanPeers,
		PeerChurnRate:       p2pSnap.ChurnRate,
		AvgPeerTenure:       p2pSnap.AvgTenure,
		PeerLatency:         p2pSnap.Latency,
	}
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
		hVerified = 0 // nothing confirmed yet -- full gap since genesis
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
func btcGapMetric(ctx sdk.Context, k *keeper.Keeper, btc *sensors.BTCSensor, anchorTracker *anchor.AnchorTracker) (gap uint64, spvFailed bool, err error) {
	src := btc.Source()
	if src == nil {
		gap, err = btc.GetMetric(ctx)
		return gap, false, err
	}

	// Reachable first: a stale pooled connection can succeed on one validator
	// and fail on another at the same instant, disagreeing on btc_gap/Healthy
	// while bitcoind is uniformly down. The fresh, stateless TCP check keeps
	// the down/up determination consistent across validators.
	if !src.Reachable(ctx) {
		return k.Params.SovereignThreshold, false, nil
	}

	hCurrent, err := src.CurrentHeight(ctx)
	if err != nil {
		// An outage degrades through btc_gap, not a failed proposal: report
		// the maximum gap (matching daGapMetric's fallback) -- zero
		// visibility is at least as severe as any gap this threshold catches.
		return k.Params.SovereignThreshold, false, nil
	}
	if err := k.HBtcCurrent.Set(ctx, hCurrent); err != nil {
		return 0, false, err
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
		return 0, spvFailed, nil
	}
	return hCurrent - hAnchored, spvFailed, nil
}

// refreshReanchoringProofValid ports UpdateSensors' reanchoring_proof_valid
// (spec/core/EngramFSM.tla:294-301). Lives here, not preblock.go, because
// the concrete layer (EngramServer.tla:151-159) never flips it back TRUE.
func refreshReanchoringProofValid(ctx sdk.Context, k *keeper.Keeper) error {
	currState, err := k.FSMState.Get(ctx)
	if err != nil {
		currState = types.StateAnchored
	}
	hAnchored, _ := k.HBtcAnchored.Get(ctx)
	hSubmitted, _ := k.HBtcSubmitted.Get(ctx)
	realProofSubmittedHeight, _ := k.RealProofSubmittedHeight.Get(ctx)

	realProofValid := false
	if realProofSubmittedHeight > 0 {
		if tipHeight, _, err := k.LatestTrackedHeader(ctx); err == nil && tipHeight-realProofSubmittedHeight <= k.Params.MaxUnprovenTailBlocks {
			realProofValid = true
		}
	}

	heuristicValid := hAnchored >= hSubmitted && hSubmitted > 0
	valid := currState == types.StateRecovering && (heuristicValid || realProofValid)
	return k.ReanchoringProofValid.Set(ctx, valid)
}
