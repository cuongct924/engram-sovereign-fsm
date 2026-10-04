package app

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/crypto/ed25519"
	"github.com/cometbft/cometbft/crypto/encoding"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	sovereigntytypes "github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
	"github.com/stretchr/testify/require"

	log "cosmossdk.io/log/v2"
)

// firstBlockAppHash drives one fresh app through InitChain and the first committed block and returns the AppHash
// it holds. A fresh vanilla cluster once left one of seven validators unable to follow the others ("wrong
// Block.Header.AppHash" at height 2), so the first block's AppHash must not depend on anything but the genesis.
func firstBlockAppHash(t *testing.T, vanilla bool, appState []byte, vals []abci.ValidatorUpdate, initialTime time.Time) string {
	t.Helper()
	engramApp := NewEngramApp(log.NewNopLogger(), dbm.NewMemDB(), "engram-determinism", vanilla, sovereigntytypes.BaselineEngram, "")
	_, err := engramApp.InitChain(&abci.RequestInitChain{
		ChainId:         "engram-determinism",
		Time:            initialTime,
		AppStateBytes:   appState,
		Validators:      vals,
		InitialHeight:   1,
		ConsensusParams: &cmtproto.ConsensusParams{Abci: &cmtproto.ABCIParams{VoteExtensionsEnableHeight: 0}},
	})
	require.NoError(t, err)
	_, err = engramApp.FinalizeBlock(&abci.RequestFinalizeBlock{Height: 1, Time: initialTime.Add(time.Second)})
	require.NoError(t, err)
	_, err = engramApp.Commit()
	require.NoError(t, err)
	info, err := engramApp.Info(&abci.RequestInfo{})
	require.NoError(t, err)
	return fmt.Sprintf("%x", info.LastBlockAppHash)
}

// Seven independent instances (and repeated trials, so that random map iteration order would show) starting from
// the same genesis must hold the same AppHash after the first block, in vanilla and in normal mode.
func TestFirstBlockAppHashIsIdenticalAcrossInstances(t *testing.T) {
	var vals []abci.ValidatorUpdate
	for i := 0; i < 7; i++ {
		pk, err := encoding.PubKeyToProto(ed25519.GenPrivKey().PubKey())
		require.NoError(t, err)
		vals = append(vals, abci.ValidatorUpdate{PubKey: pk, Power: 10})
	}
	appState, err := json.Marshal(map[string]any{"sovereignty": sovereigntytypes.DefaultGenesisWithParams(sovereigntytypes.DefaultParams())})
	require.NoError(t, err)

	for _, vanilla := range []bool{true, false} {
		t.Run(fmt.Sprintf("vanilla=%v", vanilla), func(t *testing.T) {
			want := ""
			for trial := 0; trial < 40; trial++ {
				for node := 0; node < 7; node++ {
					got := firstBlockAppHash(t, vanilla, appState, vals, time.Unix(1_790_000_000, 0).UTC())
					if want == "" {
						want = got
					}
					require.Equal(t, want, got, "trial %d node %d diverged from the first instance", trial, node)
				}
			}
		})
	}
}
