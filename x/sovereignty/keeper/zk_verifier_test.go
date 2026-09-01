package keeper

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// chunk32 feeds both CircuitVerifyRequest.public_inputs and .proof -- this only exercises
// the splitting logic itself, not any real proof data.
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

	// A trailing partial element is silently dropped, not padded or errored on -- real
	// callers only ever pass exact multiples of 32.
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

// verifyZKProofWasm must reject a real, otherwise-valid proof against the WRONG verification
// key -- confirms the VK is load-bearing in the pairing check, not just decoded and ignored
// (reanchor_test.go's TestVerifyZKProof_RealProof covers the correct-vk cases).
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

// Each verifyZKProofWasm call gets fresh module instances so barretenberg's internal C++
// static state can't leak between verifications -- repeating a valid verification is the
// direct regression test: a leak would surface as a later call failing or trapping.
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
