package sovereignty

import (
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/stretchr/testify/require"
)

// TestRecordDetectedEvidence_NoMisbehaviorIsNoOp confirms a normal block
// (the overwhelming common case) never touches evidence state.
func TestRecordDetectedEvidence_NoMisbehaviorIsNoOp(t *testing.T) {
	k, ctx := newByzantineTestKeeperCtx(t)
	require.NoError(t, recordDetectedEvidence(ctx, k, &abci.RequestFinalizeBlock{Height: 10}))

	count, err := k.DetectedEvidenceCount.Get(ctx)
	require.Error(t, err, "count item must stay unset (collections.Item.Get errors on missing key)")
	require.Zero(t, count)
}

// TestRecordDetectedEvidence_DuplicateVoteIsRecorded is E8's "Double-signing"
// row's core assertion: a real RequestFinalizeBlock.Misbehavior entry
// (docs/EXPERIMENT.md's E8, CometBFT's stock evidence pool output) must be
// committed to queryable state with correct detection-latency bookkeeping.
func TestRecordDetectedEvidence_DuplicateVoteIsRecorded(t *testing.T) {
	k, ctx := newByzantineTestKeeperCtx(t)
	req := &abci.RequestFinalizeBlock{
		Height: 42,
		Misbehavior: []abci.Misbehavior{
			{
				Type:      abci.MisbehaviorType_DUPLICATE_VOTE,
				Validator: abci.Validator{Address: []byte{0xAA, 0xBB}, Power: 10},
				Height:    40, // offense occurred 2 blocks before detection
			},
		},
	}
	require.NoError(t, recordDetectedEvidence(ctx, k, req))

	count, err := k.DetectedEvidenceCount.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(1), count)

	record, err := k.LastDetectedEvidence.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, "DuplicateVote", record.Type)
	require.Equal(t, []byte{0xAA, 0xBB}, record.ValidatorAddress)
	require.Equal(t, int64(10), record.ValidatorPower)
	require.Equal(t, int64(40), record.OffenseHeight)
	require.Equal(t, int64(42), record.DetectedAtHeight)
}

// TestRecordDetectedEvidence_MultipleEntriesIncrementCount covers a block
// reporting more than one piece of misbehavior at once.
func TestRecordDetectedEvidence_MultipleEntriesIncrementCount(t *testing.T) {
	k, ctx := newByzantineTestKeeperCtx(t)
	req := &abci.RequestFinalizeBlock{
		Height: 100,
		Misbehavior: []abci.Misbehavior{
			{Type: abci.MisbehaviorType_DUPLICATE_VOTE, Validator: abci.Validator{Address: []byte{0x01}}, Height: 98},
			{Type: abci.MisbehaviorType_LIGHT_CLIENT_ATTACK, Validator: abci.Validator{Address: []byte{0x02}}, Height: 99},
		},
	}
	require.NoError(t, recordDetectedEvidence(ctx, k, req))

	count, err := k.DetectedEvidenceCount.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(2), count)

	record, err := k.LastDetectedEvidence.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, "LightClientAttack", record.Type, "LastDetectedEvidence must reflect the most recent entry in the slice")
}
