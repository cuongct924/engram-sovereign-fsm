package sovereignty

import (
	"context"
	"encoding/json"
	"fmt"

	"cosmossdk.io/core/appmodule"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"

	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
)

// Kiểm tra interface compile-time
var (
	_ appmodule.AppModule        = AppModule{}
	_ module.HasConsensusVersion = AppModule{}
	_ module.HasGenesis          = AppModule{}
	_ module.HasServices         = AppModule{}
	_ appmodule.HasBeginBlocker  = AppModule{}
)

type AppModuleBasic struct {
	cdc codec.Codec
}

func (a AppModuleBasic) Name() string { return types.ModuleName }

func (a AppModuleBasic) RegisterLegacyAminoCodec(*codec.LegacyAmino) {}

func (a AppModuleBasic) RegisterInterfaces(registry codectypes.InterfaceRegistry) {
	types.RegisterInterfaces(registry)
}

func (a AppModuleBasic) DefaultGenesis(cdc codec.JSONCodec) json.RawMessage {
	return cdc.MustMarshalJSON(types.DefaultGenesis())
}

func (a AppModuleBasic) ValidateGenesis(cdc codec.JSONCodec, _ client.TxEncodingConfig, bz json.RawMessage) error {
	var gs types.GenesisState
	if err := cdc.UnmarshalJSON(bz, &gs); err != nil {
		return err
	}
	switch gs.FsmState {
	case types.StateAnchored, types.StateSuspicious, types.StateSovereign, types.StateRecovering:
	default:
		return fmt.Errorf("sovereignty: invalid genesis fsm_state %q", gs.FsmState)
	}
	return nil
}

func (a AppModuleBasic) RegisterGRPCGatewayRoutes(clientCtx client.Context, mux *runtime.ServeMux) {}

type AppModule struct {
	AppModuleBasic
	keeper *keeper.Keeper
}

func NewAppModule(cdc codec.Codec, k *keeper.Keeper) AppModule {
	return AppModule{
		AppModuleBasic: AppModuleBasic{cdc: cdc},
		keeper:         k,
	}
}

func (a AppModule) ConsensusVersion() uint64 { return 1 }

// RegisterServices đăng ký MsgServer của bạn
func (a AppModule) RegisterServices(cfg module.Configurator) {
	types.RegisterMsgServer(cfg.MsgServer(), keeper.NewMsgServerImpl(a.keeper))
}

// BeginBlock được gọi tại mỗi block, đây là nơi "bộ não" FSM vận hành
func (a AppModule) BeginBlock(ctx context.Context) error {
	// Gọi hàm FSMTransitionEngine từ abci.go để cập nhật trạng thái FSM
	return BeginBlocker(ctx, a.keeper)
}

func (a AppModule) InitGenesis(ctx sdk.Context, cdc codec.JSONCodec, bz json.RawMessage) {
	var gs types.GenesisState
	cdc.MustUnmarshalJSON(bz, &gs)

	if err := a.keeper.FSMState.Set(ctx, gs.FsmState); err != nil {
		panic(err)
	}
	if err := a.keeper.SafeBlocks.Set(ctx, gs.SafeBlocksCounter); err != nil {
		panic(err)
	}
	if err := a.keeper.SuspiciousDuration.Set(ctx, gs.SuspiciousDuration); err != nil {
		panic(err)
	}
	if err := a.keeper.ReanchoringProofValid.Set(ctx, gs.ReanchoringProofValid); err != nil {
		panic(err)
	}
	if gs.InitialMetrics != nil {
		if err := a.keeper.Metrics.Set(ctx, gs.InitialMetrics); err != nil {
			panic(err)
		}
	}
}

func (a AppModule) ExportGenesis(ctx sdk.Context, cdc codec.JSONCodec) json.RawMessage {
	fsmState, _ := a.keeper.FSMState.Get(ctx)
	safeBlocks, _ := a.keeper.SafeBlocks.Get(ctx)
	suspiciousDuration, _ := a.keeper.SuspiciousDuration.Get(ctx)
	proofValid, _ := a.keeper.ReanchoringProofValid.Get(ctx)
	metrics, _ := a.keeper.Metrics.Get(ctx)

	gs := &types.GenesisState{
		FsmState:              fsmState,
		SafeBlocksCounter:     safeBlocks,
		SuspiciousDuration:    suspiciousDuration,
		ReanchoringProofValid: proofValid,
		HysteresisWaitLimit:   a.keeper.Params.HysteresisWait,
		InitialMetrics:        metrics,
	}
	return cdc.MustMarshalJSON(gs)
}

func (a AppModule) IsOnePerModuleType() {}
func (a AppModule) IsAppModule()        {}
