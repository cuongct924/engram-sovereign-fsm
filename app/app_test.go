package app

import (
	"encoding/json"
	"reflect"
	"testing"

	dbm "github.com/cosmos/cosmos-db"
	sovereigntytypes "github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
	"github.com/stretchr/testify/require"

	log "cosmossdk.io/log/v2"
)

// Proves the fact E2's "Vanilla comparison" rests on: vanilla=true never
// registers PreBlocker, so FSMState stays pinned at genesis (CommitFSM
// Transition is never wired). PrepareProposal/ProcessProposal aren't
// asserted -- BaseApp auto-fills those regardless of vanilla mode.
func TestNewEngramApp_VanillaSkipsPreBlocker(t *testing.T) {
	for _, tc := range []struct {
		name    string
		vanilla bool
	}{
		{"vanilla", true},
		{"normal", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			engramApp := NewEngramApp(log.NewNopLogger(), dbm.NewMemDB(), "engram-test-1", tc.vanilla, sovereigntytypes.BaselineEngram, "")

			handlers := reflect.ValueOf(engramApp.BaseApp).Elem().FieldByName("abciHandlers")
			require.True(t, handlers.IsValid(), "baseapp.BaseApp's abciHandlers field not found -- SDK internals changed, update this test")

			preBlocker := handlers.FieldByName("PreBlocker")
			require.True(t, preBlocker.IsValid(), "abciHandlers.PreBlocker not found -- SDK internals changed, update this test")

			if tc.vanilla {
				require.True(t, preBlocker.IsNil(), "vanilla=true must never register PreBlocker -- FSMState would then be reachable from CommitFSMTransition")
			} else {
				require.False(t, preBlocker.IsNil(), "vanilla=false must register PreBlocker")
			}
		})
	}
}

// A restarted node never runs InitChain; LoadGenesisParams must restore the
// genesis Params or it computes transitions with DefaultParams and diverges.
func TestLoadGenesisParamsRestoresNonDefaultParams(t *testing.T) {
	engramApp := NewEngramApp(log.NewNopLogger(), dbm.NewMemDB(), "engram-test-1", false, sovereigntytypes.BaselineEngram, "")
	require.Equal(t, sovereigntytypes.DefaultParams(), engramApp.SovereigntyKeeper.Params)

	p := sovereigntytypes.DefaultParams()
	p.DownHysteresisThreshold = 8
	p.SuspiciousHysteresisWait = 8
	raw, err := json.Marshal(map[string]any{"sovereignty": sovereigntytypes.DefaultGenesisWithParams(p)})
	require.NoError(t, err)

	require.NoError(t, engramApp.LoadGenesisParams(raw))
	require.Equal(t, p, engramApp.SovereigntyKeeper.Params)

	require.NoError(t, engramApp.LoadGenesisParams(nil), "no app_state falls back to defaults")
	require.Equal(t, sovereigntytypes.DefaultParams(), engramApp.SovereigntyKeeper.Params)
}
