package keeper

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// chunk32 feeds both CircuitVerifyRequest.public_inputs and .proof --
// msg.ZkProof/msg.PublicInputs already arrive in exactly this concatenated
// 32-byte layout (chunk32's own doc), so this only exercises the splitting
// logic itself, not any real proof data.
func TestChunk32(t *testing.T) {
	require.Empty(t, chunk32(nil))
	require.Empty(t, chunk32(make([]byte, 31))) // shorter than one element

	one := make([]byte, 32)
	one[0] = 0xAB
	got := chunk32(one)
	require.Len(t, got, 1)
	require.Equal(t, one, got[0])

	three := make([]byte, 96)
	for i := range three {
		three[i] = byte(i)
	}
	got = chunk32(three)
	require.Len(t, got, 3)
	require.Equal(t, three[0:32], got[0])
	require.Equal(t, three[32:64], got[1])
	require.Equal(t, three[64:96], got[2])

	// A trailing partial element (not a multiple of 32) is silently
	// dropped, not padded or errored on -- real callers only ever pass
	// exact multiples (public inputs/proof are always whole field
	// elements), so this documents the behavior rather than requiring it.
	got = chunk32(append(three, make([]byte, 10)...))
	require.Len(t, got, 3)
}

// getZKVerifierRuntime must succeed (the embedded gzip decompresses and the
// wasm module compiles cleanly) and cache a single instance across calls --
// zkRuntimeOnce's whole point.
func TestGetZKVerifierRuntime_Succeeds(t *testing.T) {
	rt1, err := getZKVerifierRuntime(context.Background())
	require.NoError(t, err)
	require.NotNil(t, rt1)

	rt2, err := getZKVerifierRuntime(context.Background())
	require.NoError(t, err)
	require.Same(t, rt1, rt2, "sync.Once must return the same cached runtime, not recompile per call")
}

// verifyZKProofWasm must reject a real, otherwise-valid proof when checked
// against the WRONG verification key -- confirms the VK is load-bearing in
// the pairing check, not merely decoded and ignored. reanchor_test.go's
// TestVerifyZKProof_RealProof already covers garbage proof bytes and a
// tampered proof against the CORRECT vk; this is the complementary case.
// verifyZKProofWasm's own contract returns ok=false on every error path
// too, so a single assertion on ok covers both a clean pairing-check
// failure and a wasm-level trap on malformed key material.
func TestVerifyZKProofWasm_WrongVKRejected(t *testing.T) {
	proofValid, err := os.ReadFile("zk_assets/testdata/proof_valid")
	require.NoError(t, err)
	inputsValid, err := os.ReadFile("zk_assets/testdata/inputs_valid")
	require.NoError(t, err)

	require.True(t, len(embeddedVK) > 1100, "expected a real multi-KB verification key")
	wrongVK := append([]byte(nil), embeddedVK...)
	wrongVK[1000] ^= 0xFF // flip one byte well past any header/length-prefix bytes

	ok, _ := verifyZKProofWasm(wrongVK, proofValid, inputsValid)
	require.False(t, ok, "a corrupted verification key must never verify a proof")
}

// Each verifyZKProofWasm call instantiates fresh gofuncs/env/main module
// instances specifically so barretenberg's internal C++ static state (the
// SRS factory, CRS init flags) can't leak between verifications (see that
// function's own doc). Repeating a real, valid verification several times
// in a row is the direct regression test for that invariant: a leak would
// most plausibly surface as a later call failing or trapping once earlier
// state has been left behind.
func TestVerifyZKProofWasm_RepeatedCallsAreIndependent(t *testing.T) {
	proofValid, err := os.ReadFile("zk_assets/testdata/proof_valid")
	require.NoError(t, err)
	inputsValid, err := os.ReadFile("zk_assets/testdata/inputs_valid")
	require.NoError(t, err)

	for i := range 3 {
		ok, err := verifyZKProofWasm(embeddedVK, proofValid, inputsValid)
		require.NoError(t, err, "call %d", i)
		require.True(t, ok, "call %d: real proof against embeddedVK must verify every time", i)
	}
}
