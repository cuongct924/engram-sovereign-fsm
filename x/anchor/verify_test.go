package anchor

import (
	"testing"

	sovtypes "github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
	"github.com/stretchr/testify/require"
)

func TestTolerance(t *testing.T) {
	// See Tolerance's doc: a zero-margin bound (kDeepFinality exactly) has no
	// slack for real delay between anchor confirmation and proposal acceptance.
	require.Equal(t, uint64(4), Tolerance(0, 2))
	require.Equal(t, uint64(6), Tolerance(2, 2))
	require.Equal(t, uint64(7), Tolerance(3, 2))
	require.Equal(t, uint64(104), Tolerance(100, 2))
	require.Equal(t, uint64(10), Tolerance(3, 5), "widened bound tracks kDeepFinality, not a fixed constant")
}

func TestVerifySPVProof_AcceptsValidCheckpoint(t *testing.T) {
	r := Receipt{CheckpointBlockHeight: 50, CheckpointBlockHash: ExpectedBlockHash(50)}
	require.True(t, VerifySPVProof(r, 100, 10))
}

func TestVerifySPVProof_RejectsFutureCheckpoint(t *testing.T) {
	r := Receipt{CheckpointBlockHeight: 101, CheckpointBlockHash: ExpectedBlockHash(101)}
	require.False(t, VerifySPVProof(r, 100, 10), "checkpoint must not exceed h_btc_current")
}

// TestVerifySPVProof_RejectsCheckpointBelowAnchored covers CanElect's IsKDeep
// branch -- see VerifySPVProof's doc in verify.go for why CanElect has no
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
	// hBTCCurrent=100, kDeepFinality=2 -> tol=4 at round 0 -> lower bound=96,
	// so height 95 fails at round 0...
	r := Receipt{CheckpointBlockHeight: 95, CheckpointBlockHash: ExpectedBlockHash(95)}
	require.False(t, VerifyReceipt(r, sovtypes.StateAnchored, false, 100, 10, 0, 2))
	// ...but at round 3, tol=2+2+3=7 -> lower bound=93: the same gap is accepted.
	require.True(t, VerifyReceipt(r, sovtypes.StateAnchored, false, 100, 10, 3, 2))
}

// TestVerifyReceipt_SkippedWhenNotRequired covers the S6 combined BTC+DA
// outage escape hatch (mirrors da.VerifyReceipt's own test): during a genuine
// BTC outage a proposal degrading to SOVEREIGN cannot carry a passing
// checkpoint, so the check must not apply in that state.
func TestVerifyReceipt_SkippedWhenNotRequired(t *testing.T) {
	// SOVEREIGN + BTC unhealthy: freshness/SPV check does not apply.
	r := Receipt{CheckpointBlockHeight: 999, CheckpointBlockHash: BlockHash{Tag: "BTC_FORK", Height: 999}}
	require.True(t, VerifyReceipt(r, sovtypes.StateSovereign, false, 100, 10, 0, 2))
}

func TestVerifyReceipt_RequiredWhenBTCHealthyEvenInSovereign(t *testing.T) {
	// SOVEREIGN but isBTCHealthy=true still triggers the check (the
	// `\/ IsBTCHealthy` disjunct) -- a forged/stale checkpoint isn't exempted.
	r := Receipt{CheckpointBlockHeight: 999, CheckpointBlockHash: BlockHash{Tag: "BTC_FORK", Height: 999}}
	require.False(t, VerifyReceipt(r, sovtypes.StateSovereign, true, 100, 10, 0, 2))
}

// TestVerifyReceipt_RecoversFromExtendedRoundSkip covers the liveness bug a
// fixed (round-independent) tolerance left open: once real latency grows the
// gap past it, every later round re-fails forever, since only committing a
// block advances the checkpoint and only this check passing commits a block.
func TestVerifyReceipt_RecoversFromExtendedRoundSkip(t *testing.T) {
	r := Receipt{CheckpointBlockHeight: 203, CheckpointBlockHash: ExpectedBlockHash(203)}
	// gap=40 against hBTCCurrent=243 -- unrecoverable under the old flat
	// tol=kDeepFinality+livenessMargin=4, at any round.
	require.False(t, VerifyReceipt(r, sovtypes.StateAnchored, false, 243, 203, 0, 2))
	// round-based widening eventually exceeds the gap: tol=2+2+40=44 >= 40.
	require.True(t, VerifyReceipt(r, sovtypes.StateAnchored, false, 243, 203, 40, 2))
}

func TestVerifyReceipt_AcceptsGenuinelyKDeepConfirmedCheckpointAtRoundZero(t *testing.T) {
	// A receipt exactly kDeepFinality behind h_btc_current -- the freshest a
	// genuinely K-deep-confirmed anchor can be (MaybeSubmit only reports once
	// confirmations >= kDeepFinality+1) -- must be accepted even at round=0.
	r := Receipt{CheckpointBlockHeight: 98, CheckpointBlockHash: ExpectedBlockHash(98)}
	require.True(t, VerifyReceipt(r, sovtypes.StateAnchored, false, 100, 10, 0, 2))
}

func TestVerifyReceipt_AcceptsCheckpointAfterRealisticProposalDelay(t *testing.T) {
	// A receipt confirmed exactly kDeepFinality behind h_btc_current AT
	// CONFIRMATION TIME, but checked later after h_btc_current advanced 2 more
	// blocks (real gossip/round-skip delay) -- must still be accepted.
	confirmedAtCurrent := uint64(100)
	checkpoint := confirmedAtCurrent - 2 // kDeepFinality=2 behind at confirmation time
	laterCurrent := confirmedAtCurrent + 2
	r := Receipt{CheckpointBlockHeight: checkpoint, CheckpointBlockHash: ExpectedBlockHash(checkpoint)}
	require.True(t, VerifyReceipt(r, sovtypes.StateAnchored, false, laterCurrent, 10, 0, 2))
}
