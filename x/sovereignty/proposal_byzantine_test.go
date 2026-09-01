package sovereignty

import (
	"encoding/hex"
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

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const byzantineTestChainID = "sovereignty-byzantine-test"

// newByzantineTestKeeperCtx mirrors proposal_test.go's newTestKeeperCtx,
// registering a real quorum of signed vote extensions -- PrepareProposal
// derives health from AggregateHealthCertificates, never k.Metrics directly.
func newByzantineTestKeeperCtx(t *testing.T) (*keeper.Keeper, sdk.Context, abci.ExtendedCommitInfo) {
	t.Helper()
	storeService, ctx := colltest.MockStore()
	cdc := codec.NewProtoCodec(codectypes.NewInterfaceRegistry())
	k := keeper.NewKeeper(storeService, cdc)
	sdkCtx := sdk.NewContext(nil, cmtproto.Header{}, false, log.NewNopLogger()).WithContext(ctx).WithChainID(byzantineTestChainID)
	require.NoError(t, k.FSMState.Set(sdkCtx, types.StateAnchored))

	p := k.Params
	metrics := &types.PeripheralMetrics{
		SubnetDiversity: p.MinSubnetDiversity,
		ActiveAnchors:   p.MinAnchorPeers,
		CleanPeers:      p.MinPeers,
		AvgPeerTenure:   p.MinAvgTenure,
	}
	extCommit := signedExtendedCommit(t, k, sdkCtx, byzantineTestChainID, -1, metrics, 4) // req.Height defaults to 0 -> extHeight -1
	return k, sdkCtx, extCommit
}

// signedExtendedCommit registers n equal-power validators and builds an
// ExtendedCommitInfo whose votes all carry the same real, correctly signed
// PeripheralMetrics extension -- what CometBFT itself hands PrepareProposal.
func signedExtendedCommit(t *testing.T, k *keeper.Keeper, ctx sdk.Context, chainID string, height int64, metrics *types.PeripheralMetrics, n int) abci.ExtendedCommitInfo {
	t.Helper()
	ext, err := proto.Marshal(metrics)
	require.NoError(t, err)
	signBytes := cmttypes.VoteExtensionSignBytes(chainID, &cmtproto.Vote{Height: height, Round: 0, Extension: ext})

	votes := make([]abci.ExtendedVoteInfo, n)
	for i := 0; i < n; i++ {
		priv := ed25519.GenPrivKey()
		pub := priv.PubKey().(ed25519.PubKey)
		addr := pub.Address()
		require.NoError(t, k.ValidatorPubKeys.Set(ctx, string(addr), types.ValidatorInfo{PubKey: pub.Bytes(), Power: 1}))
		sig, err := priv.Sign(signBytes)
		require.NoError(t, err)
		votes[i] = abci.ExtendedVoteInfo{
			Validator:          abci.Validator{Address: addr, Power: 1},
			VoteExtension:      ext,
			ExtensionSignature: sig,
			BlockIdFlag:        cmtproto.BlockIDFlagCommit,
		}
	}
	return abci.ExtendedCommitInfo{Round: 0, Votes: votes}
}

// TestPrepareProposal_EmptyByzantineBehaviorIsNoOp guards the default
// (empty ENGRAM_BYZANTINE_BEHAVIOR): still yields the honest ANCHORED
// claim -- applyByzantineBehavior must never fire when unset.
func TestPrepareProposal_EmptyByzantineBehaviorIsNoOp(t *testing.T) {
	k, ctx, extCommit := newByzantineTestKeeperCtx(t)
	resp, err := NewPrepareProposalHandler(k, "")(ctx, &abci.RequestPrepareProposal{LocalLastCommit: extCommit})
	require.NoError(t, err)
	ext, ok, err := DecodeAggregatedHealthCertificate(resp.Txs[0])
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, types.StateAnchored, ext.FsmState)
}

// TestPrepareProposal_FakeFSMStateOverridesRealComputation is A6 (Malicious
// Proposer): fake_fsm_state:<STATE> forces the claimed fsm_state regardless
// of CalculateNextState. Honest rejection is covered separately by
// TestProcessProposal_RejectsFSMStateMismatch.
func TestPrepareProposal_FakeFSMStateOverridesRealComputation(t *testing.T) {
	k, ctx, extCommit := newByzantineTestKeeperCtx(t)
	resp, err := NewPrepareProposalHandler(k, "fake_fsm_state:SOVEREIGN")(ctx, &abci.RequestPrepareProposal{LocalLastCommit: extCommit})
	require.NoError(t, err)
	ext, ok, err := DecodeAggregatedHealthCertificate(resp.Txs[0])
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, types.StateSovereign, ext.FsmState,
		"byzantine behavior must override the real CalculateNextState result (would be ANCHORED here)")
}

// TestPrepareProposal_ForgeBTCHashTampersReceipt is A4 (Forged BTC Receipt).
func TestPrepareProposal_ForgeBTCHashTampersReceipt(t *testing.T) {
	k, ctx, extCommit := newByzantineTestKeeperCtx(t)
	require.NoError(t, k.HBtcAnchored.Set(ctx, 5))
	resp, err := NewPrepareProposalHandler(k, "forge_btc_hash")(ctx, &abci.RequestPrepareProposal{LocalLastCommit: extCommit})
	require.NoError(t, err)
	ext, ok, err := DecodeAggregatedHealthCertificate(resp.Txs[0])
	require.NoError(t, err)
	require.True(t, ok)
	require.NotEqual(t, "BTC_BLOCK", ext.BtcCheckpointBlockHashTag,
		"forge_btc_hash must replace the real ExpectedBlockHash tag")
}

// TestPrepareProposal_FalseDAAttestationClaimsUnverifiedData is A3 (Data
// Withholding).
func TestPrepareProposal_FalseDAAttestationClaimsUnverifiedData(t *testing.T) {
	k, ctx, extCommit := newByzantineTestKeeperCtx(t)
	resp, err := NewPrepareProposalHandler(k, "false_da_attestation")(ctx, &abci.RequestPrepareProposal{LocalLastCommit: extCommit})
	require.NoError(t, err)
	ext, ok, err := DecodeAggregatedHealthCertificate(resp.Txs[0])
	require.NoError(t, err)
	require.True(t, ok)
	require.True(t, ext.DaAttestation)
	require.Greater(t, ext.DaPublishedBlockHeight, uint64(0),
		"false_da_attestation must claim an advanced height with no real Publisher submission backing it")
}

// TestPrepareProposal_CensorTxOmitsTargetedTx is A7's adversarial half
// (Censorship): censor_tx:<hash> drops the targeted tx even though the
// honest req.Txs included it.
func TestPrepareProposal_CensorTxOmitsTargetedTx(t *testing.T) {
	k, ctx, extCommit := newByzantineTestKeeperCtx(t)
	target := []byte("forced-tx-content")
	other := []byte("unrelated-tx-content")
	resp, err := NewPrepareProposalHandler(k, "censor_tx:"+hex.EncodeToString(target))(
		ctx, &abci.RequestPrepareProposal{LocalLastCommit: extCommit, Txs: [][]byte{target, other}})
	require.NoError(t, err)

	require.Len(t, resp.Txs, 2, "certificate marker + the one non-censored tx")
	for _, tx := range resp.Txs[1:] {
		require.NotEqual(t, target, tx, "censor_tx must omit the targeted tx from the proposal")
	}
	require.Equal(t, other, resp.Txs[1], "the non-targeted tx must still be included")
}
