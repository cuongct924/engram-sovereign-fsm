package sovereignty_test

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

	"github.com/cuongct220020/engram-sovereign-fsm/x/da"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
	"github.com/cuongct220020/engram-sovereign-fsm/x/vigilante"
)

func newTestKeeperCtx(t *testing.T) (*keeper.Keeper, sdk.Context) {
	t.Helper()
	storeService, ctx := colltest.MockStore()
	cdc := codec.NewProtoCodec(codectypes.NewInterfaceRegistry())
	k := keeper.NewKeeper(storeService, cdc, memory.NewMemoryStorage())
	sdkCtx := sdk.NewContext(nil, cmtproto.Header{}, false, log.NewNopLogger()).WithContext(ctx)
	return k, sdkCtx
}

// healthyExtendedProposal builds an ExtendedProposal that ProcessProposal
// should accept given a freshly-constructed Keeper (all heights at 0,
// healthy metrics, ANCHORED state).
func healthyExtendedProposal(k *keeper.Keeper, ctx sdk.Context) sovereignty.ExtendedProposal {
	p := k.Params
	metrics := &types.PeripheralMetrics{
		SubnetDiversity: p.MinSubnetDiversity,
		ActiveAnchors:   p.MinAnchorPeers,
		CleanPeers:      p.MinPeers,
		AvgPeerTenure:   p.MinAvgTenure,
	}
	_ = k.Metrics.Set(ctx, metrics)

	return sovereignty.ExtendedProposal{
		FSMState:   types.StateAnchored,
		DAReceipt:  da.Receipt{PublishedBlockHeight: 0, Attestation: true},
		BTCReceipt: vigilante.Receipt{CheckpointBlockHeight: 0, CheckpointBlockHash: vigilante.ExpectedBlockHash(0)},
		ZKProofRef: false,
	}
}

func TestProcessProposal_AcceptsWellFormedProposal(t *testing.T) {
	k, ctx := newTestKeeperCtx(t)
	ext := healthyExtendedProposal(k, ctx)
	tx, err := sovereignty.EncodeExtendedProposal(ext)
	require.NoError(t, err)

	resp, err := sovereignty.NewProcessProposalHandler(k)(ctx, &abci.RequestProcessProposal{Txs: [][]byte{tx}})
	require.NoError(t, err)
	require.True(t, resp.IsAccepted())
}

func TestProcessProposal_RejectsEmptyTxs(t *testing.T) {
	k, ctx := newTestKeeperCtx(t)
	resp, err := sovereignty.NewProcessProposalHandler(k)(ctx, &abci.RequestProcessProposal{Txs: nil})
	require.NoError(t, err)
	require.False(t, resp.IsAccepted())
}

func TestProcessProposal_RejectsUndecodableFirstTx(t *testing.T) {
	k, ctx := newTestKeeperCtx(t)
	resp, err := sovereignty.NewProcessProposalHandler(k)(ctx, &abci.RequestProcessProposal{Txs: [][]byte{[]byte("not an extended proposal")}})
	require.NoError(t, err)
	require.False(t, resp.IsAccepted())
}

func TestProcessProposal_RejectsFSMStateMismatch(t *testing.T) {
	k, ctx := newTestKeeperCtx(t)
	ext := healthyExtendedProposal(k, ctx)
	ext.FSMState = types.StateSovereign // wrong: local computation would say ANCHORED
	tx, err := sovereignty.EncodeExtendedProposal(ext)
	require.NoError(t, err)

	resp, err := sovereignty.NewProcessProposalHandler(k)(ctx, &abci.RequestProcessProposal{Txs: [][]byte{tx}})
	require.NoError(t, err)
	require.False(t, resp.IsAccepted(), "a proposal claiming a fsm_state that doesn't match CalculateNextState must be rejected")
}

func TestProcessProposal_RejectsMissingDAAttestation(t *testing.T) {
	k, ctx := newTestKeeperCtx(t)
	ext := healthyExtendedProposal(k, ctx)
	ext.DAReceipt.Attestation = false // ANCHORED always requires attestation
	tx, err := sovereignty.EncodeExtendedProposal(ext)
	require.NoError(t, err)

	resp, err := sovereignty.NewProcessProposalHandler(k)(ctx, &abci.RequestProcessProposal{Txs: [][]byte{tx}})
	require.NoError(t, err)
	require.False(t, resp.IsAccepted())
}

