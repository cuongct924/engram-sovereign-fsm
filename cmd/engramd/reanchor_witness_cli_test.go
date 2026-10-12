package main

import (
	"bytes"
	"encoding/hex"
	"testing"
	"time"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	cmttypes "github.com/cometbft/cometbft/types"
	"github.com/stretchr/testify/require"
)

// TestExtractTimestampContent_RealCometBFTSignBytes cross-checks
// extractTimestampContent against ground-truth bytes from a real
// types.VoteSignBytes call (same fixture as main.nr's
// build_vote_sign_bytes_matches_real_cometbft_output test).
func TestExtractTimestampContent_RealCometBFTSignBytes(t *testing.T) {
	signBytes, err := hex.DecodeString(
		"6b080211393000000000000022480a200102030405060708090a0b0c0d0e0f" +
			"101112131415161718191a1b1c1d1e1f20122408011220c8c9cacbcccdce" +
			"cfd0d1d2d3d4d5d6d7d8d9dadbdcdddedfe0e1e2e3e4e5e6e72a0608c0d9" +
			"d5d406320c656e6772616d2d6465762d31")
	require.NoError(t, err)
	require.Equal(t, 108, len(signBytes))

	got, err := extractTimestampContent(signBytes, "engram-dev-1", 0)
	require.NoError(t, err)
	require.Equal(t, []byte{0x08, 0xc0, 0xd9, 0xd5, 0xd4, 0x06}, got)
}

// A wrong chainID must be rejected (the self-check the doc promises), not
// silently return a misparsed result.
func TestExtractTimestampContent_ChainIDMismatchRejected(t *testing.T) {
	signBytes, err := hex.DecodeString(
		"6b080211393000000000000022480a200102030405060708090a0b0c0d0e0f" +
			"101112131415161718191a1b1c1d1e1f20122408011220c8c9cacbcccdce" +
			"cfd0d1d2d3d4d5d6d7d8d9dadbdcdddedfe0e1e2e3e4e5e6e72a0608c0d9" +
			"d5d406320c656e6772616d2d6465762d31")
	require.NoError(t, err)

	_, err = extractTimestampContent(signBytes, "wrong-chain-id", 0)
	require.Error(t, err)
}

// A truncated buffer must error, not panic or silently return a short slice.
func TestExtractTimestampContent_TruncatedBufferErrors(t *testing.T) {
	_, err := extractTimestampContent([]byte{0x08, 0x02}, "engram-dev-1", 0)
	require.Error(t, err)
}

// TestDecompressEd25519_RealValidatorKey cross-checks decompressEd25519
// against a real ed25519 keypair's ground-truthed decompression (same
// fixture as main.nr's validator_set_hash_matches_real_cometbft_output test).
func TestDecompressEd25519_RealValidatorKey(t *testing.T) {
	compressed, err := hex.DecodeString("66e09c9b4aa9b6dbd9c320c92ccd03a503b1257ad4dc76bd4f22245a2285a89e")
	require.NoError(t, err)
	require.Equal(t, 32, len(compressed))

	x, y, err := decompressEd25519(compressed)
	require.NoError(t, err)
	require.Equal(t, "1f47edb872b022fa38f751defc31973d60ad21cf69ee7353ebe5fa067b00b047", hex.EncodeToString(x[:]))
	require.Equal(t, "66e09c9b4aa9b6dbd9c320c92ccd03a503b1257ad4dc76bd4f22245a2285a81e", hex.EncodeToString(y[:]))
}

// A commit at round r > 0 signs a round field; the walker must accept it only
// when it carries the commit's round.
func TestExtractTimestampContent_NonZeroRound(t *testing.T) {
	for _, round := range []int32{1, 2} {
		vote := &cmtproto.Vote{
			Type: cmtproto.PrecommitType, Height: 12345, Round: round,
			BlockID: cmtproto.BlockID{
				Hash:          bytes.Repeat([]byte{7}, 32),
				PartSetHeader: cmtproto.PartSetHeader{Total: 1, Hash: bytes.Repeat([]byte{9}, 32)},
			},
			Timestamp: time.Unix(1700000000, 0).UTC(),
		}
		signBytes := cmttypes.VoteSignBytes("engram-dev-1", vote)

		got, err := extractTimestampContent(signBytes, "engram-dev-1", uint64(round))
		require.NoError(t, err)
		require.NotEmpty(t, got)

		_, err = extractTimestampContent(signBytes, "engram-dev-1", 0)
		require.Error(t, err, "sign-bytes with a round field cannot be walked as round 0")
		_, err = extractTimestampContent(signBytes, "engram-dev-1", uint64(round)+1)
		require.Error(t, err, "a wrong round must be rejected")
	}
}
