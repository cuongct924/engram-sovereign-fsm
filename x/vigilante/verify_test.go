package vigilante

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTolerance(t *testing.T) {
	require.Equal(t, uint64(0), Tolerance(0))
	require.Equal(t, uint64(0), Tolerance(2))
	require.Equal(t, uint64(1), Tolerance(3))
	require.Equal(t, uint64(1), Tolerance(100))
}

func TestVerifySPVProof_AcceptsValidCheckpoint(t *testing.T) {
	r := Receipt{CheckpointBlockHeight: 50, CheckpointBlockHash: ExpectedBlockHash(50)}
	require.True(t, VerifySPVProof(r, 100, 10))
}

func TestVerifySPVProof_RejectsFutureCheckpoint(t *testing.T) {
	r := Receipt{CheckpointBlockHeight: 101, CheckpointBlockHash: ExpectedBlockHash(101)}
	require.False(t, VerifySPVProof(r, 100, 10), "checkpoint must not exceed h_btc_current")
}

func TestVerifySPVProof_RejectsCheckpointBelowAnchored(t *testing.T) {
	r := Receipt{CheckpointBlockHeight: 5, CheckpointBlockHash: ExpectedBlockHash(5)}
	require.False(t, VerifySPVProof(r, 100, 10), "checkpoint must not regress below h_btc_anchored")
}

func TestVerifySPVProof_RejectsForgedHash(t *testing.T) {
	forged := BlockHash{Tag: "BTC_FORK", Height: 50}
	r := Receipt{CheckpointBlockHeight: 50, CheckpointBlockHash: forged}
	require.False(t, VerifySPVProof(r, 100, 10), "an eclipsed proposer's forged branch must be rejected by the hash check")
}

func TestVerifyReceipt_RejectsBelowToleranceWindow(t *testing.T) {
	// hBTCCurrent=100, round=0 (tol=0) -> lower bound = 100. Height 99 must fail.
	r := Receipt{CheckpointBlockHeight: 99, CheckpointBlockHash: ExpectedBlockHash(99)}
	require.False(t, VerifyReceipt(r, 100, 10, 0))
}

func TestVerifyReceipt_WiderRoundToleranceAcceptsOlderCheckpoint(t *testing.T) {
	// Same 99 height must pass once round>=3 widens tolerance to 1 (lower bound 99).
	r := Receipt{CheckpointBlockHeight: 99, CheckpointBlockHash: ExpectedBlockHash(99)}
	require.True(t, VerifyReceipt(r, 100, 10, 3))
}
