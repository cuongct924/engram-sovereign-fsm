package sovereignty_test

import (
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cuongct220020/engram-sovereign-fsm/x/anchor"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// daOutageCertificate certifies a critical BTC gap plus a failed DA layer,
// the condition that makes Engram go SOVEREIGN. The caller picks the claimed
// state and DA attestation.
func daOutageCertificate(t *testing.T, k *keeper.Keeper, ctx sdk.Context, state string) *types.AggregatedHealthCertificate {
	t.Helper()
	p := k.Params
	metrics := &types.PeripheralMetrics{
		BtcGap:          p.SovereignThreshold,
		IsDasFailed:     true,
		SubnetDiversity: p.MinSubnetDiversity,
		ActiveAnchors:   p.MinAnchorPeers,
		CleanPeers:      p.MinPeers,
		AvgPeerTenure:   p.MinAvgTenure,
	}
	vals := newTestValidators(t, k, ctx, 4)
	return &types.AggregatedHealthCertificate{
		Height:                       0,
		Certificates:                 signCertificates(t, vals, testChainID, 0, metrics),
		FsmState:                     state,
		Critical:                     true,
		DaAttestation:                false,
		BtcCheckpointBlockHashTag:    anchor.ExpectedBlockHash(0).Tag,
		BtcCheckpointBlockHashHeight: anchor.ExpectedBlockHash(0).Height,
	}
}

func processAt(t *testing.T, k *keeper.Keeper, ctx sdk.Context, cert *types.AggregatedHealthCertificate, round int32, extraTxs ...[]byte) bool {
	t.Helper()
	tx, err := sovereignty.EncodeAggregatedHealthCertificate(cert)
	require.NoError(t, err)
	resp, err := sovereignty.NewProcessProposalHandler(k)(ctx, &abci.RequestProcessProposal{
		Txs: append([][]byte{tx}, extraTxs...), Height: 1, Round: round,
	})
	require.NoError(t, err)
	return resp.IsAccepted()
}

func TestBaselineHardGated_RejectsWhatEngramAcceptsUnderDAOutage(t *testing.T) {
	k, ctx := newTestKeeperCtx(t)
	require.True(t, processAt(t, k, ctx, daOutageCertificate(t, k, ctx, types.StateSovereign), 0),
		"control: Engram accepts a SOVEREIGN proposal without a DA receipt when the DA layer is certified failed")

	k, ctx = newTestKeeperCtx(t)
	k.Baseline = types.BaselineHardGated
	require.False(t, processAt(t, k, ctx, daOutageCertificate(t, k, ctx, types.StateAnchored), 0),
		"hard-gated still requires a DA receipt, so an ANCHORED proposal without one is invalid")
	require.False(t, processAt(t, k, ctx, daOutageCertificate(t, k, ctx, types.StateSovereign), 0),
		"hard-gated never leaves ANCHORED, so a SOVEREIGN claim is invalid")
}

func TestBTCToleranceDoesNotWidenWithRound(t *testing.T) {
	stale := func(t *testing.T, mode types.BaselineMode) bool {
		t.Helper()
		k, ctx := newTestKeeperCtx(t)
		k.Baseline = mode
		// BTC tip is far ahead of the latest checkpoint (height 0).
		metrics := &types.PeripheralMetrics{
			BtcCurrentHeight: 30,
			SubnetDiversity:  k.Params.MinSubnetDiversity,
			ActiveAnchors:    k.Params.MinAnchorPeers,
			CleanPeers:       k.Params.MinPeers,
			AvgPeerTenure:    k.Params.MinAvgTenure,
		}
		cert := &types.AggregatedHealthCertificate{
			Certificates:                 signCertificates(t, newTestValidators(t, k, ctx, 4), testChainID, 0, metrics),
			FsmState:                     types.StateAnchored,
			Healthy:                      true,
			DaAttestation:                true,
			BtcCheckpointBlockHashTag:    anchor.ExpectedBlockHash(0).Tag,
			BtcCheckpointBlockHashHeight: anchor.ExpectedBlockHash(0).Height,
		}
		return processAt(t, k, ctx, cert, 50)
	}
	// The widening is capped at round 3 (BTCTolerance), identically in every mode,
	// so a stuck height cannot re-admit a stale checkpoint however many rounds it skips.
	for _, mode := range []types.BaselineMode{types.BaselineEngram, types.BaselineSoft, types.BaselineHardGated} {
		require.False(t, stale(t, mode), "stale checkpoint must stay rejected at round 50 in mode %q", mode)
	}
}

func TestBaselineSoft_AdmitsWithdrawalWhileSovereign(t *testing.T) {
	withdrawal := []byte("TX_WITHDRAWAL")

	k, ctx := newTestKeeperCtx(t)
	require.False(t, processAt(t, k, ctx, daOutageCertificate(t, k, ctx, types.StateSovereign), 0, withdrawal),
		"control: Engram's circuit breaker rejects withdrawals while SOVEREIGN")

	k, ctx = newTestKeeperCtx(t)
	k.Baseline = types.BaselineSoft
	require.True(t, processAt(t, k, ctx, daOutageCertificate(t, k, ctx, types.StateSovereign), 0, withdrawal),
		"soft never locks withdrawals")
}

func TestBaselineHardGatedOutOfBand_ModeFlags(t *testing.T) {
	m, err := types.ParseBaselineMode("hard-gated-oob")
	require.NoError(t, err)
	require.True(t, m.HardGated(), "the out-of-band baseline is still gated on receipts at every height")
	require.True(t, m.OutOfBand())
	require.False(t, types.BaselineHardGated.OutOfBand())
	require.False(t, m.SkipsCircuitBreaker())
}

func TestBaselineHardGatedOutOfBand_StillPinnedToAnchored(t *testing.T) {
	k, ctx := newTestKeeperCtx(t)
	k.Baseline = types.BaselineHardGatedOutOfBand
	// No sensors are wired in the handler, so the live read falls back to the
	// degraded view; a SOVEREIGN claim must still be invalid.
	require.False(t, processAt(t, k, ctx, daOutageCertificate(t, k, ctx, types.StateSovereign), 0))
}
