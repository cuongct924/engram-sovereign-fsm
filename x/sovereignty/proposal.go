package sovereignty

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"strings"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cuongct220020/engram-sovereign-fsm/x/anchor"
	"github.com/cuongct220020/engram-sovereign-fsm/x/da"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ExtendedProposal mirrors the extended Proposal fields in
// spec/core/EngramTendermint.tla:143-150 (fsm_state, da_receipt, btc_receipt,
// zk_proof_ref) beyond the raw tx value. JSON-encoded and placed as Txs[0]
// by PrepareProposal below.
//
// Simplification: this repo does not yet wire ABCI++ Vote Extensions
// (the more idiomatic mechanism for consensus-level metadata); a leading
// pseudo-tx avoids needing a full TxConfig/signing pipeline for
// leader-computed system data.
type ExtendedProposal struct {
	FSMState string `json:"fsm_state"`
	// Healthy is IsHealthyCondition at proposal time (predicates.go),
	// agreed via consensus like FSMState itself -- CommitFSMTransition needs
	// it to tell a genuine "stayed ANCHORED/RECOVERING because healthy"
	// from a "stayed because down-hysteresis absorbed non-critical noise"
	// (both produce the same currState->targetState pair), and per
	// CLAUDE.md's "never write live local sensor reads into committed
	// state" rule, the committer cannot recompute this from its own local
	// sensors -- it must come from the already-agreed proposal, exactly
	// like FSMState. See NewProcessProposalHandler's check #1b, which
	// validates this the same way check #1 validates FSMState.
	Healthy    bool           `json:"healthy"`
	DAReceipt  da.Receipt     `json:"da_receipt"`
	BTCReceipt anchor.Receipt `json:"btc_receipt"`
	// ZKProofRef carries rt_new (the accepted re-anchoring proof's new state
	// root, keeper.LastAnchoredRoot) when the leader claims a proof backs
	// this round, nil otherwise -- a refinement of the abstract spec's
	// zk_proof_ref: BOOLEAN (spec/core/EngramTendermint.tla:150). Every site
	// that reads the abstract field only tests presence, never identity, so
	// this is a sound over-abstraction (spec/README.md's Sec 6.1). Carrying
	// the real hash (not a bare bool) lets a later query trace exactly which
	// proof backed a given transition.
	ZKProofRef []byte `json:"zk_proof_ref"`
}

const extendedProposalMarker = "ENGRAM_EXTENDED_PROPOSAL_V1|"

func EncodeExtendedProposal(p ExtendedProposal) ([]byte, error) {
	body, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	return append([]byte(extendedProposalMarker), body...), nil
}

// DecodeExtendedProposal returns ok=false (no error) if tx isn't a
// marker-prefixed extended proposal -- callers should treat that as "not
// present", not a decode failure.
func DecodeExtendedProposal(tx []byte) (p ExtendedProposal, ok bool, err error) {
	marker := []byte(extendedProposalMarker)
	if len(tx) < len(marker) || !bytes.Equal(tx[:len(marker)], marker) {
		return ExtendedProposal{}, false, nil
	}
	if err := json.Unmarshal(tx[len(marker):], &p); err != nil {
		return ExtendedProposal{}, true, err
	}
	return p, true, nil
}

// currentFSMInput reads the keeper's current sensor/counter state into a
// keeper.FSMInput, defaulting to FSMInit's values
// (spec/core/EngramFSM.tla:143-165) when nothing has been persisted yet.
func currentFSMInput(ctx sdk.Context, k *keeper.Keeper) (currState string, in keeper.FSMInput) {
	metrics, err := k.Metrics.Get(ctx)
	if err != nil {
		metrics = &types.PeripheralMetrics{}
	}
	currState, err = k.FSMState.Get(ctx)
	if err != nil {
		currState = types.StateAnchored
	}
	safeBlocks, _ := k.SafeBlocks.Get(ctx)
	suspiciousDuration, _ := k.SuspiciousDuration.Get(ctx)
	suspiciousSafeBlocks, _ := k.SuspiciousSafeBlocks.Get(ctx)
	unhealthyStreak, _ := k.UnhealthyStreak.Get(ctx)
	failedRecoveryAttempts, _ := k.FailedRecoveryAttempts.Get(ctx)
	proofValid, _ := k.ReanchoringProofValid.Get(ctx)

	return currState, keeper.FSMInput{
		Metrics:                metrics,
		SafeBlocks:             safeBlocks,
		SuspiciousDuration:     suspiciousDuration,
		SuspiciousSafeBlocks:   suspiciousSafeBlocks,
		UnhealthyStreak:        unhealthyStreak,
		FailedRecoveryAttempts: failedRecoveryAttempts,
		ReanchoringProofValid:  proofValid,
	}
}

