package anchor

import (
	"testing"

	sovtypes "github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
	"github.com/stretchr/testify/require"
)

func TestTolerance(t *testing.T) {
	// Mirrors BTCTolerance's CASE arms; the widening is capped at r >= 3.
	require.Equal(t, uint64(0), Tolerance(0))
	require.Equal(t, uint64(0), Tolerance(2))
	require.Equal(t, uint64(1), Tolerance(3))
	require.Equal(t, uint64(1), Tolerance(100), "widening must not grow with the round")
}

func TestThreshold(t *testing.T) {
	// See livenessMargin's doc: a zero-margin bound (kDeepFinality exactly) has
	// no slack for real delay between anchor confirmation and proposal acceptance.
	require.Equal(t, uint64(4), Threshold(2))
	require.Equal(t, uint64(7), Threshold(5), "base bound tracks kDeepFinality, not a fixed constant")
}

func TestVerifyAnchorReceipt_AcceptsValidCheckpoint(t *testing.T) {
	r := Receipt{CheckpointBlockHeight: 50, CheckpointBlockHash: ExpectedBlockHash(50)}
	require.True(t, VerifyAnchorReceipt(r, 100, 10))
}

func TestVerifyAnchorReceipt_RejectsFutureCheckpoint(t *testing.T) {
	r := Receipt{CheckpointBlockHeight: 101, CheckpointBlockHash: ExpectedBlockHash(101)}
	require.False(t, VerifyAnchorReceipt(r, 100, 10), "checkpoint must not exceed h_btc_current")
}

// TestVerifyAnchorReceipt_RejectsCheckpointBelowAnchored covers CanElect's IsKDeep
// branch -- see VerifyAnchorReceipt's doc in verify.go for why CanElect has no
// separate fork-level implementation to test beyond this.
func TestVerifyAnchorReceipt_RejectsCheckpointBelowAnchored(t *testing.T) {
	r := Receipt{CheckpointBlockHeight: 5, CheckpointBlockHash: ExpectedBlockHash(5)}
	require.False(t, VerifyAnchorReceipt(r, 100, 10), "checkpoint must not regress below h_btc_anchored")
}

func TestVerifyAnchorReceipt_RejectsForgedHash(t *testing.T) {
	forged := BlockHash{Tag: "BTC_FORK", Height: 50}
	r := Receipt{CheckpointBlockHeight: 50, CheckpointBlockHash: forged}
	require.False(t, VerifyAnchorReceipt(r, 100, 10), "an eclipsed proposer's forged branch must be rejected by the hash check")
}

func TestVerifyReceipt_RejectsBelowToleranceWindow(t *testing.T) {
	// hBTCCurrent=100, kDeepFinality=2 -> threshold=4, tol=0 at round 0 ->
	// lower bound=96, so height 95 fails at round 0...
	r := Receipt{CheckpointBlockHeight: 95, CheckpointBlockHash: ExpectedBlockHash(95)}
	require.False(t, VerifyReceipt(r, sovtypes.StateAnchored, false, 100, 10, 0, 2))
	// ...and at round 3, tol=1 -> lower bound=95: the same gap is now accepted.
	require.True(t, VerifyReceipt(r, sovtypes.StateAnchored, false, 100, 10, 3, 2))
	// A gap one block wider stays rejected at any round: the widening is capped.
	stale := Receipt{CheckpointBlockHeight: 94, CheckpointBlockHash: ExpectedBlockHash(94)}
	require.False(t, VerifyReceipt(stale, sovtypes.StateAnchored, false, 100, 10, 3, 2))
	require.False(t, VerifyReceipt(stale, sovtypes.StateAnchored, false, 100, 10, 1000, 2))
}

// TestVerifyReceipt_SkippedWhenNotRequired covers the S6 combined BTC+DA
// outage escape hatch (mirrors da.VerifyReceipt's own test): during a genuine
// BTC outage a proposal degrading to SOVEREIGN cannot carry a passing
// checkpoint, so the check must not apply in that state.
func TestVerifyReceipt_SkippedWhenNotRequired(t *testing.T) {
	// SOVEREIGN + BTC unhealthy: freshness/anchor-hash check does not apply.
	r := Receipt{CheckpointBlockHeight: 999, CheckpointBlockHash: BlockHash{Tag: "BTC_FORK", Height: 999}}
	require.True(t, VerifyReceipt(r, sovtypes.StateSovereign, false, 100, 10, 0, 2))
}

func TestVerifyReceipt_RequiredWhenBTCHealthyEvenInSovereign(t *testing.T) {
	// SOVEREIGN but isBTCHealthy=true still triggers the check (the
	// `\/ IsBTCHealthy` disjunct) -- a forged/stale checkpoint isn't exempted.
	r := Receipt{CheckpointBlockHeight: 999, CheckpointBlockHash: BlockHash{Tag: "BTC_FORK", Height: 999}}
	require.False(t, VerifyReceipt(r, sovtypes.StateSovereign, true, 100, 10, 0, 2))
}

// TestVerifyReceipt_StaleCheckpointNotAdmittedByRoundSkips pins the cap: a
// checkpoint 40 blocks behind stays rejected however many rounds a stuck height
// skips, so an attacker cannot inflate the exposure window by forcing round-skips.
// Liveness then comes from the FSM leaving ANCHORED (the check is skipped once
// the proposal degrades and BTC is unhealthy), not from widening.
func TestVerifyReceipt_StaleCheckpointNotAdmittedByRoundSkips(t *testing.T) {
	r := Receipt{CheckpointBlockHeight: 203, CheckpointBlockHash: ExpectedBlockHash(203)}
	require.False(t, VerifyReceipt(r, sovtypes.StateAnchored, false, 243, 203, 0, 2))
	require.False(t, VerifyReceipt(r, sovtypes.StateAnchored, false, 243, 203, 40, 2))
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
