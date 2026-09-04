package types

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReduceToField(t *testing.T) {
	mod := bn254ScalarFieldModulus

	t.Run("below modulus stays as-is", func(t *testing.T) {
		out := ReduceToField([]byte{0x01})
		require.Equal(t, uint64(1), new(big.Int).SetBytes(out).Uint64())
		require.Len(t, out, 32)
	})

	t.Run("above modulus is reduced", func(t *testing.T) {
		in := new(big.Int).Add(mod, big.NewInt(5)).Bytes()
		out := ReduceToField(in)
		require.Equal(t, uint64(5), new(big.Int).SetBytes(out).Uint64())
	})

	t.Run("equal to modulus reduces to zero", func(t *testing.T) {
		out := ReduceToField(mod.Bytes())
		require.Equal(t, 0, new(big.Int).SetBytes(out).Sign())
	})

	t.Run("output is always 32 bytes", func(t *testing.T) {
		for _, b := range [][]byte{{}, {0x00}, {0xff}, mod.Bytes(), new(big.Int).Add(mod, big.NewInt(1)).Bytes()} {
			require.Len(t, ReduceToField(b), 32)
		}
	})
}

// TestHashHeader_MatchesCircuitFixture cross-checks against circuit/
// reanchoring/src/main.nr's own fixture (Header{111,2,1,1000}) -- a failure
// here means Go and the circuit have drifted. Expected value from running
// std::hash::poseidon2_permutation([111,2,1,1000])[0] on this repo's pinned
// nargo 1.0.0-beta.22 toolchain, matching gnark-crypto's output exactly.
func TestHashHeader_MatchesCircuitFixture(t *testing.T) {
	prevHash := ReduceToField(big.NewInt(111).Bytes())
	stateRoot := ReduceToField(big.NewInt(1000).Bytes())
	got := HashHeader(prevHash, StateSovereign, true, stateRoot)

	want, ok := new(big.Int).SetString(
		"1d480f47dfb5f6045ee6b8393646287ac84f230ee5faa8a8d77547383b275f95", 16)
	require.True(t, ok)
	require.Equal(t, want.Bytes(), new(big.Int).SetBytes(got).Bytes())
}
