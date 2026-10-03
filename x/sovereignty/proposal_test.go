package sovereignty_test

import (
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/crypto/ed25519"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	cmttypes "github.com/cometbft/cometbft/types"
	"github.com/cuongct220020/engram-sovereign-fsm/x/anchor"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty"
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

const testChainID = "sovereignty-unit-test"

func newTestKeeperCtx(t *testing.T) (*keeper.Keeper, sdk.Context) {
	t.Helper()
	storeService, ctx := colltest.MockStore()
	cdc := codec.NewProtoCodec(codectypes.NewInterfaceRegistry())
	k := keeper.NewKeeper(storeService, cdc)
	sdkCtx := sdk.NewContext(nil, cmtproto.Header{}, false, log.NewNopLogger()).WithContext(ctx).WithChainID(testChainID)
	return k, sdkCtx
}

// testValidator bundles a registered genesis validator's key material so
// tests can sign real ABCI++ vote extensions -- AggregateHealthCertificates
// verifies a real ed25519 signature per certificate.
type testValidator struct {
	priv ed25519.PrivKey
	addr []byte
}

// newTestValidators registers n equal-power genesis validators directly in
// the keeper (bypassing InitChain, same shortcut other tests here take with
// k.Metrics/k.FSMState).
func newTestValidators(t *testing.T, k *keeper.Keeper, ctx sdk.Context, n int) []testValidator {
	t.Helper()
	vals := make([]testValidator, n)
	for i := 0; i < n; i++ {
		priv := ed25519.GenPrivKey()
		pub := priv.PubKey().(ed25519.PubKey)
		addr := pub.Address()
		require.NoError(t, k.ValidatorPubKeys.Set(ctx, string(addr), types.ValidatorInfo{PubKey: pub.Bytes(), Power: 1}))
		vals[i] = testValidator{priv: priv, addr: addr}
	}
	return vals
}

// signCertificates builds one SignedHealthCertificate per validator, all
// reporting the same metrics, signed exactly the way CometBFT signs a real
// vote extension (cmttypes.VoteExtensionSignBytes) so
// AggregateHealthCertificates' signature check passes.
func signCertificates(t *testing.T, vals []testValidator, chainID string, height int64, metrics *types.PeripheralMetrics) []*types.SignedHealthCertificate {
	t.Helper()
	return signCertificatesAtRound(t, vals, chainID, height, 0, metrics)
}

// signCertificatesAtRound is signCertificates for a commit round other than 0.
func signCertificatesAtRound(t *testing.T, vals []testValidator, chainID string, height int64, round int32, metrics *types.PeripheralMetrics) []*types.SignedHealthCertificate {
	t.Helper()
	ext, err := proto.Marshal(metrics)
	require.NoError(t, err)
	signBytes := cmttypes.VoteExtensionSignBytes(chainID, &cmtproto.Vote{Height: height, Round: round, Extension: ext})

	certs := make([]*types.SignedHealthCertificate, len(vals))
	for i, v := range vals {
		sig, err := v.priv.Sign(signBytes)
		require.NoError(t, err)
		certs[i] = &types.SignedHealthCertificate{
			ValidatorAddress:   v.addr,
			Round:              round,
			MetricsExtension:   ext,
			ExtensionSignature: sig,
		}
	}
	return certs
}

// healthyAggregatedCertificate builds an AggregatedHealthCertificate that
// ProcessProposal should accept at height reqHeight, given a fresh Keeper
// (committed heights at 0, healthy metrics, ANCHORED state).
func healthyAggregatedCertificate(t *testing.T, k *keeper.Keeper, ctx sdk.Context, reqHeight int64) *types.AggregatedHealthCertificate {
	t.Helper()
	return healthyAggregatedCertificateAtRound(t, k, ctx, reqHeight, 0)
}

// healthyAggregatedCertificateAtRound signs the observations at commitRound of
// height reqHeight-1, as when that height needed more than one round to commit.
func healthyAggregatedCertificateAtRound(t *testing.T, k *keeper.Keeper, ctx sdk.Context, reqHeight int64, commitRound int32) *types.AggregatedHealthCertificate {
	t.Helper()
	p := k.Params
	metrics := &types.PeripheralMetrics{
		SubnetDiversity: p.MinSubnetDiversity,
		ActiveAnchors:   p.MinAnchorPeers,
		CleanPeers:      p.MinPeers,
		AvgPeerTenure:   p.MinAvgTenure,
	}
	vals := newTestValidators(t, k, ctx, 4)
	extHeight := reqHeight - 1
	certs := signCertificatesAtRound(t, vals, testChainID, extHeight, commitRound, metrics)

	return &types.AggregatedHealthCertificate{
		Height:                       extHeight,
		Certificates:                 certs,
		FsmState:                     types.StateAnchored,
		Healthy:                      true, // matches the healthy metrics set above -- see check #1b's doc
		DaPublishedBlockHeight:       0,
		DaAttestation:                true,
		BtcCheckpointBlockHeight:     0,
		BtcCheckpointBlockHashTag:    anchor.ExpectedBlockHash(0).Tag,
		BtcCheckpointBlockHashHeight: anchor.ExpectedBlockHash(0).Height,
	}
}

func TestProcessProposal_AcceptsWellFormedProposal(t *testing.T) {
	k, ctx := newTestKeeperCtx(t)
	cert := healthyAggregatedCertificate(t, k, ctx, 1)
	tx, err := sovereignty.EncodeAggregatedHealthCertificate(cert)
	require.NoError(t, err)

	resp, err := sovereignty.NewProcessProposalHandler(k)(ctx, &abci.RequestProcessProposal{Txs: [][]byte{tx}, Height: 1})
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
	resp, err := sovereignty.NewProcessProposalHandler(k)(ctx, &abci.RequestProcessProposal{Txs: [][]byte{[]byte("not a certificate")}})
	require.NoError(t, err)
	require.False(t, resp.IsAccepted())
}

func TestProcessProposal_RejectsFSMStateMismatch(t *testing.T) {
	k, ctx := newTestKeeperCtx(t)
	cert := healthyAggregatedCertificate(t, k, ctx, 1)
	cert.FsmState = types.StateSovereign // wrong: local computation would say ANCHORED
	tx, err := sovereignty.EncodeAggregatedHealthCertificate(cert)
	require.NoError(t, err)

	resp, err := sovereignty.NewProcessProposalHandler(k)(ctx, &abci.RequestProcessProposal{Txs: [][]byte{tx}, Height: 1})
	require.NoError(t, err)
	require.False(t, resp.IsAccepted(), "a proposal claiming a fsm_state that doesn't match CalculateNextState must be rejected")
}

func TestProcessProposal_RejectsMissingDAAttestation(t *testing.T) {
	k, ctx := newTestKeeperCtx(t)
	cert := healthyAggregatedCertificate(t, k, ctx, 1)
	cert.DaAttestation = false // ANCHORED always requires attestation
	tx, err := sovereignty.EncodeAggregatedHealthCertificate(cert)
	require.NoError(t, err)

	resp, err := sovereignty.NewProcessProposalHandler(k)(ctx, &abci.RequestProcessProposal{Txs: [][]byte{tx}, Height: 1})
	require.NoError(t, err)
	require.False(t, resp.IsAccepted())
}

func TestProcessProposal_RejectsForgedBTCHash(t *testing.T) {
	k, ctx := newTestKeeperCtx(t)
	cert := healthyAggregatedCertificate(t, k, ctx, 1)
	cert.BtcCheckpointBlockHashTag = "BTC_FORK"
	cert.BtcCheckpointBlockHashHeight = 0
	tx, err := sovereignty.EncodeAggregatedHealthCertificate(cert)
	require.NoError(t, err)

	resp, err := sovereignty.NewProcessProposalHandler(k)(ctx, &abci.RequestProcessProposal{Txs: [][]byte{tx}, Height: 1})
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
	vals := newTestValidators(t, k, ctx, 4)
	certs := signCertificates(t, vals, testChainID, 0, metrics)

	cert := &types.AggregatedHealthCertificate{
		Height:                       0,
		Certificates:                 certs,
		FsmState:                     types.StateSovereign,
		DaAttestation:                true,
		BtcCheckpointBlockHeight:     0,
		BtcCheckpointBlockHashTag:    anchor.ExpectedBlockHash(0).Tag,
		BtcCheckpointBlockHashHeight: anchor.ExpectedBlockHash(0).Height,
	}
	tx, err := sovereignty.EncodeAggregatedHealthCertificate(cert)
	require.NoError(t, err)

	resp, err := sovereignty.NewProcessProposalHandler(k)(ctx, &abci.RequestProcessProposal{
		Txs: [][]byte{tx, []byte("TX_WITHDRAWAL")}, Height: 1,
	})
	require.NoError(t, err)
	require.False(t, resp.IsAccepted(), "circuit breaker must reject withdrawals while SOVEREIGN")
}

func TestProcessProposal_RejectsPrematureZKProofClaim(t *testing.T) {
	k, ctx := newTestKeeperCtx(t)
	cert := healthyAggregatedCertificate(t, k, ctx, 1)
	cert.ZkProofRef = []byte("claimed-root") // hysteresis not even relevant here (state isn't RECOVERING)
	tx, err := sovereignty.EncodeAggregatedHealthCertificate(cert)
	require.NoError(t, err)

	resp, err := sovereignty.NewProcessProposalHandler(k)(ctx, &abci.RequestProcessProposal{Txs: [][]byte{tx}, Height: 1})
	require.NoError(t, err)
	require.False(t, resp.IsAccepted(), "zk_proof_ref must be false outside a hysteresis-satisfied RECOVERING state")
}

func TestProcessProposal_RejectsCensoringProposal(t *testing.T) {
	k, ctx := newTestKeeperCtx(t)
	require.NoError(t, k.ForcedTxQueue.Set(ctx, "FORCED_TX_1"))
	require.NoError(t, k.TxIgnoredRounds.Set(ctx, "FORCED_TX_1", k.Params.MaxIgnoreRounds)) // already at threshold

	cert := healthyAggregatedCertificate(t, k, ctx, 1)
	tx, err := sovereignty.EncodeAggregatedHealthCertificate(cert)
	require.NoError(t, err)

	// Proposal is otherwise well-formed but omits the forced tx.
	resp, err := sovereignty.NewProcessProposalHandler(k)(ctx, &abci.RequestProcessProposal{Txs: [][]byte{tx}, Height: 1})
	require.NoError(t, err)
	require.False(t, resp.IsAccepted(), "a proposal that keeps omitting a forced tx past MaxIgnoreRounds must be rejected")
}

func TestProcessProposal_AcceptsCensoringProposalPastMaxCensorshipRounds(t *testing.T) {
	k, ctx := newTestKeeperCtx(t)
	require.NoError(t, k.ForcedTxQueue.Set(ctx, "FORCED_TX_1"))
	require.NoError(t, k.TxIgnoredRounds.Set(ctx, "FORCED_TX_1", k.Params.MaxIgnoreRounds))

	cert := healthyAggregatedCertificate(t, k, ctx, 1)
	tx, err := sovereignty.EncodeAggregatedHealthCertificate(cert)
	require.NoError(t, err)

	// Same as TestProcessProposal_RejectsCensoringProposal (forced tx still
	// omitted), but at MaxCensorshipRounds -- a forced tx that can never be
	// included must not block the chain forever (types.Params' doc).
	resp, err := sovereignty.NewProcessProposalHandler(k)(ctx, &abci.RequestProcessProposal{
		Txs:    [][]byte{tx},
		Height: 1,
		Round:  int32(k.Params.MaxCensorshipRounds),
	})
	require.NoError(t, err)
	require.True(t, resp.IsAccepted(), "IsCensoring must stop being enforced past MaxCensorshipRounds")
}

func TestProcessProposal_AcceptsCensoredTxOnceIncluded(t *testing.T) {
	k, ctx := newTestKeeperCtx(t)
	require.NoError(t, k.ForcedTxQueue.Set(ctx, "FORCED_TX_1"))
	require.NoError(t, k.TxIgnoredRounds.Set(ctx, "FORCED_TX_1", k.Params.MaxIgnoreRounds))

	cert := healthyAggregatedCertificate(t, k, ctx, 1)
	tx, err := sovereignty.EncodeAggregatedHealthCertificate(cert)
	require.NoError(t, err)

	resp, err := sovereignty.NewProcessProposalHandler(k)(ctx, &abci.RequestProcessProposal{
		Txs: [][]byte{tx, []byte("FORCED_TX_1")}, Height: 1,
	})
	require.NoError(t, err)
	require.True(t, resp.IsAccepted(), "including the forced tx must clear the censorship rejection")
}

func TestPreBlocker_TracksForcedTxIgnoredRounds(t *testing.T) {
	k, ctx := newTestKeeperCtx(t)
	require.NoError(t, k.ForcedTxQueue.Set(ctx, "FORCED_TX_1"))

	cert := healthyAggregatedCertificate(t, k, ctx, 1)
	tx, err := sovereignty.EncodeAggregatedHealthCertificate(cert)
	require.NoError(t, err)

	// Block 1: forced tx absent -> counter increments to 1.
	_, err = sovereignty.NewPreBlocker(k)(ctx, &abci.RequestFinalizeBlock{Txs: [][]byte{tx}})
	require.NoError(t, err)
	count, err := k.TxIgnoredRounds.Get(ctx, "FORCED_TX_1")
	require.NoError(t, err)
	require.Equal(t, uint64(1), count)

	// Block 2: forced tx included -> dequeued entirely, not merely reset to 0
	// and left queued. Txs are one-shot, so a leftover queued tx would trip
	// IsCensoring on every future round (see updateForcedTxTracking's doc).
	_, err = sovereignty.NewPreBlocker(k)(ctx, &abci.RequestFinalizeBlock{Txs: [][]byte{tx, []byte("FORCED_TX_1")}})
	require.NoError(t, err)
	_, err = k.TxIgnoredRounds.Get(ctx, "FORCED_TX_1")
	require.Error(t, err, "included forced tx must be dequeued, not left at count 0")
	has, err := k.ForcedTxQueue.Has(ctx, "FORCED_TX_1")
	require.NoError(t, err)
	require.False(t, has, "included forced tx must be removed from ForcedTxQueue")
}

func TestCommitFSMTransition_WritesAgreedStateAndHeights(t *testing.T) {
	k, ctx := newTestKeeperCtx(t)

	cert := &types.AggregatedHealthCertificate{
		FsmState:                 types.StateSuspicious,
		DaPublishedBlockHeight:   5,
		DaAttestation:            true,
		BtcCheckpointBlockHeight: 3,
	}
	require.NoError(t, sovereignty.CommitFSMTransition(ctx, k, cert))

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

	// h_btc_current comes from the certified quorum median of the embedded
	// certificates (PeripheralMetrics.BtcCurrentHeight), not from
	// BtcCheckpointBlockHeight -- see preblock.go's CommitFSMTransition doc.
	vals := newTestValidators(t, k, ctx, 4)
	const extHeight = int64(0)
	certs := signCertificates(t, vals, testChainID, extHeight, &types.PeripheralMetrics{BtcCurrentHeight: 10})

	cert := &types.AggregatedHealthCertificate{
		Height:                   extHeight,
		Certificates:             certs,
		FsmState:                 types.StateAnchored,
		DaPublishedBlockHeight:   10,
		DaAttestation:            true,
		BtcCheckpointBlockHeight: 10,
	}
	require.NoError(t, sovereignty.CommitFSMTransition(ctx, k, cert))

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
	require.NoError(t, k.ReanchoringProofValid.Set(ctx, true)) // stale; must be reset

	// h_btc_current comes from the certified quorum median (see
	// TestCommitFSMTransition_ForceSyncsSensorsOnRecoveringToAnchored's
	// matching comment), not a pre-set keeper value.
	vals := newTestValidators(t, k, ctx, 4)
	const extHeight = int64(0)
	certs := signCertificates(t, vals, testChainID, extHeight, &types.PeripheralMetrics{BtcCurrentHeight: 7})

	cert := &types.AggregatedHealthCertificate{
		Height:                   extHeight,
		Certificates:             certs,
		FsmState:                 types.StateRecovering,
		DaPublishedBlockHeight:   1,
		DaAttestation:            true,
		BtcCheckpointBlockHeight: 0,
		ZkProofRef:               []byte("claimed-root"),
	}
	require.NoError(t, sovereignty.CommitFSMTransition(ctx, k, cert))

	hBtcSubmitted, err := k.HBtcSubmitted.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(7), hBtcSubmitted)

	proofValid, err := k.ReanchoringProofValid.Get(ctx)
	require.NoError(t, err)
	require.False(t, proofValid, "proof is pending Bitcoin confirmation, not yet valid")
}

