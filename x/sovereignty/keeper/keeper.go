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

	// Baseline selects a same-binary RQ2 comparison system; the zero value
	// is the real Engram protocol (types.BaselineMode).
	Baseline types.BaselineMode

	// FSM state, mirroring spec/core/EngramFSM.tla's state variables.
	FSMState           collections.Item[string]
	SafeBlocks         collections.Item[uint64]
	SuspiciousDuration collections.Item[uint64]
	// UnhealthyStreak backs E5's down-hysteresis: consecutive absorbed
	// non-critical warning/unhealthy blocks before demoting.
	UnhealthyStreak collections.Item[uint64]
	// FlapLevel backs adaptive down-hysteresis: +1 per warning-driven demotion,
	// -1 per StableWindow healthy ANCHORED blocks (spec flap_level).
	FlapLevel collections.Item[uint64]
	// StableBlocks counts healthy ANCHORED blocks toward decaying FlapLevel.
	StableBlocks collections.Item[uint64]
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

	// Re-anchoring ZK proof state (spec/README.md's Re-anchoring via ZK-Proof of Recovery).
	// HeaderHistory tracks the current interval's witness headers; LastAnchoredRoot is the
	// rolling rt_last; the latch stores the proven HEIGHT so a stale proof can't re-validate.
	HeaderHistory    collections.Map[uint64, types.RecoveryHeader]
	LastAnchoredRoot collections.Item[[]byte]
	// LastAnchoredHeight is the height LastAnchoredRoot corresponds to -- lets
	// SubmitRecoveryProof enforce a proof's `count` equals the real number of headers
	// covered (newCheckpointHeight - LastAnchoredHeight), closing the fabricated-header gap.
	LastAnchoredHeight       collections.Item[uint64]
	RealProofSubmittedHeight collections.Item[uint64]
	// HEngramVerifiedAtRecoveryEntry snapshots h_engram_verified once, on RECOVERING entry.
	// The freshness gate (EngramTendermint.tla:268) must compare against this fixed value --
	// the live one ratchets every block, which would deadlock RECOVERING->ANCHORED (reproduced live).
	HEngramVerifiedAtRecoveryEntry collections.Item[uint64]
	// HeuristicProofLatched guards against re-latching HBtcSubmitted every block once
	// SafeBlocks caps (NextSafeBlocks) -- without it hAnchored could never catch up,
	// permanently stalling RECOVERING->ANCHORED (reproduced live). Reset on RECOVERING entry/exit.
	HeuristicProofLatched collections.Item[bool]
	// RecoveringEntryHeight is the block height at which the current RECOVERING episode began
	// (SOVEREIGN->RECOVERING). The SOVEREIGN interval's data is backfilled once
	// PeripheralMetrics.DaContiguousHeight reaches RecoveringEntryHeight-1 (DABackfillValid).
	RecoveringEntryHeight collections.Item[uint64]
	// BlockHashByHeight maps a height to its real CometBFT hash (from RequestFinalizeBlock.Hash,
	// not ctx.BlockHeader().LastBlockId, which carries none) -- with ValidatorSetHash, this is
	// what circuit/reanchoring's boundary-block proof binds to. A general map, not fixed
	// endpoints, so a first proof isn't circularly blocked on a not-yet-set prior checkpoint.
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

	// Genesis validator address -> {pubkey, power}, captured once at InitChain
	// (app/app.go's newInitChainer); used to verify ABCI++ vote-extension signatures
	// in ProcessProposal -- see types.ValidatorInfo's doc for why.
	ValidatorPubKeys collections.Map[string, types.ValidatorInfo]
}

