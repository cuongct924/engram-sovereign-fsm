//go:build ignore

// generate_chain.go builds a real, valid, hash-linked header chain AND a
// real, structurally-valid boundary-signature witness (7 fresh ed25519
// keypairs, 5-of-7 real precommit signatures) and prints a complete
// Prover.toml for main.nr's main() at count=N1, N_MAX=N2 (argv). The
// header-chain and boundary-signature halves are intentionally unrelated
// fabricated values -- they're independent circuit-level assertions with
// no cross-binding here (the real chain ties them together via keeper
// lookups this benchmark doesn't exercise).
//
// Usage: go run generate_chain.go <count> <n_max> > Prover.toml
package main

import (
	"bytes"
	"fmt"
	"math/big"
	"os"
	"sort"
	"strconv"
	"time"

	edwards25519 "filippo.io/edwards25519"
	"filippo.io/edwards25519/field"

	cmted25519 "github.com/cometbft/cometbft/crypto/ed25519"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	cmttypes "github.com/cometbft/cometbft/types"

	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
)

const (
	numValidators = 7
	chainID       = "engram-dev-1" // must match circuit/reanchoring/src/main.nr's CHAIN_ID
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: generate_chain.go <count> <n_max>")
		os.Exit(1)
	}
	count, err := strconv.Atoi(os.Args[1])
	if err != nil || count < 1 {
		fmt.Fprintln(os.Stderr, "count must be a positive integer:", err)
		os.Exit(1)
	}
	nMax, err := strconv.Atoi(os.Args[2])
	if err != nil || nMax < count {
		fmt.Fprintln(os.Stderr, "n_max must be >= count:", err)
		os.Exit(1)
	}

	// --- header-chain half (unrelated to the boundary half -- see doc) ---
	rtLast := types.ReduceToField(big.NewInt(111).Bytes())
	prevHash := rtLast

	type header struct {
		prevHash, stateRoot []byte
		fsmState            string
	}
	headers := make([]header, count)
	for i := 0; i < count; i++ {
		fsm := types.StateSovereign
		if i%2 == 1 {
			fsm = types.StateRecovering
		}
		stateRoot := types.ReduceToField(big.NewInt(int64(1000 + i)).Bytes())
		headers[i] = header{prevHash, stateRoot, fsm}
		prevHash = types.HashHeader(prevHash, fsm, true, stateRoot)
	}
	rtNew := prevHash

	// --- boundary-signature half ---
	firstHeight := uint64(1)
	lastHeight := firstHeight + uint64(count) - 1

	privs := make([]cmted25519.PrivKey, numValidators)
	for i := 0; i < numValidators; i++ {
		privs[i] = cmted25519.GenPrivKey()
	}
	// cmttypes.NewValidatorSet sorts by address before hashing -- the
	// circuit's leaf order must match, or validator_set_hash_7 disagrees
	// with the real ValidatorSet.Hash().
	sort.Slice(privs, func(i, j int) bool {
		return bytes.Compare(privs[i].PubKey().Address(), privs[j].PubKey().Address()) < 0
	})
	cmtValidators := make([]*cmttypes.Validator, numValidators)
	for i := 0; i < numValidators; i++ {
		cmtValidators[i] = cmttypes.NewValidator(privs[i].PubKey(), 10)
	}
	validatorSetHash := cmttypes.NewValidatorSet(cmtValidators).Hash()

	firstBlockHash := fabricatedBlockHash(1)
	lastBlockHash := fabricatedBlockHash(2)

	firstBlock := buildBoundaryBlockWitness(firstHeight, firstBlockHash, privs, "first_block")
	lastBlock := buildBoundaryBlockWitness(lastHeight, lastBlockHash, privs, "last_block")

	// --- emit Prover.toml: ALL loose keys first, then tables, then
	// [[headers]] last -- a loose key after any [table] silently attaches
	// to the last table entry instead of the top level. ---
	fmt.Printf("rt_last = \"%s\"\n", fieldDecimal(rtLast))
	fmt.Printf("rt_new = \"%s\"\n", fieldDecimal(rtNew))
	fmt.Printf("count = \"%d\"\n", count)
	fmt.Printf("first_height = \"%d\"\n", firstHeight)
	fmt.Printf("first_block_hash = \"%s\"\n", fieldDecimal(types.ReduceToField(firstBlockHash)))
	fmt.Printf("last_block_hash = \"%s\"\n", fieldDecimal(types.ReduceToField(lastBlockHash)))
	fmt.Printf("validator_set_hash = \"%s\"\n", fieldDecimal(types.ReduceToField(validatorSetHash)))
	fmt.Printf("first_block_hash_bytes = [%s]\n", byteArrayToml(firstBlockHash))
	fmt.Printf("last_block_hash_bytes = [%s]\n", byteArrayToml(lastBlockHash))
	fmt.Printf("validator_set_hash_bytes = [%s]\n", byteArrayToml(validatorSetHash))

	fmt.Println("\n[validators]")
	pubKeyXs := make([][]byte, numValidators)
	pubKeyYs := make([][]byte, numValidators)
	powers := make([]string, numValidators)
	for i := 0; i < numValidators; i++ {
		x, y, err := decompressEd25519(privs[i].PubKey().Bytes())
		if err != nil {
			fmt.Fprintln(os.Stderr, "decompressing validator pubkey:", err)
			os.Exit(1)
		}
		pubKeyXs[i] = x[:]
		pubKeyYs[i] = y[:]
		powers[i] = "10"
	}
	fmt.Printf("pubkey_x = [%s]\n", byteArraysToml(pubKeyXs))
	fmt.Printf("pubkey_y = [%s]\n", byteArraysToml(pubKeyYs))
	fmt.Printf("power = [%s]\n", strings_join_quoted(powers))

	fmt.Println("\n[first_block]")
	fmt.Print(firstBlock)
	fmt.Println("\n[last_block]")
	fmt.Print(lastBlock)

	for _, h := range headers {
		fmt.Println()
		fmt.Println("[[headers]]")
		fmt.Printf("prev_hash = \"%s\"\n", fieldDecimal(h.prevHash))
		fmt.Printf("fsm_state = \"%d\"\n", types.FSMStateToField(h.fsmState))
		fmt.Printf("withdrawal_locked = \"1\"\n")
		fmt.Printf("state_root = \"%s\"\n", fieldDecimal(h.stateRoot))
	}
	for i := count; i < nMax; i++ {
		fmt.Println()
		fmt.Println("[[headers]]")
		fmt.Print("prev_hash = \"0\"\nfsm_state = \"0\"\nwithdrawal_locked = \"0\"\nstate_root = \"0\"\n")
	}
}

