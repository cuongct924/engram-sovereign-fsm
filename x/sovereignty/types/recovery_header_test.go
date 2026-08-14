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
