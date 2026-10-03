package da

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// availableCheckTimeout bounds the synchronous Available()/ProbeHealthy in
// the ABCI hot path: must return within a consensus round even if
// celestia-bridge is unreachable (degrade via is_das_failed, never stall).
const availableCheckTimeout = 800 * time.Millisecond

// maxPendingBlocks bounds how long a pending submission gets Available()
// checks before it's abandoned and resubmitted: a pending that never resolves
// retrievable permanently gates off new Submits (pendingEngramHeight != 0).
const maxPendingBlocks = 60

// maxBatchBytes caps one backfill blob below Celestia's blob size limit,
// leaving headroom for the per-height framing.
const maxBatchBytes = 1_500_000

// heightMarkerTag prefixes the head-liveness marker blob (tag + height),
// mirroring x/anchor's AnchorTag. It is submitted only when there is no new
// block data to publish, so DA liveness is still probed every cycle.
var heightMarkerTag = []byte("ENGRAMDA")

// BlockSource returns the committed block data of height (its transactions),
// read from this node's own CometBFT RPC outside consensus. The data only goes
// to the DA layer, never into committed state, so a local read cannot diverge
// AppHash.
type BlockSource func(ctx context.Context, height uint64) ([]byte, error)

// HeightMarker builds the head-liveness marker blob payload for engramHeight.
func HeightMarker(engramHeight uint64) []byte {
	payload := make([]byte, 0, len(heightMarkerTag)+8)
	payload = append(payload, heightMarkerTag...)
	payload = binary.BigEndian.AppendUint64(payload, engramHeight)
	return payload
}

// Publisher gives h_engram_verified a concrete source (cf. AnchorTracker for
// h_btc_anchored), following DANormalUpdate/DAFailure
// (spec/core/EngramFSM.tla:196-212) with no confirmation-depth wait -- DAS is
// binary, so retrievable means h_engram_verified' = h_engram_current'.
//
// Each validator runs its own Publisher: VerifyAvailable never trusts a
// peer's claimed da_receipt ("sensors propose, consensus decides").
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

	// blockSource feeds backfill batches; nil keeps the marker-only behavior.
	blockSource BlockSource
	// contiguousHeight is the highest h such that every height <= h is
	// published and confirmed retrievable -- the queue's drain point.
	// verifiedHeight above is head liveness (the height at which a submission
	// last confirmed): during a long SOVEREIGN backfill DA is healthy, so
	// verifiedHeight tracks the head while contiguousHeight lags behind.
	contiguousHeight uint64
	pendingBatchEnd  uint64 // highest height inside the pending blob
	// pendingPayload is the data the pending submission is MEANT to carry; confirmation compares what the
	// DA layer serves against it, not merely that some blob exists.
	pendingPayload []byte

	// submitTransform, set only by the corrupt_publish test hook, alters the bytes actually submitted while
	// pendingPayload keeps the true data, so the content check has something to catch.
	submitTransform func([]byte) []byte

	// progressPath persists the confirmed progress (node-local, never consensus state), so a restart resumes
	// instead of republishing the whole history; chainID keeps a file from another chain from being reused.
	progressPath string
	chainID      string
	restored     *progressFile // loaded from disk, re-verified against the DA layer on the first MaybePublish
}

// progressFile is the persisted publisher progress. BatchSHA256 identifies the last confirmed batch at
// CelestiaHeight; restoring re-fetches that batch, so a file ahead of what the DA layer really holds is dropped.
type progressFile struct {
	ChainID          string `json:"chain_id"`
	ContiguousHeight uint64 `json:"contiguous_height"`
	CelestiaHeight   uint64 `json:"celestia_height"`
	BatchSHA256      string `json:"batch_sha256"`
}

// SetProgressFile enables persistence of the confirmed contiguous height under path, scoped to chainID. A
// missing, unreadable or foreign file is ignored: the publisher then starts from 0 as before.
func (p *Publisher) SetProgressFile(path, chainID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.progressPath, p.chainID = path, chainID
	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var pf progressFile
	if json.Unmarshal(raw, &pf) != nil || pf.ChainID != chainID || pf.ContiguousHeight == 0 || pf.BatchSHA256 == "" {
		return
	}
	p.restored = &pf
}