// fabricatedBlockHash returns a deterministic, distinguishable 32-byte
// value for the given seed -- not a real CometBFT block hash (this
// benchmark has no live block to hash), only a fixed value for the
// synthetic validators to sign consistently.
func fabricatedBlockHash(seed byte) []byte {
	h := make([]byte, 32)
	for i := range h {
		h[i] = seed*64 + byte(i)
	}
	return h
}

// buildBoundaryBlockWitness signs a real CometBFT precommit (5 of 7
// validators, >2/3 quorum) for (height, blockHash) and renders the
// BoundaryBlockWitness TOML fragment -- the offline counterpart of
// cmd/engramd/reanchor_witness_cli.go's version of this function.
func buildBoundaryBlockWitness(height uint64, blockHash []byte, privs []cmted25519.PrivKey, tableName string) string {
	psTotal := uint32(1)
	psHash := fabricatedBlockHash(3)
	ts := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	protoVote := &cmtproto.Vote{
		Type:   cmtproto.PrecommitType,
		Height: int64(height),
		Round:  0,
		BlockID: cmtproto.BlockID{
			Hash: blockHash,
			PartSetHeader: cmtproto.PartSetHeader{
				Total: psTotal,
				Hash:  psHash,
			},
		},
		Timestamp: ts,
	}
	signBytes := cmttypes.VoteSignBytes(chainID, protoVote)
	tsContent, err := extractTimestampContent(signBytes, chainID)
	if err != nil {
		fmt.Fprintln(os.Stderr, "extracting timestamp content:", err)
		os.Exit(1)
	}

	signed := make([]bool, numValidators)
	sigRXs := make([][]byte, numValidators)
	sigRYs := make([][]byte, numValidators)
	sigSs := make([][]byte, numValidators)
	var dummyRX, dummyRY, dummyS []byte
	for i := 0; i < numValidators; i++ {
		// 5 of 7 signed, matching main.nr's own test_boundary_fixture (>2/3 quorum).
		signed[i] = i < 5
		sig, err := privs[i].Sign(signBytes)
		if err != nil {
			fmt.Fprintln(os.Stderr, "signing:", err)
			os.Exit(1)
		}
		rx, ry, err := decompressEd25519(sig[:32])
		if err != nil {
			fmt.Fprintln(os.Stderr, "decompressing signature R:", err)
			os.Exit(1)
		}
		sigRXs[i] = rx[:]
		sigRYs[i] = ry[:]
		sigSs[i] = sig[32:64] // little-endian, RFC 8032 5.1.6
		if dummyRX == nil {
			dummyRX, dummyRY, dummyS = sigRXs[i], sigRYs[i], sigSs[i]
		}
	}
	// Every validator still needs a structurally valid (on-curve) signature
	// even when not counted -- ed25519_verify traps on a garbage point otherwise.
	for i := 0; i < numValidators; i++ {
		if !signed[i] {
			sigRXs[i], sigRYs[i], sigSs[i] = dummyRX, dummyRY, dummyS
		}
	}

	var b []byte
	b = append(b, []byte(fmt.Sprintf("ps_total = \"%d\"\n", psTotal))...)
	b = append(b, []byte(fmt.Sprintf("ps_hash = [%s]\n", byteArrayToml(psHash)))...)
	b = append(b, []byte(fmt.Sprintf("signed = [%s]\n", boolArrayToml(signed)))...)
	b = append(b, []byte(fmt.Sprintf("sig_r_x = [%s]\n", byteArraysToml(sigRXs)))...)
	b = append(b, []byte(fmt.Sprintf("sig_r_y = [%s]\n", byteArraysToml(sigRYs)))...)
	b = append(b, []byte(fmt.Sprintf("sig_s = [%s]\n", byteArraysToml(sigSs)))...)

	storage := make([]byte, 16)
	copy(storage, tsContent)
	if len(tsContent) > 16 {
		fmt.Fprintln(os.Stderr, "timestamp content exceeds BoundedVec<u8,16>'s max")
		os.Exit(1)
	}
	// main.nr's BoundaryBlockWitness.timestamp_content is [BoundedVec<u8,16>; 7], one
	// per validator (added by the later boundary-signature-binding change; this
	// generator predates it and needs the same [[array-of-tables]] shape [[headers]]
	// already uses below). All 7 share this one computed tsContent, matching
	// main.nr's own test_boundary_fixture()'s documented "same fixed message"
	// simplification -- signBytes above is one shared vote, not per-validator.
	for i := 0; i < numValidators; i++ {
		b = append(b, []byte(fmt.Sprintf("\n[[%s.timestamp_content]]\n", tableName))...)
		b = append(b, []byte(fmt.Sprintf("storage = [%s]\n", byteArrayToml(storage)))...)
		b = append(b, []byte(fmt.Sprintf("len = \"%d\"\n", len(tsContent)))...)
	}

	return string(b)
}

