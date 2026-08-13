package app

import (
	"reflect"
	"testing"

	dbm "github.com/cosmos/cosmos-db"
	log "cosmossdk.io/log/v2"

	"github.com/stretchr/testify/require"
)

// TestNewEngramApp_VanillaSkipsPreBlocker proves the structural fact
// docs/EXPERIMENT.md's E2 "Vanilla comparison" rests on: with vanilla=true,
// NewEngramApp never registers PreBlocker (app.go's only vanilla-conditional
// branch), so BaseApp's private abciHandlers never holds
// sovereignty.NewPreBlocker -- the sole place FSMState is ever mutated
// post-genesis (preblock.go's CommitFSMTransition). A vanilla node's
// FSMState is provably pinned at its genesis value (StateAnchored) for the
// lifetime of the process, regardless of real BTC/DA/P2P health, since the
// code path that could change it is never wired -- not something that needs
// a live fault-injection run to show.
//
// Only PreBlocker is asserted, not PrepareProposal/ProcessProposal:
// baseapp.BaseApp's own Init() (called from LoadLatestVersion) auto-fills
// NewDefaultProposalHandler's generic versions of those two whenever they
// weren't explicitly set, so they're non-nil even in vanilla mode -- just
// not the FSM-aware ones, and neither reads/writes FSMState anyway.
// PreBlocker has no such SDK-side default fallback; nil is nil.
//
// Reads baseapp.BaseApp's unexported abciHandlers field via reflection: the
// SDK exposes no public getter for "is PreBlocker set", and
// reflect.Value.IsNil() works on an unexported field's value without unsafe
// (only calling .Interface() on it would panic).
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