// byzantineFakeFSMStatePrefix / byzantineForgeBTCHash / byzantineFalseDA /
// byzantineCensorTxPrefix are ENGRAM_BYZANTINE_BEHAVIOR's recognized values
// (cmd/engramd/main.go): controlled misbehavior for docs/EXPERIMENT.md's E8
// A3/A4/A6/A7 rows, exercising the real ProcessProposal rejection path on
// honest validators. Only docker/engram-validator-node04-byzantine.yml ever
// sets this env var.
const (
	byzantineFakeFSMStatePrefix = "fake_fsm_state:"
	byzantineForgeBTCHash       = "forge_btc_hash"
	byzantineFalseDA            = "false_da_attestation"
	byzantineCensorTxPrefix     = "censor_tx:"
)

// applyByzantineBehavior mutates ext (and, for censor_tx, txs) per behavior.
// Only called from the leader path (NewPrepareProposalHandler), matching a
// real malicious proposer's actual capability: it can only lie about its
// own proposal, never rewrite what other validators independently compute.
func applyByzantineBehavior(behavior string, ext *ExtendedProposal, txs [][]byte) [][]byte {
	switch {
	case strings.HasPrefix(behavior, byzantineFakeFSMStatePrefix):
		// A6: claim a healthier state than local sensors actually support.
		ext.FSMState = strings.TrimPrefix(behavior, byzantineFakeFSMStatePrefix)
	case behavior == byzantineForgeBTCHash:
		// A4: claim the checkpoint advanced with a hash that doesn't match
		// ExpectedBlockHash -- every honest validator's own bitcoind check
		// (checks #3/#3b below) must reject this independently.
		ext.BTCReceipt.CheckpointBlockHash = anchor.BlockHash{
			Tag: "FORGED", Height: ext.BTCReceipt.CheckpointBlockHeight,
		}
	case behavior == byzantineFalseDA:
		// A3: claim DA attestation succeeded without a real submission backing it.
		ext.DAReceipt.Attestation = true
		ext.DAReceipt.PublishedBlockHeight++
	case strings.HasPrefix(behavior, byzantineCensorTxPrefix):
		// A7: omit one specific tx despite it being in the mempool/
		// ForcedTxQueue, exercising IsCensoring against a real adversary.
		// Target is hex-encoded since a real forced tx's raw bytes aren't
		// guaranteed valid UTF-8 and don't survive Compose env/YAML
		// interpolation unscathed.
		targetHex := strings.TrimPrefix(behavior, byzantineCensorTxPrefix)
		target, err := hex.DecodeString(targetHex)
		if err != nil {
			return txs
		}
		filtered := txs[:0:0]
		for _, tx := range txs {
			if bytes.Equal(tx, target) {
				continue
			}
			filtered = append(filtered, tx)
		}
		return filtered
	}
	return txs
}