// withVoteExtensionsEnabledAt mirrors cmd/engramd's genesis, which enables
// vote extensions from height 1 (so height 2 is the first proposal whose
// LocalLastCommit carries them).
func withVoteExtensionsEnabledAt(ctx sdk.Context, height int64) sdk.Context {
	return ctx.WithConsensusParams(cmtproto.ConsensusParams{
		Abci: &cmtproto.ABCIParams{VoteExtensionsEnableHeight: height},
	})
}

// commitInfoOf is the ProposedLastCommit every validator sees when all of
// certs' signers precommitted the previous height in the given round.
func commitInfoOf(certs []*types.SignedHealthCertificate, round int32) abci.CommitInfo {
	votes := make([]abci.VoteInfo, len(certs))
	for i, c := range certs {
		votes[i] = abci.VoteInfo{
			Validator:   abci.Validator{Address: c.ValidatorAddress, Power: 1},
			BlockIdFlag: cmtproto.BlockIDFlagCommit,
		}
	}
	return abci.CommitInfo{Round: round, Votes: votes}
}

// omittedQuorumProposal is a proposer keeping only 2 of 4 equal-power
// certificates (below the 2f+1=3 quorum) and claiming the SOVEREIGN state a
// maximally degraded aggregate would imply. The returned commit info still
// lists all 4 precommitters, as the real LastCommit would.
func omittedQuorumProposal(t *testing.T, k *keeper.Keeper, ctx sdk.Context, reqHeight int64) ([]byte, abci.CommitInfo) {
	t.Helper()
	cert := healthyAggregatedCertificate(t, k, ctx, reqHeight)
	lastCommit := commitInfoOf(cert.Certificates, 0)
	cert.Certificates = cert.Certificates[:2]
	cert.FsmState = types.StateSovereign
	cert.Healthy = false
	cert.Critical = true // the degraded fallback (BtcGap at SovereignThreshold) is critical
	tx, err := sovereignty.EncodeAggregatedHealthCertificate(cert)
	require.NoError(t, err)
	return tx, lastCommit
}

