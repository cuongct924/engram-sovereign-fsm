package app

import (
	"encoding/json"
	"fmt"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/crypto/encoding"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	cmttypes "github.com/cometbft/cometbft/types"
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

// EngramApp wires x/sovereignty onto a real BaseApp: codec, a single
// KVStoreKey, the circuit-breaker ante handler, and the ABCI++ hooks from
// proposal.go/preblock.go/vote_extension.go. Bank/Auth/Staking are
// unwired -- the validator set comes only from genesis, no rotation.
type EngramApp struct {
	*baseapp.BaseApp

	cdc               codec.Codec
	interfaceRegistry codectypes.InterfaceRegistry

	SovereigntyKeeper *sovereigntykeeper.Keeper

	// P2P is exported so cmd/engramd/main.go can rewire it to the CometBFT
	// fork's real lp2p.Switch.PeerHealthSnapshot() after node.NewNode() --
	// NewEngramApp runs first, so it starts on the static SetSnapshot mock
	// and gets upgraded via SetSource before any real proposal.
	Sensors *sovereignty.Sensors
}

// NewEngramApp builds a real, runnable EngramApp. chainID must match the
// genesis file's chain_id. vanilla skips the ABCI++ hook wiring below,
// giving E2/E3/E7 a same-binary baseline. baseline selects RQ2's hard-gated or
// soft comparison system (types.BaselineMode; zero value = real Engram).
// byzantineBehavior is
// ENGRAM_BYZANTINE_BEHAVIOR, empty on every real validator.
func NewEngramApp(logger log.Logger, db dbm.DB, chainID string, vanilla bool, baseline sovereigntytypes.BaselineMode, byzantineBehavior string) *EngramApp {
	// "engram" HRP: GetSigners() auto-decodes bech32 from the
	// `cosmos.msg.v1.signer` field even without SigVerificationDecorator --
	// address codec required regardless of sig verification.
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

	// Lets SubmitForcedTx reject undecodable content before it enters
	// ForcedTxQueue -- see Keeper.TxDecoder's doc for the deadlock this
	// closes. Same txConfig BaseApp itself decodes with.
	sovKeeper.TxDecoder = txConfig.TxDecoder()
	sovKeeper.Baseline = baseline

	// Real active Sybil/eclipse ingress defense (peer_filter.go's
	// FilterPeerByAddr) -- fails open until wirePeerFilter late-binds a live
	// PeerFilterSource after node.NewNode() (same ordering as Sensors.P2P).
	bApp.SetAddrPeerFilter(sovKeeper.FilterPeerByAddr)

	sensorBundle := &sovereignty.Sensors{
		BTC: sensors.NewBTCSensor(),
		DA:  sensors.NewDASensor(),
		P2P: sensors.NewP2PSensor(),
	}

	anteHandler := sdk.ChainAnteDecorators(NewCircuitBreakerDecorator(sovKeeper))
	bApp.SetAnteHandler(anteHandler)

	// Route Msg/Query services directly against BaseApp's routers -- no
	// module.Manager here, so AppModule.RegisterServices never runs.
	sovereigntytypes.RegisterMsgServer(bApp.MsgServiceRouter(), sovereigntykeeper.NewMsgServerImpl(sovKeeper))
	sovereigntytypes.RegisterQueryServer(bApp.GRPCQueryRouter(), sovereigntykeeper.NewQueryServerImpl(sovKeeper))

	if !vanilla {
		bApp.SetPrepareProposal(sovereignty.NewPrepareProposalHandler(sovKeeper, byzantineBehavior))
		bApp.SetProcessProposal(sovereignty.NewProcessProposalHandler(sovKeeper))
		bApp.SetExtendVoteHandler(sovereignty.NewExtendVoteHandler(sovKeeper, sensorBundle, byzantineBehavior))
		bApp.SetVerifyVoteExtensionHandler(sovereignty.NewVerifyVoteExtensionHandler(sovKeeper))
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

// newConsensusParamStore builds the collections.Item BaseApp's ABCI
// handshake needs -- its Get/Has/Set already match baseapp.ParamStore.
func newConsensusParamStore(storeService store.KVStoreService) collections.Item[cmtproto.ConsensusParams] {
	sb := collections.NewSchemaBuilder(storeService)
	item := collections.NewItem(sb, collections.NewPrefix(0), "consensus_params", collections.NewJSONValueCodec[cmtproto.ConsensusParams]())
	if _, err := sb.Build(); err != nil {
		panic(err)
	}
	return item
}

// genesisStateFromAppState parses the sovereignty section of genesis app_state,
// falling back to types.DefaultGenesis when it is absent.
func genesisStateFromAppState(appState []byte) (*sovereigntytypes.GenesisState, error) {
	gs := sovereigntytypes.DefaultGenesis()
	if len(appState) == 0 {
		return gs, nil
	}
	var parsed struct {
		Sovereignty json.RawMessage `json:"sovereignty"`
	}
	if err := json.Unmarshal(appState, &parsed); err != nil || len(parsed.Sovereignty) == 0 {
		return gs, nil
	}
	gs = &sovereigntytypes.GenesisState{}
	if err := json.Unmarshal(parsed.Sovereignty, gs); err != nil {
		return nil, err
	}
	return gs, nil
}

// LoadGenesisParams re-applies the genesis-supplied Params. InitChain only runs
// on a fresh chain, so a restarted node would otherwise fall back to
// DefaultParams and compute different transitions than its peers (a live
// restart diverged AppHash this way). Params are fixed by genesis, so every
// validator loads the same value.
func (a *EngramApp) LoadGenesisParams(appState []byte) error {
	gs, err := genesisStateFromAppState(appState)
	if err != nil {
		return err
	}
	params := gs.Params.ToParams()
	if err := params.Validate(); err != nil {
		return fmt.Errorf("invalid genesis params: %w", err)
	}
	a.SovereigntyKeeper.Params = params
	return nil
}

// newInitChainer seeds FSM genesis state (types.DefaultGenesis, unless
// AppState overrides it) and echoes req.Validators back as-is: BaseApp's
// handshake requires ResponseInitChain.Validators to match genesis exactly.
func newInitChainer(k *sovereigntykeeper.Keeper) sdk.InitChainer {
	return func(ctx sdk.Context, req *abci.RequestInitChain) (*abci.ResponseInitChain, error) {
		gs, err := genesisStateFromAppState(req.AppStateBytes)
		if err != nil {
			return nil, err
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

		// Genesis-supplied Params -- the ONLY point k.Params is set. All
		// validators share one generated genesis.json, so this can't
		// diverge between them the way a per-process env var could.
		params := gs.Params.ToParams()
		if err := params.Validate(); err != nil {
			return nil, fmt.Errorf("invalid genesis params: %w", err)
		}
		k.Params = params

		// Capture the genesis validator set's pubkeys -- the only point
		// they're available, since ABCI never hands ProcessProposal/
		// VerifyVoteExtension anything but a validator's address.
		var cmtValidators []*cmttypes.Validator
		for _, vu := range req.Validators {
			pubKey, err := encoding.PubKeyFromProto(vu.PubKey)
			if err != nil {
				return nil, fmt.Errorf("invalid genesis validator pubkey: %w", err)
			}
			addr := string(pubKey.Address())
			if err := k.ValidatorPubKeys.Set(ctx, addr, sovereigntytypes.ValidatorInfo{
				PubKey: pubKey.Bytes(),
				Power:  vu.Power,
			}); err != nil {
				return nil, err
			}
			cmtValidators = append(cmtValidators, cmttypes.NewValidator(pubKey, vu.Power))
		}

		// The real cmttypes.ValidatorSet.Hash() -- the genesis-fixed anchor
		// the reanchoring circuit's boundary-block binding proves against
		// (main.nr's validator_set_hash_7). Built from the real type so its
		// sort order matches upstream exactly.
		if err := k.ValidatorSetHash.Set(ctx, cmttypes.NewValidatorSet(cmtValidators).Hash()); err != nil {
			return nil, err
		}

		return &abci.ResponseInitChain{Validators: req.Validators}, nil
	}
}
