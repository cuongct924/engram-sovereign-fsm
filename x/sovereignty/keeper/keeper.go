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

	// SMT Tree
	Tree *merkletree.MerkleTree
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
