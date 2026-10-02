package sovereignty

import (
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/crypto/ed25519"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	cmttypes "github.com/cometbft/cometbft/types"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"cosmossdk.io/collections/colltest"
	log "cosmossdk.io/log/v2"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const voteExtTestChainID = "sovereignty-vote-extension-unit-test"

func newVoteExtTestKeeperCtx(t *testing.T) (*keeper.Keeper, sdk.Context) {
	t.Helper()
	storeService, ctx := colltest.MockStore()
	k := keeper.NewKeeper(storeService, nil)
	sdkCtx := sdk.NewContext(nil, cmtproto.Header{}, false, log.NewNopLogger()).WithContext(ctx).WithChainID(voteExtTestChainID)
	return k, sdkCtx
}

type voteExtTestValidator struct {
	priv ed25519.PrivKey
	addr []byte
}

// registerVoteExtTestValidators registers n genesis validators directly in
// the keeper (bypassing InitChain, matching proposal_test.go's
// newTestValidators shortcut), each with the given power.
func registerVoteExtTestValidators(t *testing.T, k *keeper.Keeper, ctx sdk.Context, powers ...int64) []voteExtTestValidator {
	t.Helper()
	vals := make([]voteExtTestValidator, len(powers))
	for i, p := range powers {
		priv := ed25519.GenPrivKey()
		pub := priv.PubKey().(ed25519.PubKey)
		addr := pub.Address()
		require.NoError(t, k.ValidatorPubKeys.Set(ctx, string(addr), types.ValidatorInfo{PubKey: pub.Bytes(), Power: p}))
		vals[i] = voteExtTestValidator{priv: priv, addr: addr}
	}
	return vals
}

// signVoteExtTestCertificate signs metrics as validator v the same way
// CometBFT signs a real vote extension (cmttypes.VoteExtensionSignBytes),
// so AggregateHealthCertificates' signature check passes.
func signVoteExtTestCertificate(t *testing.T, v voteExtTestValidator, chainID string, height int64, round int32, metrics *types.PeripheralMetrics) *types.SignedHealthCertificate {
	t.Helper()
	ext, err := proto.Marshal(metrics)
	require.NoError(t, err)
	signBytes := cmttypes.VoteExtensionSignBytes(chainID, &cmtproto.Vote{Height: height, Round: round, Extension: ext})
	sig, err := v.priv.Sign(signBytes)
	require.NoError(t, err)
	return &types.SignedHealthCertificate{
		ValidatorAddress:   v.addr,
		Round:              round,
		MetricsExtension:   ext,
		ExtensionSignature: sig,
	}
}

// --- weightedMedianUint64 ---

func TestWeightedMedianUint64_EmptyReturnsZero(t *testing.T) {
	require.Equal(t, uint64(0), weightedMedianUint64(nil, nil))
}

func TestWeightedMedianUint64_SingleValue(t *testing.T) {
	require.Equal(t, uint64(42), weightedMedianUint64([]uint64{42}, []int64{1}))
}

func TestWeightedMedianUint64_EqualPowerOddCount(t *testing.T) {
	// vals sorted: 1,2,3 -- cumulative half-power point lands on the middle.
	require.Equal(t, uint64(2), weightedMedianUint64([]uint64{3, 1, 2}, []int64{1, 1, 1}))
}

func TestWeightedMedianUint64_MinorityHighValueCannotDominate(t *testing.T) {
	// 3 honest validators (power 1 each) report 5; 1 Byzantine validator
	// (power 1) reports 1000 -- the minority-power outlier must not skew
	// the aggregate past what the honest majority actually observed.
	vals := []uint64{5, 5, 5, 1000}
	powers := []int64{1, 1, 1, 1}
	require.Equal(t, uint64(5), weightedMedianUint64(vals, powers))
}

func TestWeightedMedianUint64_PowerWeightingMatters(t *testing.T) {
	// One validator holding most of the power reports 100; two low-power
	// validators report 1 -- the high-power validator's value must win,
	// confirming this is power-weighted, not a plain unweighted median.
	vals := []uint64{1, 1, 100}
	powers := []int64{1, 1, 10}
	require.Equal(t, uint64(100), weightedMedianUint64(vals, powers))
}

// --- weightedMajorityBool ---

func TestWeightedMajorityBool_EmptyReturnsFalse(t *testing.T) {
	require.False(t, weightedMajorityBool(nil, nil))
}

func TestWeightedMajorityBool_TrueMajorityWins(t *testing.T) {
	require.True(t, weightedMajorityBool([]bool{true, true, false}, []int64{1, 1, 1}))
}

func TestWeightedMajorityBool_FalseMajorityWins(t *testing.T) {
	require.False(t, weightedMajorityBool([]bool{true, false, false}, []int64{1, 1, 1}))
}

func TestWeightedMajorityBool_ExactTieResolvesFalse(t *testing.T) {
	// truePower*2 > total is a strict inequality -- an exact 50/50 split
	// must NOT count as a true majority (no silent tie-break toward true).
	require.False(t, weightedMajorityBool([]bool{true, false}, []int64{1, 1}))
}

func TestWeightedMajorityBool_PowerWeightingMatters(t *testing.T) {
	// One high-power validator says false; two low-power validators say
	// true -- majority BY COUNT is true, but majority BY POWER is false.
	require.False(t, weightedMajorityBool([]bool{true, true, false}, []int64{1, 1, 10}))
}

// --- validatorTotalPower ---

func TestValidatorTotalPower_SumsRegisteredValidators(t *testing.T) {
	k, ctx := newVoteExtTestKeeperCtx(t)
	registerVoteExtTestValidators(t, k, ctx, 10, 20, 30)

	total, err := validatorTotalPower(ctx, k)
	require.NoError(t, err)
	require.Equal(t, int64(60), total)
}

func TestValidatorTotalPower_ZeroWhenNoValidatorsRegistered(t *testing.T) {
	k, ctx := newVoteExtTestKeeperCtx(t)
	total, err := validatorTotalPower(ctx, k)
	require.NoError(t, err)
	require.Equal(t, int64(0), total)
}

// --- certificatesFromExtendedCommit ---

func TestCertificatesFromExtendedCommit_OnlyIncludesCommitFlaggedVotes(t *testing.T) {
	extCommit := abci.ExtendedCommitInfo{
		Round: 2,
		Votes: []abci.ExtendedVoteInfo{
			{
				Validator:          abci.Validator{Address: []byte("val-commit"), Power: 1},
				VoteExtension:      []byte("ext-commit"),
				ExtensionSignature: []byte("sig-commit"),
				BlockIdFlag:        cmtproto.BlockIDFlagCommit,
			},
			{
				Validator:   abci.Validator{Address: []byte("val-absent"), Power: 1},
				BlockIdFlag: cmtproto.BlockIDFlagAbsent,
			},
			{
				Validator:   abci.Validator{Address: []byte("val-nil"), Power: 1},
				BlockIdFlag: cmtproto.BlockIDFlagNil,
			},
		},
	}

	certs := certificatesFromExtendedCommit(extCommit)
	require.Len(t, certs, 1)
	require.Equal(t, []byte("val-commit"), certs[0].ValidatorAddress)
	require.Equal(t, int32(2), certs[0].Round)
	require.Equal(t, []byte("ext-commit"), certs[0].MetricsExtension)
	require.Equal(t, []byte("sig-commit"), certs[0].ExtensionSignature)
}

func TestCertificatesFromExtendedCommit_EmptyVotesYieldsEmptyCerts(t *testing.T) {
	certs := certificatesFromExtendedCommit(abci.ExtendedCommitInfo{})
	require.Empty(t, certs)
}

// --- AggregateHealthCertificates ---

func healthyVoteExtTestMetrics() *types.PeripheralMetrics {
	return &types.PeripheralMetrics{BtcGap: 3, DaGap: 5, IsAnchorReverificationFailed: false}
}

func TestAggregateHealthCertificates_FullHonestQuorumSucceeds(t *testing.T) {
	k, ctx := newVoteExtTestKeeperCtx(t)
	vals := registerVoteExtTestValidators(t, k, ctx, 1, 1, 1, 1)
	metrics := healthyVoteExtTestMetrics()

	var certs []*types.SignedHealthCertificate
	for _, v := range vals {
		certs = append(certs, signVoteExtTestCertificate(t, v, voteExtTestChainID, 10, 0, metrics))
	}

	agg, ok := AggregateHealthCertificates(ctx, k, voteExtTestChainID, 10, certs)
	require.True(t, ok)
	require.Equal(t, uint64(3), agg.BtcGap)
	require.Equal(t, uint64(5), agg.DaGap)
	require.False(t, agg.IsAnchorReverificationFailed)
}

// da_contiguous_height is aggregated by weighted median like the other
// certified heights, so f Byzantine validators cannot pull it outside the
// honest range -- it gates RECOVERING->ANCHORED.
func TestAggregateHealthCertificates_DaContiguousHeightIsHonestBoundedMedian(t *testing.T) {
	k, ctx := newVoteExtTestKeeperCtx(t)
	vals := registerVoteExtTestValidators(t, k, ctx, 1, 1, 1, 1)

	// One validator lies high; the weighted median stays within honest values.
	heights := []uint64{40, 41, 42, 1_000_000}
	var certs []*types.SignedHealthCertificate
	for i, v := range vals {
		m := healthyVoteExtTestMetrics()
		m.DaContiguousHeight = heights[i]
		certs = append(certs, signVoteExtTestCertificate(t, v, voteExtTestChainID, 10, 0, m))
	}

	agg, ok := AggregateHealthCertificates(ctx, k, voteExtTestChainID, 10, certs)
	require.True(t, ok)
	require.GreaterOrEqual(t, agg.DaContiguousHeight, uint64(40))
	require.LessOrEqual(t, agg.DaContiguousHeight, uint64(42))
}

func TestAggregateHealthCertificates_BelowQuorumFails(t *testing.T) {
	k, ctx := newVoteExtTestKeeperCtx(t)
	vals := registerVoteExtTestValidators(t, k, ctx, 1, 1, 1, 1)
	metrics := healthyVoteExtTestMetrics()

	// Only 1 of 4 equal-power validators signs -- 1/4 < 2f+1 (=3) quorum.
	certs := []*types.SignedHealthCertificate{
		signVoteExtTestCertificate(t, vals[0], voteExtTestChainID, 10, 0, metrics),
	}

	agg, ok := AggregateHealthCertificates(ctx, k, voteExtTestChainID, 10, certs)
	require.False(t, ok)
	require.Nil(t, agg)
}

func TestAggregateHealthCertificates_NoRegisteredValidatorsFails(t *testing.T) {
	k, ctx := newVoteExtTestKeeperCtx(t)
	agg, ok := AggregateHealthCertificates(ctx, k, voteExtTestChainID, 10, nil)
	require.False(t, ok)
	require.Nil(t, agg)
}

func TestAggregateHealthCertificates_ForgedSignatureExcludedFromQuorum(t *testing.T) {
	k, ctx := newVoteExtTestKeeperCtx(t)
	vals := registerVoteExtTestValidators(t, k, ctx, 1, 1, 1, 1)
	metrics := healthyVoteExtTestMetrics()

	certs := []*types.SignedHealthCertificate{
		signVoteExtTestCertificate(t, vals[0], voteExtTestChainID, 10, 0, metrics),
		signVoteExtTestCertificate(t, vals[1], voteExtTestChainID, 10, 0, metrics),
	}
	// Tamper with the 3rd validator's signature -- must be excluded, not
	// treated as a valid vote for vals[2]'s claimed metrics.
	forged := signVoteExtTestCertificate(t, vals[2], voteExtTestChainID, 10, 0, metrics)
	forged.ExtensionSignature = append([]byte(nil), forged.ExtensionSignature...)
	forged.ExtensionSignature[0] ^= 0xFF
	certs = append(certs, forged)

	// 2 valid of 4 equal-power validators -- 2/4 < 2f+1 (=3) quorum, so the
	// forged cert being silently dropped (not counted) must fail the round.
	agg, ok := AggregateHealthCertificates(ctx, k, voteExtTestChainID, 10, certs)
	require.False(t, ok)
	require.Nil(t, agg)
}

func TestAggregateHealthCertificates_ForgedSignatureStillSucceedsIfQuorumHoldsWithoutIt(t *testing.T) {
	k, ctx := newVoteExtTestKeeperCtx(t)
	vals := registerVoteExtTestValidators(t, k, ctx, 1, 1, 1, 1)
	metrics := healthyVoteExtTestMetrics()

	certs := []*types.SignedHealthCertificate{
		signVoteExtTestCertificate(t, vals[0], voteExtTestChainID, 10, 0, metrics),
		signVoteExtTestCertificate(t, vals[1], voteExtTestChainID, 10, 0, metrics),
		signVoteExtTestCertificate(t, vals[2], voteExtTestChainID, 10, 0, metrics),
	}
	forged := signVoteExtTestCertificate(t, vals[3], voteExtTestChainID, 10, 0, metrics)
	forged.ExtensionSignature[0] ^= 0xFF
	certs = append(certs, forged)

	// 3 valid of 4 equal-power validators -- 3/4 >= 2f+1 (=3) quorum even
	// with the forged 4th cert excluded.
	agg, ok := AggregateHealthCertificates(ctx, k, voteExtTestChainID, 10, certs)
	require.True(t, ok)
	require.Equal(t, uint64(3), agg.BtcGap)
}

func TestAggregateHealthCertificates_DuplicateCertificateFromSameValidatorCountedOnce(t *testing.T) {
	k, ctx := newVoteExtTestKeeperCtx(t)
	vals := registerVoteExtTestValidators(t, k, ctx, 1, 1, 1, 1)
	metrics := healthyVoteExtTestMetrics()

	cert0 := signVoteExtTestCertificate(t, vals[0], voteExtTestChainID, 10, 0, metrics)
	certs := []*types.SignedHealthCertificate{
		cert0,
		cert0, // duplicate -- must not double-count vals[0]'s power
		signVoteExtTestCertificate(t, vals[1], voteExtTestChainID, 10, 0, metrics),
	}

	// Real distinct power backing this round is only vals[0]+vals[1] = 2,
	// below the 2f+1 (=3) quorum, even though 3 certs were submitted.
	agg, ok := AggregateHealthCertificates(ctx, k, voteExtTestChainID, 10, certs)
	require.False(t, ok)
	require.Nil(t, agg)
}

func TestAggregateHealthCertificates_UnknownValidatorExcluded(t *testing.T) {
	k, ctx := newVoteExtTestKeeperCtx(t)
	vals := registerVoteExtTestValidators(t, k, ctx, 1, 1, 1, 1)
	metrics := healthyVoteExtTestMetrics()

	stranger := ed25519.GenPrivKey()
	strangerVal := voteExtTestValidator{priv: stranger, addr: stranger.PubKey().Address()}

	certs := []*types.SignedHealthCertificate{
		signVoteExtTestCertificate(t, vals[0], voteExtTestChainID, 10, 0, metrics),
		signVoteExtTestCertificate(t, vals[1], voteExtTestChainID, 10, 0, metrics),
		// A well-signed certificate from a validator never registered in
		// genesis -- must be excluded entirely, not just unweighted.
		signVoteExtTestCertificate(t, strangerVal, voteExtTestChainID, 10, 0, metrics),
	}

	agg, ok := AggregateHealthCertificates(ctx, k, voteExtTestChainID, 10, certs)
	require.False(t, ok) // only 2/4 real power -- below 2f+1 (=3) quorum
	require.Nil(t, agg)
}

func TestAggregateHealthCertificates_WrongHeightSignatureRejected(t *testing.T) {
	k, ctx := newVoteExtTestKeeperCtx(t)
	vals := registerVoteExtTestValidators(t, k, ctx, 1, 1, 1, 1)
	metrics := healthyVoteExtTestMetrics()

	// Certificates signed for height 10 presented as if for height 11 --
	// VoteExtensionSignBytes covers height, so this must fail verification.
	var certs []*types.SignedHealthCertificate
	for _, v := range vals {
		certs = append(certs, signVoteExtTestCertificate(t, v, voteExtTestChainID, 10, 0, metrics))
	}

	agg, ok := AggregateHealthCertificates(ctx, k, voteExtTestChainID, 11, certs)
	require.False(t, ok)
	require.Nil(t, agg)
}

// --- NewExtendVoteHandler / NewVerifyVoteExtensionHandler ---

func TestNewExtendVoteHandler_NilSensorsStillProducesDecodableExtension(t *testing.T) {
	k, ctx := newVoteExtTestKeeperCtx(t)
	handler := NewExtendVoteHandler(k, nil)

	resp, err := handler(ctx, &abci.RequestExtendVote{})
	require.NoError(t, err)
	require.NotNil(t, resp)

	var decoded types.PeripheralMetrics
	require.NoError(t, proto.Unmarshal(resp.VoteExtension, &decoded))
}

func TestNewVerifyVoteExtensionHandler_AcceptsWellFormedExtension(t *testing.T) {
	k, _ := newVoteExtTestKeeperCtx(t)
	handler := NewVerifyVoteExtensionHandler(k)
	_, ctx := newVoteExtTestKeeperCtx(t)

	ext, err := proto.Marshal(healthyVoteExtTestMetrics())
	require.NoError(t, err)

	resp, err := handler(ctx, &abci.RequestVerifyVoteExtension{VoteExtension: ext})
	require.NoError(t, err)
	require.Equal(t, abci.ResponseVerifyVoteExtension_ACCEPT, resp.Status)
}

func TestNewVerifyVoteExtensionHandler_RejectsUndecodableExtension(t *testing.T) {
	k, ctx := newVoteExtTestKeeperCtx(t)
	handler := NewVerifyVoteExtensionHandler(k)

	resp, err := handler(ctx, &abci.RequestVerifyVoteExtension{VoteExtension: []byte{0xFF, 0xFF, 0xFF}})
	require.NoError(t, err)
	require.Equal(t, abci.ResponseVerifyVoteExtension_REJECT, resp.Status)
}
