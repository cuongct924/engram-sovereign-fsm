package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cometbft/cometbft/crypto/ed25519"
	cmttypes "github.com/cometbft/cometbft/types"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty"
	"github.com/stretchr/testify/require"
)

func genesisValidators(names ...string) []cmttypes.GenesisValidator {
	var out []cmttypes.GenesisValidator
	for _, n := range names {
		pub := ed25519.GenPrivKey().PubKey()
		out = append(out, cmttypes.GenesisValidator{Address: pub.Address(), PubKey: pub, Power: 10, Name: n})
	}
	return out
}

func TestHonestAddresses(t *testing.T) {
	vals := genesisValidators("n1", "n2", "n3", "n4")

	got, err := honestAddresses(vals, "n1,n3", "")
	require.NoError(t, err)
	require.Equal(t, [][]byte{vals[0].Address, vals[2].Address}, got)

	got, err = honestAddresses(vals, "", "n2,n4")
	require.NoError(t, err)
	require.Equal(t, [][]byte{vals[0].Address, vals[2].Address}, got, "--byzantine selects the complement")

	got, err = honestAddresses(vals, "", "")
	require.NoError(t, err)
	require.Nil(t, got, "no flag disables the honest-range check")

	_, err = honestAddresses(vals, "n9", "")
	require.Error(t, err)
	_, err = honestAddresses(vals, "n1", "n2")
	require.Error(t, err)
}

func TestSovereigntyGenesisFallsBackToDefault(t *testing.T) {
	gs, err := sovereigntyGenesis(nil)
	require.NoError(t, err)
	require.NotNil(t, gs)

	gs, err = sovereigntyGenesis([]byte(`{"other":{}}`))
	require.NoError(t, err)
	require.NotNil(t, gs)
}

func TestWriteViolationsCSV(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v.csv")
	require.NoError(t, writeViolationsCSV(path, []sovereignty.ReplayViolation{{Height: 7, Kind: "k", Detail: "a,b"}}))
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "height,kind,detail\n7,k,\"a,b\"\n", string(raw))

	empty := filepath.Join(t.TempDir(), "e.csv")
	require.NoError(t, writeViolationsCSV(empty, nil))
	raw, err = os.ReadFile(empty)
	require.NoError(t, err)
	require.Equal(t, "height,kind,detail\n", string(raw))
}
