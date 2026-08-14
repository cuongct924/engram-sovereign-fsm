package types

import "math/big"

// RecoveryHeader mirrors the ZK re-anchoring circuit's Header witness field
// (circuit/reanchoring/src/main.nr's Header{prev_hash, fsm_state,
// withdrawal_locked, state_root}) for one SOVEREIGN/RECOVERING block
// (spec/README.md's §Re-anchoring via ZK-Proof of Recovery).
//
// prev_hash is omitted: derived by the circuit's Poseidon/pedersen_hash in
// the witness-helper crate (circuit/reanchoring_witness/) at proof time,
// never in Go. StateRoot is CometBFT's real per-block AppHash.
type RecoveryHeader struct {
	FsmState         string
	WithdrawalLocked bool
	StateRoot        []byte
}

// bn254ScalarFieldModulus is the scalar-field modulus behind
// Noir/Barretenberg's UltraHonk. A raw 32-byte hash can exceed it and
// Noir/bb reject (not wrap) that, so any Field witness must be reduced
// first (see ReduceToField).
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
