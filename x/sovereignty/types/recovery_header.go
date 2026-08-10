package types

import "math/big"

// RecoveryHeader mirrors the ZK re-anchoring circuit's Header witness field
// (circuit/reanchoring/src/main.nr's Header{prev_hash, fsm_state,
// withdrawal_locked, state_root}) for one block committed while SOVEREIGN or
// RECOVERING (spec/README.md's §Re-anchoring via ZK-Proof of Recovery).
//
// prev_hash is not stored here: it's derived via the circuit's own
// Poseidon/pedersen_hash, computed only by the witness-helper crate
// (circuit/reanchoring_witness/) at proof time, never in Go.
//
// StateRoot is CometBFT's real per-block AppHash, not the keeper's SMT --
// this prototype has no account state yet to put in an SMT leaf.
type RecoveryHeader struct {
	FsmState         string
	WithdrawalLocked bool
	StateRoot        []byte
}

// bn254ScalarFieldModulus is the scalar field modulus for the curve backing
// Noir/Barretenberg's UltraHonk. A raw 32-byte hash can exceed this modulus;
// Noir/bb reject that outright rather than wrapping it, so any value bound
// for a Field witness must be reduced first (see ReduceToField).
var bn254ScalarFieldModulus, _ = new(big.Int).SetString(
	"21888242871839275222246405745257275088548364400416034343698204186575808495617", 10)

// ReduceToField maps b into a valid element of the circuit's scalar field
// (big-endian, 32 bytes), for use as a Header.state_root/rt_last/rt_new witness.
func ReduceToField(b []byte) []byte {
	n := new(big.Int).Mod(new(big.Int).SetBytes(b), bn254ScalarFieldModulus)
	out := make([]byte, 32)
	n.FillBytes(out)
	return out
}
