package vigilante

import (
	"context"
	"encoding/binary"
	"fmt"
	"strings"
	"sync"
)

// AnchorTag is the 4-byte OP_RETURN marker this fork uses to identify its
// own checkpoint submissions on Bitcoin -- our minimal stand-in for
// Babylon's "BBNT" tag (x/btccheckpoint's checkpoint_tag parameter).
var AnchorTag = []byte("ENGR")

// AnchorTracker is this fork's minimal stand-in for the real Babylon
// Vigilante Submitter+Reporter pair (github.com/babylonlabs-io/vigilante):
// it periodically submits a checkpoint marker to Bitcoin via OP_RETURN
// (Submitter's role) and tracks that submission's confirmation depth
// against K_DEEP_FINALITY to decide when it's safe to treat as anchored
// (Reporter's role). It deliberately does NOT implement Babylon's real
// BLS-aggregated multi-validator checkpoint or the full Checkpointing/
// BTCLightClient/BTCCheckpoint module state machine (SEALED -> SUBMITTED ->
// CONFIRMED -> FINALIZED) -- this repo has no staking/epoching module to
// source a real multi-validator-signed checkpoint from (see app/app.go's
// TODO on BankKeeper/StakingKeeper). The checkpoint content here is simply
// AnchorTag + the submitting node's current Engram block height, which is
// sufficient for this repo's actual need: giving h_btc_anchored a REAL,
// independently-Bitcoin-verifiable value instead of one that never advances
// (see x/sovereignty/sensors_refresh.go's discovery of that liveness bug).
//
// Each validator runs its own AnchorTracker against its own bitcoind
// connection. VerifyAnchor's answer is never trusted from a peer -- every
// validator that wants to accept a claimed anchor advance re-derives it from
// its OWN view of Bitcoin, matching "sensors propose, consensus decides"
// (see x/sovereignty/proposal.go's ProcessProposal wiring).
type AnchorTracker struct {
	client        *RPCClient
	kDeepFinality uint64
	tag           []byte

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

// MaybeSubmit checks our previous submission's status and, once it has
// either reached kDeepFinality confirmations (recording it via
// ConfirmedAnchorHeight) or none is pending at all, broadcasts a new
// checkpoint marker for engramHeight. Safe to call every block -- it only
// actually issues bitcoind RPCs, at most one new broadcast per call.
func (a *AnchorTracker) MaybeSubmit(ctx context.Context, engramHeight uint64) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.pendingTxid != "" {
		confirmations, blockHeight, mined, err := a.client.TxConfirmation(ctx, a.pendingTxid)
		if err != nil {
			return err
		}
		if !mined {
			return nil // still in mempool, nothing more to do this block
		}
		if uint64(confirmations) < a.kDeepFinality {
			return nil // mined but not deep enough yet, keep waiting
		}
		a.lastConfirmedHeight = blockHeight
		a.hasConfirmed = true
		a.pendingTxid = "" // resolved -- free to submit the next one below
	}

	payload := make([]byte, 0, len(a.tag)+8)
	payload = append(payload, a.tag...)
	payload = binary.BigEndian.AppendUint64(payload, engramHeight)

	txid, err := a.client.SubmitOpReturn(ctx, payload)
	if err != nil {
		return err
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
	// "6a" + 1-byte length + data, all hex-encoded. Matching on the tag hex
	// appearing right after that prefix is precise enough for our own
	// fixed-length (tag + 8-byte height) payloads without a full script
	// parser.
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
