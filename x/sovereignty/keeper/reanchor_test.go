package keeper

import (
	"testing"

	"github.com/stretchr/testify/require"

	"cosmossdk.io/collections/colltest"
)

// VerifyZKProof must fail closed: garbage proof/public-input bytes are always
// rejected (never accepted, never panic), regardless of whether bb is
// installed -- a misconfigured validator rejects rather than silently accepts.
func TestVerifyZKProof_FailsClosedOnGarbage(t *testing.T) {
	storeService, _ := colltest.MockStore()
	k := NewKeeper(storeService, nil)

	require.False(t, k.VerifyZKProof([]byte("not a real proof"), []byte("not real inputs")))
	require.False(t, k.VerifyZKProof(nil, nil))
	require.False(t, k.VerifyZKProof(make([]byte, 1024), make([]byte, 1024)))
}