func TestProcessProposal_RejectsForgedBTCHash(t *testing.T) {
	k, ctx := newTestKeeperCtx(t)
	ext := healthyExtendedProposal(k, ctx)
	ext.BTCReceipt.CheckpointBlockHash = vigilante.BlockHash{Tag: "BTC_FORK", Height: 0}
	tx, err := sovereignty.EncodeExtendedProposal(ext)
	require.NoError(t, err)

	resp, err := sovereignty.NewProcessProposalHandler(k)(ctx, &abci.RequestProcessProposal{Txs: [][]byte{tx}})
	require.NoError(t, err)
	require.False(t, resp.IsAccepted())
}

func TestProcessProposal_RejectsWithdrawalWhileSovereign(t *testing.T) {
	k, ctx := newTestKeeperCtx(t)
	// Force SOVEREIGN via a critical BTC gap so CalculateNextState agrees.
	p := k.Params
	metrics := &types.PeripheralMetrics{
		BtcGap:          p.SovereignThreshold,
		SubnetDiversity: p.MinSubnetDiversity,
		ActiveAnchors:   p.MinAnchorPeers,
		CleanPeers:      p.MinPeers,
		AvgPeerTenure:   p.MinAvgTenure,
	}
	require.NoError(t, k.Metrics.Set(ctx, metrics))

	ext := sovereignty.ExtendedProposal{
		FSMState:   types.StateSovereign,
		DAReceipt:  da.Receipt{Attestation: true},
		BTCReceipt: vigilante.Receipt{CheckpointBlockHeight: 0, CheckpointBlockHash: vigilante.ExpectedBlockHash(0)},
	}
	tx, err := sovereignty.EncodeExtendedProposal(ext)
	require.NoError(t, err)

	resp, err := sovereignty.NewProcessProposalHandler(k)(ctx, &abci.RequestProcessProposal{
		Txs: [][]byte{tx, []byte("TX_WITHDRAWAL")},
	})
	require.NoError(t, err)
	require.False(t, resp.IsAccepted(), "circuit breaker must reject withdrawals while SOVEREIGN")
}

func TestProcessProposal_RejectsPrematureZKProofClaim(t *testing.T) {
	k, ctx := newTestKeeperCtx(t)
	ext := healthyExtendedProposal(k, ctx)
	ext.ZKProofRef = true // hysteresis not even relevant here (state isn't RECOVERING)
	tx, err := sovereignty.EncodeExtendedProposal(ext)
	require.NoError(t, err)

	resp, err := sovereignty.NewProcessProposalHandler(k)(ctx, &abci.RequestProcessProposal{Txs: [][]byte{tx}})
	require.NoError(t, err)
	require.False(t, resp.IsAccepted(), "zk_proof_ref must be false outside a hysteresis-satisfied RECOVERING state")
}

func TestProcessProposal_RejectsCensoringProposal(t *testing.T) {
	k, ctx := newTestKeeperCtx(t)
	require.NoError(t, k.ForcedTxQueue.Set(ctx, "FORCED_TX_1"))
	require.NoError(t, k.TxIgnoredRounds.Set(ctx, "FORCED_TX_1", k.Params.MaxIgnoreRounds)) // already at threshold

	ext := healthyExtendedProposal(k, ctx)
	tx, err := sovereignty.EncodeExtendedProposal(ext)
	require.NoError(t, err)

	// Proposal is otherwise well-formed but omits the forced tx.
	resp, err := sovereignty.NewProcessProposalHandler(k)(ctx, &abci.RequestProcessProposal{Txs: [][]byte{tx}})
	require.NoError(t, err)
	require.False(t, resp.IsAccepted(), "a proposal that keeps omitting a forced tx past MaxIgnoreRounds must be rejected")
}

func TestProcessProposal_AcceptsCensoredTxOnceIncluded(t *testing.T) {
	k, ctx := newTestKeeperCtx(t)
	require.NoError(t, k.ForcedTxQueue.Set(ctx, "FORCED_TX_1"))
	require.NoError(t, k.TxIgnoredRounds.Set(ctx, "FORCED_TX_1", k.Params.MaxIgnoreRounds))

	ext := healthyExtendedProposal(k, ctx)
	tx, err := sovereignty.EncodeExtendedProposal(ext)
	require.NoError(t, err)

	resp, err := sovereignty.NewProcessProposalHandler(k)(ctx, &abci.RequestProcessProposal{
		Txs: [][]byte{tx, []byte("FORCED_TX_1")},
	})
	require.NoError(t, err)
	require.True(t, resp.IsAccepted(), "including the forced tx must clear the censorship rejection")
}

