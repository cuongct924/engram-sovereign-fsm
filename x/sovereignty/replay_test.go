package sovereignty

import (
	"testing"

	cmttypes "github.com/cometbft/cometbft/types"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
	"github.com/stretchr/testify/require"
)

const replayLie = `{"btc_gap":"6","da_gap":"40","is_das_failed":true,"is_anchor_reverification_failed":true}`

// replayerFor mirrors the fixture's validator set and trusts vals[from:].
func replayerFor(t *testing.T, f *collusionFixture, from int) *Replayer {
	t.Helper()
	var genesis []cmttypes.GenesisValidator
	var honest [][]byte
	for i, v := range f.vals {
		genesis = append(genesis, cmttypes.GenesisValidator{Address: v.addr, PubKey: v.priv.PubKey(), Power: 1})
		if i >= from {
			honest = append(honest, v.addr)
		}
	}
	r, err := NewReplayer(voteExtTestChainID, types.DefaultGenesis(), genesis, honest)
	require.NoError(t, err)
	return r
}

func kinds(vs []ReplayViolation) []string {
	var out []string
	for _, v := range vs {
		out = append(out, v.Kind)
	}
	return out
}

func TestReplay_CleanBlockUnderTwoColludingValidatorsHasNoViolation(t *testing.T) {
	f := newCollusionFixture(t, 2, replayLie, nil)
	_, prep, accepted := f.propose(t, "bias_certificate", nil)
	require.True(t, accepted)

	vs, err := replayerFor(t, f, 2).ApplyBlock(collusionReqHeight, nil, prep.Txs)
	require.NoError(t, err)
	require.Empty(t, vs)
}

func TestReplay_FlagsBeyondThresholdAggregateOutsideHonestRange(t *testing.T) {
	f := newCollusionFixture(t, 3, replayLie, nil)
	_, prep, accepted := f.propose(t, "bias_certificate", nil)
	require.True(t, accepted, "valid proposal: AppHash agreement alone would call this fine")

	vs, err := replayerFor(t, f, 3).ApplyBlock(collusionReqHeight, nil, prep.Txs)
	require.NoError(t, err)
	require.Equal(t, []string{ViolationOutsideHonestRange}, kinds(vs))
}

func TestReplay_FlagsTamperedFSMState(t *testing.T) {
	f := newCollusionFixture(t, 0, "", nil)
	cert, _, _ := f.propose(t, "fake_fsm_state:SOVEREIGN", nil)
	tx, err := EncodeAggregatedHealthCertificate(cert)
	require.NoError(t, err)

	vs, err := replayerFor(t, f, 0).ApplyBlock(collusionReqHeight, nil, [][]byte{tx})
	require.NoError(t, err)
	require.Contains(t, kinds(vs), ViolationFSMStateMismatch)
}

func TestReplay_FlagsWithdrawalCommittedWhileLocked(t *testing.T) {
	sovereign := func(m *types.PeripheralMetrics) { m.BtcGap = types.DefaultParams().SovereignThreshold }
	f := newCollusionFixture(t, 0, "", sovereign)
	cert, _, _ := f.propose(t, "", nil)
	require.Equal(t, types.StateSovereign, cert.FsmState)
	tx, err := EncodeAggregatedHealthCertificate(cert)
	require.NoError(t, err)

	vs, err := replayerFor(t, f, 0).ApplyBlock(collusionReqHeight, nil, [][]byte{tx, []byte("TX_WITHDRAWAL|x")})
	require.NoError(t, err)
	require.Equal(t, []string{ViolationWithdrawalWhileLock}, kinds(vs))
}

func TestReplay_CarriesStateAcrossBlocks(t *testing.T) {
	sovereign := func(m *types.PeripheralMetrics) { m.BtcGap = types.DefaultParams().SovereignThreshold }
	f := newCollusionFixture(t, 0, "", sovereign)
	cert, _, _ := f.propose(t, "", nil)
	tx, err := EncodeAggregatedHealthCertificate(cert)
	require.NoError(t, err)

	r := replayerFor(t, f, 0)
	_, err = r.ApplyBlock(collusionReqHeight, nil, [][]byte{tx})
	require.NoError(t, err)
	require.Equal(t, types.StateSovereign, r.FSMState())
}
