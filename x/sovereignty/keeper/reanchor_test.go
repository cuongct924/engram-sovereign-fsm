package keeper

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"cosmossdk.io/collections/colltest"
)

// VerifyZKProof must fail closed: garbage proof/public-input bytes are always
// rejected, never accepted, never panic.
func TestVerifyZKProof_FailsClosedOnGarbage(t *testing.T) {
	storeService, _ := colltest.MockStore()
	k := NewKeeper(storeService, nil)

	require.False(t, k.VerifyZKProof([]byte("not a real proof"), []byte("not real inputs")))
	require.False(t, k.VerifyZKProof(nil, nil))
	require.False(t, k.VerifyZKProof(make([]byte, 1024), make([]byte, 1024)))
}

// VerifyZKProof must accept a real proof from the pinned `bb prove` toolchain against
// embeddedVK, and reject the same proof tampered by one byte -- cross-validates the embedded
// wasm32-wasi verifier against real circuit output. Regenerate zk_assets/testdata/* via
// `bb prove`/`bb write_vk` whenever the circuit or embeddedVK changes.
func TestVerifyZKProof_RealProof(t *testing.T) {
	storeService, _ := colltest.MockStore()
	k := NewKeeper(storeService, nil)

	proofValid, err := os.ReadFile("zk_assets/testdata/proof_valid")
	require.NoError(t, err)
	proofTampered, err := os.ReadFile("zk_assets/testdata/proof_tampered")
	require.NoError(t, err)
	inputsValid, err := os.ReadFile("zk_assets/testdata/inputs_valid")
	require.NoError(t, err)

	require.True(t, k.VerifyZKProof(proofValid, inputsValid), "real proof against embeddedVK must verify")
	require.False(t, k.VerifyZKProof(proofTampered, inputsValid), "tampered proof must be rejected")
}