// SetSubmitTransform makes the publisher submit transform(payload) while still expecting payload back
// (test hook for ENGRAM_BYZANTINE_BEHAVIOR=corrupt_publish).
func (p *Publisher) SetSubmitTransform(f func([]byte) []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.submitTransform = f
}

// persistLocked writes the progress atomically. Failure is not fatal: the publisher only loses the shortcut.
func (p *Publisher) persistLocked(celestiaHeight uint64, payload []byte) {
	if p.progressPath == "" {
		return
	}
	sum := sha256.Sum256(payload)
	raw, err := json.Marshal(progressFile{ChainID: p.chainID, ContiguousHeight: p.contiguousHeight,
		CelestiaHeight: celestiaHeight, BatchSHA256: hex.EncodeToString(sum[:])})
	if err != nil {
		return
	}
	tmp := p.progressPath + ".tmp"
	if os.MkdirAll(filepath.Dir(p.progressPath), 0o755) != nil || os.WriteFile(tmp, raw, 0o644) != nil {
		return
	}
	_ = os.Rename(tmp, p.progressPath)
}

// restoreLocked re-verifies the persisted last batch on the DA layer and, if it is really there, resumes from
// the persisted height. No answer from the DA layer leaves the file for a later attempt; a definite mismatch
// drops it.
func (p *Publisher) restoreLocked(ctx context.Context) {
	if p.restored == nil {
		return
	}
	fetchCtx, cancel := context.WithTimeout(ctx, availableCheckTimeout)
	defer cancel()
	blobs, err := p.client.Fetch(fetchCtx, p.restored.CelestiaHeight, p.namespace)
	if err != nil || len(blobs) == 0 {
		return
	}
	for _, b := range blobs {
		sum := sha256.Sum256(b)
		if hex.EncodeToString(sum[:]) == p.restored.BatchSHA256 {
			if p.contiguousHeight < p.restored.ContiguousHeight {
				p.contiguousHeight = p.restored.ContiguousHeight
			}
			break
		}
	}
	p.restored = nil
}

// NewPublisher builds a tracker submitting Engram block data as blobs under
// namespace via client.
func NewPublisher(client *RPCClient, namespace Namespace) *Publisher {
	return &Publisher{client: client, namespace: namespace}
}

// SetBlockSource enables publishing real block data: each submission then
// carries the next heights after ContiguousHeight, in order. Without one, only
// markers are submitted and ContiguousHeight never advances.
func (p *Publisher) SetBlockSource(src BlockSource) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.blockSource = src
}

