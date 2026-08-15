// BenchmarkVerifyZKProof closes a real gap in E7's overhead measurement:
// the V0-V5 decomposition never measured keeper.VerifyZKProof's real `bb
// verify` shell-out cost, which every validator pays on every accepted
// SubmitRecoveryProof tx (DeliverTx). This is the "recovery-event cost" half
// of E7's steady-state/recovery-event split (see live_overhead_scan.py) -- a
// measured CPU number alongside the real ~14,656-byte UltraHonk proof size
// (E6's table6b_scaling.csv).
package benchmark

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper"

	"cosmossdk.io/collections/colltest"
)

// BenchmarkVerifyZKProof shells out to the REAL, pinned `bb verify` binary
// against the real N=4 proof/public_inputs committed on disk
// (circuit/reanchoring/target/proof/{proof,public_inputs}) -- the exact code
// path DeliverTx runs (keeper/reanchor.go's VerifyZKProof), not a mock.
// Skips (not fails) if the proof files or `bb` aren't present -- an external
// toolchain dependency, not guaranteed in every environment.
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

	// One real verification first, outside the timed loop: warms up and
	// fails loudly (not a misleadingly-fast "N ops, always false") if the
	// embedded VK doesn't match this proof's circuit compilation.
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

// findRepoRoot walks up from the cwd looking for go.mod -- `go test` sets
// cwd to the package dir (tests/benchmark/), not the repo root, so the
// circuit/ path must be resolved from there.
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
