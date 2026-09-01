package sovereignty

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strings"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cuongct220020/engram-sovereign-fsm/x/anchor"
	"github.com/cuongct220020/engram-sovereign-fsm/x/da"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
	"google.golang.org/protobuf/proto"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const aggregatedHealthCertMarker = "ENGRAM_AGGREGATED_HEALTH_CERT_V1|"

// EncodeAggregatedHealthCertificate marshals the certificate (real protobuf,
// carries raw signature bytes) with a marker prefix, smuggled via Txs[0] --
// ProcessProposal gets no vote-extension payloads under ABCI 2.0.
func EncodeAggregatedHealthCertificate(c *types.AggregatedHealthCertificate) ([]byte, error) {
	body, err := proto.Marshal(c)
	if err != nil {
		return nil, err
	}
	return append([]byte(aggregatedHealthCertMarker), body...), nil
}

// DecodeAggregatedHealthCertificate returns ok=false (no error) if tx isn't
// a marker-prefixed certificate -- callers should treat that as "not
// present", not a decode failure.
func DecodeAggregatedHealthCertificate(tx []byte) (c *types.AggregatedHealthCertificate, ok bool, err error) {
	marker := []byte(aggregatedHealthCertMarker)
	if len(tx) < len(marker) || !bytes.Equal(tx[:len(marker)], marker) {
		return nil, false, nil
	}
	c = &types.AggregatedHealthCertificate{}
	if err := proto.Unmarshal(tx[len(marker):], c); err != nil {
		return nil, true, err
	}
	return c, true, nil
}

