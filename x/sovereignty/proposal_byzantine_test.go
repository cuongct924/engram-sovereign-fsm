package sovereignty

import (
	"testing"

	"cosmossdk.io/collections/colltest"
	log "cosmossdk.io/log/v2"
	abci "github.com/cometbft/cometbft/abci/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/iden3/go-merkletree-sql/v2/db/memory"
	"github.com/stretchr/testify/require"

	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
)

// newByzantineTestKeeperCtx mirrors proposal_test.go's newTestKeeperCtx
// (external package, can't be reused directly from here) with healthy
// metrics preset so CalculateNextState computes ANCHORED absent any
// byzantine override -- makes fake_fsm_state's effect observable by
// comparison.
func newByzantineTestKeeperCtx(t *testing.T) (*keeper.Keeper, sdk.Context) {
	t.Helper()
	storeService, ctx := colltest.MockStore()
	cdc := codec.NewProtoCodec(codectypes.NewInterfaceRegistry())
	k := keeper.NewKeeper(storeService, cdc, memory.NewMemoryStorage())
	sdkCtx := sdk.NewContext(nil, cmtproto.Header{}, false, log.NewNopLogger()).WithContext(ctx)

	p := k.Params
	metrics := &types.PeripheralMetrics{
		SubnetDiversity: p.MinSubnetDiversity,
		ActiveAnchors:   p.MinAnchorPeers,
		CleanPeers:      p.MinPeers,
		AvgPeerTenure:   p.MinAvgTenure,
	}
	require.NoError(t, k.Metrics.Set(sdkCtx, metrics))
	require.NoError(t, k.FSMState.Set(sdkCtx, types.StateAnchored))
	return k, sdkCtx
}

// TestPrepareProposal_EmptyByzantineBehaviorIsNoOp confirms the default
// (empty ENGRAM_BYZANTINE_BEHAVIOR, every real validator) produces the same
// honest ANCHORED claim as before this field existed -- a regression guard
// against applyByzantineBehavior ever accidentally firing when unset.
func TestPrepareProposal_EmptyByzantineBehaviorIsNoOp(t *testing.T) {
	k, ctx := newByzantineTestKeeperCtx(t)
	resp, err := NewPrepareProposalHandler(k, nil, "")(ctx, &abci.RequestPrepareProposal{})
	require.NoError(t, err)
	ext, ok, err := DecodeExtendedProposal(resp.Txs[0])
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, types.StateAnchored, ext.FSMState)
}

// TestPrepareProposal_FakeFSMStateOverridesRealComputation is A6 (Malicious
// Proposer): confirms ENGRAM_BYZANTINE_BEHAVIOR=fake_fsm_state:<STATE>
// actually forces the claimed fsm_state, regardless of what
// CalculateNextState really computed -- the mutation
// docker/engram-validator-node04-byzantine.yml's live scenario depends on.
// The complementary half (that the OTHER 3 honest validators' real
// ProcessProposal rejects this) is already covered by
// TestProcessProposal_RejectsFSMStateMismatch (proposal_test.go) -- this
// test only needs to confirm the byzantine leader-side production, not
// re-prove the already-covered rejection.
func TestPrepareProposal_FakeFSMStateOverridesRealComputation(t *testing.T) {
	k, ctx := newByzantineTestKeeperCtx(t)
	resp, err := NewPrepareProposalHandler(k, nil, "fake_fsm_state:SOVEREIGN")(ctx, &abci.RequestPrepareProposal{})
	require.NoError(t, err)
	ext, ok, err := DecodeExtendedProposal(resp.Txs[0])
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, types.StateSovereign, ext.FSMState,
		"byzantine behavior must override the real CalculateNextState result (would be ANCHORED here)")
}

// TestPrepareProposal_ForgeBTCHashTampersReceipt is A4 (Forged BTC Receipt).
func TestPrepareProposal_ForgeBTCHashTampersReceipt(t *testing.T) {
	k, ctx := newByzantineTestKeeperCtx(t)
	require.NoError(t, k.HBtcAnchored.Set(ctx, 5))
	resp, err := NewPrepareProposalHandler(k, nil, "forge_btc_hash")(ctx, &abci.RequestPrepareProposal{})
	require.NoError(t, err)
	ext, ok, err := DecodeExtendedProposal(resp.Txs[0])
	require.NoError(t, err)
	require.True(t, ok)
	require.NotEqual(t, "BTC_BLOCK", ext.BTCReceipt.CheckpointBlockHash.Tag,
		"forge_btc_hash must replace the real ExpectedBlockHash tag")
}

// TestPrepareProposal_FalseDAAttestationClaimsUnverifiedData is A3 (Data
// Withholding).
func TestPrepareProposal_FalseDAAttestationClaimsUnverifiedData(t *testing.T) {
	k, ctx := newByzantineTestKeeperCtx(t)
	resp, err := NewPrepareProposalHandler(k, nil, "false_da_attestation")(ctx, &abci.RequestPrepareProposal{})
	require.NoError(t, err)
	ext, ok, err := DecodeExtendedProposal(resp.Txs[0])
	require.NoError(t, err)
	require.True(t, ok)
	require.True(t, ext.DAReceipt.Attestation)
	require.Greater(t, ext.DAReceipt.PublishedBlockHeight, uint64(0),
		"false_da_attestation must claim an advanced height with no real Publisher submission backing it")
}

// TestPrepareProposal_CensorTxOmitsTargetedTx is A7's adversarial half
// (Censorship): confirms censor_tx:<hash> actually drops the targeted tx
// from the outgoing Txs, even though the honest mempool/req.Txs included it.
func TestPrepareProposal_CensorTxOmitsTargetedTx(t *testing.T) {
	k, ctx := newByzantineTestKeeperCtx(t)
	target := []byte("forced-tx-content")
	other := []byte("unrelated-tx-content")
	resp, err := NewPrepareProposalHandler(k, nil, "censor_tx:"+string(target))(
		ctx, &abci.RequestPrepareProposal{Txs: [][]byte{target, other}})
	require.NoError(t, err)

	require.Len(t, resp.Txs, 2, "extended-proposal marker + the one non-censored tx")
	for _, tx := range resp.Txs[1:] {
		require.NotEqual(t, target, tx, "censor_tx must omit the targeted tx from the proposal")
	}
	require.Equal(t, other, resp.Txs[1], "the non-targeted tx must still be included")
}
