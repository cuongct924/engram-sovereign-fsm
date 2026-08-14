package anchor

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"strings"
	"sync"
)

// AnchorTag is the 4-byte OP_RETURN marker this fork uses to identify its
// own checkpoint submissions on Bitcoin -- our minimal stand-in for
// Babylon's "BBNT" tag (x/btccheckpoint's checkpoint_tag parameter).
var AnchorTag = []byte("ENGR")

// AnchorTracker is this fork's minimal stand-in for Babylon's Vigilante
// Submitter+Reporter: periodically submits a checkpoint OP_RETURN to Bitcoin
// and tracks its confirmation depth against K_DEEP_FINALITY. No BLS-aggregated
// multi-validator checkpoint (no staking module to source one from); content
// is AnchorTag + the submitting node's Engram height.
//
// Each validator runs its own tracker against its own bitcoind. VerifyAnchor
// never trusts a peer's claimed anchor -- every validator re-derives it from
// its own view of Bitcoin ("sensors propose, consensus decides").
type AnchorTracker struct {
	client        *RPCClient
	kDeepFinality uint64
	tag           []byte

	// submissionPausedFile, when set, makes MaybeSubmit skip NEW checkpoint
	// broadcasts (an already-pending one still confirms normally), checked
	// fresh every call. Stands in for BTC congestion for E2's S2: only the
	// submission path falls behind, growing btc_gap.
	submissionPausedFile string

	mu                  sync.Mutex
	pendingTxid         string
	lastConfirmedHeight uint64
	hasConfirmed        bool
}

// NewAnchorTracker builds a tracker against client, requiring kDeepFinality
// confirmations before a submission counts as anchored (K_DEEP_FINALITY,
// spec/core/EngramConsensus.tla's IsKDeep).
func NewAnchorTracker(client *RPCClient, kDeepFinality uint64) *AnchorTracker {
	return &AnchorTracker{client: client, kDeepFinality: kDeepFinality, tag: AnchorTag}
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
// new checkpoint marker for engramHeight once it has reached kDeepFinality+1
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

	payload := make([]byte, 0, len(a.tag)+8)
	payload = append(payload, a.tag...)
	payload = binary.BigEndian.AppendUint64(payload, engramHeight)

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

// VerifyAnchor independently confirms that height carries a real,
// kDeepFinality-confirmed Bitcoin transaction tagged with our AnchorTag --
// used to check a PEER's (e.g. the leader's) claimed anchor height without
// trusting them, unlike ConfirmedAnchorHeight which only reports on our OWN
// submissions.
func (a *AnchorTracker) VerifyAnchor(ctx context.Context, height uint64) (bool, error) {
	current, err := a.client.CurrentHeight(ctx)
	if err != nil {
		return false, err
	}
	if current < height || current-height < a.kDeepFinality {
		return false, nil
	}
	return a.client.BlockContainsTag(ctx, height, a.tag)
}

// BlockContainsTag scans every transaction in the block at height for an
// OP_RETURN output whose pushed data starts with tag.
func (c *RPCClient) BlockContainsTag(ctx context.Context, height uint64, tag []byte) (bool, error) {
	hash, err := c.BlockHashAt(ctx, height)
	if err != nil {
		return false, err
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
		return false, fmt.Errorf("getblock: %w", err)
	}

	// An OP_RETURN script is 0x6a + single-byte push length (valid for our
	// <= 76-byte payloads) + data -- matching the tag hex after that prefix
	// is precise enough for our fixed-length payloads.
	tagHex := fmt.Sprintf("%x", tag)
	for _, tx := range block.Tx {
		for _, vout := range tx.Vout {
			script := vout.ScriptPubKey.Hex
			if strings.HasPrefix(script, "6a") && len(script) >= 4 &&
				strings.Contains(script[4:], tagHex) {
				return true, nil
			}
		}
	}
	return false, nil
}