func NewKeeper(storeService store.KVStoreService, cdc codec.Codec) *Keeper {
	sb := collections.NewSchemaBuilder(storeService)

	k := &Keeper{
		cdc:                            cdc,
		storeService:                   storeService,
		Params:                         types.DefaultParams(),
		FSMState:                       collections.NewItem(sb, collections.NewPrefix(1), "fsm_state", collections.StringValue),
		SafeBlocks:                     collections.NewItem(sb, collections.NewPrefix(2), "safe_blocks", collections.Uint64Value),
		Metrics:                        collections.NewItem(sb, collections.NewPrefix(3), "metrics", collections.NewJSONValueCodec[*types.PeripheralMetrics]()),
		SuspiciousDuration:             collections.NewItem(sb, collections.NewPrefix(4), "suspicious_duration", collections.Uint64Value),
		ReanchoringProofValid:          collections.NewItem(sb, collections.NewPrefix(5), "reanchoring_proof_valid", collections.BoolValue),
		HBtcCurrent:                    collections.NewItem(sb, collections.NewPrefix(6), "h_btc_current", collections.Uint64Value),
		HBtcAnchored:                   collections.NewItem(sb, collections.NewPrefix(7), "h_btc_anchored", collections.Uint64Value),
		HBtcSubmitted:                  collections.NewItem(sb, collections.NewPrefix(8), "h_btc_submitted", collections.Uint64Value),
		HEngramCurrent:                 collections.NewItem(sb, collections.NewPrefix(9), "h_engram_current", collections.Uint64Value),
		HEngramVerified:                collections.NewItem(sb, collections.NewPrefix(10), "h_engram_verified", collections.Uint64Value),
		ForcedTxQueue:                  collections.NewKeySet(sb, collections.NewPrefix(11), "forced_tx_queue", collections.StringKey),
		TxIgnoredRounds:                collections.NewMap(sb, collections.NewPrefix(12), "tx_ignored_rounds", collections.StringKey, collections.Uint64Value),
		HeaderHistory:                  collections.NewMap(sb, collections.NewPrefix(13), "header_history", collections.Uint64Key, collections.NewJSONValueCodec[types.RecoveryHeader]()),
		LastAnchoredRoot:               collections.NewItem(sb, collections.NewPrefix(14), "last_anchored_root", collections.BytesValue),
		RealProofSubmittedHeight:       collections.NewItem(sb, collections.NewPrefix(15), "real_proof_submitted_height", collections.Uint64Value),
		DetectedEvidenceCount:          collections.NewItem(sb, collections.NewPrefix(16), "detected_evidence_count", collections.Uint64Value),
		LastDetectedEvidence:           collections.NewItem(sb, collections.NewPrefix(17), "last_detected_evidence", collections.NewJSONValueCodec[types.EvidenceRecord]()),
		UnhealthyStreak:                collections.NewItem(sb, collections.NewPrefix(18), "unhealthy_streak", collections.Uint64Value),
		FlapLevel:                      collections.NewItem(sb, collections.NewPrefix(19), "flap_level", collections.Uint64Value),
		StableBlocks:                   collections.NewItem(sb, collections.NewPrefix(31), "stable_blocks", collections.Uint64Value),
		SuspiciousSafeBlocks:           collections.NewItem(sb, collections.NewPrefix(20), "suspicious_safe_blocks", collections.Uint64Value),
		RecoveryProofDAHeights:         collections.NewMap(sb, collections.NewPrefix(21), "recovery_proof_da_heights", collections.Uint64Key, collections.Uint64Value),
		ValidatorPubKeys:               collections.NewMap(sb, collections.NewPrefix(22), "validator_pub_keys", collections.StringKey, collections.NewJSONValueCodec[types.ValidatorInfo]()),
		LastAnchoredHeight:             collections.NewItem(sb, collections.NewPrefix(23), "last_anchored_height", collections.Uint64Value),
		ValidatorSetHash:               collections.NewItem(sb, collections.NewPrefix(26), "validator_set_hash", collections.BytesValue),
		BlockHashByHeight:              collections.NewMap(sb, collections.NewPrefix(27), "block_hash_by_height", collections.Uint64Key, collections.BytesValue),
		HEngramVerifiedAtRecoveryEntry: collections.NewItem(sb, collections.NewPrefix(28), "h_engram_verified_at_recovery_entry", collections.Uint64Value),
		HeuristicProofLatched:          collections.NewItem(sb, collections.NewPrefix(29), "heuristic_proof_latched", collections.BoolValue),
		RecoveringEntryHeight:          collections.NewItem(sb, collections.NewPrefix(30), "recovering_entry_height", collections.Uint64Value),
	}

	schema, err := sb.Build()
	if err != nil {
		panic(err)
	}
	k.Schema = schema

	return k
}