func TestProcessProposal_RejectsSubQuorumCertificateOnceExtensionsExist(t *testing.T) {
	k, ctx := newTestKeeperCtx(t)
	ctx = withVoteExtensionsEnabledAt(ctx, 1)
	tx, lastCommit := omittedQuorumProposal(t, k, ctx, 2)

	resp, err := sovereignty.NewProcessProposalHandler(k)(ctx, &abci.RequestProcessProposal{Txs: [][]byte{tx}, Height: 2, ProposedLastCommit: lastCommit})
	require.NoError(t, err)
	require.False(t, resp.IsAccepted(),
		"a proposer omitting observations below 2f+1 must not be able to force SOVEREIGN via the degraded fallback")
}

func TestProcessProposal_AcceptsFullQuorumCertificateOnceExtensionsExist(t *testing.T) {
	k, ctx := newTestKeeperCtx(t)
	ctx = withVoteExtensionsEnabledAt(ctx, 1)
	cert := healthyAggregatedCertificate(t, k, ctx, 2)
	tx, err := sovereignty.EncodeAggregatedHealthCertificate(cert)
	require.NoError(t, err)

	resp, err := sovereignty.NewProcessProposalHandler(k)(ctx, &abci.RequestProcessProposal{
		Txs: [][]byte{tx}, Height: 2, ProposedLastCommit: commitInfoOf(cert.Certificates, 0),
	})
	require.NoError(t, err)
	require.True(t, resp.IsAccepted())
}

