package anchor

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"sync"
)

// AnchorTag is the 4-byte OP_RETURN marker this fork uses to identify its
// own checkpoint submissions on Bitcoin -- our minimal stand-in for
// Babylon's "BBNT" tag (x/btccheckpoint's checkpoint_tag parameter).
var AnchorTag = []byte("ENGR")

// RootSize is the length of the state root a checkpoint commits: the
// CometBFT block hash (SHA-256) of the checkpointed Engram height.
const RootSize = 32

// payloadSize is len(AnchorTag) + height(8) + root(32) = 44 bytes, under
// bitcoind's 80-byte standard OP_RETURN limit.
const payloadSize = 4 + 8 + RootSize

// EncodePayload builds the checkpoint OP_RETURN payload
// AnchorTag || height(8, big-endian) || root(32). Binding root to the
// payload is what lets a Bitcoin observer detect two conflicting histories:
// only one can match the timestamped root. The spec abstracts the checkpoint
// as ExpectedBlockHash(height) (spec/core/EngramTendermint.tla:266) and does
// not model the payload, so this is an implementation-level strengthening.
func EncodePayload(height uint64, root []byte) ([]byte, error) {
	if len(root) != RootSize {
		return nil, fmt.Errorf("anchor: root must be %d bytes, got %d", RootSize, len(root))
	}
	payload := make([]byte, 0, payloadSize)
	payload = append(payload, AnchorTag...)
	payload = binary.BigEndian.AppendUint64(payload, height)
	payload = append(payload, root...)
	return payload, nil
}

// AnchorTracker is this fork's minimal stand-in for Babylon's Vigilante
// Submitter+Reporter: periodically submits a checkpoint OP_RETURN to Bitcoin
// and tracks its confirmation depth against K_DEEP_FINALITY. No BLS-aggregated
// multi-validator checkpoint (no staking module to source one from); content
// is AnchorTag + the checkpointed Engram height + that height's block hash
// (see EncodePayload).
//
// Each validator runs its own tracker against its own bitcoind. VerifyAnchor
// never trusts a peer's claimed anchor -- every validator re-derives it from
// its own view of Bitcoin ("sensors propose, consensus decides").
type AnchorTracker struct {
	client        *RPCClient
	kDeepFinality uint64

	// submissionPausedFile, when set, makes MaybeSubmit skip NEW checkpoint
	// broadcasts (an already-pending one still confirms normally), checked
	// fresh every call. Stands in for BTC congestion for E2's S2: only the
	// submission path falls behind, growing btc_gap.
	submissionPausedFile string

	// rootSource returns the CometBFT block hash of a committed Engram
	// height, read from the node's own block store outside consensus. The
	// root only goes to Bitcoin, never into committed state, so a node's
	// local read cannot diverge AppHash.
	rootSource RootSource

	mu                  sync.Mutex
	pendingTxid         string
	lastConfirmedHeight uint64
	hasConfirmed        bool
}

// NewAnchorTracker builds a tracker against client, requiring kDeepFinality
// confirmations before a submission counts as anchored (K_DEEP_FINALITY,
// spec/core/EngramConsensus.tla's IsKDeep).
func NewAnchorTracker(client *RPCClient, kDeepFinality uint64) *AnchorTracker {
	return &AnchorTracker{client: client, kDeepFinality: kDeepFinality}
}

// RootSource returns the block hash of the committed Engram height.
type RootSource func(ctx context.Context, height uint64) ([]byte, error)

// SetRootSource wires where MaybeSubmit reads the checkpoint root from.
// Without one, MaybeSubmit never broadcasts.
func (a *AnchorTracker) SetRootSource(src RootSource) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.rootSource = src
}

// SetSubmissionPausedFile wires the path MaybeSubmit checks to skip new
// checkpoint submissions (see the field's doc). Empty disables the check
// (default, every production validator).
func (a *AnchorTracker) SetSubmissionPausedFile(path string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.submissionPausedFile = path
}

