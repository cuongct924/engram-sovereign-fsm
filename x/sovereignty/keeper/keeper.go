package keeper

import (
	"context"

	"cosmossdk.io/collections"
	"cosmossdk.io/core/store"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
	merkletree "github.com/iden3/go-merkletree-sql/v2"
)

type Keeper struct {
	cdc          codec.Codec
	storeService store.KVStoreService
	Schema       collections.Schema

	// FSM thresholds -- defaulted to spec/core/MC_StressC1Safety.cfg's verified
	// values (types.DefaultParams). Genesis only carries HysteresisWaitLimit
	// today (see module.go's InitGenesis); the rest of Params is not yet
	// genesis-configurable.
	Params types.Params

	// State lưu trữ FSM
	FSMState              collections.Item[string]
	SafeBlocks            collections.Item[uint64]
	SuspiciousDuration    collections.Item[uint64]
	ReanchoringProofValid collections.Item[bool]
	Metrics               collections.Item[*types.PeripheralMetrics]

	// Height tracking, mirroring spec/core/EngramFSM.tla / EngramTendermint.tla's
	// h_btc_current/h_btc_anchored/h_btc_submitted/h_engram_current/h_engram_verified.
	// Used to build and verify btc_receipt/da_receipt in PrepareProposal/
	// ProcessProposal (see x/sovereignty/proposal.go).
	HBtcCurrent     collections.Item[uint64]
	HBtcAnchored    collections.Item[uint64]
	HBtcSubmitted   collections.Item[uint64]
	HEngramCurrent  collections.Item[uint64]
	HEngramVerified collections.Item[uint64]

	// Censorship-resistance state, mirroring spec/core/EngramTendermint.tla's
	// forced_tx_queue / tx_ignored_rounds[self][tx] (M0d). Keys are raw tx
	// byte content, matching x/sovereignty/proposal.go's containsWithdrawal
	// raw-byte-marker convention. See x/sovereignty/types/censorship.go for
	// the pure IsCensoring/NextIgnoredRounds functions that consume these.
	ForcedTxQueue   collections.KeySet[string]
	TxIgnoredRounds collections.Map[string, uint64]

	// Re-anchoring ZK proof state, mirroring spec/README.md's §Re-anchoring
	// via ZK-Proof of Recovery: HeaderHistory tracks the witness headers
	// (types.RecoveryHeader) for the CURRENT SOVEREIGN/RECOVERING interval
	// only (populated by preblock.go's CommitFSMTransition; pruned there in
	// full on a full return to ANCHORED, and pruned incrementally by
	// keeper.MsgServerImpl.pruneHeaderHistoryUpTo on each rolling checkpoint
	// advance below). LastAnchoredRoot is rt_last -- initially captured when
	// the interval starts, but a ROLLING checkpoint from then on: it also
	// advances every time keeper.MsgServerImpl.SubmitRecoveryProof accepts a
	// real proof, since the circuit's N is fixed at compile time
	// (circuit/reanchoring/src/main.nr) while a real interval's length is
	// environment-controlled and unbounded -- one proof spanning the entire
	// interval-start-to-tip range would make the circuit unusable the
	// moment the interval outgrows N (confirmed live: this happened for
	// real, HeaderHistory grew past 2,000 entries in a single never-yet-
	// ANCHORED run). Rolling checkpoints let a sequence of real, independent
	// N-sized proofs cover an arbitrarily long interval instead.
	// RealProofSubmittedHeight is set by the same handler, once a real proof
	// verifies AND its public inputs match on-chain state, to the HEIGHT the
	// checkpoint was just advanced to (0 = no real proof accepted yet for
	// the current interval) -- storing the height, not just a bool, is
	// load-bearing: the interval can keep growing (new headers appended)
	// after a proof is submitted but before RECOVERING is reached, and a
	// proof only covers headers up to the height it was built against, so a
	// flat bool would stay stale-true even once newer, never-proven headers
	// have been appended (confirmed by actually running this pipeline: a
	// proof submitted while 4 headers were tracked must NOT still read as
	// valid once a 5th has been appended). Consumed (and reset to 0 only
	// when RECOVERING is left) by sensors_refresh.go's
	// refreshReanchoringProofValid, which requires this to equal the
	// CURRENT latest tracked header's height, not just be nonzero -- which
	// is exactly what makes a rolling-checkpoint proof landing with no
	// further headers appended since double as the final proof that
	// completes RECOVERING -> ANCHORED.
	HeaderHistory            collections.Map[uint64, types.RecoveryHeader]
	LastAnchoredRoot         collections.Item[[]byte]
	RealProofSubmittedHeight collections.Item[uint64]

	// SMT Tree
	Tree *merkletree.MerkleTree

	// peerFilterSrc backs FilterPeerByAddr (peer_filter.go) -- nil until
	// SetPeerFilterSource is called (cmd/engramd's wirePeerFilter, late-bound
	// after node.NewNode() constructs the real *p2p.Switch, since
	// NewEngramApp registers FilterPeerByAddr on BaseApp before that).
	peerFilterSrc PeerFilterSource
}

