package keeper

import (
	_ "embed"
	"fmt"
)

// embeddedVK is circuit/reanchoring's compiled verification key (via `bb write_vk`), copied
// here since target/ is gitignored -- replace by hand if the circuit is ever recompiled.
//
//go:embed zk_assets/vk
var embeddedVK []byte

// VerifyZKProof runs proof/inputs against embeddedVK via the embedded wasm32-wasi UltraHonk
// verifier (spec/core/EngramTendermint.tla's VerifyZkProof) -- deterministic across
// validators (pure-Go wazero, no `bb` CLI/PATH dependency); fails closed on any error.
func (k *Keeper) VerifyZKProof(proof, inputs []byte) bool {
	ok, err := verifyZKProofWasm(embeddedVK, proof, inputs)
	if err != nil {
		fmt.Println("engramd: ZK proof verification failed:", err)
		return false
	}
	return ok
}
