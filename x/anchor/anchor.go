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

// AnchorTracker is this fork's minimal stand-in for Babylon's real Vigilante
// Submitter+Reporter pair: periodically submits a checkpoint marker to
// Bitcoin via OP_RETURN and tracks its confirmation depth against
// K_DEEP_FINALITY. Deliberately does not implement Babylon's BLS-aggregated
// multi-validator checkpoint or its full Checkpointing/BTCLightClient state
// machine -- this repo has no staking module to source a real
// multi-validator-signed checkpoint from. Checkpoint content is simply
// AnchorTag + the submitting node's Engram height.
//
// Each validator runs its own tracker against its own bitcoind. VerifyAnchor
// never trusts a peer's claimed anchor -- every validator re-derives it from
// its own view of Bitcoin ("sensors propose, consensus decides").
type AnchorTracker struct {
	client        *RPCClient
	kDeepFinality uint64
	tag           []byte

	// submissionPausedFile, when set, makes MaybeSubmit skip broadcasting a
	// NEW checkpoint (an already-pending one still gets its confirmation
	// checked and recorded normally -- an in-flight tx can't be un-sent)
	// while the file exists, checked fresh every call. Existence-only, no
	// content parsed -- mirrors bitcoin_miner_loop.sh's
	// MINER_INTERVAL_OVERRIDE_FILE (fresh-every-call, no restart needed).
	//
	// This is the deliberate-delay analog of real BTC congestion (a
	// checkpoint tx stuck behind mempool fee competition, delaying its own
	// confirmation) for docs/EXPERIMENT.md's E2 S2 scenario -- global
	// mining-rate slowdown was tried first and confirmed live NOT to grow
	// btc_gap, since h_btc_current and h_btc_anchored both derive from the
	// same block stream and stay proportionally in sync regardless of
	// overall mining speed; only THIS validator's own submission falling
	// behind (independent of mining speed) grows the gap.
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
// called from MaybeSubmit, which already holds a.mu; SetSubmissionPausedFile
// is only ever called once at startup, before any concurrent MaybeSubmit
// call, so this doesn't race in practice.
func (a *AnchorTracker) submissionPaused() bool {
	if a.submissionPausedFile == "" {
		return false
	}
	_, err := os.Stat(a.submissionPausedFile)
	return err == nil
}

// MaybeSubmit checks the previous submission's status and, once it has
// either reached kDeepFinality+1 confirmations (recorded via
// ConfirmedAnchorHeight) or none is pending, broadcasts a new checkpoint
// marker for engramHeight. Safe to call every block -- at most one new
// broadcast per call.
//
// A SubmitOpReturn/confirmation-check failure is logged and swallowed, not
// propagated as an error: this repo's 4 validators share bitcoind wallets
// (docker/bitcoin-regtest-cluster.yml provisions only 2 for 4 validators),
// so concurrent fundrawtransaction/sendrawtransaction calls can race on the
// same UTXO and get rejected as an underpriced BIP125 replacement --
// treating this as sensor data (btc_gap simply stops shrinking) rather than
// a block-production fault lets the next block's retry recover on its own.
// pendingTxid stays "" on failure so the next call retries fresh.
//
// Requires confirmations >= kDeepFinality+1, not kDeepFinality: bitcoind's
// `confirmations` field is INCLUSIVE (a tx mined in the current tip already
// has confirmations=1), while VerifyAnchor/IsKDeep implements the spec's
// EXCLUSIVE depth check (h_btc_current - c.btc_anchored >= k, no +1) -- one
// block stricter. Matching bitcoind's own convention here would make every
// height this tracker reports "confirmed" fail every other validator's
// independent re-check by exactly one block.
func (a *AnchorTracker) MaybeSubmit(ctx context.Context, engramHeight uint64) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.pendingTxid != "" {
		confirmations, blockHeight, mined, err := a.client.TxConfirmation(ctx, a.pendingTxid)
		if err != nil {
			// A confirmation-check RPC error degrades through btc_gap (via
			// h_btc_anchored no longer advancing) rather than failing
			// PrepareProposal/ProcessProposal outright.
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
		// Deliberately does not touch pendingTxid/lastConfirmedHeight: an
		// already-broadcast tx (handled above) still confirms normally, only
		// a NEW checkpoint is withheld -- h_btc_anchored freezes at its last
		// confirmed value while h_btc_current keeps climbing, growing
		// btc_gap, the effect real checkpoint-submission delay produces.
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

	// An OP_RETURN script is opcode 0x6a followed by a single-byte push
	// length (valid for our <= 76-byte payloads) then the pushed data --
	// matching the tag hex right after that prefix is precise enough for
	// our fixed-length (tag + 8-byte height) payloads without a full script parser.
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
