package anchor

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTolerance(t *testing.T) {
	// Tolerance is kDeepFinality+livenessMargin at every round, including
	// round 0 -- see Tolerance's own doc for why (a) the earlier round<3
	// zero-tolerance case was removed (unsatisfiable by any genuinely
	// K-deep-confirmed receipt) and (b) exactly kDeepFinality (no margin)
	// still wasn't enough (zero slack for real-world delay between anchor
	// confirmation and proposal acceptance) -- both confirmed live.
	require.Equal(t, uint64(4), Tolerance(0, 2))
	require.Equal(t, uint64(4), Tolerance(2, 2))
	require.Equal(t, uint64(4), Tolerance(3, 2))
	require.Equal(t, uint64(4), Tolerance(100, 2))
	require.Equal(t, uint64(7), Tolerance(3, 5), "widened bound tracks kDeepFinality, not a fixed constant")
}

func TestVerifySPVProof_AcceptsValidCheckpoint(t *testing.T) {
	r := Receipt{CheckpointBlockHeight: 50, CheckpointBlockHash: ExpectedBlockHash(50)}
	require.True(t, VerifySPVProof(r, 100, 10))
}

func TestVerifySPVProof_RejectsFutureCheckpoint(t *testing.T) {
	r := Receipt{CheckpointBlockHeight: 101, CheckpointBlockHash: ExpectedBlockHash(101)}
	require.False(t, VerifySPVProof(r, 100, 10), "checkpoint must not exceed h_btc_current")
}

// TestVerifySPVProof_RejectsCheckpointBelowAnchored is M0c's concrete test
// coverage for CanElect's IsKDeep branch (ANCHORED/SUSPICIOUS state) --
// see VerifySPVProof's doc in verify.go for why CanElect itself has no
// separate fork-level implementation to test beyond this.
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
	// hBTCCurrent=100, kDeepFinality=2 -> tol=4 -> lower bound = 96 at every
	// round (Tolerance no longer has a round<3 zero-tolerance case, see its
	// own doc). Height 95 is below that window and must fail regardless of round.
	r := Receipt{CheckpointBlockHeight: 95, CheckpointBlockHash: ExpectedBlockHash(95)}
	require.False(t, VerifyReceipt(r, 100, 10, 0, 2))
	require.False(t, VerifyReceipt(r, 100, 10, 3, 2))
}

func TestVerifyReceipt_AcceptsGenuinelyKDeepConfirmedCheckpointAtRoundZero(t *testing.T) {
	// A receipt exactly kDeepFinality behind h_btc_current -- the freshest a
	// genuinely K-deep-confirmed anchor can ever be (AnchorTracker.MaybeSubmit
	// only reports ConfirmedAnchorHeight once confirmations >= kDeepFinality+1)
	// -- must be accepted even at round=0. Confirmed live: with the old
	// round<3 zero-tolerance case, this exact shape was rejected on every
	// single real block regardless of BTC health, forcing a guaranteed
	// round-skip cycle every height.
	r := Receipt{CheckpointBlockHeight: 98, CheckpointBlockHash: ExpectedBlockHash(98)}
	require.True(t, VerifyReceipt(r, 100, 10, 0, 2))
}

func TestVerifyReceipt_AcceptsCheckpointAfterRealisticProposalDelay(t *testing.T) {
	// A receipt confirmed exactly kDeepFinality behind h_btc_current AT THE
	// TIME IT WAS CONFIRMED, but by the time the proposal carrying it is
	// actually checked, h_btc_current has advanced 2 MORE blocks (real
	// gossip/round-skip delay) -- must still be accepted. Without
	// livenessMargin (tolerance == kDeepFinality exactly), the only valid
	// checkpoint height is a single point (h_btc_current - kDeepFinality) with
	// zero slack for this delay -- confirmed live: real proposals kept
	// failing this exact way even after the round<3 fix, with BTC otherwise
	// completely healthy.
	confirmedAtCurrent := uint64(100)
	checkpoint := confirmedAtCurrent - 2 // kDeepFinality=2 behind at confirmation time
	laterCurrent := confirmedAtCurrent + 2
	r := Receipt{CheckpointBlockHeight: checkpoint, CheckpointBlockHash: ExpectedBlockHash(checkpoint)}
	require.True(t, VerifyReceipt(r, laterCurrent, 10, 0, 2))
}
