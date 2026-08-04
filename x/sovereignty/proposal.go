package sovereignty

import (
	"bytes"
	"encoding/json"

	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/cuongct220020/engram-sovereign-fsm/x/da"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
	"github.com/cuongct220020/engram-sovereign-fsm/x/vigilante"
)

// ExtendedProposal mirrors the extended Proposal fields in
// spec/core/EngramTendermint.tla:143-150 (fsm_state, da_receipt, btc_receipt,
// zk_proof_ref) beyond the raw tx value. It is JSON-encoded and placed as
// Txs[0] of the block by the PrepareProposal handler below.
//
// This repo does not yet wire full ABCI++ Vote Extensions
// (ExtendVote/VerifyVoteExtension) -- that would be the more idiomatic
// Cosmos SDK mechanism for consensus-level metadata like this. Using a
// leading pseudo-tx is a deliberate simplification that avoids needing a
// full TxConfig/signing pipeline for what is leader-computed system data,
// not a user transaction. Revisit if/when M0-series CometBFT fork work
// (see the Phase 5 plan) exposes vote extensions cleanly to this layer.
type ExtendedProposal struct {
	FSMState   string            `json:"fsm_state"`
	DAReceipt  da.Receipt        `json:"da_receipt"`
	BTCReceipt vigilante.Receipt `json:"btc_receipt"`
	ZKProofRef bool              `json:"zk_proof_ref"`
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
// marker-prefixed extended proposal at all -- callers should treat that as
// "not present", not as a decode failure.
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
// keeper.FSMInput, defaulting to FSMInit's values (spec/core/EngramFSM.tla:143-165)
// when nothing has been persisted yet.
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
	proofValid, _ := k.ReanchoringProofValid.Get(ctx)

	return currState, keeper.FSMInput{
		Metrics:               metrics,
		SafeBlocks:            safeBlocks,
		SuspiciousDuration:    suspiciousDuration,
		ReanchoringProofValid: proofValid,
	}
}

// NewPrepareProposalHandler builds the sdk.PrepareProposalHandler for this
// module, mirroring ServerInsertProposal (spec/core/EngramServer.tla:52-102):
// the leader computes target_state via CalculateNextState (reused verbatim
// from Phase 1, not reimplemented), builds da_receipt/btc_receipt from the
// tracked heights, and only attempts zk_proof_ref once hysteresis is
// satisfied. Wiring this onto a real BaseApp via SetPrepareProposal is M5's
// job -- this function only builds the handler.
func NewPrepareProposalHandler(k *keeper.Keeper) sdk.PrepareProposalHandler {
	return func(ctx sdk.Context, req *abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error) {
		currState, in := currentFSMInput(ctx, k)
		targetState := keeper.CalculateNextState(currState, in, k.Params)

		hEngramVerified, _ := k.HEngramVerified.Get(ctx)
		hBtcAnchored, _ := k.HBtcAnchored.Get(ctx)

		daReceipt := da.Receipt{
			PublishedBlockHeight: hEngramVerified,
			Attestation:          !in.Metrics.IsAttestationFailed,
		}
		btcReceipt := vigilante.Receipt{
			CheckpointBlockHeight: hBtcAnchored,
			CheckpointBlockHash:   vigilante.ExpectedBlockHash(hBtcAnchored),
		}

		// zk_proof_ref: mirrors ServerInsertProposal's proof_search_space
		// (EngramServer.tla:73-76), which is only ever non-trivially TRUE once
		// state=RECOVERING and safe_blocks has reached HysteresisWait. Unlike
		// the spec's nondeterministic choice, a concrete leader must decide
		// deterministically -- it claims a proof once the receipt backing it
		// (VerifyZkProof's conditions, EngramTendermint.tla:257-260) is
		// actually satisfiable, not before.
		zkProofRef := false
		if targetState == types.StateRecovering && in.SafeBlocks == k.Params.HysteresisWait {
			zkProofRef = daReceipt.Attestation && daReceipt.PublishedBlockHeight > hEngramVerified
		}

		encoded, err := EncodeExtendedProposal(ExtendedProposal{
			FSMState:   targetState,
			DAReceipt:  daReceipt,
			BTCReceipt: btcReceipt,
			ZKProofRef: zkProofRef,
		})
		if err != nil {
			return nil, err
		}

		txs := make([][]byte, 0, len(req.Txs)+1)
		txs = append(txs, encoded)
		txs = append(txs, req.Txs...)
		return &abci.ResponsePrepareProposal{Txs: txs}, nil
	}
}

