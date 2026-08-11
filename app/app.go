package app

import (
	"encoding/json"
	"fmt"

	abci "github.com/cometbft/cometbft/abci/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/gogoproto/proto"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty"
	sovereigntykeeper "github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper/sensors"
	sovereigntytypes "github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"

	"cosmossdk.io/collections"
	store "cosmossdk.io/core/store"
	log "cosmossdk.io/log/v2"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec"
	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/std"
	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	txsigning "github.com/cosmos/cosmos-sdk/x/tx/signing"
)

const Name = "engramd"

// EngramApp wires x/sovereignty onto a real BaseApp: codec/interface
// registry, a single KVStoreKey (the module holds all its own state via
// cosmossdk.io/collections), the ante handler (circuit breaker), and the
// three ABCI++ hooks built in x/sovereignty/proposal.go + preblock.go.
//
// TODO(future work): BankKeeper, AuthKeeper, StakingKeeper (and
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

	// Sensors backs PrepareProposal/ProcessProposal's live sensor refresh
	// (x/sovereignty/sensors_refresh.go). P2P is exported so
	// cmd/engramd/main.go can wire it to the CometBFT fork's real
	// lp2p.Switch.PeerHealthSnapshot() once node.NewNode() constructs the
	// Switch -- NewEngramApp runs before that, so this starts on the static
	// SetSnapshot-based mock reading and gets upgraded via SetSource
	// shortly after, before the node starts handling any real proposals.
	Sensors *sovereignty.Sensors
}

// NewEngramApp constructs a real, runnable EngramApp: BaseApp + codec +
// store + keeper + ante handler + the ABCI++ handlers, loaded and ready.
// chainID must match the genesis file's chain_id -- BaseApp's InitChain
// handshake rejects a mismatch.
//
// vanilla, when true, skips SetPrepareProposal/SetProcessProposal/
// SetPreBlocker entirely -- BaseApp falls back to its own default handlers
// (plain CometBFT/Cosmos SDK consensus with no ExtendedProposal), docs/
// EXPERIMENT.md's E2/E3/E7 vanilla-CometBFT baseline: same binary, same
// x/sovereignty module mounted, only the ABCI hook wiring differs.
//
// byzantineBehavior is ENGRAM_BYZANTINE_BEHAVIOR, empty on every real
// validator -- forwarded to NewPrepareProposalHandler, see
// applyByzantineBehavior's doc for the recognized values.
func NewEngramApp(logger log.Logger, db dbm.DB, chainID string, vanilla bool, byzantineBehavior string) *EngramApp {
	// "engram" HRP: GetSigners() auto-decodes bech32 from the
	// `cosmos.msg.v1.signer` field (MsgSubmitRecoveryProofRequest's
	// authority) even without SigVerificationDecorator -- address codec
	// required regardless of sig verification.
	interfaceRegistry, err := codectypes.NewInterfaceRegistryWithOptions(codectypes.InterfaceRegistryOptions{
		ProtoFiles: proto.HybridResolver,
		SigningOptions: txsigning.Options{
			AddressCodec:          addresscodec.NewBech32Codec("engram"),
			ValidatorAddressCodec: addresscodec.NewBech32Codec("engramvaloper"),
		},
	})
	if err != nil {
		panic(err)
	}
	std.RegisterInterfaces(interfaceRegistry)
	sovereigntytypes.RegisterInterfaces(interfaceRegistry)
	cdc := codec.NewProtoCodec(interfaceRegistry)
	txConfig := authtx.NewTxConfig(cdc, authtx.DefaultSignModes)

	storeKey := storetypes.NewKVStoreKey(sovereigntytypes.StoreKey)
	// separate from storeKey: ABCI handshake (SetParamStore) requires it, unrelated to x/sovereignty state
	consensusParamsKey := storetypes.NewKVStoreKey("consensus_params")

	bApp := baseapp.NewBaseApp(Name, logger, db, txConfig.TxDecoder(), baseapp.SetChainID(chainID))
	bApp.MountKVStores(map[string]*storetypes.KVStoreKey{
		sovereigntytypes.StoreKey: storeKey,
		"consensus_params":        consensusParamsKey,
	})
	bApp.SetInterfaceRegistry(interfaceRegistry)
	bApp.SetParamStore(newConsensusParamStore(runtime.NewKVStoreService(consensusParamsKey)))

	sovKeeper := sovereigntykeeper.NewKeeper(runtime.NewKVStoreService(storeKey), cdc)

	// Lets SubmitForcedTx reject content that can never appear as real
	// block-tx bytes before it ever enters ForcedTxQueue -- see
	// Keeper.TxDecoder's doc for the live permanent-deadlock incident this
	// closes. Uses the SAME txConfig BaseApp itself decodes with, just
	// below.
	sovKeeper.TxDecoder = txConfig.TxDecoder()

	// Real active Sybil/eclipse ingress defense (peer_filter.go's
	// FilterPeerByAddr) -- fails open until wirePeerFilter late-binds a
	// live PeerFilterSource after node.NewNode() (same ordering as
	// Sensors.P2P above).
	bApp.SetAddrPeerFilter(sovKeeper.FilterPeerByAddr)

	sensorBundle := &sovereignty.Sensors{
		BTC: sensors.NewBTCSensor(),
		DA:  sensors.NewDASensor(),
		P2P: sensors.NewP2PSensor(),
	}

	anteHandler := sdk.ChainAnteDecorators(NewCircuitBreakerDecorator(sovKeeper))
	bApp.SetAnteHandler(anteHandler)

	// Route x/sovereignty's Msg/Query services directly against BaseApp's
	// routers -- this app has no module.Manager (InitChain is a hand-rolled
	// InitChainer below), so AppModule.RegisterServices(module.Configurator)
	// is never called; without this, no Msg/Query is routable via a real
	// submitted tx, only by calling MsgServerImpl/QueryServerImpl directly.
	sovereigntytypes.RegisterMsgServer(bApp.MsgServiceRouter(), sovereigntykeeper.NewMsgServerImpl(sovKeeper))
	sovereigntytypes.RegisterQueryServer(bApp.GRPCQueryRouter(), sovereigntykeeper.NewQueryServerImpl(sovKeeper))

	if !vanilla {
		bApp.SetPrepareProposal(sovereignty.NewPrepareProposalHandler(sovKeeper, sensorBundle, byzantineBehavior))
		bApp.SetProcessProposal(sovereignty.NewProcessProposalHandler(sovKeeper, sensorBundle))
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
		Sensors:           sensorBundle,
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
// AppState overrides it) and echoes back req.Validators as-is: BaseApp's
// ABCI handshake requires ResponseInitChain.Validators to match the genesis
// validator count exactly.
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

		// Genesis-supplied Params (nil -> DefaultParams(), see
		// GenesisParams.ToParams) -- the ONLY point k.Params is ever set.
		// Every validator's genesis.json is generated once and copied
		// identically to all nodes (cmd/engramd/main.go's testnetInitFiles),
		// so this is safe: unlike an env var read per-process, it can't
		// diverge between validators. Validated here, not just at
		// genesis-generation time, so a hand-edited or malformed
		// AppStateBytes still can't start a chain with unsafe params.
		params := gs.Params.ToParams()
		if err := params.Validate(); err != nil {
			return nil, fmt.Errorf("invalid genesis params: %w", err)
		}
		k.Params = params

		return &abci.ResponseInitChain{Validators: req.Validators}, nil
	}
}
