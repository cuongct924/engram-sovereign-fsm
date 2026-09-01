package keeper

import (
	_ "embed"
	"fmt"
)

// embeddedVK is circuit/reanchoring/src/main.nr's compiled verification key
// (circuit/reanchoring/target/proof/vk, via `bb write_vk`), copied here
// since the embed directive below can't reach outside this package and
// target/ is gitignored. Replace by hand if the circuit is ever recompiled.
//
//go:embed zk_assets/vk
var embeddedVK []byte

// VerifyZKProof runs proof/inputs against embeddedVK through the embedded
// wasm32-wasi UltraHonk verifier (zk_verifier_wasm.go), the concrete
// counterpart of spec/core/EngramTendermint.tla's VerifyZkProof, backing
// MsgSubmitRecoveryProofRequest (proposal.go's verifyZkProofFlag is the
// separate abstract consensus-hot-path check).
//
// Runs inside DeliverTx, executed identically -- same wasm bytecode, same
// pure-Go wazero runtime -- by every validator regardless of OS/CPU, so
// unlike the previous `bb verify` CLI shell-out this has no external-binary
// or PATH dependency to keep pinned across validators. Fails closed (false,
// never panics) on any error, so a corrupted/misbuilt asset rejects rather
// than silently accepts.
func (k *Keeper) VerifyZKProof(proof, inputs []byte) bool {
	ok, err := verifyZKProofWasm(embeddedVK, proof, inputs)
	if err != nil {
		fmt.Println("engramd: ZK proof verification failed:", err)
		return false
	}
	return ok
}
