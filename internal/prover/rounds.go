package prover

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// BoundaryRounds reads the commit rounds of the two boundary blocks from the boundary-witness TOML (the `round = "N"` line of the
// [first_block] and [last_block] tables). A table without a round line yields -1.
func BoundaryRounds(toml string) (first, last int) {
	first, last = -1, -1
	section := ""
	for _, l := range strings.Split(toml, "\n") {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "[") && !strings.HasPrefix(l, "[[") {
			section = strings.Trim(l, "[]")
			continue
		}
		v, ok := strings.CutPrefix(l, "round = \"")
		if !ok {
			continue
		}
		n, err := strconv.Atoi(strings.TrimSuffix(v, `"`))
		if err != nil {
			continue
		}
		switch section {
		case "first_block":
			first = n
		case "last_block":
			last = n
		}
	}
	return first, last
}

// archiveProof keeps a copy of a proof and its public inputs (the negative tests of the recovery experiment mutate them).
func archiveProof(dir string, id int, last uint64, proofPath, pubPath string, daHeight uint64) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	base := filepath.Join(dir, fmt.Sprintf("seg%d_h%d", id, last))
	for src, dst := range map[string]string{proofPath: base + "_proof", pubPath: base + "_public_inputs"} {
		b, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		if err := os.WriteFile(dst, b, 0o644); err != nil {
			return err
		}
	}
	return os.WriteFile(base+"_da_height", []byte(strconv.FormatUint(daHeight, 10)), 0o644)
}

// Control is the test control of the recovery experiment, read from files of Config.ControlDir so that it can change while the prover runs
// (a genesis recovery must go through unchanged, the experiment's segments must not):
//
//	want_round  the number N > 0: a segment ends only at a block that committed in round N
//	hold        when the file exists, a finished proof is archived and held until the file is removed
type Control struct{ WantRound int }

func ReadControl(dir string) Control {
	if dir == "" {
		return Control{}
	}
	b, err := os.ReadFile(filepath.Join(dir, "want_round"))
	if err != nil {
		return Control{}
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || n < 0 {
		return Control{}
	}
	return Control{WantRound: n}
}

// HoldRequested reports whether the hold file exists.
func HoldRequested(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "hold"))
	return err == nil
}

// waitWhileHeld blocks until the hold file is removed or ctx ends.
func waitWhileHeld(ctxDone <-chan struct{}, dir string) bool {
	for HoldRequested(dir) {
		select {
		case <-ctxDone:
			return false
		case <-time.After(50 * time.Millisecond):
		}
	}
	return true
}
