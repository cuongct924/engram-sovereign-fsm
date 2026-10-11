// Package prover schedules the re-anchoring proofs: it prepares each segment's witness, has the proof built (here or on worker hosts) and
// submits the proofs in order, keeping several in flight so that the unproven tail stays short. It replaces the shell scripts
// of the first version (a segment script, a worker hand-off script and a scheduler) and keeps their protocol: a segment proves the headers from the end of the
// previous segment, so segments can be proven at the same time and are accepted by SubmitRecoveryProof only in order.
package prover

import (
	"bufio"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// NMax is the capacity of circuit/reanchoring (headers per proof); it must match the circuit's `global N_MAX`.
const NMax = 256

// Header is one tracked header as printed by `engramd query-recovery-headers`.
type Header struct {
	Height           uint64
	FSMState         string
	WithdrawalLocked string
	StateRoot        string
}

// Linked is a header after the witness circuit has linked it into the prev_hash chain.
type Linked struct {
	PrevHash         string
	FSMState         string
	WithdrawalLocked string
	StateRoot        string
}

// ParseHeaders reads the output of `engramd query-recovery-headers`: the last anchored root (rt_last, decimal) and the tracked headers.
// Other lines (first_height, validator and block hash lines) are ignored.
func ParseHeaders(out string) (rtLast string, headers []Header, err error) {
	sc := bufio.NewScanner(strings.NewReader(out))
	sc.Buffer(make([]byte, 0, 1<<20), 1<<26)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case strings.HasPrefix(line, "rt_last="):
			rtLast = strings.TrimPrefix(line, "rt_last=")
		case strings.HasPrefix(line, "height="):
			h, perr := parseHeaderLine(line)
			if perr != nil {
				return "", nil, perr
			}
			headers = append(headers, h)
		}
	}
	if err := sc.Err(); err != nil {
		return "", nil, err
	}
	if rtLast == "" {
		return "", nil, fmt.Errorf("no rt_last in the query output")
	}
	return rtLast, headers, nil
}

func parseHeaderLine(line string) (Header, error) {
	var h Header
	for _, f := range strings.Fields(line) {
		k, v, ok := strings.Cut(f, "=")
		if !ok {
			continue
		}
		switch k {
		case "height":
			n, err := strconv.ParseUint(v, 10, 64)
			if err != nil {
				return h, fmt.Errorf("bad height in %q: %w", line, err)
			}
			h.Height = n
		case "fsm_state":
			h.FSMState = v
		case "withdrawal_locked":
			h.WithdrawalLocked = v
		case "state_root":
			h.StateRoot = v
		}
	}
	if h.FSMState == "" || h.WithdrawalLocked == "" || h.StateRoot == "" {
		return h, fmt.Errorf("incomplete header line %q", line)
	}
	return h, nil
}

var linkedRe = regexp.MustCompile(`^Header \{ prev_hash: (\S+), fsm_state: (\S+), withdrawal_locked: (\S+), state_root: (\S+) \}$`)

// ParseLinked reads the output of `nargo execute` on circuit/reanchoring_witness: one `Header { ... }` line per proven header and the
// `rt_new_hash=` line (hash_header of the last header, not that header's raw state root).
func ParseLinked(out string) (linked []Linked, rtNew string, err error) {
	sc := bufio.NewScanner(strings.NewReader(out))
	sc.Buffer(make([]byte, 0, 1<<20), 1<<26)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "rt_new_hash=") {
			rtNew = strings.TrimPrefix(line, "rt_new_hash=")
			continue
		}
		if m := linkedRe.FindStringSubmatch(line); m != nil {
			linked = append(linked, Linked{PrevHash: m[1], FSMState: m[2], WithdrawalLocked: m[3], StateRoot: m[4]})
		}
	}
	if err := sc.Err(); err != nil {
		return nil, "", err
	}
	if rtNew == "" {
		return nil, "", fmt.Errorf("the witness circuit printed no rt_new_hash")
	}
	return linked, rtNew, nil
}

