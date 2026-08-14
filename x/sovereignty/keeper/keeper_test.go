package keeper

import (
	"testing"

	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/collections/colltest"
)

// NewKeeper is deterministic: every collection is registered with the schema
// builder (so sb.Build() doesn't panic) and default genesis params are baked
// in, ready for InitChainer to override.
func TestNewKeeper(t *testing.T) {
	storeService, _ := colltest.MockStore()
	k := NewKeeper(storeService, nil)

	require.NotNil(t, k.Schema)
	require.Equal(t, types.DefaultParams(), k.Params)
}