func TestProcessProposal_AcceptsDegradedFallbackBeforeExtensionsExist(t *testing.T) {
	k, ctx := newTestKeeperCtx(t)
	ctx = withVoteExtensionsEnabledAt(ctx, 1)
	// Height 1's LocalLastCommit (height 0) has no extensions at all, so no
	// proposer can reach quorum -- the degraded fallback is the only option.
	tx, _ := omittedQuorumProposal(t, k, ctx, 1)

	resp, err := sovereignty.NewProcessProposalHandler(k)(ctx, &abci.RequestProcessProposal{Txs: [][]byte{tx}, Height: 1})
	require.NoError(t, err)
	require.True(t, resp.IsAccepted())
}

func processWithCommit(t *testing.T, k *keeper.Keeper, ctx sdk.Context, cert *types.AggregatedHealthCertificate, last abci.CommitInfo) bool {
	t.Helper()
	tx, err := sovereignty.EncodeAggregatedHealthCertificate(cert)
	require.NoError(t, err)
	resp, err := sovereignty.NewProcessProposalHandler(k)(ctx, &abci.RequestProcessProposal{
		Txs: [][]byte{tx}, Height: cert.Height + 1, ProposedLastCommit: last,
	})
	require.NoError(t, err)
	return resp.IsAccepted()
}