// verifyZkProofFlag ports VerifyZkProof (spec/core/EngramTendermint.tla:257-260).
func verifyZkProofFlag(zkProofRef bool, receipt da.Receipt, hEngramVerified uint64) bool {
	return zkProofRef && receipt.Attestation && receipt.PublishedBlockHeight > hEngramVerified
}

// containsWithdrawal is a placeholder for ContainsWithdrawal (spec/core/EngramTendermint.tla:253):
// the spec treats tx values as an opaque ValidValues enum where "TX_WITHDRAWAL"
// is a distinguished member. Real tx classification (bank MsgSend, IBC
// MsgTransfer, ...) belongs to app/ante.go's isHighRiskTransaction once real
// Msg decoding is wired here -- this raw-byte marker check is a stand-in until
// M5 gives ProcessProposal access to a real TxDecoder.
func containsWithdrawal(tx []byte) bool {
	return bytes.Contains(tx, []byte("TX_WITHDRAWAL"))
}

// NewProcessProposalHandler builds the sdk.ProcessProposalHandler for this
// module, porting IsValidProposal (spec/core/EngramTendermint.tla:281-307)
// branch-for-branch against the ExtendedProposal decoded from Txs[0].
func NewProcessProposalHandler(k *keeper.Keeper) sdk.ProcessProposalHandler {
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

		// 0. Censorship check (IsCensoring, EngramTendermint.tla:310-315), evaluated
		// before IsValidProposal per UponProposalInPropose's IF/ELSE branch order
		// (EngramTendermint.tla:579-590): a proposal that still omits a forced tx
		// past its ignore-round threshold is rejected regardless of everything else.
		//
		// Gap vs. spec: the spec's censoring branch also forces an immediate round
		// advance (StartRound(p, r+1)) alongside the reject. Vanilla ABCI 2.0 gives
		// ProcessProposal no lever to shorten CometBFT's round timer -- REJECT here
		// only causes a nil prevote, so the round still advances on the existing
		// local timeout rather than immediately. Closing this gap needs the M0b
		// fork-level round-skip work (see CLAUDE.md's "Not yet done" section).
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

		// 1. fsm_state cross-check (IsValidProposal:288 -- prop.fsm_state = CalculateNextFSMState).
		expectedState := keeper.CalculateNextState(currState, in, k.Params)
		if ext.FSMState != expectedState {
			return reject, nil
		}

		// 2. DA pipeline check (IsValidProposal:290-294). Round-based tolerance
		// widening (DATolerance) is not applied here -- vanilla ABCI 2.0 does
		// not expose the consensus round to PrepareProposal/ProcessProposal, so
		// round=0 (no widening) is used, which is the strictest case and never
		// less safe than the spec's tolerance-widened acceptance.
		hEngramCurrent, _ := k.HEngramCurrent.Get(ctx)
		isDAHealthy := types.IsDAHealthy(in.Metrics, k.Params)
		if !da.VerifyReceipt(ext.DAReceipt, ext.FSMState, isDAHealthy, hEngramCurrent, k.Params.DAThreshold, 0) {
			return reject, nil
		}

		// 3. Settlement monotonicity & BTC light-client hash check (IsValidProposal:296-298).
		hBtcCurrent, _ := k.HBtcCurrent.Get(ctx)
		hBtcAnchored, _ := k.HBtcAnchored.Get(ctx)
		if !vigilante.VerifyReceipt(ext.BTCReceipt, hBtcCurrent, hBtcAnchored, 0) {
			return reject, nil
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
		} else if ext.ZKProofRef {
			return reject, nil
		}

		return accept, nil
	}
}