// submissionPaused reads a.submissionPausedFile without locking -- only
// called from MaybeSubmit (which holds a.mu) after SetSubmissionPausedFile's
// single startup call, so no race in practice.
func (a *AnchorTracker) submissionPaused() bool {
	if a.submissionPausedFile == "" {
		return false
	}
	_, err := os.Stat(a.submissionPausedFile)
	return err == nil
}

// MaybeSubmit confirms the previous submission's status, then broadcasts a
// new checkpoint for engramHeight, committing that height's block hash from
// the RootSource, once the previous one has reached kDeepFinality+1
// confirmations (recorded via ConfirmedAnchorHeight) or none is pending.
// Safe to call every block -- at most one new broadcast per call.
//
// RPC failures are logged and swallowed, not propagated: concurrent
// submitters race the same wallet UTXO and get rejected -- degrading through
// btc_gap instead of failing block production.
//
// Chocks at kDeepFinality+1, not kDeepFinality: bitcoind's `confirmations` is
// INCLUSIVE (a tx in the current tip already has 1), while IsKDeep's check
// (h_btc_current - btc_anchored >= k) is one block stricter -- matching
// bitcoind here would fail every validator's re-check by exactly one block.
func (a *AnchorTracker) MaybeSubmit(ctx context.Context, engramHeight uint64) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.pendingTxid != "" {
		confirmations, blockHeight, mined, err := a.client.TxConfirmation(ctx, a.pendingTxid)
		if err != nil {
			// RPC error degrades through btc_gap (h_btc_anchored stops
			// advancing) rather than failing the ABCI call outright.
			fmt.Println("engramd: anchor confirmation check failed this block, will retry next block:", err)
			return nil
		}
		if !mined {
			return nil // still in mempool, nothing more to do this block
		}
		if uint64(confirmations) < a.kDeepFinality+1 {
			return nil // mined but not deep enough yet, keep waiting
		}
		a.lastConfirmedHeight = blockHeight
		a.hasConfirmed = true
		a.pendingTxid = "" // resolved -- free to submit the next one below
	}

	if a.submissionPaused() {
		// Withhold only NEW submissions -- an already-broadcast tx (handled
		// above) still confirms normally, so h_btc_anchored freezes while
		// h_btc_current climbs, growing btc_gap.
		return nil
	}

	// Same degrade-not-fail policy as an RPC error: a missing or invalid root
	// skips this block's submission and grows btc_gap.
	if a.rootSource == nil {
		return nil
	}
	root, err := a.rootSource(ctx, engramHeight)
	if err != nil {
		fmt.Println("engramd: anchor root unavailable this block, will retry next block:", err)
		return nil
	}
	payload, err := EncodePayload(engramHeight, root)
	if err != nil {
		fmt.Println("engramd: anchor payload invalid this block, will retry next block:", err)
		return nil
	}

	txid, err := a.client.SubmitOpReturn(ctx, payload)
	if err != nil {
		fmt.Println("engramd: anchor submission failed this block, will retry next block:", err)
		return nil
	}
	a.pendingTxid = txid
	return nil
}

// ConfirmedAnchorHeight returns the Bitcoin height of our most recent
// submission that reached kDeepFinality confirmations, and whether we have
// one at all yet. A pure local read -- MaybeSubmit is what does the RPC
// work to keep this up to date.
func (a *AnchorTracker) ConfirmedAnchorHeight() (height uint64, ok bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.lastConfirmedHeight, a.hasConfirmed
}

// IsAnchorDeepEnough reports whether height has reached kDeepFinality
// confirmations yet. Split out of VerifyAnchor so callers can distinguish
// "not deep enough yet" (normal, transient) from a real tag-verification
// failure -- see VerifyAnchor's doc.
func (a *AnchorTracker) IsAnchorDeepEnough(ctx context.Context, height uint64) (bool, error) {
	current, err := a.client.CurrentHeight(ctx)
	if err != nil {
		return false, err
	}
	return current >= height && current-height >= a.kDeepFinality, nil
}

