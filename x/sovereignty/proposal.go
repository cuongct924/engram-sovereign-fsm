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
// first refreshes k.Metrics from this node's own live sensors (RefreshMetrics,
// Phase 7 -- previously missing, so target_state was always computed from
// stale keeper state), then the leader computes target_state via
// CalculateNextState (reused verbatim from Phase 1, not reimplemented),
// builds da_receipt/btc_receipt from the tracked heights, and only attempts
// zk_proof_ref once hysteresis is satisfied. Wiring this onto a real BaseApp
// via SetPrepareProposal is M5's job -- this function only builds the handler.
func NewPrepareProposalHandler(k *keeper.Keeper, s *Sensors) sdk.PrepareProposalHandler {
	return func(ctx sdk.Context, req *abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error) {
		if err := RefreshMetrics(ctx, k, s); err != nil {
			return nil, err
		}
		currState, in := currentFSMInput(ctx, k)
		targetState := keeper.CalculateNextState(currState, in, k.Params)

		hEngramVerified, _ := k.HEngramVerified.Get(ctx)
		hBtcAnchored, _ := k.HBtcAnchored.Get(ctx)
		// Adopt our own AnchorTracker's confirmed height if it has advanced
		// past what's already committed -- this is the ONLY place
		// h_btc_anchored gets a chance to move forward at all (PreBlocker's
		// CommitFSMTransition just re-persists whatever height ends up in
		// the winning proposal's btc_receipt); without this, hBtcAnchored
		// above is always just an echo of the last-committed value and can
		// never advance (see sensors_refresh.go's btcGapMetric doc for the
		// liveness bug this closes).
		if s != nil && s.Anchor != nil {
			if confirmed, ok := s.Anchor.ConfirmedAnchorHeight(); ok && confirmed > hBtcAnchored {
				hBtcAnchored = confirmed
			}
		}
		// Same liveness fix, same reason, for DA: without this, hEngramVerified
		// above is always just an echo of the last-committed value and can
		// never advance (see sensors_refresh.go's daGapMetric doc -- this is
		// the exact same bug class the BTC fix above closes).
		if s != nil && s.DAPublisher != nil {
			if verified, ok := s.DAPublisher.VerifiedHeight(); ok && verified > hEngramVerified {
				hEngramVerified = verified
			}
		}

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
// branch-for-branch against the ExtendedProposal decoded from Txs[0]. Like
// PrepareProposal, it refreshes k.Metrics from this node's own live sensors
// (RefreshMetrics, Phase 7) before computing expectedState -- each validator
// cross-checks the proposal against ITS OWN current readings, never the
// leader's, matching "sensors propose, consensus decides."
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
		// widening (DATolerance) now uses the real consensus round (Phase 7):
		// req.Round, a fork-level addition to RequestProcessProposal (vanilla
		// ABCI 2.0 does not expose this -- see the fork's
		// proto/tendermint/abci/types.proto and consensus/state.go's
		// CreateProposalBlock/ProcessProposal callers, which now thread
		// cs.Round through). Previously hardcoded to 0 (no widening, the
		// strictest case) -- harmless for DA (whose freshness window was
		// static anyway pre-Phase-7) but load-bearing for BTC below, where a
		// real, continuously-advancing chain height combined with a
		// K-deep-confirmed (hence inherently lagging) anchor made round=0's
		// zero tolerance permanently unsatisfiable (see
		// x/vigilante/verify.go's Tolerance doc).
		round := uint64(req.Round)
		hEngramCurrent, _ := k.HEngramCurrent.Get(ctx)
		isDAHealthy := types.IsDAHealthy(in.Metrics, k.Params)
		if !da.VerifyReceipt(ext.DAReceipt, ext.FSMState, isDAHealthy, hEngramCurrent, k.Params.DAThreshold, round) {
			return reject, nil
		}

		// 3. Settlement monotonicity & BTC light-client hash check (IsValidProposal:296-298).
		hBtcCurrent, _ := k.HBtcCurrent.Get(ctx)
		hBtcAnchored, _ := k.HBtcAnchored.Get(ctx)
		if !vigilante.VerifyReceipt(ext.BTCReceipt, hBtcCurrent, hBtcAnchored, round, k.Params.KDeepFinality) {
			return reject, nil
		}

		// 3b. Real anchor advance verification (Phase 7, no spec line -- this
		// repo's concrete addition, not present in the abstract model since
		// EngramConsensus.tla's CanElect/IsKDeep are refinement-proof-only,
		// never called by the concrete layer; see vigilante.VerifySPVProof's
		// doc). If the leader's btc_receipt claims h_btc_anchored has moved
		// forward, don't just trust it (check #3 above only bounds it near
		// h_btc_current and checks the spec-abstracted hash) -- independently
		// confirm, via OUR OWN bitcoind connection, that a real,
		// kDeepFinality-confirmed checkpoint transaction actually exists at
		// that height. Matches "sensors propose, consensus decides": this
		// validator never accepts another node's word for what Bitcoin says.
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
		} else if ext.ZKProofRef {
			return reject, nil
		}

		return accept, nil
	}
}