// MaybePublish confirms the previous submission, records engramHeight as
// VerifiedHeight (DANormalUpdate) when retrievable, then starts submitting the
// next batch of committed block data (heights ContiguousHeight+1 ..
// engramHeight-1), or markerData when there is none. Submit runs in a
// background goroutine: blob.Submit blocks ~12s for Celestia inclusion, past a
// consensus round's timeout, and assembling a long backfill batch is one local
// RPC per height.
func (p *Publisher) MaybePublish(ctx context.Context, engramHeight uint64, markerData []byte) error {
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

	p.restoreLocked(ctx)

	if p.pendingEngramHeight != 0 {
		fetchCtx, cancel := context.WithTimeout(ctx, availableCheckTimeout)
		blobs, err := p.client.Fetch(fetchCtx, p.pendingCelestiaHt, p.namespace)
		cancel()
		if err != nil {
			// DAFailure: verifiedHeight stays frozen, retry the pending
			// submission later. Return nil, not err: an error here becomes a
			// hard ABCI failure that stalls block production, instead of the
			// graceful SUSPICIOUS degrade the FSM applies via the failure
			// flags.
			p.lastSubmitFailed = true
			return nil
		}
		if len(blobs) == 0 {
			return nil // not yet retrievable, keep waiting -- gap grows (DAFailure-shaped, but not a hard failure)
		}
		matched := false
		for _, b := range blobs {
			if bytes.Equal(b, p.pendingPayload) {
				matched = true
				break
			}
		}
		if !matched {
			// The DA layer serves other data under our namespace at that height but not ours: the
			// submission is not retrievable as published. Drop it and submit again.
			p.lastSubmitFailed = true
			p.pendingEngramHeight = 0
			p.pendingPayload = nil
			return nil
		}
		p.verifiedHeight = p.pendingEngramHeight
		p.hasVerified = true
		if p.pendingBatchEnd > p.contiguousHeight {
			p.contiguousHeight = p.pendingBatchEnd
			p.persistLocked(p.pendingCelestiaHt, p.pendingPayload)
		}
		p.lastSubmitFailed = false
		p.pendingEngramHeight = 0 // resolved -- free to submit the next one below
		p.pendingPayload = nil
	}

	if p.submitting {
		return nil // a background Submit is already in flight, don't start another
	}
	p.submitting = true
	startHeight := p.contiguousHeight
	src := p.blockSource
	go func() {
		// context.Background(), not ctx: ctx is scoped to this single ABCI
		// call and may be canceled/reused well before a real Celestia block
		// is produced -- this submission must outlive the call that started it.
		payload, batchEnd := markerData, startHeight
		if src != nil {
			if batch, end := buildBatch(context.Background(), src, startHeight, engramHeight); end > startHeight {
				payload, batchEnd = batch, end
			}
		}
		submitted := payload
		p.mu.Lock()
		transform := p.submitTransform
		p.mu.Unlock()
		if transform != nil {
			submitted = transform(append([]byte(nil), payload...))
		}
		celestiaHeight, err := p.client.Submit(context.Background(), p.namespace, submitted)

		p.mu.Lock()
		defer p.mu.Unlock()
		p.submitting = false
		if err != nil {
			// The batch stays queued: the next attempt rebuilds it from the
			// unchanged contiguousHeight.
			p.lastSubmitFailed = true
			return
		}
		p.pendingEngramHeight = engramHeight
		p.pendingCelestiaHt = celestiaHeight
		p.pendingSinceHeight = engramHeight
		p.pendingBatchEnd = batchEnd
		p.pendingPayload = payload
	}()
	return nil
}

// buildBatch frames the committed heights startHeight+1 .. headHeight-1 as
// repeated [height(8) | len(4) | data], stopping at maxBatchBytes or at the
// first height src cannot return. It never skips a failed height: the batch
// end only moves over contiguous heights. end == startHeight means no data.
func buildBatch(ctx context.Context, src BlockSource, startHeight, headHeight uint64) (batch []byte, end uint64) {
	end = startHeight
	for h := startHeight + 1; h < headHeight; h++ {
		data, err := src(ctx, h)
		if err != nil {
			break
		}
		if len(batch) > 0 && len(batch)+12+len(data) > maxBatchBytes {
			break
		}
		batch = binary.BigEndian.AppendUint64(batch, h)
		batch = binary.BigEndian.AppendUint32(batch, uint32(len(data)))
		batch = append(batch, data...)
		end = h
	}
	return batch, end
}

// ContiguousHeight returns the highest height h such that the block data of
// every height <= h is published and confirmed retrievable (0 before the first
// batch confirms). Unlike VerifiedHeight, which tracks head liveness, it lags
// while a SOVEREIGN interval is being backfilled -- a pure local read.
func (p *Publisher) ContiguousHeight() uint64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.contiguousHeight
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

// ProbeHealthy is a fresh, stateless TCP check against celestia-bridge --
// unlike Failed(), it reads no mutex or pending bookkeeping, so its answer
// can never be stale. daGapMetric ORs it into Failed() to spot an outage
// within ~1 block on every validator, instead of waiting for the stale
// Failed() to flip in sync with ProcessProposal's Healthy cross-check
// (proposal.go #1b).
func (p *Publisher) ProbeHealthy(ctx context.Context) bool {
	probeCtx, cancel := context.WithTimeout(ctx, availableCheckTimeout)
	defer cancel()
	return p.client.Reachable(probeCtx)
}
