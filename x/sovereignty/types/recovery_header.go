package types

import (
	"math/big"

	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr/poseidon2"
)

// RecoveryHeader mirrors the ZK re-anchoring circuit's Header witness field
// (circuit/reanchoring/src/main.nr's Header{prev_hash, fsm_state,
// withdrawal_locked, state_root}) for one SOVEREIGN/RECOVERING block
// (spec/README.md's §Re-anchoring via ZK-Proof of Recovery).
//
// prev_hash is omitted: derived at proof time by the off-chain witness
// helper, chaining HashHeader from LastAnchoredRoot. Accumulator is this
// height's HashHeader output -- msg_server.go binds rt_new to it, not StateRoot.
type RecoveryHeader struct {
	FsmState         string
	WithdrawalLocked bool
	StateRoot        []byte
	Accumulator      []byte
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

// FSMStateToField mirrors circuit/reanchoring/src/main.nr's Header.fsm_state
// encoding: 2 = SOVEREIGN, 3 = RECOVERING (the only two values HeaderHistory
// ever stores, per preblock.go's WithdrawLocked gate).
func FSMStateToField(state string) int64 {
	if state == StateRecovering {
		return 3
	}
	return 2
}

// BoolToField mirrors the circuit's withdrawal_locked encoding (1 = true, 0 = false).
func BoolToField(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// HashHeader ports circuit/reanchoring/src/main.nr's hash_header exactly: the raw Poseidon2 t=4
// permutation (gnark-crypto's poseidon2.NewPermutation(4, 8, 56), matching Barretenberg/Noir's
// std::hash::poseidon2_permutation -- cross-verified byte-for-byte across 6 test vectors against
// this repo's pinned nargo 1.0.0-beta.22 toolchain before this switch) over the 4 fields, taking
// output[0] as the digest. prevHash/stateRoot must already be field-reduced (see ReduceToField).
func HashHeader(prevHash []byte, fsmState string, withdrawalLocked bool, stateRoot []byte) []byte {
	var state [4]fr.Element
	state[0].SetBytes(prevHash)
	state[1].SetInt64(FSMStateToField(fsmState))
	state[2].SetInt64(BoolToField(withdrawalLocked))
	state[3].SetBytes(stateRoot)

	if err := poseidon2.NewPermutation(4, 8, 56).Permutation(state[:]); err != nil {
		panic(err)
	}
	out := state[0].Bytes()
	return out[:]
}
