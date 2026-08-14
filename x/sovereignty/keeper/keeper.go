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
	HeaderHistory            collections.Map[uint64, types.RecoveryHeader]
	LastAnchoredRoot         collections.Item[[]byte]
	RealProofSubmittedHeight collections.Item[uint64]

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
	}

	schema, err := sb.Build()
	if err != nil {
		panic(err)
	}
	k.Schema = schema

	return k
}
