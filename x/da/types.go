// Package da implements the Celestia data-availability receipt type and
// verification logic from spec/core/EngramTendermint.tla -- the "DA Pipeline
// Check" conjunct of IsValidProposal (lines 290-294). It does not replace
// x/sovereignty/keeper/sensors' DASensor (which feeds the FSM's IsDAHealthy
// predicate); this package verifies the da_receipt attached to a proposal,
// a separate concern from FSM-state computation.
package da

// Receipt mirrors da_receipt in spec/core/EngramTendermint.tla:
//
//	da_receipt: { published_block_height: Int, attestation: Bool }
type Receipt struct {
	PublishedBlockHeight uint64
	Attestation          bool
}
