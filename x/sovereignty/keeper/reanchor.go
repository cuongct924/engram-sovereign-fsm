package keeper

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// embeddedVK is circuit/reanchoring/src/main.nr's compiled verification key
// (circuit/reanchoring/target/proof/vk, via `bb write_vk`), copied here
// since the embed directive below can't reach outside this package and
// target/ is gitignored. Replace by hand if the circuit is ever recompiled.
//
//go:embed zk_assets/vk
var embeddedVK []byte

// bbBinary is the pinned bb (Barretenberg) CLI used for verification -- see
// VerifyZKProof's doc for why every validator must run the same build.
const bbBinary = "bb"

// VerifyZKProof shells out to `bb verify` against the embedded verification
// key -- the concrete counterpart of spec/core/EngramTendermint.tla's
// VerifyZkProof, backing MsgSubmitRecoveryProofRequest (proposal.go's
// verifyZkProofFlag is the separate abstract consensus-hot-path check).
//
// Runs inside DeliverTx, executed identically by every validator, so every
// validator must run the same pinned bb release. Fails closed (false, never
// panics) on any error, so a misconfigured validator rejects rather than
// silently accepts.
func (k *Keeper) VerifyZKProof(proof, inputs []byte) bool {
	dir, err := os.MkdirTemp("", "engram-zkverify-*")
	if err != nil {
		fmt.Println("engramd: ZK proof verification failed to allocate temp dir:", err)
		return false
	}
	defer os.RemoveAll(dir)

	vkPath := filepath.Join(dir, "vk")
	proofPath := filepath.Join(dir, "proof")
	inputsPath := filepath.Join(dir, "public_inputs")
	if err := os.WriteFile(vkPath, embeddedVK, 0o600); err != nil {
		fmt.Println("engramd: ZK proof verification failed writing vk:", err)
		return false
	}
	if err := os.WriteFile(proofPath, proof, 0o600); err != nil {
		fmt.Println("engramd: ZK proof verification failed writing proof:", err)
		return false
	}
	if err := os.WriteFile(inputsPath, inputs, 0o600); err != nil {
		fmt.Println("engramd: ZK proof verification failed writing public_inputs:", err)
		return false
	}

	// 10s is a hang safety-valve, not a tight bound -- real verify_s is ~20ms.
	execCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(execCtx, bbBinary, "verify", "-k", vkPath, "-p", proofPath, "-i", inputsPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("engramd: ZK proof verification rejected: %v: %s\n", err, stderr.String())
		return false
	}
	return true
}