func quotedArray(vals []string, n int) string {
	var b strings.Builder
	b.WriteString("[")
	for i := 0; i < n; i++ {
		v := "0"
		if i < len(vals) {
			v = vals[i]
		}
		b.WriteString(`"` + v + `",`)
	}
	b.WriteString("]")
	return b.String()
}

// WitnessToml is the Prover.toml of circuit/reanchoring_witness: the headers' raw fields padded with zeros up to NMax.
func WitnessToml(rtLast string, headers []Header) string {
	col := func(get func(Header) string) []string {
		v := make([]string, len(headers))
		for i, h := range headers {
			v[i] = get(h)
		}
		return v
	}
	var b strings.Builder
	fmt.Fprintf(&b, "rt_last = \"%s\"\n", rtLast)
	fmt.Fprintf(&b, "count = \"%d\"\n", len(headers))
	fmt.Fprintf(&b, "raw_fsm_state = %s\n", quotedArray(col(func(h Header) string { return h.FSMState }), NMax))
	fmt.Fprintf(&b, "raw_withdrawal_locked = %s\n", quotedArray(col(func(h Header) string { return h.WithdrawalLocked }), NMax))
	fmt.Fprintf(&b, "raw_state_root = %s\n", quotedArray(col(func(h Header) string { return h.StateRoot }), NMax))
	return b.String()
}

// SplitBoundary splits the boundary-witness TOML into the loose keys before the first table and the tables from the first `[` line: TOML
// needs every loose key before any table, or nargo misattributes later keys to the wrong scope.
func SplitBoundary(toml string) (loose, tables string) {
	lines := strings.Split(toml, "\n")
	for i, l := range lines {
		if strings.HasPrefix(l, "[") {
			return strings.Join(lines[:i], "\n"), strings.Join(lines[i:], "\n")
		}
	}
	return toml, ""
}

func headersToml(linked []Linked, padTo int) string {
	var b strings.Builder
	for _, l := range linked {
		fmt.Fprintf(&b, "\n[[headers]]\nprev_hash = \"%s\"\nfsm_state = \"%s\"\nwithdrawal_locked = \"%s\"\nstate_root = \"%s\"\n",
			l.PrevHash, l.FSMState, l.WithdrawalLocked, l.StateRoot)
	}
	for i := len(linked); i < padTo; i++ {
		b.WriteString("\n[[headers]]\nprev_hash = \"0\"\nfsm_state = \"0\"\nwithdrawal_locked = \"0\"\nstate_root = \"0\"\n")
	}
	return b.String()
}

// CircuitToml is the Prover.toml of circuit/reanchoring: the roots, the count, the boundary-block witness and the linked headers padded to
// NMax (the padding slots are unconstrained by the circuit's `active` gate once i >= count).
func CircuitToml(rtLast, rtNew string, linked []Linked, boundary string) string {
	loose, tables := SplitBoundary(boundary)
	var b strings.Builder
	fmt.Fprintf(&b, "rt_last = \"%s\"\n", rtLast)
	fmt.Fprintf(&b, "rt_new = \"%s\"\n", rtNew)
	fmt.Fprintf(&b, "count = \"%d\"\n", len(linked))
	b.WriteString(loose)
	b.WriteString("\n\n")
	b.WriteString(tables)
	b.WriteString("\n")
	b.WriteString(headersToml(linked, NMax))
	return b.String()
}

// AuditToml is the file published to Celestia as the header chain of a proof (no boundary witness, no padding).
func AuditToml(rtLast, rtNew string, linked []Linked) string {
	var b strings.Builder
	fmt.Fprintf(&b, "rt_last = \"%s\"\nrt_new = \"%s\"\ncount = \"%d\"\n", rtLast, rtNew, len(linked))
	b.WriteString(headersToml(linked, 0))
	return b.String()
}
