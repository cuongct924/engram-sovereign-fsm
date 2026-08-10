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

	// Params defaults to spec/core/MC_StressC1Safety.cfg's verified values
	// (types.DefaultParams). Not yet genesis-configurable beyond HysteresisWaitLimit.
	Params types.Params

	// FSM state, mirroring spec/core/EngramFSM.tla's state variables.
	FSMState              collections.Item[string]
	SafeBlocks            collections.Item[uint64]
	SuspiciousDuration    collections.Item[uint64]
	ReanchoringProofValid collections.Item[bool]
	Metrics               collections.Item[*types.PeripheralMetrics]

	// Height tracking, mirroring spec/core/EngramFSM.tla/EngramTendermint.tla's
	// h_btc_current/h_btc_anchored/h_btc_submitted/h_engram_current/
	// h_engram_verified. Built/verified in PrepareProposal/ProcessProposal
	// (x/sovereignty/proposal.go).
	HBtcCurrent     collections.Item[uint64]
	HBtcAnchored    collections.Item[uint64]
	HBtcSubmitted   collections.Item[uint64]
	HEngramCurrent  collections.Item[uint64]
	HEngramVerified collections.Item[uint64]

	// Censorship-resistance state, mirroring spec/core/EngramTendermint.tla's
	// forced_tx_queue / tx_ignored_rounds[self][tx] (M0d). Keys are raw tx
	// byte content. See types/censorship.go for the pure functions consuming these.
	ForcedTxQueue   collections.KeySet[string]
	TxIgnoredRounds collections.Map[string, uint64]

	// Re-anchoring ZK proof state (spec/README.md's §Re-anchoring via
	// ZK-Proof of Recovery). HeaderHistory tracks witness headers for the
	// CURRENT SOVEREIGN/RECOVERING interval only (preblock.go's
	// CommitFSMTransition). LastAnchoredRoot is rt_last, a rolling
	// checkpoint that advances every time SubmitRecoveryProof accepts a
	// proof (the circuit's N_MAX is fixed at compile time; a real interval
	// isn't, so one proof can't always span the whole interval).
	// RealProofSubmittedHeight stores the HEIGHT the checkpoint was last
	// advanced to (not just a bool) so a stale proof can't read as valid
	// once newer, unproven headers are appended -- see
	// refreshReanchoringProofValid, its consumer.
	HeaderHistory            collections.Map[uint64, types.RecoveryHeader]
	LastAnchoredRoot         collections.Item[[]byte]
	RealProofSubmittedHeight collections.Item[uint64]

	// peerFilterSrc backs FilterPeerByAddr (peer_filter.go) -- nil until
	// SetPeerFilterSource is called (cmd/engramd's wirePeerFilter, late-bound
	// after node.NewNode() constructs the real *p2p.Switch).
	peerFilterSrc PeerFilterSource

	// TxDecoder backs SubmitForcedTx's validation that queued content
	// decodes as a real tx -- nil until SetTxDecoder is called (app.go).
	// Without it, undecodable content can permanently trip IsCensoring on
	// every future proposal, since it can never appear in req.Txs.
	TxDecoder sdk.TxDecoder

	// Double-signing detection (docs/EXPERIMENT.md's E8), written from
	// preblock.go's NewPreBlocker reading RequestFinalizeBlock.Misbehavior
	// directly -- safe to commit since it's deterministic, agreed block data
	// (see types.EvidenceRecord's doc).
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
	}

	schema, err := sb.Build()
	if err != nil {
		panic(err)
	}
	k.Schema = schema

	return k
}