// NewPrepareProposalHandler builds the sdk.PrepareProposalHandler for this
// module, mirroring ServerInsertProposal (spec/core/EngramServer.tla:52-102):
// refreshes k.Metrics from live sensors, computes target_state via
// CalculateNextState, builds da_receipt/btc_receipt from tracked heights,
// and only attempts zk_proof_ref once hysteresis is satisfied.
//
// byzantineBehavior is ENGRAM_BYZANTINE_BEHAVIOR, empty on every real
// validator (see applyByzantineBehavior's doc).
func NewPrepareProposalHandler(k *keeper.Keeper, s *Sensors, byzantineBehavior string) sdk.PrepareProposalHandler {
	return func(ctx sdk.Context, req *abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error) {
		if err := RefreshMetrics(ctx, k, s); err != nil {
			return nil, err
		}
		currState, in := currentFSMInput(ctx, k)
		targetState := keeper.CalculateNextState(currState, in, k.Params)

		hEngramVerified, _ := k.HEngramVerified.Get(ctx)
		hBtcAnchored, _ := k.HBtcAnchored.Get(ctx)
		// The only place h_btc_anchored can advance: PreBlocker just
		// re-persists whatever height lands in the winning proposal's
		// btc_receipt, so without adopting AnchorTracker's own confirmed
		// height here, it can never move forward (sensors_refresh.go's
		// btcGapMetric doc).
		if s != nil && s.Anchor != nil {
			if confirmed, ok := s.Anchor.ConfirmedAnchorHeight(); ok && confirmed > hBtcAnchored {
				hBtcAnchored = confirmed
			}
		}
		// Same fix, same reason, for DA (sensors_refresh.go's daGapMetric doc).
		if s != nil && s.DAPublisher != nil {
			if verified, ok := s.DAPublisher.VerifiedHeight(); ok && verified > hEngramVerified {
				hEngramVerified = verified
			}
		}

		daReceipt := da.Receipt{
			PublishedBlockHeight: hEngramVerified,
			// Attestation reflects whether PublishedBlockHeight was EVER
			// genuinely confirmed retrievable -- a historical fact that,
			// once true, stays true (DA availability, once DAS-confirmed,
			// doesn't retroactively un-confirm). NOT in.Metrics.IsAttestationFailed
			// (a live, momentary health probe): using the live flag here
			// made VerifyReceipt's `!receipt.Attestation -> reject` fire on
			// the very first degraded block, regardless of how fresh
			// PublishedBlockHeight still was -- defeating DATolerance's
			// freshness window (spec/core/EngramTendermint.tla:290-294),
			// which exists precisely to let a still-fresh-enough prior
			// attestation carry a proposal through a brief outage.
			Attestation: hEngramVerified > 0,
		}
		btcReceipt := anchor.Receipt{
			CheckpointBlockHeight: hBtcAnchored,
			CheckpointBlockHash:   anchor.ExpectedBlockHash(hBtcAnchored),
		}

		// zk_proof_ref mirrors ServerInsertProposal's proof_search_space
		// (EngramServer.tla:73-76): only claimed once RECOVERING and
		// safe_blocks == HysteresisWait, and only once VerifyZkProof's
		// conditions (EngramTendermint.tla:257-260) are actually satisfiable.
		// The claimed value is this validator's own LastAnchoredRoot.
		var zkProofRef []byte
		if targetState == types.StateRecovering && in.SafeBlocks == k.Params.HysteresisWait &&
			daReceipt.Attestation && daReceipt.PublishedBlockHeight > hEngramVerified {
			if root, rerr := k.LastAnchoredRoot.Get(ctx); rerr == nil {
				zkProofRef = root
			}
		}

		ext := ExtendedProposal{
			FSMState:   targetState,
			Healthy:    types.IsHealthyCondition(in.Metrics, k.Params),
			DAReceipt:  daReceipt,
			BTCReceipt: btcReceipt,
			ZKProofRef: zkProofRef,
		}
		innerTxs := req.Txs
		// Filter withdrawal-marked txs out of our own proposal before
		// ProcessProposal's check #4 ever sees them -- otherwise a locked
		// withdrawal tx sitting in mempool gets re-proposed and rejected by
		// every leader forever (a permanent round-skip deadlock). Check #4
		// stays as defense-in-depth against a byzantine leader.
		if types.WithdrawLocked(targetState) {
			filtered := innerTxs[:0:0]
			for _, tx := range innerTxs {
				if containsWithdrawal(tx) {
					continue
				}
				filtered = append(filtered, tx)
			}
			innerTxs = filtered
		}
		if byzantineBehavior != "" {
			innerTxs = applyByzantineBehavior(byzantineBehavior, &ext, innerTxs)
		}

		encoded, err := EncodeExtendedProposal(ext)
		if err != nil {
			return nil, err
		}

		txs := make([][]byte, 0, len(innerTxs)+1)
		txs = append(txs, encoded)
		txs = append(txs, innerTxs...)
		return &abci.ResponsePrepareProposal{Txs: txs}, nil
	}
}

// verifyZkProofFlag ports VerifyZkProof (spec/core/EngramTendermint.tla:257-260).
// zkProofRef's presence (non-nil), not its value, is safety-relevant here,
// matching the abstract spec's BOOLEAN check.
func verifyZkProofFlag(zkProofRef []byte, receipt da.Receipt, hEngramVerified uint64) bool {
	return len(zkProofRef) > 0 && receipt.Attestation && receipt.PublishedBlockHeight > hEngramVerified
}

// containsWithdrawal is a placeholder for ContainsWithdrawal
// (spec/core/EngramTendermint.tla:253), which treats tx values as an opaque
// enum with a distinguished TX_WITHDRAWAL member. Real Msg-based
// classification belongs to app/ante.go once a TxDecoder is wired here.
func containsWithdrawal(tx []byte) bool {
	return bytes.Contains(tx, []byte("TX_WITHDRAWAL"))
}

