package da

import (
	"testing"

	sovtypes "github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
	"github.com/stretchr/testify/require"
)

func TestTolerance(t *testing.T) {
	require.Equal(t, uint64(0), Tolerance(0))
	require.Equal(t, uint64(0), Tolerance(1))
	require.Equal(t, uint64(2), Tolerance(2))
	require.Equal(t, uint64(4), Tolerance(3))
	require.Equal(t, uint64(4), Tolerance(100))
}

func TestVerifyReceipt_SkippedWhenNotRequired(t *testing.T) {
	// SOVEREIGN + DA unhealthy: freshness check does not apply, an empty/stale
	// receipt must not itself invalidate the proposal.
	ok := VerifyReceipt(Receipt{}, sovtypes.StateSovereign, false, 100, 5, 0)
	require.True(t, ok)
}

func TestVerifyReceipt_RequiresAttestationWhenAnchored(t *testing.T) {
	ok := VerifyReceipt(Receipt{Attestation: false, PublishedBlockHeight: 100}, sovtypes.StateAnchored, false, 100, 5, 0)
	require.False(t, ok, "ANCHORED must require attestation=true regardless of local DA health")
}

func TestVerifyReceipt_RejectsFutureHeight(t *testing.T) {
	ok := VerifyReceipt(Receipt{Attestation: true, PublishedBlockHeight: 101}, sovtypes.StateAnchored, false, 100, 5, 0)
	require.False(t, ok, "receipt height must not exceed h_engram_current")
}

func TestVerifyReceipt_RejectsStaleHeightOutsideTolerance(t *testing.T) {
	// hEngramCurrent=100, daThreshold=5, round=0 (tolerance=0) -> lower bound = 95.
	ok := VerifyReceipt(Receipt{Attestation: true, PublishedBlockHeight: 94}, sovtypes.StateAnchored, false, 100, 5, 0)
	require.False(t, ok)
}

func TestVerifyReceipt_AcceptsWithinToleranceWindow(t *testing.T) {
	ok := VerifyReceipt(Receipt{Attestation: true, PublishedBlockHeight: 95}, sovtypes.StateAnchored, false, 100, 5, 0)
	require.True(t, ok)
}

func TestVerifyReceipt_WiderRoundToleranceAcceptsOlderReceipt(t *testing.T) {
	// Same 94 height that failed at round=0 must pass once round>=3 widens
	// tolerance to 4 (lower bound becomes 100-5-4=91).
	ok := VerifyReceipt(Receipt{Attestation: true, PublishedBlockHeight: 94}, sovtypes.StateAnchored, false, 100, 5, 3)
	require.True(t, ok)
}

func TestVerifyReceipt_RequiredWhenDAHealthyEvenInSovereign(t *testing.T) {
	// SOVEREIGN but IsDAHealthy=true still triggers the check (per the `\/ IsDAHealthy` disjunct).
	ok := VerifyReceipt(Receipt{Attestation: false}, sovtypes.StateSovereign, true, 100, 5, 0)
	require.False(t, ok)
}