// VerifyAnchor independently confirms that the Bitcoin block at btcHeight
// carries a real, kDeepFinality-confirmed checkpoint whose root equals THIS
// node's own block hash of the checkpointed Engram height -- used to check a
// PEER's (e.g. the leader's) claimed anchor without trusting them, unlike
// ConfirmedAnchorHeight which only reports on our OWN submissions.
//
// A block may hold several checkpoints (one per submitting validator, each
// for its own height), plus arbitrary OP_RETURNs anyone can post. The anchor
// verifies if any one checkpoint matches, so a bogus ENGR-tagged output cannot
// fail a genuine anchor. A root that differs from the local block hash counts
// as a failed re-verification, the same as a missing tag. A RootSource error
// is returned, not swallowed: an unverifiable anchor is treated as failed.
func (a *AnchorTracker) VerifyAnchor(ctx context.Context, btcHeight uint64) (bool, error) {
	deepEnough, err := a.IsAnchorDeepEnough(ctx, btcHeight)
	if err != nil {
		return false, err
	}
	if !deepEnough {
		return false, nil
	}
	a.mu.Lock()
	rootSource := a.rootSource
	a.mu.Unlock()
	if rootSource == nil {
		return false, fmt.Errorf("anchor: no RootSource wired, cannot verify checkpoint root")
	}

	checkpoints, err := a.client.BlockCheckpoints(ctx, btcHeight)
	if err != nil {
		return false, err
	}
	for _, cp := range checkpoints {
		localRoot, err := rootSource(ctx, cp.Height)
		if err != nil {
			return false, err
		}
		if bytes.Equal(localRoot, cp.Root) {
			return true, nil
		}
	}
	return false, nil
}

// Checkpoint is one decoded checkpoint OP_RETURN.
type Checkpoint struct {
	Height uint64
	Root   []byte
}

// DecodePayload parses AnchorTag || height(8) || root(32); ok is false for
// any other OP_RETURN data, including the pre-root 12-byte payload.
func DecodePayload(data []byte) (cp Checkpoint, ok bool) {
	if len(data) != payloadSize || !bytes.Equal(data[:len(AnchorTag)], AnchorTag) {
		return Checkpoint{}, false
	}
	return Checkpoint{
		Height: binary.BigEndian.Uint64(data[4:12]),
		Root:   append([]byte(nil), data[12:]...),
	}, true
}

// BlockCheckpoints returns every well-formed checkpoint in the OP_RETURN
// outputs of the Bitcoin block at height.
func (c *RPCClient) BlockCheckpoints(ctx context.Context, height uint64) ([]Checkpoint, error) {
	hash, err := c.BlockHashAt(ctx, height)
	if err != nil {
		return nil, err
	}

	var block struct {
		Tx []struct {
			Vout []struct {
				ScriptPubKey struct {
					Hex string `json:"hex"`
				} `json:"scriptPubKey"`
			} `json:"vout"`
		} `json:"tx"`
	}
	if err := c.call(ctx, "getblock", []any{hash, 2}, &block); err != nil {
		return nil, fmt.Errorf("getblock: %w", err)
	}

	// An OP_RETURN script is 0x6a + single-byte push length (valid for our
	// 44-byte payload, < 76) + data.
	prefix := fmt.Sprintf("6a%02x", payloadSize)
	var out []Checkpoint
	for _, tx := range block.Tx {
		for _, vout := range tx.Vout {
			script := vout.ScriptPubKey.Hex
			if !strings.HasPrefix(script, prefix) {
				continue
			}
			data, err := hex.DecodeString(script[len(prefix):])
			if err != nil {
				continue
			}
			if cp, ok := DecodePayload(data); ok {
				out = append(out, cp)
			}
		}
	}
	return out, nil
}
