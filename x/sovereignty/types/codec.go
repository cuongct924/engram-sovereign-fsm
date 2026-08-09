package types

import (
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// RegisterInterfaces registers this module's Msg implementations, generated
// from proto/engram/sovereignty/v1/tx.proto via `make proto-gen`.
func RegisterInterfaces(registry codectypes.InterfaceRegistry) {
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgInjectFaultRequest{},
		&MsgSubmitRecoveryProofRequest{},
		&MsgSubmitForcedTxRequest{},
	)
}
