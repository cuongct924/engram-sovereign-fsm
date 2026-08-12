package sovereignty

import (
	"github.com/cuongct220020/engram-sovereign-fsm/x/anchor"
	"github.com/cuongct220020/engram-sovereign-fsm/x/da"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper/sensors"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Sensors bundles the three per-node background sensors that RefreshMetrics
// snapshots into a PeripheralMetrics every block -- the real-ABCI-path
// counterpart of tests/e2e/harness.go's Advance.
//
// BTC/DA/P2P's Source, and Anchor/DAPublisher, can each be wired to a real
// observer (x/anchor, x/da, the CometBFT fork's lp2p.Switch) by
// cmd/engramd/main.go. Anchor nil means h_btc_anchored never advances past
// its last committed value (VerifyReceipt's monotonicity check eventually
// rejects every proposal without it); DAPublisher nil is the same for
// h_engram_verified. Fine for tests/fault-injection that don't wire a live
// Source either.
type Sensors struct {
	BTC         *sensors.BTCSensor
	DA          *sensors.DASensor
	P2P         *sensors.P2PSensor
	Anchor      *anchor.AnchorTracker
	DAPublisher *da.Publisher
}

// RefreshMetrics snapshots s's sensors and writes the result into k.Metrics,
// so the following FSM-state computation (currentFSMInput,
// CalculateNextState) sees this block's live readings rather than a stale
// prior value. Called by both PrepareProposal and ProcessProposal so each
// node's target_state is always driven by its own current readings, never a
// value another node reported ("sensors propose, consensus decides").
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
		// Idempotent per pending submission: only actually broadcasts a new
		// tx once the previous one has resolved, so calling this from both
		// PrepareProposal and ProcessProposal in the same block never
		// double-submits (see AnchorTracker.MaybeSubmit's doc).
		if err := s.Anchor.MaybeSubmit(ctx, uint64(ctx.BlockHeight())); err != nil {
			return err
		}
	}
	if s.DAPublisher != nil {
		// Idempotent per pending submission, same as Anchor.MaybeSubmit above --
		// calling this from both PrepareProposal and ProcessProposal in the same
		// block never double-submits (see da.Publisher.MaybePublish's doc).
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

// daGapMetric computes da_gap (spec/core/EngramFSM.tla:87:
// h_engram_current - h_engram_verified) plus is_das_failed/
// is_attestation_failed. With no live Source wired, mirrors DASensor's
// static SetAvailable/SetFailureFlags reading. With a Source (da.Publisher),
// computes the real gap against this validator's own current chain height,
// persisted to k.HEngramCurrent (mirrors btcGapMetric's k.HBtcCurrent).
//
// Simplification: Source.Failed() stands in for both is_das_failed and
// is_attestation_failed -- da.Publisher doesn't distinguish "sampling
// failed" from "attestation failed" the way the abstract spec's two
// booleans do.
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

	// failed combines Failed()'s slower, stateful submission-bookkeeping
	// signal with ProbeHealthy's fresh, stateless reachability check --
	// Failed() alone can lag an in-flight background Submit by up to its
	// own timeout, during which different validators observe different
	// stale values and disagree on Healthy (see ProbeHealthy's doc).
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

// btcGapMetric computes btc_gap. With no live Source wired, it's exactly
// btc.GetMetric's static SetGap value. With a Source, it applies the
// CONCRETE layer's formula, not the abstract one: ServerUponProposalInPrecommitNoDecision
// Step 3 (spec/core/EngramServer.tla:148) writes h_btc_anchored' from the
// committed proposal's btc_receipt every block, independent of FSM state --
// so h_btc_anchored alone is the live anchor baseline here, unlike the
// abstract btc_gap = H_current - min(H_submitted, H_anchored)
// (spec/core/EngramFSM.tla:95). That min() only resolves to h_btc_anchored
// abstractly because BTCNormalUpdate/BTCSPVFailure keep the two in lockstep
// (EngramFSM.tla:173-188); concretely h_btc_submitted is written only by
// Step 4's RECOVERING+zk_proof_ref path (EngramServer.tla:151-159), so
// outside a re-anchoring cycle it's always 0 -- applying min() here would
// collapse btc_gap to ~h_btc_current regardless of the real anchor state.
// h_btc_submitted's actual role is refreshReanchoringProofValid, below.
//
// spvFailed independently re-verifies the agreed h_btc_anchored (committed
// from the leader's btc_receipt via CommitFSMTransition, never locally
// re-derived -- see preblock.go) against our OWN bitcoind via
// AnchorTracker.VerifyAnchor's OP_RETURN scan, mirroring is_btc_spv_failed
// (spec/core/EngramFSM.tla:184: "OP_RETURN inclusion check & Block header
// verification failure flag"). Without this, a forged or since-reorged
// h_btc_anchored is trusted until btc_gap organically grows past
// SOVEREIGN_THRESHOLD -- a multi-block window where withdrawals stay
// unlocked against an unverifiable anchor.
func btcGapMetric(ctx sdk.Context, k *keeper.Keeper, btc *sensors.BTCSensor, anchorTracker *anchor.AnchorTracker) (gap uint64, spvFailed bool, err error) {
	src := btc.Source()
	if src == nil {
		gap, err = btc.GetMetric(ctx)
		return gap, false, err
	}

	hCurrent, err := src.CurrentHeight(ctx)
	if err != nil {
		// A bitcoind outage degrades through btc_gap rather than failing
		// PrepareProposal/ProcessProposal outright: report the maximum gap
		// (matching daGapMetric's unhealthy-DASensor fallback), since zero
		// visibility into Bitcoin is at least as severe as any gap this
		// threshold catches, and unlike freezing at a stale value, doesn't
		// silently look healthy.
		return k.Params.SovereignThreshold, false, nil
	}
	if err := k.HBtcCurrent.Set(ctx, hCurrent); err != nil {
		return 0, false, err
	}

	hAnchored, _ := k.HBtcAnchored.Get(ctx)
	if anchorTracker != nil && hAnchored > 0 {
		// A verification-RPC error is treated as failed, not swallowed like
		// MaybeSubmit's own RPC errors above: silently trusting an
		// unverifiable anchor is exactly the exploitation window this check
		// exists to close, unlike MaybeSubmit's own submission retries.
		ok, verr := anchorTracker.VerifyAnchor(ctx, hAnchored)
		spvFailed = verr != nil || !ok
	}

	if hCurrent <= hAnchored {
		return 0, spvFailed, nil
	}
	return hCurrent - hAnchored, spvFailed, nil
}

// refreshReanchoringProofValid ports UpdateSensors' reanchoring_proof_valid
// (spec/core/EngramFSM.tla:294-301) -- lives here, not preblock.go's commit
// path, since the concrete layer (EngramServer.tla:151-159) never flips it
// back to TRUE itself.
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
