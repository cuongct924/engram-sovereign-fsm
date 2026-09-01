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

// EncodeAggregatedHealthCertificate marshals a types.AggregatedHealthCertificate
// (real protobuf -- it carries raw signature bytes, unlike this package's
// usual JSON keeper codec) and prepends a marker, the same Txs[0]-smuggling
// mechanism ExtendedProposal used (still needed: ABCI 2.0 hands
// ProcessProposal a plain, non-extended CommitInfo with no vote-extension
// payloads -- see NewProcessProposalHandler's doc -- so the certificate has
// to travel as ordinary block data, not as a vote extension itself).
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

// fsmInputFromMetrics reads the keeper's committed counters into a
// keeper.FSMInput, pairing them with the caller-supplied metrics --
// defaulting to FSMInit's values (spec/core/EngramFSM.tla:195-220) when
// nothing has been persisted yet. metrics must be a quorum-certified
// AggregateHealthCertificates result (PrepareProposal, ProcessProposal),
// never k.Metrics directly -- that Item is this validator's own local
// sensor snapshot, only meaningful inside NewExtendVoteHandler.
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

// degradedMetrics is the fail-safe aggregate used when
// AggregateHealthCertificates can't reach quorum (chain start, before
// vote extensions have accumulated enough history, or a live >f Byzantine
// coalition withholding/forging extensions): treat peripheral health as
// maximally degraded rather than inventing or assuming a healthy value,
// mirroring RefreshMetrics's own btcGapMetric/daGapMetric fallbacks.
func degradedMetrics(params types.Params) *types.PeripheralMetrics {
	return &types.PeripheralMetrics{
		BtcGap: params.SovereignThreshold,
		DaGap:  params.DAThreshold,
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

// applyByzantineBehavior mutates cert's leader-claimed derived fields (and,
// for censor_tx, txs) per behavior. Only called from the leader path
// (NewPrepareProposalHandler), matching a real malicious proposer's
// capability: it can only lie about what IT claims the certificates'
// deterministic aggregation resolved to, never rewrite what other
// validators independently recompute from the same certificates via
// AggregateHealthCertificates.
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

// NewPrepareProposalHandler builds the sdk.PrepareProposalHandler, mirroring
// ServerInsertProposal (spec/core/EngramServer.tla:52-102). Unlike the
// pre-vote-extension version, it never reads live sensors itself: the
// leader's own last-height precommit vote extensions
// (req.LocalLastCommit -- an ExtendedCommitInfo, available to the proposer
// only) are aggregated into the SAME quorum-certified PeripheralMetrics
// every other validator will independently recompute in ProcessProposal
// from the identical certificates embedded below.
//
// byzantineBehavior is ENGRAM_BYZANTINE_BEHAVIOR, empty on every real
// validator (see applyByzantineBehavior's doc).
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
		// hBtcCurrent: THIS round's own quorum-certified median (metrics.
		// BtcCurrentHeight), not a committed keeper read -- a committed
		// h_btc_current is always at least one block stale, and staleness
		// here specifically breaks the cap below (see PeripheralMetrics.
		// btc_current_height's doc for the live-cluster failure this caused
		// when h_btc_current was frozen at its genesis value forever).
		hBtcCurrent := metrics.BtcCurrentHeight
		// Advance h_btc_anchored via the quorum-certified median of every
		// validator's own ConfirmedAnchorHeight (folded into `metrics` above
		// by NewExtendVoteHandler->RefreshMetrics), capped at hBtcCurrent to
		// enforce checkpoint_block_height <= h_btc_current (VerifySPVProof,
		// EngramTendermint.tla:279-283).
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

		// Attach zk_proof_ref (EngramServer.tla:73-76) once in RECOVERING,
		// hysteresis is satisfied, and verification conditions are met.
		// baseline: zkProofFreshnessBaseline's doc -- NOT hEngramVerifiedPrev
		// directly once already in RECOVERING (that value ratchets up every
		// block and can permanently starve this gate). >=, not >:
		// verifyZkProofFlag's doc -- a strict > against this round's own
		// certificate-supplied value is unsatisfiable once any round-skip
		// has happened at this height (ExtendVote isn't solicited for nil
		// precommits, so no later round can ever produce a fresher value).
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

// verifyZkProofFlag ports VerifyZkProof (spec/core/EngramTendermint.tla:265-268).
// zkProofRef's presence (non-nil), not its value, is safety-relevant here,
// matching the abstract spec's BOOLEAN check. baseline is
// zkProofFreshnessBaseline's result, not a raw committed h_engram_verified
// read -- see HEngramVerifiedAtRecoveryEntry's doc for why.
//
// >= baseline, not the spec's literal > (VerifyZkProof's da_receipt.
// published_block_height > h_engram_verified): the abstract spec can choose
// a favorable interleaving where h_engram_verified' advances in the exact
// step this check needs; the concrete system cannot. ext.Certificates for a
// given height are fixed once that height's proposer collects them from the
// PREVIOUS height's own precommit vote extensions (ExtendVote is only
// solicited for a NON-nil precommit -- confirmed live: zero ExtendVote
// calls, hence zero sensor refreshes, across 48+ consecutive nil-prevote
// rounds at one stuck height). So once a height round-skips even once, NO
// number of retries can ever produce a materially different DaVerifiedHeight
// for that height's own certificate -- a strict `>` against a value that
// cert itself supplies is unsatisfiable by construction, not just unlikely.
// `>=` still requires real, current DA attestation (Attestation=true, and
// baseline is itself a real previously-verified height); it only stops
// requiring NEWER-than-recovery-entry, which the concrete refinement cannot
// reliably produce on demand.
func verifyZkProofFlag(zkProofRef []byte, receipt da.Receipt, baseline uint64) bool {
	return len(zkProofRef) > 0 && receipt.Attestation && receipt.PublishedBlockHeight >= baseline
}

// zkProofFreshnessBaseline returns the fixed h_engram_verified reference the
// zk_proof_ref gate's freshness check must exceed (VerifyZkProof,
// spec/core/EngramTendermint.tla:268). While already in RECOVERING, this is
// the snapshot captured once on entry (HEngramVerifiedAtRecoveryEntry's
// doc); the round that is itself entering RECOVERING has no such snapshot
// yet (CommitFSMTransition writes it only once the transition commits), so
// that round's own pre-round hEngramVerifiedPrev IS what the snapshot is
// about to become -- using it directly keeps this function total without an
// error return.
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

// NewProcessProposalHandler builds ProcessProposalHandler, porting
// IsValidProposal (EngramTendermint.tla:289-338).
//
// Unlike the version this replaces, it never reads a live sensor or calls
// out to bitcoind/Celestia: ACCEPT/REJECT is now a pure function of the
// proposal bytes plus committed state. The certificate embedded in Txs[0]
// carries raw, signed per-validator vote extensions
// (types.SignedHealthCertificate); AggregateHealthCertificates -- the exact
// same deterministic function PrepareProposal used -- re-verifies every
// signature against the genesis validator registry and recomputes the
// aggregate here, so two honest validators reading identical proposal bytes
// always reach the identical accept/reject decision (the AppHash-divergence
// failure mode this closes).
//
// Trade-off, by design: a NEWLY claimed BTC/DA checkpoint advancement at
// height H can only be independently spot-checked by real bitcoind/Celestia
// RPC one block later, once height H's own vote extensions (signed during
// H's precommit) feed into height H+1's aggregate -- ABCI++'s vote
// extensions inherently lag proposals by one height, so an immediate
// per-proposal RPC veto (the old check 3b) is not reproducible without
// reintroducing non-determinism. Structural checks (anchor.VerifyReceipt's
// ExpectedBlockHash/monotonicity/KDeepFinality) still gate acceptance at
// proposal time; a forged claim that passes those is caught and demotes the
// FSM within one block once is_btc_spv_failed propagates through the next
// quorum-certified aggregate.
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

		// 0. Censorship check (IsCensoring, EngramTendermint.tla:343-347; call
		// site UponProposalInPropose, EngramTendermint.tla:609).
		// Note: ABCI 2.0 cannot force immediate round advance; REJECT yields a nil prevote.
		//
		// MaxCensorshipRounds (types.Params doc): past that many rounds, skip
		// the check -- an uncommittable forced tx would otherwise reject every
		// proposal forever instead of just nil-prevoting one round.
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
		// h_engram_current is this chain's own height -- trivially known to
		// every validator (req.Height), not external sensor data, so unlike
		// h_btc_current it needs no quorum certification. A committed
		// k.HEngramCurrent read is the same dead-write bug h_btc_current had
		// (only ever written inside ExtendVote's throwaway CacheMultiStore,
		// via daGapMetric -- ExtendVote's context is never written back to
		// committed state): it stays frozen at its genesis value forever,
		// and da.VerifyReceipt then rejects any real (growing)
		// DaPublishedBlockHeight as "ahead of h_engram_current" -- confirmed
		// live (round-skipped forever the moment DaVerifiedHeight first grew
		// past 0, right after fixing the matching h_btc_current bug above).
		hEngramCurrent := uint64(req.Height)
		isDAHealthy := types.IsDAHealthy(in.Metrics, k.Params)
		if !da.VerifyReceipt(daReceipt, ext.FsmState, isDAHealthy, hEngramCurrent, k.Params.DAThreshold, round) {
			fmt.Printf("engramd: ProcessProposal REJECT height=%d round=%d reason=da_verify_receipt daReceipt=%+v fsmState=%s isDAHealthy=%v hEngramCurrent=%d daThreshold=%d\n",
				req.Height, req.Round, daReceipt, ext.FsmState, isDAHealthy, hEngramCurrent, k.Params.DAThreshold)
			return reject, nil
		}

		// 3. Settlement monotonicity & BTC check (IsValidProposal:327-329).
		// hBtcCurrent: this round's independently-recomputed certified
		// median (in.Metrics.BtcCurrentHeight), not a leader-claimed ext
		// field and not a committed keeper read -- see PrepareProposal's
		// matching comment for why a committed read is stale/wrong here,
		// and AggregateHealthCertificates' doc for why every honest
		// validator recomputes the identical value from ext.Certificates.
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

		// 5. Re-anchoring ZK-proof gating (IsValidProposal:336-338). baseline:
		// zkProofFreshnessBaseline's doc -- not a raw k.HEngramVerified read,
		// which ratchets up every block and can permanently starve this gate.
		// hEngramVerifiedCommitted is this round's pre-value (ProcessProposal
		// never itself advances it), used only as the fallback for the round
		// that is itself entering RECOVERING (zkProofFreshnessBaseline's doc).
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
