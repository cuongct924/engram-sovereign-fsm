package da

import (
	"context"
	"encoding/binary"
	"sync"
	"time"
)

// availableCheckTimeout bounds MaybePublish's synchronous Available() call
// and ProbeHealthy -- unlike Submit (backgrounded, below), these run inline
// in PrepareProposal/ProcessProposal's ABCI hot path, so they must return
// well within a consensus round's own timeout even if celestia-bridge is
// completely unreachable, degrading through is_das_failed rather than
// stalling block production.
//
// Bounded well under CometBFT's timeout_propose (3s default) -- a single
// RefreshMetrics call can run this alongside x/anchor's own sequential BTC
// RPC calls (same reasoning as rpcTimeout's doc there), and 3s here alone
// already ate most of that budget. Confirmed live: a combined BTC+DA outage
// produced "ProposalBlock is nil" round-skips for an entire 90s scenario
// window with the old 3s value.
const availableCheckTimeout = 800 * time.Millisecond

// maxPendingBlocks bounds how long MaybePublish waits on ONE pending
// submission's Available() check before giving up and starting a fresh
// Submit instead. Without this, a pending submission that never resolves
// retrievable (confirmed live: celestia-bridge reported blob.Submit
// succeeding at a given height, but blob.GetAll at that same height never
// returned it available, for 40+ minutes straight) permanently blocks
// VerifiedHeight from ever advancing -- since pendingEngramHeight != 0 gates
// off the Submit branch entirely, no new attempt is ever made. 60 blocks is
// several multiples of Celestia's own ~12s block time (at this testnet's
// ~1.3s/block cadence), generous for a genuine in-flight confirmation, but
// bounded well short of "stuck forever".
const maxPendingBlocks = 60

// heightMarkerTag is the payload prefix identifying this repo's simplified
// DA blob content -- mirrors x/anchor/anchor.go's AnchorTag (tag + height)
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
// x/anchor/anchor.go's AnchorTracker for h_btc_anchored. Follows
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
	pendingSinceHeight  uint64 // engramHeight when pendingEngramHeight was set, for maxPendingBlocks
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

	if p.pendingEngramHeight != 0 && engramHeight > p.pendingSinceHeight &&
		engramHeight-p.pendingSinceHeight > maxPendingBlocks {
		// Gave it maxPendingBlocks worth of Available() checks; it never
		// resolved retrievable. Abandon it and fall through to a fresh
		// Submit below rather than waiting forever.
		p.pendingEngramHeight = 0
		p.lastSubmitFailed = true
	}

	if p.pendingEngramHeight != 0 {
		availCtx, cancel := context.WithTimeout(ctx, availableCheckTimeout)
		available, err := p.client.Available(availCtx, p.pendingCelestiaHt, p.namespace)
		cancel()
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
		p.pendingSinceHeight = engramHeight
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

// ProbeHealthy does a fresh, bounded, stateless TCP reachability check
// against celestia-bridge -- unlike Failed(), it never reads p's mutex or
// its pending-submission bookkeeping, so it can't return a value that's
// stale relative to this exact call. Failed() alone is not enough to
// declare DA down: it only updates on the next MaybePublish call that
// actually runs its synchronous branch, which can lag by however long an
// in-flight background Submit (up to rpcTimeout) takes to resolve -- during
// that lag, one validator's Failed() may still read healthy while another's
// has already flipped, and ProcessProposal's Healthy cross-check
// (proposal.go's check #1b) rejects every proposal until they happen to
// agree. Called alongside Failed() in daGapMetric so an outage is detected
// within about one block on every honest validator, not after an
// unpredictable, potentially many-round race to converge.
func (p *Publisher) ProbeHealthy(ctx context.Context) bool {
	probeCtx, cancel := context.WithTimeout(ctx, availableCheckTimeout)
	defer cancel()
	return p.client.Reachable(probeCtx)
}
