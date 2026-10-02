package keeper

import (
	_ "embed"
	"encoding/binary"
	"fmt"
)

// embeddedVK is circuit/reanchoring's compiled verification key (via `bb write_vk`), copied
// here since target/ is gitignored -- replace by hand if the circuit is ever recompiled.
//
//go:embed zk_assets/vk
var embeddedVK []byte

// embeddedAggregateVK is circuit/reanchoring_aggregator's verification key. It hardcodes the
// recursive-target leaf VK and K, so regenerate it whenever circuit/reanchoring changes.
//
//go:embed zk_assets/vk_aggregate
var embeddedAggregateVK []byte

const (
	// Public-input layout shared with SubmitRecoveryProof: 7 big-endian 32-byte fields, `count`
	// being the u32 in the low 4 bytes of the third one.
	recoveryPublicInputsLen = 224
	recoveryCountOffset     = 92

	// recoveryLeafNMax is circuit/reanchoring's N_MAX; a longer interval can only come from the
	// aggregator.
	recoveryLeafNMax = 256
)

// recoveryVK picks the verification key by the interval length the proof claims: a single leaf
// proof covers at most N_MAX headers, anything longer must be a chunk aggregate. Malformed inputs
// fall through to the leaf VK and fail verification there.
func recoveryVK(inputs []byte) []byte {
	if len(inputs) == recoveryPublicInputsLen &&
		binary.BigEndian.Uint32(inputs[recoveryCountOffset:recoveryCountOffset+4]) > recoveryLeafNMax {
		return embeddedAggregateVK
	}
	return embeddedVK
}

// VerifyZKProof runs proof/inputs against the matching embedded VK via the embedded wasm32-wasi
// UltraHonk verifier (spec/core/EngramTendermint.tla's VerifyZkProof) -- deterministic across
// validators (pure-Go wazero, no `bb` CLI/PATH dependency); fails closed on any error.
func (k *Keeper) VerifyZKProof(proof, inputs []byte) bool {
	ok, err := verifyZKProofWasm(recoveryVK(inputs), proof, inputs)
	if err != nil {
		fmt.Println("engramd: ZK proof verification failed:", err)
		return false
	}
	return ok
}
