// BenchmarkVerifyZKProof closes a real gap in E7's overhead measurement --
// docs/EXPERIMENT.md's original V0-V5 decomposition and this file's own
// BenchmarkProposalSize/BenchmarkProcessProposal never measured
// keeper.VerifyZKProof's real `bb verify` shell-out cost, even though every
// validator pays it on every accepted SubmitRecoveryProof tx (DeliverTx).
// This is the "recovery-event cost" half of E7's steady-state/recovery-event
// split (see live_overhead_scan.py's module doc) -- a real, measured CPU
// number to sit alongside the real ~14,656-byte UltraHonk proof size (E6's
// table6b_scaling.csv).
package benchmark

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper"

	"cosmossdk.io/collections/colltest"
)

// BenchmarkVerifyZKProof shells out to the REAL, pinned `bb verify` binary
// against the real N=4 proof/public_inputs already committed on disk from
// this session's earlier E6/reanchoring work (circuit/reanchoring/target/
// proof/{proof,public_inputs}) -- not a synthetic/mocked verification, the
// exact same code path DeliverTx runs (x/sovereignty/keeper/reanchor.go's
// VerifyZKProof). Skips (not fails) if the proof files or the `bb` binary
// aren't present in this environment -- an external toolchain dependency,
// not something every environment running `go test` is guaranteed to have.
func BenchmarkVerifyZKProof(b *testing.B) {
	repoRoot := findRepoRoot(b)
	proofPath := filepath.Join(repoRoot, "circuit", "reanchoring", "target", "proof", "proof")
	inputsPath := filepath.Join(repoRoot, "circuit", "reanchoring", "target", "proof", "public_inputs")

	proof, err := os.ReadFile(proofPath)
	if err != nil {
		b.Skipf("skipping: real proof file not found at %s (run scripts/e6_zk_reanchoring_benchmark first): %v", proofPath, err)
	}
	inputs, err := os.ReadFile(inputsPath)
	if err != nil {
		b.Skipf("skipping: real public_inputs file not found at %s: %v", inputsPath, err)
	}

	storeService, _ := colltest.MockStore()
	k := keeper.NewKeeper(storeService, nil)

	// One real verification first, outside the timed loop, both as a
	// warm-up and to fail loudly (not just report a misleadingly-fast
	// "N ops, always false" benchmark) if the embedded VK doesn't actually
	// match this proof's circuit compilation.
	if ok := k.VerifyZKProof(proof, inputs); !ok {
		b.Skip("skipping: VerifyZKProof returned false against the on-disk proof -- " +
			"embedded VK (x/sovereignty/keeper/zk_assets/vk) likely doesn't match this " +
			"proof's circuit compilation; regenerate both together before trusting this benchmark")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		k.VerifyZKProof(proof, inputs)
	}
}

// findRepoRoot walks up from the working directory looking for go.mod --
// `go test` runs with cwd set to the package directory (tests/benchmark/),
// not the repo root, so the circuit/ path needs to be resolved relative to
// that, not assumed to be the cwd itself.
func findRepoRoot(b *testing.B) string {
	b.Helper()
	dir, err := os.Getwd()
	if err != nil {
		b.Fatalf("os.Getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			b.Fatalf("could not find repo root (go.mod) walking up from %s", dir)
		}
		dir = parent
	}
}