// decompressEd25519 recovers the affine (x, y) little-endian coordinates of
// a compressed ed25519 point -- identical to reanchor_witness_cli.go's version.
func decompressEd25519(compressed []byte) (xLE, yLE [32]byte, err error) {
	p, err := edwards25519.NewIdentityPoint().SetBytes(compressed)
	if err != nil {
		return xLE, yLE, err
	}
	X, Y, Z, _ := p.ExtendedCoordinates()
	zInv := new(field.Element).Invert(Z)
	x := new(field.Element).Multiply(X, zInv)
	y := new(field.Element).Multiply(Y, zInv)
	copy(xLE[:], x.Bytes())
	copy(yLE[:], y.Bytes())
	return xLE, yLE, nil
}

// extractTimestampContent walks a real types.VoteSignBytes(chainID, vote)
// output tag-by-tag to pull out the Timestamp submessage's content bytes --
// identical to cmd/engramd/reanchor_witness_cli.go's version.
func extractTimestampContent(signBytes []byte, chainID string) ([]byte, error) {
	pos := 0
	_, n, err := readVarint(signBytes, pos)
	if err != nil {
		return nil, fmt.Errorf("outer length prefix: %w", err)
	}
	pos = n
	if pos+2 > len(signBytes) || signBytes[pos] != 0x08 {
		return nil, fmt.Errorf("expected field1(type) tag 0x08 at offset %d", pos)
	}
	pos += 2
	if pos+9 > len(signBytes) || signBytes[pos] != 0x11 {
		return nil, fmt.Errorf("expected field2(height) tag 0x11 at offset %d", pos)
	}
	pos += 9
	if pos >= len(signBytes) || signBytes[pos] != 0x22 {
		return nil, fmt.Errorf("expected field4(block_id) tag 0x22 at offset %d", pos)
	}
	pos++
	blockIDLen, n, err := readVarint(signBytes, pos)
	if err != nil {
		return nil, fmt.Errorf("block_id length: %w", err)
	}
	pos = n + int(blockIDLen)
	if pos >= len(signBytes) || signBytes[pos] != 0x2a {
		return nil, fmt.Errorf("expected field5(timestamp) tag 0x2a at offset %d", pos)
	}
	pos++
	tsLen, n, err := readVarint(signBytes, pos)
	if err != nil {
		return nil, fmt.Errorf("timestamp length: %w", err)
	}
	pos = n
	if pos+int(tsLen) > len(signBytes) {
		return nil, fmt.Errorf("timestamp content length %d overruns sign-bytes", tsLen)
	}
	tsContent := signBytes[pos : pos+int(tsLen)]
	pos += int(tsLen)
	if pos >= len(signBytes) || signBytes[pos] != 0x32 {
		return nil, fmt.Errorf("expected field6(chain_id) tag 0x32 at offset %d", pos)
	}
	pos++
	cidLen, n, err := readVarint(signBytes, pos)
	if err != nil {
		return nil, fmt.Errorf("chain_id length: %w", err)
	}
	pos = n
	if pos+int(cidLen) > len(signBytes) {
		return nil, fmt.Errorf("chain_id length %d overruns sign-bytes", cidLen)
	}
	gotChainID := string(signBytes[pos : pos+int(cidLen)])
	if gotChainID != chainID {
		return nil, fmt.Errorf("chain_id mismatch: got %q, want %q", gotChainID, chainID)
	}
	return tsContent, nil
}

