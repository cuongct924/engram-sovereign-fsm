package main

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestExtractTimestampContent_RealCometBFTSignBytes cross-checks
// extractTimestampContent's tag-by-tag walk against the exact ground-truth
// bytes captured via a real types.VoteSignBytes("engram-dev-1", vote) call
// against the engram-consensus-core fork (same fixture as circuit/
// reanchoring/src/main.nr's build_vote_sign_bytes_matches_real_cometbft_output
// test) -- a Precommit at height 12345, round 0, block_hash=[1..32],
// part_set{total:1, hash=[200..231]}, timestamp content
// [0x08,0xc0,0xd9,0xd5,0xd4,0x06], chain_id="engram-dev-1".
func TestExtractTimestampContent_RealCometBFTSignBytes(t *testing.T) {
	signBytes, err := hex.DecodeString(
		"6b080211393000000000000022480a200102030405060708090a0b0c0d0e0f" +
			"101112131415161718191a1b1c1d1e1f20122408011220c8c9cacbcccdce" +
			"cfd0d1d2d3d4d5d6d7d8d9dadbdcdddedfe0e1e2e3e4e5e6e72a0608c0d9" +
			"d5d406320c656e6772616d2d6465762d31")
	require.NoError(t, err)
	require.Equal(t, 108, len(signBytes))

	got, err := extractTimestampContent(signBytes, "engram-dev-1")
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

	_, err = extractTimestampContent(signBytes, "wrong-chain-id")
	require.Error(t, err)
}

// A truncated buffer must error, not panic or silently return a short slice.
func TestExtractTimestampContent_TruncatedBufferErrors(t *testing.T) {
	_, err := extractTimestampContent([]byte{0x08, 0x02}, "engram-dev-1")
	require.Error(t, err)
}

// TestDecompressEd25519_RealValidatorKey cross-checks decompressEd25519
// against a real ed25519 keypair's ground-truthed decompression (same
// fixture as circuit/reanchoring/src/main.nr's
// validator_set_hash_matches_real_cometbft_output test, validator index 0):
// compressed pubkey 66e09c9b...a89e decompresses to this exact (x, y).
func TestDecompressEd25519_RealValidatorKey(t *testing.T) {
	compressed, err := hex.DecodeString("66e09c9b4aa9b6dbd9c320c92ccd03a503b1257ad4dc76bd4f22245a2285a89e")
	require.NoError(t, err)
	require.Equal(t, 32, len(compressed))

	x, y, err := decompressEd25519(compressed)
	require.NoError(t, err)
	require.Equal(t, "1f47edb872b022fa38f751defc31973d60ad21cf69ee7353ebe5fa067b00b047", hex.EncodeToString(x[:]))
	require.Equal(t, "66e09c9b4aa9b6dbd9c320c92ccd03a503b1257ad4dc76bd4f22245a2285a81e", hex.EncodeToString(y[:]))
}