func TestProcessProposal_AcceptsCertificateSignedInTheCommitRound(t *testing.T) {
	k, ctx := newTestKeeperCtx(t)
	ctx = withVoteExtensionsEnabledAt(ctx, 1)
	cert := healthyAggregatedCertificateAtRound(t, k, ctx, 2, 2)

	require.True(t, processWithCommit(t, k, ctx, cert, commitInfoOf(cert.Certificates, 2)),
		"an honest proposal after a round-skip at the previous height must stay valid")
}

func TestProcessProposal_RejectsCertificateFromEarlierRound(t *testing.T) {
	k, ctx := newTestKeeperCtx(t)
	ctx = withVoteExtensionsEnabledAt(ctx, 1)
	// Correctly signed at round 0, but round 1 is the one that committed.
	cert := healthyAggregatedCertificateAtRound(t, k, ctx, 2, 0)

	require.False(t, processWithCommit(t, k, ctx, cert, commitInfoOf(cert.Certificates, 1)),
		"observations signed in an earlier round of the previous height must not count")
}

func TestProcessProposal_RejectsCertificateFromValidatorOutsideTheCommit(t *testing.T) {
	k, ctx := newTestKeeperCtx(t)
	ctx = withVoteExtensionsEnabledAt(ctx, 1)
	cert := healthyAggregatedCertificate(t, k, ctx, 2)

	last := commitInfoOf(cert.Certificates, 0)
	last.Votes[0].BlockIdFlag = cmtproto.BlockIDFlagNil // precommitted nil: no extension in the commit
	require.False(t, processWithCommit(t, k, ctx, cert, last))

	last = commitInfoOf(cert.Certificates, 0)
	last.Votes[1].BlockIdFlag = cmtproto.BlockIDFlagAbsent
	require.False(t, processWithCommit(t, k, ctx, cert, last))

	require.False(t, processWithCommit(t, k, ctx, cert, commitInfoOf(cert.Certificates[1:], 0)),
		"a validator missing from the last commit cannot contribute an observation")
}
