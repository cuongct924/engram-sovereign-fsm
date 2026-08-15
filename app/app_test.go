package app

import (
	"reflect"
	"testing"

	log "cosmossdk.io/log/v2"
	dbm "github.com/cosmos/cosmos-db"

	"github.com/stretchr/testify/require"
)

// Proves the fact E2's "Vanilla comparison" rests on: with vanilla=true,
// NewEngramApp never registers PreBlocker (app.go's only vanilla-conditional
// branch), so a vanilla node's FSMState stays pinned at genesis forever --
// the sole post-genesis mutation path (CommitFSMTransition) is never wired.
//
// Only PreBlocker is asserted: BaseApp.Init() auto-fills defaults for
// PrepareProposal/ProcessProposal, so they're non-nil even in vanilla mode;
// PreBlocker has no such fallback. Read via reflection on the unexported
// abciHandlers field (no public getter; IsNil() needs no unsafe).
func TestNewEngramApp_VanillaSkipsPreBlocker(t *testing.T) {
	for _, tc := range []struct {
		name    string
		vanilla bool
	}{
		{"vanilla", true},
		{"normal", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			engramApp := NewEngramApp(log.NewNopLogger(), dbm.NewMemDB(), "engram-test-1", tc.vanilla, "")

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