func readVarint(b []byte, pos int) (uint64, int, error) {
	var v uint64
	for shift := 0; ; shift += 7 {
		if pos >= len(b) {
			return 0, 0, fmt.Errorf("varint runs past end of buffer")
		}
		if shift >= 64 {
			return 0, 0, fmt.Errorf("varint too long")
		}
		c := b[pos]
		pos++
		v |= uint64(c&0x7f) << shift
		if c&0x80 == 0 {
			break
		}
	}
	return v, pos, nil
}

func fieldDecimal(reduced []byte) string {
	return new(big.Int).SetBytes(reduced).String()
}

// byteArrayToml renders a byte slice as a Noir/TOML `[u8; N]` array literal
// (quoted per-byte hex strings) -- nargo rejects a single hex string for a
// whole array-typed field; only Field-typed params accept that shorthand.
func byteArrayToml(b []byte) string {
	parts := make([]string, len(b))
	for i, v := range b {
		parts[i] = fmt.Sprintf("%q", fmt.Sprintf("0x%02x", v))
	}
	return strings_join(parts)
}

func byteArraysToml(bs [][]byte) string {
	parts := make([]string, len(bs))
	for i, b := range bs {
		parts[i] = "[" + byteArrayToml(b) + "]"
	}
	return strings_join(parts)
}

func boolArrayToml(bs []bool) string {
	parts := make([]string, len(bs))
	for i, v := range bs {
		parts[i] = fmt.Sprintf("%v", v)
	}
	return strings_join(parts)
}

func strings_join(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}

func strings_join_quoted(parts []string) string {
	q := make([]string, len(parts))
	for i, p := range parts {
		q[i] = fmt.Sprintf("%q", p)
	}
	return strings_join(q)
}
