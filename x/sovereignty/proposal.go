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

// ExtendedProposal mirrors proposal fields in EngramTendermint.tla:143-150
// (fsm_state, da_receipt, btc_receipt, zk_proof_ref). Encoded as JSON and
// prepended as the first pseudo-transaction (Txs[0]) to bypass ABCI++
// Vote Extension wiring complexity.
type ExtendedProposal struct {
	FSMState string `json:"fsm_state"`
	// Healthy is IsHealthyCondition at proposal time -- committed consensus on
	// this field lets CommitFSMTransition distinguish health from hysteresis
	// without reading local sensors (violating "sensors propose, consensus
	// decides").
	Healthy    bool           `json:"healthy"`
	DAReceipt  da.Receipt     `json:"da_receipt"`
	BTCReceipt anchor.Receipt `json:"btc_receipt"`
	// ZKProofRef carries rt_new (keeper.LastAnchoredRoot) when a recovery proof
	// is present, nil otherwise -- refines the abstract BOOLEAN zk_proof_ref
	// (EngramTendermint.tla:150) to trace which proof backed a transition.
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
// A3/A4/A6/A7 rows, exercising the real ProcessProposal rejection path.
// Only docker/engram-node04-byzantine.yml ever sets this env var.
const (
	byzantineFakeFSMStatePrefix = "fake_fsm_state:"
	byzantineForgeBTCHash       = "forge_btc_hash"
	byzantineFalseDA            = "false_da_attestation"
	byzantineCensorTxPrefix     = "censor_tx:"
)

// applyByzantineBehavior mutates ext (and, for censor_tx, txs) per behavior.
// Only called from the leader path (NewPrepareProposalHandler), matching a
// real malicious proposer's capability: it can only lie about its own
// proposal, never rewrite what other validators independently compute.
func applyByzantineBehavior(behavior string, ext *ExtendedProposal, txs [][]byte) [][]byte {
	switch {
	case strings.HasPrefix(behavior, byzantineFakeFSMStatePrefix):
		// A6: claim a healthier state than local sensors actually support.
		ext.FSMState = strings.TrimPrefix(behavior, byzantineFakeFSMStatePrefix)
	case behavior == byzantineForgeBTCHash:
		// A4: claim the checkpoint advanced with a hash that doesn't match
		// ExpectedBlockHash -- every honest validator's own bitcoind check
		// (checks #3/#3b) must reject this independently.
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
		// Target is hex-encoded since raw bytes aren't guaranteed valid UTF-8
		// and don't survive Compose env/YAML interpolation unscathed.
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

// NewPrepareProposalHandler builds the sdk.PrepareProposalHandler, mirroring
// ServerInsertProposal (spec/core/EngramServer.tla:52-102): refreshes
// k.Metrics from live sensors, computes target_state via CalculateNextState,
// builds da_receipt/btc_receipt from tracked heights, and only attempts
// zk_proof_ref once hysteresis is satisfied.
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
		// Advance h_btc_anchored via ConfirmedAnchorHeight, capped at
		// hBtcCurrent to enforce checkpoint_block_height <= h_btc_current
		// (EngramTendermint.tla:271-275) -- prevents proposer deadlocks when a
		// local confirmed cache exceeds the committed, frozen hBtcCurrent.
		if s != nil && s.Anchor != nil {
			hBtcCurrent, _ := k.HBtcCurrent.Get(ctx)
			if confirmed, ok := s.Anchor.ConfirmedAnchorHeight(); ok && confirmed > hBtcAnchored && confirmed <= hBtcCurrent {
				hBtcAnchored = confirmed
			}
		}
		// Preserve the start-of-round value for the zk_proof_ref freshness check
		// before applying any live updates from s.DAPublisher below.
		hEngramVerifiedPrev := hEngramVerified
		if s != nil && s.DAPublisher != nil {
			if verified, ok := s.DAPublisher.VerifiedHeight(); ok && verified > hEngramVerified {
				hEngramVerified = verified
			}
		}

		daReceipt := da.Receipt{
			PublishedBlockHeight: hEngramVerified,
			// Attestation represents historical retrievability (PublishedBlockHeight > 0).
			// We do not use the transient in.Metrics.IsAttestationFailed probe here
			// to preserve DATolerance's freshness window (EngramTendermint.tla:290-294).
			Attestation: hEngramVerified > 0,
		}
		btcReceipt := anchor.Receipt{
			CheckpointBlockHeight: hBtcAnchored,
			CheckpointBlockHash:   anchor.ExpectedBlockHash(hBtcAnchored),
		}

		// Attach zk_proof_ref (EngramServer.tla:73-76) once in RECOVERING,
		// hysteresis is satisfied, and verification conditions are met.
		var zkProofRef []byte
		if targetState == types.StateRecovering && in.SafeBlocks == k.Params.HysteresisWait &&
			daReceipt.Attestation && daReceipt.PublishedBlockHeight > hEngramVerifiedPrev {
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
		// Filter withdrawal transactions early to avoid proposing rejected TXs
		// and deadlocking rounds when withdrawals are locked (EngramTendermint.tla:300-301).
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

// NewProcessProposalHandler builds ProcessProposalHandler, porting IsValidProposal
// (EngramTendermint.tla:281-307). Refreshes metrics locally to enforce
// "sensors propose, consensus decides."
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

		// 0. Censorship check (IsCensoring, EngramTendermint.tla:310-315/579-590).
		// Note: ABCI 2.0 cannot force immediate round advance; REJECT yields a nil prevote.
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

		// 1b. Healthy cross-check. Independent recomputation of health status
		// to safely validate ext.Healthy before CommitFSMTransition.
		if ext.Healthy != types.IsHealthyCondition(in.Metrics, k.Params) {
			return reject, nil
		}

		// 2. DA pipeline check (IsValidProposal:290-294), utilizing req.Round
		// for round-based tolerance widening.
		round := uint64(req.Round)
		hEngramCurrent, _ := k.HEngramCurrent.Get(ctx)
		isDAHealthy := types.IsDAHealthy(in.Metrics, k.Params)
		if !da.VerifyReceipt(ext.DAReceipt, ext.FSMState, isDAHealthy, hEngramCurrent, k.Params.DAThreshold, round) {
			return reject, nil
		}

		// 3. Settlement monotonicity & BTC check (IsValidProposal:296-312).
		hBtcCurrent, _ := k.HBtcCurrent.Get(ctx)
		hBtcAnchored, _ := k.HBtcAnchored.Get(ctx)
		isBTCHealthy := types.IsBTCHealthy(in.Metrics, k.Params)
		if !anchor.VerifyReceipt(ext.BTCReceipt, ext.FSMState, isBTCHealthy, hBtcCurrent, hBtcAnchored, round, k.Params.KDeepFinality) {
			return reject, nil
		}

		// 3b. Local anchor verification to independently confirm leader's
		// h_btc_anchored claim via bitcoind before accepting.
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
