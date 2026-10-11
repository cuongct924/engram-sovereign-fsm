package prover

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, _ := os.Getwd()
	return filepath.Join(wd, "..", "..")
}

// The witness circuit's real output must parse: this guards the format ParseLinked depends on. Skipped without nargo.
func TestExecLinkerWithRealNargo(t *testing.T) {
	if _, err := exec.LookPath("nargo"); err != nil {
		t.Skip("nargo not installed")
	}
	l := &ExecLinker{Nargo: "nargo", CircuitDir: filepath.Join(repoRoot(t), "circuit", "reanchoring_witness")}
	hs := []Header{{1, "2", "1", "11"}, {2, "2", "1", "12"}, {3, "3", "1", "13"}}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	linked, rtNew, err := l.Link(ctx, "1", hs)
	if err != nil {
		t.Fatal(err)
	}
	if len(linked) != 3 || linked[0].PrevHash != "0x01" || linked[2].FSMState != "0x03" || len(rtNew) < 10 {
		t.Fatalf("linked %+v rt_new %q", linked, rtNew)
	}
}

// ENGRAM_E2E_PROVER_TOML=<a valid Prover.toml of circuit/reanchoring> runs one real proof with nargo and bb (about two minutes).
func TestLocalProverWithRealTools(t *testing.T) {
	path := os.Getenv("ENGRAM_E2E_PROVER_TOML")
	if path == "" {
		t.Skip("set ENGRAM_E2E_PROVER_TOML to run a real proof")
	}
	toml, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	root := repoRoot(t)
	p := &LocalProver{
		Nargo: "nargo", BB: "bb", CircuitDir: filepath.Join(root, "circuit", "reanchoring"),
		VK: filepath.Join(root, "x", "sovereignty", "keeper", "zk_assets", "vk"),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	pr, err := p.Prove(ctx, "e2e", toml)
	if err != nil {
		t.Fatal(err)
	}
	if len(pr.Proof) != 14656 || len(pr.PublicInputs) != 224 {
		t.Fatalf("proof %d bytes, public inputs %d bytes", len(pr.Proof), len(pr.PublicInputs))
	}
}
