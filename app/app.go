package app

import (
	"encoding/json"

	"cosmossdk.io/collections"
	log "cosmossdk.io/log/v2"
	store "cosmossdk.io/core/store"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/std"
	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"

	abci "github.com/cometbft/cometbft/abci/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	memoryzk "github.com/iden3/go-merkletree-sql/v2/db/memory"

	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty"
	sovereigntykeeper "github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper"
	sovereigntytypes "github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
)

const Name = "engramd"

// EngramApp wires x/sovereignty onto a real BaseApp: codec/interface
// registry, a single KVStoreKey (the module holds all its own state via
// cosmossdk.io/collections), the ante handler (circuit breaker), and the
// three ABCI++ hooks built in x/sovereignty/proposal.go + preblock.go.
//
// TODO(future work, beyond M5): BankKeeper, AuthKeeper, StakingKeeper (and
// therefore real signed transactions, fee handling, and a dynamic multi-
// validator set) are intentionally not wired -- this app demonstrates the
// FSM/ABCI++ flow driven by sensor data, which doesn't require any of that.
// InitChainer below relies entirely on CometBFT's genesis validator list
// (RequestInitChain.Validators), matching a fixed single/few-validator
// testnet rather than staking-driven validator rotation.
type EngramApp struct {
	*baseapp.BaseApp

	cdc               codec.Codec
	interfaceRegistry codectypes.InterfaceRegistry

	SovereigntyKeeper *sovereigntykeeper.Keeper
}

// NewEngramApp constructs a real, runnable EngramApp: BaseApp + codec +
// store + keeper + ante handler + the M4 ABCI++ handlers, loaded and ready.
// chainID must match the genesis file's chain_id -- BaseApp's InitChain
// handshake rejects a mismatch (this was found by actually running the node,
// not by inspection: the first `engramd start` failed with "invalid chain-id
// on InitChain; expected: , got: engram-dev-1" because this parameter was
// missing entirely).
//
// vanilla, when true, skips SetPrepareProposal/SetProcessProposal/
// SetPreBlocker entirely -- BaseApp falls back to its own default handlers
// (default PrepareProposal just packs mempool txs up to MaxTxBytes, default
// ProcessProposal always accepts, no PreBlocker runs at all), i.e. plain
// CometBFT/Cosmos SDK consensus with no ExtendedProposal. This is docs/
// EXPERIMENT.md's E2/E3/E7 vanilla-CometBFT baseline: same binary, same
// x/sovereignty module mounted (so genesis/store layout doesn't diverge),
// only the ABCI hook wiring differs.
func NewEngramApp(logger log.Logger, db dbm.DB, chainID string, vanilla bool) *EngramApp {
	interfaceRegistry := codectypes.NewInterfaceRegistry()
	std.RegisterInterfaces(interfaceRegistry)
	sovereigntytypes.RegisterInterfaces(interfaceRegistry)
	cdc := codec.NewProtoCodec(interfaceRegistry)
	txConfig := authtx.NewTxConfig(cdc, authtx.DefaultSignModes)

	storeKey := storetypes.NewKVStoreKey(sovereigntytypes.StoreKey)
	// A separate store for BaseApp's own ConsensusParams -- required by the
	// ABCI handshake (`SetParamStore`) but conceptually unrelated to
	// x/sovereignty's own state, hence its own key rather than reusing storeKey.
	consensusParamsKey := storetypes.NewKVStoreKey("consensus_params")

	bApp := baseapp.NewBaseApp(Name, logger, db, txConfig.TxDecoder(), baseapp.SetChainID(chainID))
	bApp.MountKVStores(map[string]*storetypes.KVStoreKey{
		sovereigntytypes.StoreKey: storeKey,
		"consensus_params":        consensusParamsKey,
	})
	bApp.SetInterfaceRegistry(interfaceRegistry)
	bApp.SetParamStore(newConsensusParamStore(runtime.NewKVStoreService(consensusParamsKey)))

	sovKeeper := sovereigntykeeper.NewKeeper(runtime.NewKVStoreService(storeKey), cdc, memoryzk.NewMemoryStorage())

	anteHandler := sdk.ChainAnteDecorators(NewCircuitBreakerDecorator(sovKeeper))
	bApp.SetAnteHandler(anteHandler)

	if !vanilla {
		bApp.SetPrepareProposal(sovereignty.NewPrepareProposalHandler(sovKeeper))
		bApp.SetProcessProposal(sovereignty.NewProcessProposalHandler(sovKeeper))
		bApp.SetPreBlocker(sovereignty.NewPreBlocker(sovKeeper))
	}
	bApp.SetInitChainer(newInitChainer(sovKeeper))

	if err := bApp.LoadLatestVersion(); err != nil {
		panic(err)
	}

	return &EngramApp{
		BaseApp:           bApp,
		cdc:               cdc,
		interfaceRegistry: interfaceRegistry,
		SovereigntyKeeper: sovKeeper,
	}
}

// newConsensusParamStore builds the collections.Item[cmtproto.ConsensusParams]
// BaseApp's ABCI handshake needs (baseapp.SetParamStore) -- collections.Item's
// Get/Has/Set already match the baseapp.ParamStore interface exactly, no
// wrapper type needed.
func newConsensusParamStore(storeService store.KVStoreService) collections.Item[cmtproto.ConsensusParams] {
	sb := collections.NewSchemaBuilder(storeService)
	item := collections.NewItem(sb, collections.NewPrefix(0), "consensus_params", collections.NewJSONValueCodec[cmtproto.ConsensusParams]())
	if _, err := sb.Build(); err != nil {
		panic(err)
	}
	return item
}

// newInitChainer seeds FSM genesis state (types.DefaultGenesis, unless
// AppState overrides it) and echoes back req.Validators as-is: BaseApp's ABCI
// handshake requires ResponseInitChain.Validators to match the genesis
// validator count exactly (found by actually running the node -- an earlier
// version omitted this, assuming CometBFT would default to the genesis list
// on its own; it does not).
func newInitChainer(k *sovereigntykeeper.Keeper) sdk.InitChainer {
	return func(ctx sdk.Context, req *abci.RequestInitChain) (*abci.ResponseInitChain, error) {
		gs := sovereigntytypes.DefaultGenesis()
		if len(req.AppStateBytes) > 0 {
			var parsed struct {
				Sovereignty json.RawMessage `json:"sovereignty"`
			}
			if err := json.Unmarshal(req.AppStateBytes, &parsed); err == nil && len(parsed.Sovereignty) > 0 {
				gs = &sovereigntytypes.GenesisState{}
				if err := json.Unmarshal(parsed.Sovereignty, gs); err != nil {
					return nil, err
				}
			}
		}

		if err := k.FSMState.Set(ctx, gs.FsmState); err != nil {
			return nil, err
		}
		if err := k.SafeBlocks.Set(ctx, gs.SafeBlocksCounter); err != nil {
			return nil, err
		}
		if err := k.SuspiciousDuration.Set(ctx, gs.SuspiciousDuration); err != nil {
			return nil, err
		}
		if err := k.ReanchoringProofValid.Set(ctx, gs.ReanchoringProofValid); err != nil {
			return nil, err
		}
		if gs.InitialMetrics != nil {
			if err := k.Metrics.Set(ctx, gs.InitialMetrics); err != nil {
				return nil, err
			}
		}

		return &abci.ResponseInitChain{Validators: req.Validators}, nil
	}
}
