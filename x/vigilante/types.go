// Package vigilante implements the Bitcoin settlement checkpoint receipt
// type and SPV verification logic from spec/core/EngramTendermint.tla --
// the "Settlement Monotonicity & BTC Light Client Hash Check" conjunct of
// IsValidProposal (lines 296-298).
package vigilante

// BlockHash mirrors ExpectedBlockHash's abstraction (spec/core/EngramTendermint.tla:266):
//
//	ExpectedBlockHash(height) == <<"BTC_BLOCK", height>>
//
// The spec deliberately abstracts a cryptographic hash as an identity mapping
// -- keep this abstraction here too, do not invent a "real" hash scheme not
// present in the spec.
type BlockHash struct {
	Tag    string
	Height uint64
}

// ExpectedBlockHash ports ExpectedBlockHash verbatim.
func ExpectedBlockHash(height uint64) BlockHash {
	return BlockHash{Tag: "BTC_BLOCK", Height: height}
}

// Receipt mirrors btc_receipt in spec/core/EngramTendermint.tla:
//
//	btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }
type Receipt struct {
	CheckpointBlockHeight uint64
	CheckpointBlockHash   BlockHash
}