// NewProcessProposalHandler builds the sdk.ProcessProposalHandler for this
// module, porting IsValidProposal (spec/core/EngramTendermint.tla:281-307)
// branch-for-branch. Refreshes k.Metrics from this validator's own live
// sensors before computing expectedState -- never the leader's readings,
// matching "sensors propose, consensus decides."
func NewProcessProposalHandler(k *keeper.Keeper, s *Sensors) sdk.ProcessProposalHandler {
	reject := &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_REJECT}
	accept := &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_ACCEPT}

	return func(ctx sdk.Context, req *abci.RequestProcessProposal) (*abci.ResponseProcessProposal, error) {
		if len(req.Txs) == 0 {
			return reject, nil
		}
		ext, ok, err := DecodeExtendedProposal(req.Txs[0])
		if err != nil || !ok {
			return reject, nil
		}
		if err := RefreshMetrics(ctx, k, s); err != nil {
			return nil, err
		}

		// 0. Censorship check (IsCensoring, EngramTendermint.tla:310-315),
		// evaluated before IsValidProposal per UponProposalInPropose's
		// IF/ELSE order (EngramTendermint.tla:579-590).
		//
		// Gap vs. spec: the spec's censoring branch also forces an immediate
		// round advance (StartRound(p, r+1)); vanilla ABCI 2.0 gives
		// ProcessProposal no lever to shorten CometBFT's round timer, so
		// REJECT here only yields a nil prevote, not an immediate skip.
		forcedTxQueue, err := k.ForcedTxQueueSlice(ctx)
		if err != nil {
			return reject, nil
		}
		if len(forcedTxQueue) > 0 {
			ignoredRounds := k.IgnoredRoundsMap(ctx, forcedTxQueue)
			included := make(map[string]bool, len(req.Txs)-1)
			for _, tx := range req.Txs[1:] {
				included[string(tx)] = true
			}
			if types.IsCensoring(forcedTxQueue, ignoredRounds, included, k.Params.MaxIgnoreRounds) {
				return reject, nil
			}
		}

		currState, in := currentFSMInput(ctx, k)

		// 1. fsm_state cross-check (IsValidProposal:288).
		expectedState := keeper.CalculateNextState(currState, in, k.Params)
		if ext.FSMState != expectedState {
			return reject, nil
		}

		// 1b. Healthy cross-check -- this repo's concrete addition (no spec
		// line; TLA+'s CalculateNextFSMState/ExecuteFSMTransition read
		// IsHealthyCondition directly, since the spec has no wire-format
		// boundary to cross). Every honest validator recomputes
		// IsHealthyCondition independently here, same as expectedState above
		// -- CommitFSMTransition trusts ext.Healthy only because it's
		// cross-checked against every honest validator's own sensors first.
		if ext.Healthy != types.IsHealthyCondition(in.Metrics, k.Params) {
			return reject, nil
		}

		// 2. DA pipeline check (IsValidProposal:290-294). req.Round is a
		// fork-level addition to RequestProcessProposal (vanilla ABCI 2.0
		// doesn't expose consensus round) that DATolerance's round-based
		// tolerance widening needs -- load-bearing for the BTC check below,
		// where a K-deep-confirmed (inherently lagging) anchor makes
		// zero tolerance permanently unsatisfiable otherwise.
		round := uint64(req.Round)
		hEngramCurrent, _ := k.HEngramCurrent.Get(ctx)
		isDAHealthy := types.IsDAHealthy(in.Metrics, k.Params)
		if !da.VerifyReceipt(ext.DAReceipt, ext.FSMState, isDAHealthy, hEngramCurrent, k.Params.DAThreshold, round) {
			return reject, nil
		}

		// 3. Settlement monotonicity & BTC light-client hash check (IsValidProposal:296-298).
		hBtcCurrent, _ := k.HBtcCurrent.Get(ctx)
		hBtcAnchored, _ := k.HBtcAnchored.Get(ctx)
		if !anchor.VerifyReceipt(ext.BTCReceipt, hBtcCurrent, hBtcAnchored, round, k.Params.KDeepFinality) {
			return reject, nil
		}

		// 3b. Real anchor advance verification -- this repo's concrete
		// addition (no spec line; EngramConsensus.tla's CanElect/IsKDeep are
		// refinement-proof-only). If the leader claims h_btc_anchored moved
		// forward, independently confirm via our OWN bitcoind connection
		// rather than trusting the claim, matching "sensors propose,
		// consensus decides."
		if s != nil && s.Anchor != nil && ext.BTCReceipt.CheckpointBlockHeight > hBtcAnchored {
			verified, verr := s.Anchor.VerifyAnchor(ctx, ext.BTCReceipt.CheckpointBlockHeight)
			if verr != nil || !verified {
				return reject, nil
			}
		}

		// 4. Circuit breaker: no withdrawal while SOVEREIGN/RECOVERING (IsValidProposal:300-301).
		if types.WithdrawLocked(ext.FSMState) {
			for _, tx := range req.Txs[1:] {
				if containsWithdrawal(tx) {
					return reject, nil
				}
			}
		}

		// 5. Re-anchoring ZK-proof gating (IsValidProposal:304-307).
		hEngramVerified, _ := k.HEngramVerified.Get(ctx)
		if ext.FSMState == types.StateRecovering && in.SafeBlocks == k.Params.HysteresisWait {
			if !verifyZkProofFlag(ext.ZKProofRef, ext.DAReceipt, hEngramVerified) {
				return reject, nil
			}
		} else if len(ext.ZKProofRef) > 0 {
			return reject, nil
		}

		return accept, nil
	}
}
