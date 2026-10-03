package sovereignty

import (
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// docs/EXPERIMENT.md's E8 collusion suite (B1-B9) at the threat model's own
// size, n=7 equal-power validators (f=2, quorum 5), checked at the ABCI
// handler level: the leader's PrepareProposal output goes through every
// honest validator's ProcessProposal, and the committed aggregate is compared
// with the range of the honest validators' own observations.

const (
	collusionN         = 7
	collusionReqHeight = int64(2)
)

type collusionFixture struct {
	k       *keeper.Keeper
	ctx     sdk.Context
	vals    []voteExtTestValidator
	metrics []*types.PeripheralMetrics // per validator, index == vals index
	byz     int                        // validators [0, byz) are Byzantine
}

// healthyObservation is a healthy reading with a small per-validator spread,
// so "within the honest range" is a real interval rather than one point.
func healthyObservation(p types.Params, i int) *types.PeripheralMetrics {
	return &types.PeripheralMetrics{
		BtcGap:           uint64(i % 3),
		DaGap:            uint64(i % 3),
		DaVerifiedHeight: 1, // a published block, so ANCHORED's DA attestation holds
		SubnetDiversity:  p.MinSubnetDiversity,
		ActiveAnchors:    p.MinAnchorPeers,
		CleanPeers:       p.MinPeers,
		AvgPeerTenure:    p.MinAvgTenure,
	}
}

// newCollusionFixture builds n validators of which the first byz report
// lie(honest observation); honest ones report mutate(honest observation).
func newCollusionFixture(t *testing.T, byz int, lie string, honest func(*types.PeripheralMetrics)) *collusionFixture {
	t.Helper()
	k, ctx := newVoteExtTestKeeperCtx(t)
	ctx = ctx.WithConsensusParams(cmtproto.ConsensusParams{Abci: &cmtproto.ABCIParams{VoteExtensionsEnableHeight: 1}})
	require.NoError(t, k.FSMState.Set(ctx, types.StateAnchored))

	powers := make([]int64, collusionN)
	for i := range powers {
		powers[i] = 1
	}
	f := &collusionFixture{k: k, ctx: ctx, vals: registerVoteExtTestValidators(t, k, ctx, powers...), byz: byz}
	for i := 0; i < collusionN; i++ {
		m := healthyObservation(k.Params, i)
		if honest != nil {
			honest(m)
		}
		if i < byz && lie != "" {
			var err error
			m, err = applyFalseObservation(m, lie)
			require.NoError(t, err)
		}
		f.metrics = append(f.metrics, m)
	}
	return f
}

// extendedCommit is what the leader's LocalLastCommit carries: one correctly
// signed extension per validator, for the previous height.
func (f *collusionFixture) extendedCommit(t *testing.T, extHeight int64) abci.ExtendedCommitInfo {
	t.Helper()
	votes := make([]abci.ExtendedVoteInfo, collusionN)
	for i, v := range f.vals {
		c := signVoteExtTestCertificate(t, v, voteExtTestChainID, extHeight, 0, f.metrics[i])
		votes[i] = abci.ExtendedVoteInfo{
			Validator:          abci.Validator{Address: v.addr, Power: 1},
			VoteExtension:      c.MetricsExtension,
			ExtensionSignature: c.ExtensionSignature,
			BlockIdFlag:        cmtproto.BlockIDFlagCommit,
		}
	}
	return abci.ExtendedCommitInfo{Round: 0, Votes: votes}
}

// proposedLastCommit is the validators' view (ProposedLastCommit) of the
// extended commit the leader received: same round and signers, no extensions.
func proposedLastCommit(ec abci.ExtendedCommitInfo) abci.CommitInfo {
	votes := make([]abci.VoteInfo, len(ec.Votes))
	for i, v := range ec.Votes {
		votes[i] = abci.VoteInfo{Validator: v.Validator, BlockIdFlag: v.BlockIdFlag}
	}
	return abci.CommitInfo{Round: ec.Round, Votes: votes}
}

// propose runs the leader (behavior "" = honest) then every honest
// validator's ProcessProposal over the same bytes.
func (f *collusionFixture) propose(t *testing.T, behavior string, txs [][]byte) (*types.AggregatedHealthCertificate, *abci.ResponsePrepareProposal, bool) {
	t.Helper()
	prep, err := NewPrepareProposalHandler(f.k, behavior)(f.ctx, &abci.RequestPrepareProposal{
		Height:          collusionReqHeight,
		LocalLastCommit: f.extendedCommit(t, collusionReqHeight-1),
		Txs:             txs,
	})
	require.NoError(t, err)
	cert, ok, err := DecodeAggregatedHealthCertificate(prep.Txs[0])
	require.NoError(t, err)
	require.True(t, ok)

	resp, err := NewProcessProposalHandler(f.k)(f.ctx, &abci.RequestProcessProposal{
		Height: collusionReqHeight, Txs: prep.Txs, ProposedLastCommit: proposedLastCommit(f.extendedCommit(t, collusionReqHeight-1)),
	})
	require.NoError(t, err)
	return cert, prep, resp.IsAccepted()
}

// aggregateWithinHonestRange is replay-fsm's per-field check: every numeric
// field of the committed aggregate lies in [min, max] of the honest
// validators' own observations, every boolean agrees with at least one honest.
func (f *collusionFixture) aggregateWithinHonestRange(t *testing.T, cert *types.AggregatedHealthCertificate) bool {
	t.Helper()
	agg, ok := AggregateHealthCertificates(f.ctx, f.k, voteExtTestChainID, cert.Height, cert.Certificates)
	require.True(t, ok)
	honest := f.metrics[f.byz:]
	return len(metricsOutsideRange(agg, honest)) == 0
}

func TestCollusion_B1_CoordinatedFalseObservations_HonestLeader(t *testing.T) {
	lie := `{"btc_gap":"6","da_gap":"40","is_das_failed":true,"is_anchor_reverification_failed":true}`
	f := newCollusionFixture(t, 2, lie, nil)
	cert, _, accepted := f.propose(t, "", nil)

	require.True(t, accepted)
	require.Equal(t, types.StateAnchored, cert.FsmState, "2 coordinated liars (f) must not move a healthy chain")
	require.True(t, f.aggregateWithinHonestRange(t, cert))
}

func TestCollusion_B2_ByzantineLeaderBiasesCertificateSet(t *testing.T) {
	lie := `{"btc_gap":"6","da_gap":"40","is_das_failed":true,"is_anchor_reverification_failed":true}`
	f := newCollusionFixture(t, 2, lie, nil)
	cert, _, accepted := f.propose(t, "bias_certificate", nil)

	require.Len(t, cert.Certificates, 5, "bias_certificate keeps exactly a quorum")
	require.True(t, accepted, "a quorum-sized biased set is a valid proposal")
	require.True(t, f.aggregateWithinHonestRange(t, cert),
		"2 Byzantine + 3 worst honest: the weighted median is still an honest value (Lemma 1)")
	require.Equal(t, types.StateAnchored, cert.FsmState)
}

func TestCollusion_B2_ByzantineLeaderCannotHideRealOutage(t *testing.T) {
	// All honest validators see DA down; the 2 colluders report healthy.
	honestDown := func(m *types.PeripheralMetrics) { m.IsDasFailed = true; m.IsAttestationFailed = true; m.DaGap = 40 }
	f := newCollusionFixture(t, 0, "", honestDown)
	for i := 0; i < 2; i++ {
		f.metrics[i] = healthyObservation(f.k.Params, i)
	}
	f.byz = 2
	cert, _, _ := f.propose(t, "bias_certificate:best", nil)

	require.True(t, f.aggregateWithinHonestRange(t, cert), "healthy-biased set must still land in the honest range")
	// The state itself only moves after the down-hysteresis streak; the
	// certified health must already show the outage.
	require.False(t, cert.Healthy, "the real DA outage must still be reflected")
}

func TestCollusion_B3a_OmitDownToExactlyQuorumIsAccepted(t *testing.T) {
	f := newCollusionFixture(t, 2, `{"btc_gap":"6"}`, nil)
	cert, _, accepted := f.propose(t, "omit_certs:5", nil)

	require.Len(t, cert.Certificates, 5)
	require.True(t, accepted)
	require.True(t, f.aggregateWithinHonestRange(t, cert))
}

func TestCollusion_B3b_OmitBelowQuorumIsRejected(t *testing.T) {
	f := newCollusionFixture(t, 2, `{"btc_gap":"6"}`, nil)
	_, _, accepted := f.propose(t, "omit_certs:2", nil)
	require.False(t, accepted, "below 2f+1 once extensions exist: sub_quorum_certificate")
}

func TestCollusion_B5_WithdrawalInSovereignIsRejected(t *testing.T) {
	sovereign := func(m *types.PeripheralMetrics) { m.BtcGap = types.DefaultParams().SovereignThreshold }
	f := newCollusionFixture(t, 2, "", sovereign)
	withdrawal := []byte("TX_WITHDRAWAL|honest-user")

	// Honest leader filters it out ...
	cert, prep, accepted := f.propose(t, "", [][]byte{withdrawal})
	require.Equal(t, types.StateSovereign, cert.FsmState)
	require.True(t, accepted)
	require.Len(t, prep.Txs, 1, "honest leader drops the withdrawal while locked")

	// ... a Byzantine leader that includes one gets the block rejected.
	_, prep, accepted = f.propose(t, "include_withdrawal", [][]byte{withdrawal})
	require.Greater(t, len(prep.Txs), 1)
	require.False(t, accepted, "withdrawal_locked: honest validators never prevote it")
}

func TestCollusion_B6_StaleReceiptIsRejected(t *testing.T) {
	synced := func(m *types.PeripheralMetrics) {
		m.BtcCurrentHeight, m.ConfirmedAnchorHeight, m.DaVerifiedHeight = 100, 100, 100
	}
	f := newCollusionFixture(t, 2, "", synced)
	require.NoError(t, f.k.HEngramVerified.Set(f.ctx, 100))
	require.NoError(t, f.k.HBtcAnchored.Set(f.ctx, 100))

	// Baseline for the test itself: with these heights an honest proposal is valid.
	req := func(behavior string) bool {
		ec := f.extendedCommit(t, 100)
		prep, err := NewPrepareProposalHandler(f.k, behavior)(f.ctx, &abci.RequestPrepareProposal{
			Height: 101, LocalLastCommit: ec,
		})
		require.NoError(t, err)
		resp, err := NewProcessProposalHandler(f.k)(f.ctx, &abci.RequestProcessProposal{Height: 101, Txs: prep.Txs, ProposedLastCommit: proposedLastCommit(ec)})
		require.NoError(t, err)
		return resp.IsAccepted()
	}
	require.True(t, req(""), "honest proposal at these heights must be accepted")
	require.False(t, req("stale_receipt"), "a 50-height-old receipt fails freshness / monotonicity")
}

// B9: the sweep's negative control. With b = f+1 = 3 colluders, a Byzantine
// leader can pull the aggregate outside the honest range -- the proposal is
// still cryptographically valid, so honest validators cannot reject it.
func TestCollusion_B9_BeyondThresholdByzantineLeaderEscapesHonestRange(t *testing.T) {
	lie := `{"btc_gap":"6","da_gap":"40","is_das_failed":true,"is_anchor_reverification_failed":true}`

	for _, b := range []int{0, 1, 2} {
		f := newCollusionFixture(t, b, lie, nil)
		cert, _, accepted := f.propose(t, "bias_certificate", nil)
		require.True(t, accepted, "b=%d", b)
		require.True(t, f.aggregateWithinHonestRange(t, cert), "b=%d <= f must stay honest-bounded", b)
		require.Equal(t, types.StateAnchored, cert.FsmState, "b=%d", b)
	}

	f := newCollusionFixture(t, 3, lie, nil)
	cert, _, accepted := f.propose(t, "bias_certificate", nil)
	require.True(t, accepted, "the forced transition is a valid proposal")
	require.False(t, f.aggregateWithinHonestRange(t, cert), "b=f+1 with a Byzantine leader escapes the honest range")
	require.NotEqual(t, types.StateAnchored, cert.FsmState)

	// Same 3 liars, honest leader that sees all 7 certificates: 4 honest > 7/2.
	cert, _, accepted = f.propose(t, "", nil)
	require.True(t, accepted)
	require.True(t, f.aggregateWithinHonestRange(t, cert), "an honest leader keeps even b=3 honest-bounded")
}

func TestApplyFalseObservation(t *testing.T) {
	base := &types.PeripheralMetrics{BtcGap: 1, DaGap: 2, CleanPeers: 9}
	lied, err := applyFalseObservation(base, `{"btc_gap":"7","is_das_failed":true}`)
	require.NoError(t, err)
	require.Equal(t, uint64(7), lied.BtcGap)
	require.True(t, lied.IsDasFailed)
	require.Equal(t, uint64(2), lied.DaGap, "fields not named stay honest")
	require.Equal(t, uint64(9), lied.CleanPeers)

	_, err = applyFalseObservation(base, `{"no_such_field":1}`)
	require.Error(t, err)
	_, err = applyFalseObservation(base, `not json`)
	require.Error(t, err)
}

func TestExtendVoteHandler_FalseObservationSignsTheLie(t *testing.T) {
	k, ctx := newVoteExtTestKeeperCtx(t)
	resp, err := NewExtendVoteHandler(k, nil, `false_observation:{"btc_gap":"9"}`)(ctx, &abci.RequestExtendVote{})
	require.NoError(t, err)
	var m types.PeripheralMetrics
	require.NoError(t, proto.Unmarshal(resp.VoteExtension, &m))
	require.Equal(t, uint64(9), m.BtcGap)

	// A malformed spec must not crash the node; it signs the honest reading.
	resp, err = NewExtendVoteHandler(k, nil, `false_observation:oops`)(ctx, &abci.RequestExtendVote{})
	require.NoError(t, err)
	var honest types.PeripheralMetrics
	require.NoError(t, proto.Unmarshal(resp.VoteExtension, &honest))
	require.Equal(t, uint64(0), honest.BtcGap)
}

func TestSplitByzantineBehaviorsComposes(t *testing.T) {
	parts := splitByzantineBehaviors(`false_observation:{"btc_gap":"6"}; bias_certificate ;`)
	require.Equal(t, []string{`false_observation:{"btc_gap":"6"}`, "bias_certificate"}, parts)
	require.True(t, hasByzantineBehavior(parts, "bias_certificate"))
	require.False(t, hasByzantineBehavior(parts, "omit_certs"))
	require.True(t, hasByzantineBehavior([]string{"bias_certificate:best"}, "bias_certificate"))
}
