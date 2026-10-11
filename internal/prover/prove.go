package prover

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Proof is what bb produces for one segment.
type Proof struct {
	Proof        []byte
	PublicInputs []byte
	Worker       string // where it was proven ("local" or user@host)
}

// Prover builds the proof of one segment from its Prover.toml. job names the segment (it names the container on a worker).
type Prover interface {
	Prove(ctx context.Context, job string, proverToml []byte) (Proof, error)
}

// LocalProver runs nargo execute and bb prove on this host, one proof per call (calls may overlap; each uses a private copy of the circuit).
type LocalProver struct {
	Nargo, BB  string
	CircuitDir string // circuit/reanchoring, compiled (target/reanchoring.json)
	VK         string // x/sovereignty/keeper/zk_assets/vk
	BBThreads  int    // HARDWARE_CONCURRENCY for bb, 0 for all cores
}

func (p *LocalProver) Prove(ctx context.Context, job string, toml []byte) (Proof, error) {
	dir, err := os.MkdirTemp("", "prove-*")
	if err != nil {
		return Proof{}, err
	}
	defer os.RemoveAll(dir)
	if err := copyDir(p.CircuitDir, dir); err != nil {
		return Proof{}, err
	}
	_ = os.RemoveAll(filepath.Join(dir, "target", "proof"))
	if err := os.MkdirAll(filepath.Join(dir, "target", "proof"), 0o700); err != nil {
		return Proof{}, err
	}
	if err := os.WriteFile(filepath.Join(dir, "Prover.toml"), toml, 0o600); err != nil {
		return Proof{}, err
	}
	vk, err := os.ReadFile(p.VK)
	if err != nil {
		return Proof{}, err
	}
	if err := os.WriteFile(filepath.Join(dir, "target", "proof", "vk"), vk, 0o600); err != nil {
		return Proof{}, err
	}
	var logs bytes.Buffer
	run := func(env []string, name string, args ...string) error {
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Dir = dir
		cmd.Stdout, cmd.Stderr = &logs, &logs
		cmd.Env = append(os.Environ(), env...)
		return cmd.Run()
	}
	if err := run(nil, p.Nargo, "execute", "witness"); err != nil {
		return Proof{}, fmt.Errorf("nargo execute: %w: %s", err, tail(logs.String(), 400))
	}
	var env []string
	if p.BBThreads > 0 {
		env = []string{"HARDWARE_CONCURRENCY=" + strconv.Itoa(p.BBThreads)}
	}
	if err := run(env, p.BB, "prove", "-b", "target/reanchoring.json", "-w", "target/witness.gz", "-o", "target/proof", "-k", "target/proof/vk"); err != nil {
		return Proof{}, fmt.Errorf("bb prove: %w: %s", err, tail(logs.String(), 400))
	}
	return readProof(filepath.Join(dir, "target", "proof"), "local")
}

func readProof(dir, worker string) (Proof, error) {
	pr, err := os.ReadFile(filepath.Join(dir, "proof"))
	if err != nil {
		return Proof{}, err
	}
	pub, err := os.ReadFile(filepath.Join(dir, "public_inputs"))
	if err != nil {
		return Proof{}, err
	}
	if len(pr) == 0 || len(pub) == 0 {
		return Proof{}, fmt.Errorf("empty proof or public inputs")
	}
	return Proof{Proof: pr, PublicInputs: pub, Worker: worker}, nil
}

// Runner starts a command with the given stdin and returns its stdout; it must stop the command when ctx ends. It is a field so that tests
// can stand in for ssh.
type Runner func(ctx context.Context, stdin io.Reader, name string, args ...string) (stdout []byte, stderr string, err error)

// ExecRunner runs a real command.
func ExecRunner(ctx context.Context, stdin io.Reader, name string, args ...string) ([]byte, string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = stdin
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	cmd.WaitDelay = 5 * time.Second
	err := cmd.Run()
	return out.Bytes(), errb.String(), err
}

// SSHPool hands each proof to a free worker host: one proof per worker at a time. The worker runs worker_prove.sh inside the prover image
// (Prover.toml on stdin, a tar of proof and public_inputs on stdout).
type SSHPool struct {
	Workers    []string // user@host
	Key        string   // private key of the SSH user
	Image      string   // prover image on the workers
	KnownHosts string   // file for the workers' host keys
	Run        Runner   // default ExecRunner

	free chan string
}

// NewSSHPool returns a pool with every worker free.
func NewSSHPool(workers []string, key, image, knownHosts string) *SSHPool {
	p := &SSHPool{Workers: workers, Key: key, Image: image, KnownHosts: knownHosts, Run: ExecRunner, free: make(chan string, len(workers))}
	for _, w := range workers {
		p.free <- w
	}
	return p
}

func (p *SSHPool) sshArgs(host, cmd string) []string {
	return []string{
		"-i", p.Key, "-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=accept-new", "-o", "UserKnownHostsFile=" + p.KnownHosts,
		"-o", "ServerAliveInterval=15", "-o", "ServerAliveCountMax=4", host, cmd,
	}
}

func (p *SSHPool) Prove(ctx context.Context, job string, toml []byte) (Proof, error) {
	var host string
	select {
	case host = <-p.free:
	case <-ctx.Done():
		return Proof{}, ctx.Err()
	}
	defer func() { p.free <- host }()
	name := "engram-prove-" + job
	remote := fmt.Sprintf("docker run --rm -i --name %s --entrypoint /app/scripts/reanchoring_prover/worker_prove.sh %s", name, p.Image)
	out, stderr, err := p.Run(ctx, bytes.NewReader(toml), "ssh", p.sshArgs(host, remote)...)
	if ctx.Err() != nil {
		// canceled: remove the container so that the worker is not kept busy by a proof nobody wants (own context, ctx has ended)
		rctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_, _, _ = p.Run(rctx, nil, "ssh", p.sshArgs(host, "docker rm -f "+name)...)
		return Proof{}, ctx.Err()
	}
	if err != nil {
		return Proof{}, fmt.Errorf("worker %s: %w: %s", host, err, tail(strings.TrimSpace(stderr), 400))
	}
	files, err := untar(out)
	if err != nil {
		return Proof{}, fmt.Errorf("worker %s: %w", host, err)
	}
	if len(files["proof"]) == 0 || len(files["public_inputs"]) == 0 {
		return Proof{}, fmt.Errorf("worker %s returned no proof: %s", host, tail(strings.TrimSpace(stderr), 400))
	}
	return Proof{Proof: files["proof"], PublicInputs: files["public_inputs"], Worker: host}, nil
}

func untar(b []byte) (map[string][]byte, error) {
	files := map[string][]byte{}
	tr := tar.NewReader(bytes.NewReader(b))
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return files, nil
		}
		if err != nil {
			return nil, fmt.Errorf("reading the worker's tar: %w", err)
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}
		files[filepath.Base(h.Name)] = data
	}
}
