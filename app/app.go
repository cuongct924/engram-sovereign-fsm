package app

import (
	"github.com/cosmos/cosmos-sdk/baseapp"

	sovereigntykeeper "github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper"
)

// EngramApp extends the BaseApp of Cosmos SDK.
type EngramApp struct {
	*baseapp.BaseApp

	// TODO(Phase 5): BankKeeper, AuthKeeper, StakingKeeper, DAKeeper,
	// VigilanteKeeper (x/da, x/vigilante modules don't exist yet), real
	// module manager + genesis wiring, ABCI++ vote extensions per
	// docs/ARCHITECTURE.md.

	// Keeper for the Sovereign FSM (the logic under test in the paper).
	SovereigntyKeeper *sovereigntykeeper.Keeper
}

// NewEngramApp wires the minimal set of keepers that exist today.
// TODO(Phase 5): full BaseApp construction (codec, store keys, module
// manager order, InitChain/genesis, ABCI++ vote extensions). Phase 0-4
// exercise SovereigntyKeeper directly in-process (see tests/e2e/), not
// through a running EngramApp/BaseApp — this constructor is a placeholder
// for when that real wiring is built.
func NewEngramApp(bApp *baseapp.BaseApp, sovereigntyKeeper *sovereigntykeeper.Keeper) *EngramApp {
	return &EngramApp{
		BaseApp:           bApp,
		SovereigntyKeeper: sovereigntyKeeper,
	}
}
