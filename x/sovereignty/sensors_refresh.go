package sovereignty

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/cuongct220020/engram-sovereign-fsm/x/da"
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
// BTC, DA and P2P's Source can all be wired to real observers: BTC to
// vigilante.RPCClient's real bitcoind getblockcount (x/vigilante/rpc.go,
// Phase 7), DA to da.Publisher's real celestia-bridge blob.Submit/GetAll
// round-trip (x/da/rpc.go+publisher.go, Phase 7), P2P to the CometBFT fork's
// live lp2p.Switch.PeerHealthSnapshot() (cmd/engramd/main.go, Phase 7).
//
// Anchor, when set, is this node's own vigilante.AnchorTracker -- the real
// checkpoint-submission-and-confirmation pipeline that gives h_btc_anchored
// somewhere to actually come from (see btcGapMetric's doc for the liveness
// bug this closes: without it, h_btc_anchored never advances while
// h_btc_current does, so VerifyReceipt's monotonicity check eventually
// rejects every proposal). nil means h_btc_anchored stays static, matching
// pre-Phase-7 behavior (fine for tests/fault-injection that don't wire BTC's
// live Source either).
//
// DAPublisher, when set, is this node's own da.Publisher -- the same class
// of fix for h_engram_verified, which had the identical liveness bug (see
// daGapMetric's doc). nil means h_engram_verified stays static, matching
// pre-Phase-7 behavior.
type Sensors struct {
	BTC         *sensors.BTCSensor
	DA          *sensors.DASensor
	P2P         *sensors.P2PSensor
	Anchor      *vigilante.AnchorTracker
	DAPublisher *da.Publisher
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
		DaGap:                daGap,
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
// h_engram_current - h_engram_verified) and the is_das_failed/
// is_attestation_failed flags. With no live Source wired, mirrors
// DASensor's static SetAvailable/SetFailureFlags-injected reading
// (fault-injection tests, unchanged -- and now actually reads the failure
// flags DASensor tracks, previously silently dropped as a hardcoded false
// regardless of what SetFailureFlags had set). With a live Source
// (da.Publisher, Phase 7), computes the real gap from the Source's live
// VerifiedHeight reading against this validator's own current chain height,
// also persisting the latter to k.HEngramCurrent (mirrors btcGapMetric's
// k.HBtcCurrent persistence). Source.Failed() stands in for BOTH
// is_das_failed and is_attestation_failed -- da.Publisher does not
// distinguish "sampling failed" from "attestation failed" the way the
// abstract spec's two separate booleans do, a documented simplification
// (same class as vigilante.AnchorTracker's collapsed checkpoint content).
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

	failed := src.Failed()
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
// btc.GetMetric's static SetGap-injected value (fault-injection tests,
// unchanged). With a live Source (vigilante.RPCClient, Phase 7), it applies
// the CONCRETE layer's formula, not the abstract one: ServerUponProposalInPrecommitNoDecision
// Step 3 (spec/core/EngramServer.tla:148) writes h_btc_anchored' directly
// from the committed proposal's btc_receipt on every block, independent of
// FSM state, so h_btc_anchored alone is already the authoritative, live
// anchor baseline here -- unlike EngramFSM.tla's abstract btc_gap
// (spec/core/EngramFSM.tla:95, Δ_BTC = H_current - min(H_submitted,
// H_anchored)), whose min() only ever resolves to h_btc_anchored anyway
// (BTCNormalUpdate/BTCSPVFailure, EngramFSM.tla:173-188, always keep
// h_btc_anchored <= h_btc_submitted in lockstep). Applying that min() here
// instead breaks: h_btc_submitted is updated ONLY by Step 4's RECOVERING +
// zk_proof_ref path (EngramServer.tla:151-159), so outside a re-anchoring
// cycle it never leaves its zero value, collapsing min(0, h_btc_anchored) to
// 0 and inflating btc_gap to ~h_btc_current regardless of the real anchor
// state (confirmed live: this held the FSM in SOVEREIGN across an entire
// test session). h_btc_submitted is NOT unused -- see
// refreshReanchoringProofValid, its actual concrete-layer role.
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

	hAnchored, _ := k.HBtcAnchored.Get(ctx)
	if hCurrent <= hAnchored {
		return 0, nil
	}
	return hCurrent - hAnchored, nil
}

// refreshReanchoringProofValid ports UpdateSensors' reanchoring_proof_valid
// update (spec/core/EngramFSM.tla:294-301): the ZK re-anchoring proof
// (submitted by Step 4 above while entering/staying RECOVERING) becomes
// valid once Bitcoin has confirmed an anchor at or past the submitted
// height. The concrete layer (EngramServer.tla) only ever writes FALSE to
// this at submission time or holds it UNCHANGED (Step 4,
// EngramServer.tla:151-159) -- nothing in the concrete hooks ever flips it
// back to TRUE, so this recomputation has to live on the sensor-refresh path
// (mirroring how the abstract UpdateSensors action recomputes it every
// environment tick) rather than in preblock.go's commit path. Previously
// missing entirely: ReanchoringProofValid was only ever set to false anywhere
// in this module, so CalculateNextState's RECOVERING -> ANCHORED transition
// (keeper/circuit_breaker.go) could never actually fire via a successful
// reanchoring proof.
//
// Two independent sources feed this, OR'd together: the BTC-anchor
// heuristic above (hAnchored >= hSubmitted), and RealProofSubmittedHeight,
// the latch set by keeper.MsgServerImpl.SubmitRecoveryProof once a REAL
// Noir/bb proof has verified (x/sovereignty/keeper/reanchor.go) AND its
// public inputs were cross-checked against on-chain tracked state -- both
// are legitimate ways to demonstrate the same underlying fact ("this
// recovery interval is provably safe to re-anchor"), so either is
// sufficient. Both only ever apply while currently RECOVERING.
//
// The real-proof latch must ALSO still be within Params.MaxUnprovenTailBlocks
// of the CURRENT tip height, not merely be nonzero: the SOVEREIGN/RECOVERING
// interval can keep growing (new headers appended) after a proof was
// submitted but before RECOVERING is reached, and a proof only covers
// headers up to the height it was built against -- confirmed by actually
// running the real prover pipeline end-to-end and appending a header after
// submitting a real proof. An EXACT match (tipHeight == realProofSubmittedHeight)
// was tried first and found live to be a genuine liveness bug: on a
// fast-producing testnet (block time well under one query+prove+submit
// round trip), the tip is a moving target a proof built from an earlier
// snapshot can only equal by coincidence -- confirmed live, dozens of real
// proofs landing successfully (checkpoint advancing every time) without
// ever once satisfying an exact match, RECOVERING never completing despite
// the network being genuinely healthy throughout. MaxUnprovenTailBlocks'
// own doc (types.Params) has the full reasoning for why a bounded gap is
// safe here, not a heuristic bypass. RealProofSubmittedHeight is reset to 0
// by preblock.go's CommitFSMTransition the moment RECOVERING is left (Step
// 5b), so a stale latch from a prior interval can never leak into a new
// one either.
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
