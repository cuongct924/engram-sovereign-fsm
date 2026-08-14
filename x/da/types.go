package da

// Receipt mirrors da_receipt in spec/core/EngramTendermint.tla:
//
//	da_receipt: { published_block_height: Int, attestation: Bool }
type Receipt struct {
	PublishedBlockHeight uint64
	Attestation          bool
}
