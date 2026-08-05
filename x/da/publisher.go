package da

import (
	"context"
	"encoding/binary"
	"sync"
)

// heightMarkerTag is the payload prefix identifying this repo's simplified
// DA blob content -- mirrors x/vigilante/anchor.go's AnchorTag (tag + height)
// as a documented stand-in for publishing the block's actual transaction
// data. RefreshMetrics (x/sovereignty/sensors_refresh.go), where MaybePublish
// is called, only has access to sdk.Context, not the block's req.Txs -- real
// block-data publication needs wiring at PrepareProposal itself, where Txs
// are available. Available's DAS round-trip (blob.GetAll against a real
// celestia-bridge) already exercises the real Celestia retrieval path; only
// the blob's CONTENT is a placeholder here, not the availability mechanism.
var heightMarkerTag = []byte("ENGRAMDA")

// HeightMarker builds the placeholder blob payload for engramHeight.
func HeightMarker(engramHeight uint64) []byte {
	payload := make([]byte, 0, len(heightMarkerTag)+8)
	payload = append(payload, heightMarkerTag...)
	payload = binary.BigEndian.AppendUint64(payload, engramHeight)
	return payload
}

// Publisher is this app's DA-availability tracker -- the concrete mechanism
// giving h_engram_verified somewhere to actually come from, mirroring
// x/vigilante/anchor.go's AnchorTracker for h_btc_anchored. Per the user's
// directive to keep every Go development decision closely aligned with
// spec/core, this follows DANormalUpdate / DAFailure exactly
// (spec/core/EngramFSM.tla:196-212):
//
//	DANormalUpdate: h_engram_verified' = h_engram_current'   (exact equality)
//	DAFailure:      h_engram_verified' = h_engram_verified   (frozen)
//
// Unlike AnchorTracker, there is deliberately NO confirmation-depth
// (K-deep-style) waiting period here: Celestia's DAS is binary per the spec
// (da_gap == h_engram_current - h_engram_verified, no depth term anywhere
// in DANormalUpdate), so once a submitted block's blob is independently
// confirmed retrievable via Available, h_engram_verified is set EQUAL to
// that block's height immediately -- not just "advanced past" it.
//
// Each validator runs its own Publisher against its own celestia-node
// connection, matching "sensors propose, consensus decides" (CLAUDE.md):
// VerifyAvailable never trusts a peer's claimed da_receipt, it independently
// re-derives availability from this validator's own view of Celestia.
type Publisher struct {
	client    *RPCClient
	namespace Namespace

	mu                  sync.Mutex
	submitting          bool
	pendingEngramHeight uint64
	pendingCelestiaHt   uint64
	verifiedHeight      uint64
	hasVerified         bool
	lastSubmitFailed    bool
}

// NewPublisher builds a tracker submitting Engram block data as blobs under
// namespace via client.
func NewPublisher(client *RPCClient, namespace Namespace) *Publisher {
	return &Publisher{client: client, namespace: namespace}
}

// MaybePublish checks our previous submission's availability and, once it
// is confirmed retrievable, records engramHeight as the new VerifiedHeight
// (per DANormalUpdate) and starts submitting blockData as a new blob.
//
// Unlike AnchorTracker.MaybeSubmit (whose bitcoind SubmitOpReturn call
// returns as soon as the tx is broadcast, not confirmed -- fast), a real
// celestia-node's blob.Submit blocks until the blob is actually INCLUDED in
// a Celestia block, i.e. up to one full Celestia block time (~12s by
// default). Calling that synchronously from here would block whichever
// ABCI hook called RefreshMetrics -- PrepareProposal or ProcessProposal --
// for longer than a consensus round's own timeouts (found by actually
// running a live node against a live celestia-bridge: every round timed
// out and round-skipped via M0b's f+1-timeout quorum before the leader's
// own proposal was even ready, forever, since submitting blocked ~9-12s
// against 3-4.5s round timeouts). So Submit runs in a background goroutine
// here -- MaybePublish itself always returns near-instantly, and the
// submission's result is picked up on a LATER call once it completes.
func (p *Publisher) MaybePublish(ctx context.Context, engramHeight uint64, blockData []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.pendingEngramHeight != 0 {
		available, err := p.client.Available(ctx, p.pendingCelestiaHt, p.namespace)
		if err != nil {
			// DAFailure: is_das_failed'/is_attestation_failed' may become
			// TRUE while h_engram_current still advances -- record the
			// failure but leave verifiedHeight frozen (DAFailure's
			// h_engram_verified' = h_engram_verified), and keep the pending
			// submission in place to retry against on the next call.
			p.lastSubmitFailed = true
			return err
		}
		if !available {
			return nil // not yet retrievable, keep waiting -- gap grows (DAFailure-shaped, but not a hard failure)
		}
		p.verifiedHeight = p.pendingEngramHeight
		p.hasVerified = true
		p.lastSubmitFailed = false
		p.pendingEngramHeight = 0 // resolved -- free to submit the next one below
	}

	if p.submitting {
		return nil // a background Submit is already in flight, don't start another
	}
	p.submitting = true
	go func() {
		// context.Background(), not ctx: ctx is scoped to this single ABCI
		// call and may be cancelled/reused well before a real Celestia block
		// is produced -- this submission must outlive the call that started it.
		celestiaHeight, err := p.client.Submit(context.Background(), p.namespace, blockData)

		p.mu.Lock()
		defer p.mu.Unlock()
		p.submitting = false
		if err != nil {
			p.lastSubmitFailed = true
			return
		}
		p.pendingEngramHeight = engramHeight
		p.pendingCelestiaHt = celestiaHeight
	}()
	return nil
}

// VerifiedHeight returns h_engram_verified as this validator's own Publisher
// has independently confirmed it -- a pure local read, MaybePublish is what
// does the RPC work to keep it up to date.
func (p *Publisher) VerifiedHeight() (height uint64, ok bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.verifiedHeight, p.hasVerified
}

// Failed reports whether the most recent submission/availability check
// errored -- the concrete counterpart of is_das_failed \/ is_attestation_failed
// (spec/core/EngramFSM.tla:206-207) for this validator's own Publisher.
func (p *Publisher) Failed() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.lastSubmitFailed
}
