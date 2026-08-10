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
// giving h_engram_verified somewhere to come from, mirroring
// x/vigilante/anchor.go's AnchorTracker for h_btc_anchored. Follows
// DANormalUpdate/DAFailure exactly (spec/core/EngramFSM.tla:196-212):
//
//	DANormalUpdate: h_engram_verified' = h_engram_current'   (exact equality)
//	DAFailure:      h_engram_verified' = h_engram_verified   (frozen)
//
// Unlike AnchorTracker, deliberately no confirmation-depth waiting period:
// Celestia's DAS is binary per spec (no depth term in DANormalUpdate), so
// once a submission is confirmed retrievable, h_engram_verified is set
// EQUAL to that height immediately.
//
// Each validator runs its own Publisher against its own celestia-node --
// VerifyAvailable never trusts a peer's claimed da_receipt ("sensors
// propose, consensus decides").
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

// MaybePublish checks the previous submission's availability and, once
// confirmed retrievable, records engramHeight as the new VerifiedHeight
// (DANormalUpdate) before starting to submit blockData as a new blob.
//
// Unlike AnchorTracker.MaybeSubmit (bitcoind returns as soon as a tx is
// broadcast), a real celestia-node's blob.Submit blocks until the blob is
// actually included in a Celestia block (~12s by default) -- calling that
// synchronously here would block PrepareProposal/ProcessProposal past a
// consensus round's own timeout. Submit runs in a background goroutine
// instead; MaybePublish always returns near-instantly, and the result is
// picked up on a later call.
func (p *Publisher) MaybePublish(ctx context.Context, engramHeight uint64, blockData []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.pendingEngramHeight != 0 {
		available, err := p.client.Available(ctx, p.pendingCelestiaHt, p.namespace)
		if err != nil {
			// DAFailure: is_das_failed'/is_attestation_failed' may become
			// TRUE while h_engram_current still advances -- record the
			// failure but leave verifiedHeight frozen (DAFailure's
			// h_engram_verified' = h_engram_verified), keeping the pending
			// submission to retry.
			//
			// Returning nil (not err) is load-bearing: propagating a DA
			// availability-check error up through RefreshMetrics is a hard
			// ABCI failure (block production stalls) rather than the
			// graceful "reject this proposal, degrade through SUSPICIOUS"
			// path this protocol depends on -- the failure belongs in
			// is_das_failed/is_attestation_failed, which the FSM already
			// has a well-defined response to.
			p.lastSubmitFailed = true
			return nil
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
		// call and may be canceled/reused well before a real Celestia block
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
