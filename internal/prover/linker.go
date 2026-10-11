package prover

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Linker runs circuit/reanchoring_witness: it chains the headers by prev_hash and returns the new root.
type Linker interface {
	Link(ctx context.Context, rtLast string, headers []Header) (linked []Linked, rtNew string, err error)
}

// ExecLinker runs `nargo execute` in a private copy of the witness circuit directory.
type ExecLinker struct {
	Nargo      string
	CircuitDir string // circuit/reanchoring_witness
}

func (l *ExecLinker) Link(ctx context.Context, rtLast string, headers []Header) ([]Linked, string, error) {
	dir, err := os.MkdirTemp("", "witness-*")
	if err != nil {
		return nil, "", err
	}
	defer os.RemoveAll(dir)
	if err := copyDir(l.CircuitDir, dir); err != nil {
		return nil, "", err
	}
	if err := os.WriteFile(filepath.Join(dir, "Prover.toml"), []byte(WitnessToml(rtLast, headers)), 0o600); err != nil {
		return nil, "", err
	}
	cmd := exec.CommandContext(ctx, l.Nargo, "execute", "witness") //nolint:gosec // Nargo is the operator-configured nargo binary, the arguments are constants
	cmd.Dir = dir
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return nil, "", fmt.Errorf("nargo execute (witness circuit): %w: %s", err, tail(errb.String()+out.String(), 400))
	}
	linked, rtNew, err := ParseLinked(out.String())
	if err != nil {
		return nil, "", err
	}
	if len(linked) != len(headers) {
		return nil, "", fmt.Errorf("the witness circuit linked %d headers, expected %d", len(linked), len(headers))
	}
	return linked, rtNew, nil
}

func tail(s string, n int) string {
	if len(s) > n {
		return s[len(s)-n:]
	}
	return s
}

// copyDir copies the files of src into the existing, empty directory dst (the circuit directories are small once target/proof is absent).
func copyDir(src, dst string) error {
	return os.CopyFS(dst, os.DirFS(src))
}
