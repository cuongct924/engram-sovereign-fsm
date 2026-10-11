package prover

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Chain is what the prover needs from the node: the tracked headers, the boundary-block witness, the tip, and the two transactions.
type Chain interface {
	// Headers returns the last anchored root and the tracked headers (the interval not yet covered by an accepted proof).
	Headers(ctx context.Context) (rtLast string, headers []Header, err error)
	// BoundaryWitness returns the boundary-witness TOML for a segment that ends at lastHeight; firstHeight is 0 for the interval's start.
	BoundaryWitness(ctx context.Context, firstHeight, lastHeight uint64) (string, error)
	// Tip is the latest committed height, 0 when it cannot be read (it is only used for the log line of a proof).
	Tip(ctx context.Context) uint64
	// PublishWitness publishes an audit TOML to Celestia and returns the DA height.
	PublishWitness(ctx context.Context, tomlPath string) (uint64, error)
	// Submit sends the proof; daHeight is 0 when the witness was not published.
	Submit(ctx context.Context, proofPath, publicInputsPath string, daHeight uint64) error
}

// ExecChain implements Chain with the engramd binary (the same commands the shell scripts called).
type ExecChain struct {
	Engramd string
	NodeURL string
	ChainID string
}

func (c *ExecChain) run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, c.Engramd, args...) //nolint:gosec // Engramd is the operator-configured binary; the arguments are built by this package
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("%s %s: %w: %s", filepath.Base(c.Engramd), args[0], err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

func (c *ExecChain) Headers(ctx context.Context) (string, []Header, error) {
	out, err := c.run(ctx, "query-recovery-headers", "--node", c.NodeURL)
	if err != nil {
		return "", nil, err
	}
	return ParseHeaders(out)
}

func (c *ExecChain) BoundaryWitness(ctx context.Context, firstHeight, lastHeight uint64) (string, error) {
	f, err := os.CreateTemp("", "boundary-*.toml")
	if err != nil {
		return "", err
	}
	f.Close()
	defer os.Remove(f.Name())
	args := []string{"build-boundary-witness", "--node", c.NodeURL, "--chain-id", c.ChainID, "--last-height", strconv.FormatUint(lastHeight, 10), "--out", f.Name()}
	if firstHeight != 0 {
		args = append(args, "--first-height", strconv.FormatUint(firstHeight, 10))
	}
	if _, err := c.run(ctx, args...); err != nil {
		return "", err
	}
	b, err := os.ReadFile(f.Name())
	return string(b), err
}

func (c *ExecChain) Tip(ctx context.Context) uint64 {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(c.NodeURL, "/")+"/status", nil)
	if err != nil {
		return 0
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	var st struct {
		Result struct {
			SyncInfo struct {
				Height string `json:"latest_block_height"`
			} `json:"sync_info"`
		} `json:"result"`
	}
	if json.NewDecoder(resp.Body).Decode(&st) != nil {
		return 0
	}
	h, _ := strconv.ParseUint(st.Result.SyncInfo.Height, 10, 64)
	return h
}

func (c *ExecChain) PublishWitness(ctx context.Context, tomlPath string) (uint64, error) {
	out, err := c.run(ctx, "publish-recovery-witness", "--headers", tomlPath)
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(out, "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "da_celestia_height="); ok {
			return strconv.ParseUint(v, 10, 64)
		}
	}
	return 0, fmt.Errorf("no da_celestia_height in the publish output")
}

func (c *ExecChain) Submit(ctx context.Context, proofPath, publicInputsPath string, daHeight uint64) error {
	args := []string{"tx-submit-recovery-proof", "--node", c.NodeURL, "--proof", proofPath, "--public-inputs", publicInputsPath}
	if daHeight != 0 {
		args = append(args, "--da-height", strconv.FormatUint(daHeight, 10))
	}
	_, err := c.run(ctx, args...)
	return err
}
