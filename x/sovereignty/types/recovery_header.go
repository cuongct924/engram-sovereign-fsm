package types

import (
	"crypto/sha256"
	"math/big"
)

// RecoveryHeader mirrors the ZK re-anchoring circuit's Header witness field
// (circuit/reanchoring/src/main.nr's Header{prev_hash, fsm_state,
// withdrawal_locked, state_root}) for one SOVEREIGN/RECOVERING block
// (spec/README.md's §Re-anchoring via ZK-Proof of Recovery).
//
// prev_hash is omitted: derived at proof time by the off-chain witness
// helper (circuit/reanchoring_witness/), chaining HashHeader from
// LastAnchoredRoot -- never stored in Go. StateRoot is CometBFT's real
// per-block AppHash, reduced to field. Accumulator is this height's
// HashHeader output (preblock.go's CommitFSMTransition), i.e. what a real,
// honest witness chain from rt_last would produce by this height -- see
// HashHeader's doc for why msg_server.go binds rt_new to this, not StateRoot.
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

// HashHeader ports circuit/reanchoring/src/main.nr's hash_header exactly:
// concatenates prevHash/fsm_state/withdrawal_locked/stateRoot as 32-byte
// big-endian field elements, SHA-256s the 128-byte result, then folds the
// digest back into the scalar field the same way the circuit does (a
// big-endian byte fold, equivalent to ReduceToField -- cross-verified
// byte-for-byte against the Noir implementation before this landed).
// prevHash and stateRoot must already be 32-byte field encodings (e.g.
// ReduceToField's output); fsmState is a string (StateSovereign/
// StateRecovering), converted via FSMStateToField.
func HashHeader(prevHash []byte, fsmState string, withdrawalLocked bool, stateRoot []byte) []byte {
	var buf [128]byte
	copy(buf[0:32], leftPad32(prevHash))
	copy(buf[32:64], leftPad32(big.NewInt(FSMStateToField(fsmState)).Bytes()))
	copy(buf[64:96], leftPad32(big.NewInt(BoolToField(withdrawalLocked)).Bytes()))
	copy(buf[96:128], leftPad32(stateRoot))

	digest := sha256.Sum256(buf[:])
	return ReduceToField(digest[:])
}

// leftPad32 zero-pads b on the left to exactly 32 bytes (big-endian),
// matching Noir's Field.to_be_bytes::<32>() -- panics if b is already longer,
// which would mean a caller passed a non-field-reduced value.
func leftPad32(b []byte) []byte {
	if len(b) > 32 {
		panic("leftPad32: input longer than 32 bytes")
	}
	out := make([]byte, 32)
	copy(out[32-len(b):], b)
	return out
}
