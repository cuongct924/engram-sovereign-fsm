package types

import "math/big"

// RecoveryHeader mirrors the ZK re-anchoring circuit's Header witness field
// (circuit/reanchoring/src/main.nr: Header{prev_hash, fsm_state,
// withdrawal_locked, state_root}) for a single block committed while the FSM
// was SOVEREIGN or RECOVERING (spec/README.md's §Re-anchoring via
// ZK-Proof of Recovery, witness w = (H_{k+1}, ..., H_{k+n})).
//
// prev_hash is deliberately NOT stored here: it's a pure function of the
// previous header's fields via the circuit's own Poseidon/pedersen_hash, and
// is only ever computed by the Noir witness-helper crate
// (circuit/reanchoring_witness/) at proof-generation time, never in Go --
// replicating that hash bit-exact in Go would be a real, unnecessary
// correctness risk for no benefit (the helper crate already exists for this).
//
// StateRoot uses CometBFT's real per-block AppHash rather than the keeper's
// SMT (Keeper.Tree): this prototype has no x/bank/account state yet (see
// CLAUDE.md's app/app.go note), so there's no real data to put in an SMT
// leaf -- AppHash is real, existing data, while a populated SMT here would
// need fabricated leaves.
type RecoveryHeader struct {
	FsmState         string
	WithdrawalLocked bool
	StateRoot        []byte
}

// bn254ScalarFieldModulus is the scalar field modulus for the curve backing
// Noir/Barretenberg's UltraHonk (circuit/reanchoring/src/main.nr's `Field`
// type). A raw 32-byte hash (e.g. CometBFT's AppHash) has roughly a 1-in-16
// chance of exceeding this modulus if used verbatim as a Field witness value
// -- the Noir/bb toolchain rejects that outright ("Non-canonical ... value
// >= field modulus") rather than silently wrapping it, so anything destined
// for a Header.state_root / rt_last / rt_new witness must be reduced first.
var bn254ScalarFieldModulus, _ = new(big.Int).SetString(
	"21888242871839275222246405745257275088548364400416034343698204186575808495617", 10)

// ReduceToField maps an arbitrary byte string into a valid element of the
// circuit's scalar field (big-endian, 32 bytes), so it can safely be used as
// a Header.state_root / rt_last / rt_new witness value. See
// bn254ScalarFieldModulus's doc for why this is necessary at all.
func ReduceToField(b []byte) []byte {
	n := new(big.Int).Mod(new(big.Int).SetBytes(b), bn254ScalarFieldModulus)
	out := make([]byte, 32)
	n.FillBytes(out)
	return out
}