func NewKeeper(storeService store.KVStoreService, cdc codec.Codec, smtStore merkletree.Storage) *Keeper {
	sb := collections.NewSchemaBuilder(storeService)

	k := &Keeper{
		cdc:                   cdc,
		storeService:          storeService,
		Params:                types.DefaultParams(),
		FSMState:              collections.NewItem(sb, collections.NewPrefix(1), "fsm_state", collections.StringValue),
		SafeBlocks:            collections.NewItem(sb, collections.NewPrefix(2), "safe_blocks", collections.Uint64Value),
		Metrics:               collections.NewItem(sb, collections.NewPrefix(3), "metrics", collections.NewJSONValueCodec[*types.PeripheralMetrics]()),
		SuspiciousDuration:    collections.NewItem(sb, collections.NewPrefix(4), "suspicious_duration", collections.Uint64Value),
		ReanchoringProofValid: collections.NewItem(sb, collections.NewPrefix(5), "reanchoring_proof_valid", collections.BoolValue),
		HBtcCurrent:           collections.NewItem(sb, collections.NewPrefix(6), "h_btc_current", collections.Uint64Value),
		HBtcAnchored:          collections.NewItem(sb, collections.NewPrefix(7), "h_btc_anchored", collections.Uint64Value),
		HBtcSubmitted:         collections.NewItem(sb, collections.NewPrefix(8), "h_btc_submitted", collections.Uint64Value),
		HEngramCurrent:        collections.NewItem(sb, collections.NewPrefix(9), "h_engram_current", collections.Uint64Value),
		HEngramVerified:       collections.NewItem(sb, collections.NewPrefix(10), "h_engram_verified", collections.Uint64Value),
		ForcedTxQueue:         collections.NewKeySet(sb, collections.NewPrefix(11), "forced_tx_queue", collections.StringKey),
		TxIgnoredRounds:       collections.NewMap(sb, collections.NewPrefix(12), "tx_ignored_rounds", collections.StringKey, collections.Uint64Value),
		HeaderHistory:            collections.NewMap(sb, collections.NewPrefix(13), "header_history", collections.Uint64Key, collections.NewJSONValueCodec[types.RecoveryHeader]()),
		LastAnchoredRoot:         collections.NewItem(sb, collections.NewPrefix(14), "last_anchored_root", collections.BytesValue),
		RealProofSubmittedHeight: collections.NewItem(sb, collections.NewPrefix(15), "real_proof_submitted_height", collections.Uint64Value),
	}

	// Khởi tạo SMT với storage adapter được inject vào
	tree, err := merkletree.NewMerkleTree(context.Background(), smtStore, 256)
	if err != nil {
		panic(err)
	}
	k.Tree = tree

	schema, err := sb.Build()
	if err != nil {
		panic(err)
	}
	k.Schema = schema

	return k
}