// fsmInputFromMetrics pairs committed counters with the caller-supplied
// metrics (FSMInit defaults, spec/core/EngramFSM.tla:195-220). metrics must
// be quorum-certified, never k.Metrics (this validator's own local sensor).
func fsmInputFromMetrics(ctx sdk.Context, k *keeper.Keeper, metrics *types.PeripheralMetrics) (currState string, in keeper.FSMInput) {
	currState, err := k.FSMState.Get(ctx)
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

// degradedMetrics is the fail-safe aggregate when quorum can't be reached
// (chain start, or a Byzantine coalition withholding extensions): treat
// health as maximally degraded rather than assume healthy.
func degradedMetrics(params types.Params) *types.PeripheralMetrics {
	return &types.PeripheralMetrics{
		BtcGap: params.SovereignThreshold,
		DaGap:  params.DAThreshold,
	}
}

// byzantineFakeFSMStatePrefix / byzantineForgeBTCHash / byzantineFalseDA /
// byzantineCensorTxPrefix are ENGRAM_BYZANTINE_BEHAVIOR's recognized values
// (docs/EXPERIMENT.md's E8 A3/A4/A6/A7 rows) -- only engram-node04-
// byzantine.yml ever sets this env var.
const (
	byzantineFakeFSMStatePrefix = "fake_fsm_state:"
	byzantineForgeBTCHash       = "forge_btc_hash"
	byzantineFalseDA            = "false_da_attestation"
	byzantineCensorTxPrefix     = "censor_tx:"
)

// applyByzantineBehavior mutates cert's leader-claimed fields (and, for
// censor_tx, txs). Only the leader path calls this -- it can lie about its
// own claim, never rewrite what honest validators independently recompute.
func applyByzantineBehavior(behavior string, cert *types.AggregatedHealthCertificate, txs [][]byte) [][]byte {
	switch {
	case strings.HasPrefix(behavior, byzantineFakeFSMStatePrefix):
		// A6: claim a healthier state than the certified aggregate actually supports.
		cert.FsmState = strings.TrimPrefix(behavior, byzantineFakeFSMStatePrefix)
	case behavior == byzantineForgeBTCHash:
		// A4: claim the checkpoint advanced with a hash that doesn't match
		// ExpectedBlockHash -- every honest validator's check #3
		// (anchor.VerifyReceipt) must reject this independently.
		cert.BtcCheckpointBlockHashTag = "FORGED"
		cert.BtcCheckpointBlockHashHeight = cert.BtcCheckpointBlockHeight
	case behavior == byzantineFalseDA:
		// A3: claim DA attestation succeeded without a real submission backing it.
		cert.DaAttestation = true
		cert.DaPublishedBlockHeight++
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

// NewPrepareProposalHandler mirrors ServerInsertProposal
// (spec/core/EngramServer.tla:52-102). Never reads live sensors: aggregates
// the leader's own last-height precommit vote extensions
// (req.LocalLastCommit) into the same certified PeripheralMetrics every
// validator independently recomputes in ProcessProposal.
func NewPrepareProposalHandler(k *keeper.Keeper, byzantineBehavior string) sdk.PrepareProposalHandler {
	return func(ctx sdk.Context, req *abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error) {
		certs := certificatesFromExtendedCommit(req.LocalLastCommit)
		extHeight := req.Height - 1
		metrics, ok := AggregateHealthCertificates(ctx, k, ctx.ChainID(), extHeight, certs)
		if !ok {
			metrics = degradedMetrics(k.Params)
		}

		currState, in := fsmInputFromMetrics(ctx, k, metrics)
		targetState := keeper.CalculateNextState(currState, in, k.Params)

		hEngramVerified, _ := k.HEngramVerified.Get(ctx)
		hBtcAnchored, _ := k.HBtcAnchored.Get(ctx)
		// hBtcCurrent: THIS round's certified median, not a committed keeper
		// read -- a committed value is always stale by one block, which
		// broke the cap below (froze h_btc_current at genesis forever).
		hBtcCurrent := metrics.BtcCurrentHeight
		// Advance h_btc_anchored via the certified median ConfirmedAnchorHeight,
		// capped at hBtcCurrent (checkpoint_block_height <= h_btc_current,
		// VerifySPVProof, EngramTendermint.tla:279-283).
		if metrics.ConfirmedAnchorHeight > hBtcAnchored && metrics.ConfirmedAnchorHeight <= hBtcCurrent {
			hBtcAnchored = metrics.ConfirmedAnchorHeight
		}
		// Preserve the start-of-round value for the zk_proof_ref freshness
		// check before applying the certified update below.
		hEngramVerifiedPrev := hEngramVerified
		if metrics.DaVerifiedHeight > hEngramVerified {
			hEngramVerified = metrics.DaVerifiedHeight
		}

		daReceipt := da.Receipt{
			PublishedBlockHeight: hEngramVerified,
			// Attestation represents historical retrievability (PublishedBlockHeight > 0).
			Attestation: hEngramVerified > 0,
		}
		btcReceipt := anchor.Receipt{
			CheckpointBlockHeight: hBtcAnchored,
			CheckpointBlockHash:   anchor.ExpectedBlockHash(hBtcAnchored),
		}

		// Attach zk_proof_ref (EngramServer.tla:73-76) once RECOVERING,
		// hysteresis is satisfied, and verification passes -- baseline via
		// zkProofFreshnessBaseline, not a raw hEngramVerifiedPrev read.
		baseline := zkProofFreshnessBaseline(ctx, k, currState, hEngramVerifiedPrev)
		var zkProofRef []byte
		if targetState == types.StateRecovering && in.SafeBlocks == k.Params.HysteresisWait &&
			daReceipt.Attestation && daReceipt.PublishedBlockHeight >= baseline {
			if root, rerr := k.LastAnchoredRoot.Get(ctx); rerr == nil {
				zkProofRef = root
			}
		}

		cert := &types.AggregatedHealthCertificate{
			Height:                       extHeight,
			Certificates:                 certs,
			FsmState:                     targetState,
			Healthy:                      types.IsHealthyCondition(in.Metrics, k.Params),
			DaPublishedBlockHeight:       daReceipt.PublishedBlockHeight,
			DaAttestation:                daReceipt.Attestation,
			BtcCheckpointBlockHeight:     btcReceipt.CheckpointBlockHeight,
			BtcCheckpointBlockHashTag:    btcReceipt.CheckpointBlockHash.Tag,
			BtcCheckpointBlockHashHeight: btcReceipt.CheckpointBlockHash.Height,
			ZkProofRef:                   zkProofRef,
		}

		innerTxs := req.Txs
		// Filter withdrawal transactions early to avoid proposing rejected TXs
		// and deadlocking rounds when withdrawals are locked (EngramTendermint.tla:332).
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
			innerTxs = applyByzantineBehavior(byzantineBehavior, cert, innerTxs)
		}

		encoded, err := EncodeAggregatedHealthCertificate(cert)
		if err != nil {
			return nil, err
		}

		txs := make([][]byte, 0, len(innerTxs)+1)
		txs = append(txs, encoded)
		txs = append(txs, innerTxs...)
		return &abci.ResponsePrepareProposal{Txs: txs}, nil
	}
}

// verifyZkProofFlag ports VerifyZkProof (spec/core/EngramTendermint.tla:
// 265-268), using >= not the spec's strict >: ExtendVote never runs for a
// nil precommit, so after any round-skip a strict > against this round's
// own certificate-supplied value is unsatisfiable by construction (confirmed
// live: 48+ stuck nil-prevote rounds). >= still requires real attestation.
func verifyZkProofFlag(zkProofRef []byte, receipt da.Receipt, baseline uint64) bool {
	return len(zkProofRef) > 0 && receipt.Attestation && receipt.PublishedBlockHeight >= baseline
}

// zkProofFreshnessBaseline returns the freshness-check reference for
// zk_proof_ref (VerifyZkProof, EngramTendermint.tla:268): the snapshot from
// HEngramVerifiedAtRecoveryEntry once in RECOVERING, or the round's own
// pre-round value for the round that is itself entering RECOVERING.
func zkProofFreshnessBaseline(ctx sdk.Context, k *keeper.Keeper, currState string, hEngramVerifiedPrev uint64) uint64 {
	if currState != types.StateRecovering {
		return hEngramVerifiedPrev
	}
	baseline, err := k.HEngramVerifiedAtRecoveryEntry.Get(ctx)
	if err != nil {
		return hEngramVerifiedPrev
	}
	return baseline
}

// containsWithdrawal is a placeholder for ContainsWithdrawal
// (spec/core/EngramTendermint.tla:261), which treats tx values as an opaque
// enum with a distinguished TX_WITHDRAWAL member. Real Msg-based
// classification belongs to app/ante.go once a TxDecoder is wired here.
func containsWithdrawal(tx []byte) bool {
	return bytes.Contains(tx, []byte("TX_WITHDRAWAL"))
}

// NewProcessProposalHandler ports IsValidProposal (EngramTendermint.tla:
// 289-338) as a pure function of proposal bytes + committed state --
// AggregateHealthCertificates re-verifies Txs[0]'s certificate
// deterministically, closing an AppHash-divergence bug. Trade-off: a newly
// claimed BTC/DA checkpoint can only be spot-checked one block later (vote
// extensions lag proposals by a height); structural checks still gate
// proposal-time acceptance.
func NewProcessProposalHandler(k *keeper.Keeper) sdk.ProcessProposalHandler {
	reject := &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_REJECT}
	accept := &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_ACCEPT}

	return func(ctx sdk.Context, req *abci.RequestProcessProposal) (*abci.ResponseProcessProposal, error) {
		if len(req.Txs) == 0 {
			fmt.Printf("engramd: ProcessProposal REJECT height=%d round=%d reason=empty_txs\n", req.Height, req.Round)
			return reject, nil
		}
		ext, ok, err := DecodeAggregatedHealthCertificate(req.Txs[0])
		if err != nil || !ok {
			fmt.Printf("engramd: ProcessProposal REJECT height=%d round=%d reason=decode_cert err=%v ok=%v\n", req.Height, req.Round, err, ok)
			return reject, nil
		}
		if ext.Height != req.Height-1 {
			fmt.Printf("engramd: ProcessProposal REJECT height=%d round=%d reason=height_mismatch ext.Height=%d\n", req.Height, req.Round, ext.Height)
			return reject, nil
		}

		// 0. Censorship check (IsCensoring, EngramTendermint.tla:343-347). REJECT
		// yields a nil prevote (ABCI 2.0 can't force an immediate round advance).
		// MaxCensorshipRounds caps this so an uncommittable forced tx can't
		// reject every proposal forever.
		if uint64(req.Round) < k.Params.MaxCensorshipRounds {
			forcedTxQueue, err := k.ForcedTxQueueSlice(ctx)
			if err != nil {
				fmt.Printf("engramd: ProcessProposal REJECT height=%d round=%d reason=forced_tx_queue_err err=%v\n", req.Height, req.Round, err)
				return reject, nil
			}
			if len(forcedTxQueue) > 0 {
				ignoredRounds := k.IgnoredRoundsMap(ctx, forcedTxQueue)
				included := make(map[string]bool, len(req.Txs)-1)
				for _, tx := range req.Txs[1:] {
					included[string(tx)] = true
				}
				if types.IsCensoring(forcedTxQueue, ignoredRounds, included, k.Params.MaxIgnoreRounds) {
					fmt.Printf("engramd: ProcessProposal REJECT height=%d round=%d reason=censoring\n", req.Height, req.Round)
					return reject, nil
				}
			}
		}

		// Recompute the SAME deterministic aggregation the leader claims to
		// have derived -- pure function over ext.Certificates, no live read.
		metrics, aggOk := AggregateHealthCertificates(ctx, k, ctx.ChainID(), ext.Height, ext.Certificates)
		if !aggOk {
			metrics = degradedMetrics(k.Params)
		}
		currState, in := fsmInputFromMetrics(ctx, k, metrics)

		// 1. fsm_state cross-check (IsValidProposal:296).
		expectedState := keeper.CalculateNextState(currState, in, k.Params)
		if ext.FsmState != expectedState {
			fmt.Printf("engramd: ProcessProposal REJECT height=%d round=%d reason=fsm_state_mismatch ext=%s expected=%s currState=%s aggOk=%v safeBlocks=%d unhealthyStreak=%d susDur=%d susSafeBlocks=%d proofValid=%v\n",
				req.Height, req.Round, ext.FsmState, expectedState, currState, aggOk, in.SafeBlocks, in.UnhealthyStreak, in.SuspiciousDuration, in.SuspiciousSafeBlocks, in.ReanchoringProofValid)
			return reject, nil
		}

		// 1b. Healthy cross-check. Independent recomputation of health status
		// to safely validate ext.Healthy before CommitFSMTransition.
		if ext.Healthy != types.IsHealthyCondition(in.Metrics, k.Params) {
			fmt.Printf("engramd: ProcessProposal REJECT height=%d round=%d reason=healthy_mismatch ext.Healthy=%v computed=%v\n",
				req.Height, req.Round, ext.Healthy, types.IsHealthyCondition(in.Metrics, k.Params))
			return reject, nil
		}

		daReceipt := da.Receipt{PublishedBlockHeight: ext.DaPublishedBlockHeight, Attestation: ext.DaAttestation}
		btcReceipt := anchor.Receipt{
			CheckpointBlockHeight: ext.BtcCheckpointBlockHeight,
			CheckpointBlockHash:   anchor.BlockHash{Tag: ext.BtcCheckpointBlockHashTag, Height: ext.BtcCheckpointBlockHashHeight},
		}

		// 2. DA pipeline check (IsValidProposal:308-311), utilizing req.Round
		// for round-based tolerance widening.
		round := uint64(req.Round)
		// h_engram_current is just req.Height -- trivially known, needs no
		// quorum certification. A committed k.HEngramCurrent read has the same
		// dead-write bug h_btc_current had (frozen at genesis, ExtendVote's
		// writes never stick), which round-skipped forever once
		// DaVerifiedHeight grew past 0.
		hEngramCurrent := uint64(req.Height)
		isDAHealthy := types.IsDAHealthy(in.Metrics, k.Params)
		if !da.VerifyReceipt(daReceipt, ext.FsmState, isDAHealthy, hEngramCurrent, k.Params.DAThreshold, round) {
			fmt.Printf("engramd: ProcessProposal REJECT height=%d round=%d reason=da_verify_receipt daReceipt=%+v fsmState=%s isDAHealthy=%v hEngramCurrent=%d daThreshold=%d\n",
				req.Height, req.Round, daReceipt, ext.FsmState, isDAHealthy, hEngramCurrent, k.Params.DAThreshold)
			return reject, nil
		}

		// 3. Settlement monotonicity & BTC check (IsValidProposal:327-329).
		// hBtcCurrent: this round's independently-recomputed certified median,
		// not a leader-claimed field or committed read -- see PrepareProposal's
		// matching comment for why a committed read is stale here.
		hBtcCurrent := in.Metrics.BtcCurrentHeight
		hBtcAnchored, _ := k.HBtcAnchored.Get(ctx)
		isBTCHealthy := types.IsBTCHealthy(in.Metrics, k.Params)
		if !anchor.VerifyReceipt(btcReceipt, ext.FsmState, isBTCHealthy, hBtcCurrent, hBtcAnchored, round, k.Params.KDeepFinality) {
			fmt.Printf("engramd: ProcessProposal REJECT height=%d round=%d reason=btc_verify_receipt btcReceipt=%+v fsmState=%s isBTCHealthy=%v hBtcCurrent=%d hBtcAnchored=%d kDeepFinality=%d\n",
				req.Height, req.Round, btcReceipt, ext.FsmState, isBTCHealthy, hBtcCurrent, hBtcAnchored, k.Params.KDeepFinality)
			return reject, nil
		}

		// 4. Circuit breaker: no withdrawal while SOVEREIGN/RECOVERING (IsValidProposal:332).
		if types.WithdrawLocked(ext.FsmState) {
			for _, tx := range req.Txs[1:] {
				if containsWithdrawal(tx) {
					fmt.Printf("engramd: ProcessProposal REJECT height=%d round=%d reason=withdrawal_locked\n", req.Height, req.Round)
					return reject, nil
				}
			}
		}

		// 5. Re-anchoring ZK-proof gating (IsValidProposal:336-338). baseline
		// uses zkProofFreshnessBaseline, not a raw k.HEngramVerified read
		// (which ratchets every block and would starve this gate).
		hEngramVerifiedCommitted, _ := k.HEngramVerified.Get(ctx)
		baseline := zkProofFreshnessBaseline(ctx, k, currState, hEngramVerifiedCommitted)
		if ext.FsmState == types.StateRecovering && in.SafeBlocks == k.Params.HysteresisWait {
			if !verifyZkProofFlag(ext.ZkProofRef, daReceipt, baseline) {
				fmt.Printf("engramd: ProcessProposal REJECT height=%d round=%d reason=zk_proof_gate zkProofRefLen=%d daReceipt=%+v baseline=%d\n",
					req.Height, req.Round, len(ext.ZkProofRef), daReceipt, baseline)
				return reject, nil
			}
		} else if len(ext.ZkProofRef) > 0 {
			fmt.Printf("engramd: ProcessProposal REJECT height=%d round=%d reason=unexpected_zk_proof_ref fsmState=%s safeBlocks=%d hysteresisWait=%d\n",
				req.Height, req.Round, ext.FsmState, in.SafeBlocks, k.Params.HysteresisWait)
			return reject, nil
		}

		return accept, nil
	}
}
