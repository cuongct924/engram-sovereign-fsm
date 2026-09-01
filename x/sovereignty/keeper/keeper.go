package keeper

import (
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"

	"cosmossdk.io/collections"
	"cosmossdk.io/core/store"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type Keeper struct {
	cdc          codec.Codec
	storeService store.KVStoreService
	Schema       collections.Schema

	// Genesis-configured params (defaults from MC_StressC1Safety.cfg's
	// verified values), overridden by InitChainer from validated genesis.
	Params types.Params

	// FSM state, mirroring spec/core/EngramFSM.tla's state variables.
	FSMState           collections.Item[string]
	SafeBlocks         collections.Item[uint64]
	SuspiciousDuration collections.Item[uint64]
	// UnhealthyStreak backs E5's down-hysteresis: consecutive absorbed
	// non-critical warning/unhealthy blocks before demoting.
	UnhealthyStreak collections.Item[uint64]
	// FailedRecoveryAttempts backs the flapping-attack exponential-backoff
	// hardening: RECOVERING->SOVEREIGN regressions since last recovery.
	FailedRecoveryAttempts collections.Item[uint64]
	// SuspiciousSafeBlocks backs the Gray Failure Arbitrage fix: consecutive
	// healthy blocks absorbed in SUSPICIOUS before exiting.
	SuspiciousSafeBlocks  collections.Item[uint64]
	ReanchoringProofValid collections.Item[bool]
	Metrics               collections.Item[*types.PeripheralMetrics]

	// Height tracking, mirroring EngramFSM.tla/EngramTendermint.tla's h_*
	// variables. Built/verified in x/sovereignty/proposal.go.
	HBtcCurrent     collections.Item[uint64]
	HBtcAnchored    collections.Item[uint64]
	HBtcSubmitted   collections.Item[uint64]
	HEngramCurrent  collections.Item[uint64]
	HEngramVerified collections.Item[uint64]

	// Censorship-resistance state, mirroring EngramTendermint.tla's
	// forced_tx_queue / tx_ignored_rounds.
	ForcedTxQueue   collections.KeySet[string]
	TxIgnoredRounds collections.Map[string, uint64]

	// Re-anchoring ZK proof state (spec/README.md's §Re-anchoring via
	// ZK-Proof of Recovery). HeaderHistory tracks the CURRENT interval's
	// witness headers; LastAnchoredRoot is the rolling rt_last; the latch
	// stores the proven HEIGHT (not a bool) so a stale proof can't read as
	// valid once newer headers are appended.
	HeaderHistory    collections.Map[uint64, types.RecoveryHeader]
	LastAnchoredRoot collections.Item[[]byte]
	// LastAnchoredHeight is the height LastAnchoredRoot's value corresponds
	// to (the incoming-AppHash seed's height on first interval entry, or the
	// checkpoint height of the most recently accepted recovery proof) --
	// lets SubmitRecoveryProof enforce that a proof's `count` public input
	// equals the REAL number of headers covered (newCheckpointHeight -
	// LastAnchoredHeight), closing the fabricated-single-header gap (see
	// circuit/reanchoring/src/main.nr's rt_new-binding doc).
	LastAnchoredHeight       collections.Item[uint64]
	RealProofSubmittedHeight collections.Item[uint64]
	// HEngramVerifiedAtRecoveryEntry snapshots h_engram_verified once, the
	// block RECOVERING is first entered -- the zk_proof_ref freshness gate
	// (EngramTendermint.tla:268's "da_receipt.published_block_height >
	// h_engram_verified") must compare against THIS fixed reference, not the
	// previous block's committed h_engram_verified: the latter ratchets up
	// every block regardless of whether a proof is attached, so requiring
	// the DA receipt to exceed it demands DA verification advance on the
	// EXACT same block the proposal is built -- a race real (non-abstract)
	// DA publish/confirm latency essentially never wins once it falls behind
	// block cadence, permanently deadlocking RECOVERING->ANCHORED (reproduced
	// live: a 7-node cluster round-skipped forever once h_engram_verified's
	// block-over-block advancement stopped keeping pace). A fixed per-episode
	// reference gives every subsequent block a chance to satisfy the check,
	// not just one.
	HEngramVerifiedAtRecoveryEntry collections.Item[uint64]
	// HeuristicProofLatched is true once THIS RECOVERING episode has already
	// latched HBtcSubmitted/reset ReanchoringProofValid via a heuristic
	// zk_proof_ref (CommitFSMTransition's Step 4) -- guards against re-
	// latching on every subsequent block. Once SafeBlocks caps at
	// HysteresisWait (NextSafeBlocks' doc), it stays capped for the rest of
	// the episode, so PrepareProposal/ProcessProposal's check #5 require a
	// non-empty zk_proof_ref on EVERY such block, not just the first --
	// without this guard, Step 4 re-set HBtcSubmitted to the current (ever-
	// climbing) h_btc_current every block, so hAnchored (BTC-confirmation-
	// lagged) could never catch up and ReanchoringProofValid could never
	// settle true, permanently stalling RECOVERING->ANCHORED (reproduced
	// live: 226+ blocks in RECOVERING with zero progress toward ANCHORED,
	// immediately after fixing the freshness-gate deadlock above). Reset to
	// false on entering AND on leaving RECOVERING (preblock.go).
	HeuristicProofLatched collections.Item[bool]
	// BlockHashByHeight maps a real block height to that block's own real
	// CometBFT hash -- populated from RequestFinalizeBlock.Hash ("the merkle
	// root hash of the fields of the decided block") for every WithdrawLocked
	// height (x/sovereignty/preblock.go's NewPreBlocker; CommitFSMTransition
	// itself has no access to the ABCI request, so this write lives in the
	// caller, not there). ctx.BlockHeader().LastBlockId is NOT usable for
	// this: RequestFinalizeBlock carries no LastBlockId field at all, unlike
	// AppHash (which BaseApp threads through from its own prior response) --
	// an earlier version of this code wrongly assumed it did and silently
	// stored empty hashes. Together with ValidatorSetHash, this is what
	// circuit/reanchoring/src/main.nr's boundary-block binding proves >=2/3
	// of the genesis validator set really precommitted for the interval's
	// first and last real header -- see circuit/README.md's Scope section.
	//
	// Deliberately general (any tracked height, not just two fixed
	// endpoints): an earlier version stored only the interval's start/end
	// hashes, requiring SubmitRecoveryProof's newCheckpointHeight to equal a
	// value (LastIntervalFinalHeight) that was itself only ever set AFTER a
	// proof was accepted -- circular, so no proof could ever be first.
	// Populated only while ext.FsmState is WithdrawLocked, bounding growth to
	// roughly one interval's span; pruned below the checkpoint height on
	// every accepted SubmitRecoveryProof (pruneBlockHashesUpTo), mirroring
	// HeaderHistory's own pruning.
	BlockHashByHeight collections.Map[uint64, []byte]
	// ValidatorSetHash is cmttypes.ValidatorSet.Hash() for the genesis
	// validator set, computed once at InitChain (app/app.go's
	// newInitChainer) -- immutable for the network's lifetime (CLAUDE.md:
	// "no staking-driven rotation").
	ValidatorSetHash collections.Item[[]byte]

	// Audit pointer: Celestia height where each accepted proof's witness
	// chain was published (0 = not). Concrete-only, no spec line.
	RecoveryProofDAHeights collections.Map[uint64, uint64]

	// Live per-subnet peer count for FilterPeerByAddr; nil until wired
	// (late-bound after node.NewNode(), fails open).
	peerFilterSrc PeerFilterSource

	// Decodes queued forced-tx content; nil = skip validation.
	TxDecoder sdk.TxDecoder

	// E8 double-signing detection, written from block Misbehavior (agreed
	// data, safe to commit).
	DetectedEvidenceCount collections.Item[uint64]
	LastDetectedEvidence  collections.Item[types.EvidenceRecord]

	// Genesis validator address -> {pubkey, power}, captured once at
	// InitChain (app/app.go's newInitChainer). Used only to verify ABCI++
	// vote-extension signatures in ProcessProposal (see
	// x/sovereignty/vote_extension.go) -- see types.ValidatorInfo's doc for
	// why this can't be read from ABCI at that point.
	ValidatorPubKeys collections.Map[string, types.ValidatorInfo]
}

func NewKeeper(storeService store.KVStoreService, cdc codec.Codec) *Keeper {
	sb := collections.NewSchemaBuilder(storeService)

	k := &Keeper{
		cdc:                      cdc,
		storeService:             storeService,
		Params:                   types.DefaultParams(),
		FSMState:                 collections.NewItem(sb, collections.NewPrefix(1), "fsm_state", collections.StringValue),
		SafeBlocks:               collections.NewItem(sb, collections.NewPrefix(2), "safe_blocks", collections.Uint64Value),
		Metrics:                  collections.NewItem(sb, collections.NewPrefix(3), "metrics", collections.NewJSONValueCodec[*types.PeripheralMetrics]()),
		SuspiciousDuration:       collections.NewItem(sb, collections.NewPrefix(4), "suspicious_duration", collections.Uint64Value),
		ReanchoringProofValid:    collections.NewItem(sb, collections.NewPrefix(5), "reanchoring_proof_valid", collections.BoolValue),
		HBtcCurrent:              collections.NewItem(sb, collections.NewPrefix(6), "h_btc_current", collections.Uint64Value),
		HBtcAnchored:             collections.NewItem(sb, collections.NewPrefix(7), "h_btc_anchored", collections.Uint64Value),
		HBtcSubmitted:            collections.NewItem(sb, collections.NewPrefix(8), "h_btc_submitted", collections.Uint64Value),
		HEngramCurrent:           collections.NewItem(sb, collections.NewPrefix(9), "h_engram_current", collections.Uint64Value),
		HEngramVerified:          collections.NewItem(sb, collections.NewPrefix(10), "h_engram_verified", collections.Uint64Value),
		ForcedTxQueue:            collections.NewKeySet(sb, collections.NewPrefix(11), "forced_tx_queue", collections.StringKey),
		TxIgnoredRounds:          collections.NewMap(sb, collections.NewPrefix(12), "tx_ignored_rounds", collections.StringKey, collections.Uint64Value),
		HeaderHistory:            collections.NewMap(sb, collections.NewPrefix(13), "header_history", collections.Uint64Key, collections.NewJSONValueCodec[types.RecoveryHeader]()),
		LastAnchoredRoot:         collections.NewItem(sb, collections.NewPrefix(14), "last_anchored_root", collections.BytesValue),
		RealProofSubmittedHeight: collections.NewItem(sb, collections.NewPrefix(15), "real_proof_submitted_height", collections.Uint64Value),
		DetectedEvidenceCount:    collections.NewItem(sb, collections.NewPrefix(16), "detected_evidence_count", collections.Uint64Value),
		LastDetectedEvidence:     collections.NewItem(sb, collections.NewPrefix(17), "last_detected_evidence", collections.NewJSONValueCodec[types.EvidenceRecord]()),
		UnhealthyStreak:          collections.NewItem(sb, collections.NewPrefix(18), "unhealthy_streak", collections.Uint64Value),
		FailedRecoveryAttempts:   collections.NewItem(sb, collections.NewPrefix(19), "failed_recovery_attempts", collections.Uint64Value),
		SuspiciousSafeBlocks:     collections.NewItem(sb, collections.NewPrefix(20), "suspicious_safe_blocks", collections.Uint64Value),
		RecoveryProofDAHeights:   collections.NewMap(sb, collections.NewPrefix(21), "recovery_proof_da_heights", collections.Uint64Key, collections.Uint64Value),
		ValidatorPubKeys:         collections.NewMap(sb, collections.NewPrefix(22), "validator_pub_keys", collections.StringKey, collections.NewJSONValueCodec[types.ValidatorInfo]()),
		LastAnchoredHeight:         collections.NewItem(sb, collections.NewPrefix(23), "last_anchored_height", collections.Uint64Value),
		ValidatorSetHash:  collections.NewItem(sb, collections.NewPrefix(26), "validator_set_hash", collections.BytesValue),
		BlockHashByHeight: collections.NewMap(sb, collections.NewPrefix(27), "block_hash_by_height", collections.Uint64Key, collections.BytesValue),
		HEngramVerifiedAtRecoveryEntry: collections.NewItem(sb, collections.NewPrefix(28), "h_engram_verified_at_recovery_entry", collections.Uint64Value),
		HeuristicProofLatched: collections.NewItem(sb, collections.NewPrefix(29), "heuristic_proof_latched", collections.BoolValue),
	}

	schema, err := sb.Build()
	if err != nil {
		panic(err)
	}
	k.Schema = schema

	return k
}