func TestPreBlocker_TracksForcedTxIgnoredRounds(t *testing.T) {
	k, ctx := newTestKeeperCtx(t)
	require.NoError(t, k.ForcedTxQueue.Set(ctx, "FORCED_TX_1"))

	ext := healthyExtendedProposal(k, ctx)
	tx, err := sovereignty.EncodeExtendedProposal(ext)
	require.NoError(t, err)

	// Block 1: forced tx absent -> counter increments to 1.
	_, err = sovereignty.NewPreBlocker(k)(ctx, &abci.RequestFinalizeBlock{Txs: [][]byte{tx}})
	require.NoError(t, err)
	count, err := k.TxIgnoredRounds.Get(ctx, "FORCED_TX_1")
	require.NoError(t, err)
	require.Equal(t, uint64(1), count)

	// Block 2: forced tx included -> counter resets to 0.
	_, err = sovereignty.NewPreBlocker(k)(ctx, &abci.RequestFinalizeBlock{Txs: [][]byte{tx, []byte("FORCED_TX_1")}})
	require.NoError(t, err)
	count, err = k.TxIgnoredRounds.Get(ctx, "FORCED_TX_1")
	require.NoError(t, err)
	require.Equal(t, uint64(0), count)
}

func TestCommitFSMTransition_WritesAgreedStateAndHeights(t *testing.T) {
	k, ctx := newTestKeeperCtx(t)

	ext := sovereignty.ExtendedProposal{
		FSMState:   types.StateSuspicious,
		DAReceipt:  da.Receipt{PublishedBlockHeight: 5, Attestation: true},
		BTCReceipt: vigilante.Receipt{CheckpointBlockHeight: 3, CheckpointBlockHash: vigilante.ExpectedBlockHash(3)},
	}
	require.NoError(t, sovereignty.CommitFSMTransition(ctx, k, ext))

	state, err := k.FSMState.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, types.StateSuspicious, state)

	hBtcAnchored, err := k.HBtcAnchored.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(3), hBtcAnchored)

	hEngramVerified, err := k.HEngramVerified.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(5), hEngramVerified)
}

func TestCommitFSMTransition_ForceSyncsSensorsOnRecoveringToAnchored(t *testing.T) {
	k, ctx := newTestKeeperCtx(t)
	require.NoError(t, k.FSMState.Set(ctx, types.StateRecovering))
	require.NoError(t, k.Metrics.Set(ctx, &types.PeripheralMetrics{IsDasFailed: true, IsAttestationFailed: true}))

	ext := sovereignty.ExtendedProposal{
		FSMState:   types.StateAnchored,
		DAReceipt:  da.Receipt{PublishedBlockHeight: 10, Attestation: true},
		BTCReceipt: vigilante.Receipt{CheckpointBlockHeight: 10, CheckpointBlockHash: vigilante.ExpectedBlockHash(10)},
	}
	require.NoError(t, sovereignty.CommitFSMTransition(ctx, k, ext))

	metrics, err := k.Metrics.Get(ctx)
	require.NoError(t, err)
	require.False(t, metrics.IsDasFailed, "recovering->anchored must clear stale local failure flags")
	require.False(t, metrics.IsAttestationFailed)

	hBtcCurrent, err := k.HBtcCurrent.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(10), hBtcCurrent)
}

func TestCommitFSMTransition_TracksZKProofSubmission(t *testing.T) {
	k, ctx := newTestKeeperCtx(t)
	require.NoError(t, k.HBtcCurrent.Set(ctx, 7))
	require.NoError(t, k.ReanchoringProofValid.Set(ctx, true)) // stale; must be reset

	ext := sovereignty.ExtendedProposal{
		FSMState:   types.StateRecovering,
		DAReceipt:  da.Receipt{PublishedBlockHeight: 1, Attestation: true},
		BTCReceipt: vigilante.Receipt{CheckpointBlockHeight: 0, CheckpointBlockHash: vigilante.ExpectedBlockHash(0)},
		ZKProofRef: true,
	}
	require.NoError(t, sovereignty.CommitFSMTransition(ctx, k, ext))

	hBtcSubmitted, err := k.HBtcSubmitted.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(7), hBtcSubmitted)

	proofValid, err := k.ReanchoringProofValid.Get(ctx)
	require.NoError(t, err)
	require.False(t, proofValid, "proof is pending Bitcoin confirmation, not yet valid")
}
