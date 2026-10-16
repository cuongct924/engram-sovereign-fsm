package prover

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Config holds the scheduling parameters.
type Config struct {
	Provers                   int           // proofs in flight at most
	Stagger                   time.Duration // one new segment at most this often (about L / Provers)
	Poll                      time.Duration // how often the scheduler looks at the chain
	MinBatch                  int           // headers that must wait before a segment starts
	BoundaryRetries           int           // times a segment may end one header earlier when its last block has no usable commit
	ControlDir                string        // test control (empty: off), read again for every segment: see Control
	WitnessPublishMinInterval time.Duration // the audit trail goes to Celestia at most this often (one shared bridge account)
	MaxChecks                 int           // stop after this many polls (0: run until the context ends)
	Logf                      func(format string, args ...any)
	ProofLog                  func(line string) // receives one PROOFLOG line per submitted proof
}

func (c *Config) defaults() {
	if c.Provers < 1 {
		c.Provers = 1
	}
	if c.Poll <= 0 {
		c.Poll = 2 * time.Second
	}
	if c.MinBatch < 1 {
		c.MinBatch = 1
	}
	if c.BoundaryRetries == 0 {
		c.BoundaryRetries = 16
	}
	if c.Logf == nil {
		c.Logf = func(string, ...any) {}
	}
	if c.ProofLog == nil {
		c.ProofLog = func(string) {}
	}
}

// Stats is what Run reports.
type Stats struct {
	Submitted int // proofs accepted
	Failed    int // segments that failed (each restarts the pipeline from the chain's checkpoint)
}

// Pipeline keeps up to Config.Provers segments in flight. A segment k+1 starts its witness after segment k has fixed its end (the stage A
// of the shell version), its proof is built at the same time as k's, and it is submitted after k is accepted: SubmitRecoveryProof checks
// rt_last and first_height against the chain at submission, so only the order of the submissions matters.
type Pipeline struct {
	Cfg    Config
	Chain  Chain
	Linker Linker
	Prover Prover

	pubMu       sync.Mutex
	lastPublish time.Time
}

type segment struct {
	id       int
	prev     *segment
	stageA   chan struct{} // closed when end, rtNew and count are known
	done     chan struct{} // closed when the proof was accepted
	failed   chan struct{} // closed when the segment failed
	finished chan struct{} // closed when its goroutine ended
	failOnce sync.Once
	err      error

	end   uint64
	rtNew string
	count int
}

func newSegment(id int, prev *segment) *segment {
	return &segment{id: id, prev: prev, stageA: make(chan struct{}), done: make(chan struct{}), failed: make(chan struct{}), finished: make(chan struct{})}
}

func (s *segment) fail(err error) {
	s.failOnce.Do(func() { s.err = err; close(s.failed) })
}

func closed(c <-chan struct{}) bool {
	select {
	case <-c:
		return true
	default:
		return false
	}
}

// Run schedules segments until ctx ends or Config.MaxChecks polls have passed.
func (p *Pipeline) Run(ctx context.Context) Stats {
	cfg := &p.Cfg
	cfg.defaults()
	var st Stats
	nextID := 1
	var lastLaunch time.Time

	newEpoch := func() (context.Context, context.CancelFunc, *sync.WaitGroup) {
		c, cancel := context.WithCancel(ctx)
		return c, cancel, &sync.WaitGroup{}
	}
	ectx, cancel, wg := newEpoch()
	var live []*segment // segments of this epoch, oldest first
	var prev *segment

	reset := func() {
		cancel()
		wg.Wait()
		ectx, cancel, wg = newEpoch()
		live, prev = nil, nil
	}
	defer func() { cancel(); wg.Wait() }()

	for check := 0; cfg.MaxChecks == 0 || check < cfg.MaxChecks; check++ {
		if ctx.Err() != nil {
			break
		}
		// reap finished segments and react to a failure
		running := 0
		restart := false
		kept := live[:0]
		for _, s := range live {
			switch {
			case closed(s.failed):
				st.Failed++
				cfg.Logf("segment %d failed (%v): stopping the later ones and restarting from the chain's checkpoint", s.id, s.err)
				restart = true
			case closed(s.done):
				if closed(s.finished) {
					st.Submitted++
					continue // reaped
				}
				running++
				kept = append(kept, s)
			default:
				running++
				kept = append(kept, s)
			}
			if restart {
				break
			}
		}
		if restart {
			reset()
			running = 0
		} else {
			live = kept
		}
		if running == 0 {
			prev = nil
		}

		now := time.Now()
		if running < cfg.Provers && now.Sub(lastLaunch) >= cfg.Stagger && (prev == nil || closed(prev.stageA)) {
			var from uint64 // first height not covered by the previous segment
			if prev != nil {
				from = prev.end + 1
			}
			_, headers, err := p.Chain.Headers(ectx)
			avail := 0
			for _, h := range headers {
				if h.Height >= from {
					avail++
				}
			}
			if err != nil {
				cfg.Logf("check %d: %v", check+1, err)
			} else {
				cfg.Logf("check %d: %d headers beyond %d, in flight %d, submitted %d, failed %d", check+1, avail, int64(from)-1, running, st.Submitted, st.Failed)
			}
			if err == nil && avail >= cfg.MinBatch {
				s := newSegment(nextID, prev)
				nextID++
				wg.Add(1)
				segCtx := ectx // the epoch's context now, not whatever a later restart sets
				go func() {
					defer wg.Done()
					defer close(s.finished)
					if err := p.runSegment(segCtx, s); err != nil {
						s.fail(err)
					}
				}()
				live = append(live, s)
				cfg.Logf("segment %d started (follows %s)", s.id, followsName(prev))
				prev = s
				lastLaunch = now
			}
		}
		select {
		case <-ctx.Done():
		case <-time.After(cfg.Poll):
		}
	}
	// the epoch's segments still running are canceled by the deferred cancel; count what was accepted on the way
	for _, s := range live {
		if closed(s.done) {
			st.Submitted++
		}
	}
	return st
}

func followsName(prev *segment) string {
	if prev == nil {
		return "the chain's checkpoint"
	}
	return fmt.Sprintf("segment %d", prev.id)
}

func unixSec(t time.Time) int64 { return t.Unix() }

// runSegment is stage A (the end of the segment and its witness), the proof and stage C (the submission, after the previous segment's).
func (p *Pipeline) runSegment(ctx context.Context, s *segment) error {
	cfg := &p.Cfg
	t0 := time.Now()
	tipQuery := p.Chain.Tip(ctx)

	var minHeight uint64
	var rtLastPrev string
	if s.prev != nil {
		select {
		case <-s.prev.stageA:
		case <-s.prev.failed:
			return fmt.Errorf("segment %d failed before this one's witness", s.prev.id)
		case <-ctx.Done():
			return ctx.Err()
		}
		minHeight = s.prev.end + 1
		rtLastPrev = s.prev.rtNew
	}

	rtLast, all, err := p.Chain.Headers(ctx)
	if err != nil {
		return err
	}
	if s.prev != nil {
		rtLast = rtLastPrev
	}
	var headers []Header
	for _, h := range all {
		if h.Height >= minHeight {
			headers = append(headers, h)
		}
	}
	total := len(headers)
	if total < cfg.MinBatch {
		return fmt.Errorf("only %d headers beyond height %d", total, minHeight)
	}
	if len(headers) > NMax {
		headers = headers[:NMax]
	}

	// The segment ends at a block whose commit the witness builder can read (any round); when it cannot, or when the test control
	// WantRound asks for another round, the segment ends one header earlier. Every such step is logged with its reason.
	var boundary string
	var firstRound, lastRound, dropped int
	ctl := ReadControl(cfg.ControlDir)
	retries := cfg.BoundaryRetries
	if ctl.WantRound > 0 && retries < 64 {
		retries = 64 // a given round is rare among the blocks of an interval
	}
	for attempt := 0; ; attempt++ {
		last := headers[len(headers)-1].Height
		boundary, err = p.Chain.BoundaryWitness(ctx, minHeight, last)
		if err == nil {
			firstRound, lastRound = BoundaryRounds(boundary)
			if ctl.WantRound > 0 && lastRound != ctl.WantRound {
				err = fmt.Errorf("the last block committed in round %d, the test wants round %d", lastRound, ctl.WantRound)
			}
		}
		if err == nil {
			break
		}
		if attempt >= retries || len(headers) <= 1 {
			return fmt.Errorf("no boundary block with a usable commit within %d headers of the end: %w", retries, err)
		}
		cfg.Logf("segment %d: boundary at height %d unusable (%v), ending the segment one header earlier", s.id, last, err)
		headers = headers[:len(headers)-1]
		dropped++
	}
	cfg.Logf("segment %d: boundary blocks first=%d (round %d) last=%d (round %d), %d header(s) dropped from the end", s.id, headers[0].Height, firstRound, headers[len(headers)-1].Height, lastRound, dropped)
	tBoundary := time.Now()

	linked, rtNew, err := p.Linker.Link(ctx, rtLast, headers)
	if err != nil {
		return err
	}
	tWitness := time.Now()
	last := headers[len(headers)-1].Height
	s.end, s.rtNew, s.count = last, rtNew, len(headers)
	close(s.stageA) // the next segment may start its witness

	proof, err := p.Prover.Prove(ctx, fmt.Sprintf("seg%d", s.id), []byte(CircuitToml(rtLast, rtNew, linked, boundary)))
	if err != nil {
		return err
	}
	tProve := time.Now()

	if s.prev != nil {
		select {
		case <-s.prev.done:
		case <-s.prev.failed:
			return fmt.Errorf("segment %d failed before this one's submission", s.prev.id)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	tTurn := time.Now()

	daHeight := p.publish(ctx, rtLast, rtNew, linked)
	tDA := time.Now()

	dir, err := os.MkdirTemp("", "submit-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	proofPath, pubPath := filepath.Join(dir, "proof"), filepath.Join(dir, "public_inputs")
	if err := os.WriteFile(proofPath, proof.Proof, 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(pubPath, proof.PublicInputs, 0o600); err != nil {
		return err
	}
	if cfg.ControlDir != "" {
		if err := archiveProof(filepath.Join(cfg.ControlDir, "archive"), s.id, last, proofPath, pubPath, daHeight); err != nil {
			cfg.Logf("segment %d: the proof could not be archived: %v", s.id, err)
		}
		if HoldRequested(cfg.ControlDir) {
			cfg.Logf("segment %d: proof for heights %d-%d is ready, holding the submission while %s/hold exists", s.id, headers[0].Height, last, cfg.ControlDir)
			if !waitWhileHeld(ctx.Done(), cfg.ControlDir) {
				return ctx.Err()
			}
		}
	}
	if err := p.Chain.Submit(ctx, proofPath, pubPath, daHeight); err != nil {
		return fmt.Errorf("submission rejected: %w", err)
	}
	tSubmit := time.Now()
	close(s.done)

	// Same fields as the shell version's line (stage seconds: boundary witness, header linking, proof, DA publish, submit; then the total), then the
	// segment, the seconds spent waiting for the previous segment's submission and the worker.
	sec := func(a, b time.Time) int64 { return unixSec(b) - unixSec(a) }
	cfg.ProofLog(fmt.Sprintf("PROOFLOG,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%s",
		unixSec(t0), len(headers), headers[0].Height, last, total, tipQuery, p.Chain.Tip(ctx),
		sec(t0, tBoundary), sec(tBoundary, tWitness), sec(tWitness, tProve), sec(tTurn, tDA), sec(tDA, tSubmit), sec(t0, tSubmit),
		s.id, sec(tProve, tTurn), proof.Worker))
	return nil
}

// publish puts the header chain on Celestia as an audit trail, at most once per WitnessPublishMinInterval; a failure only means that the proof
// is submitted without a DA height.
func (p *Pipeline) publish(ctx context.Context, rtLast, rtNew string, linked []Linked) uint64 {
	cfg := &p.Cfg
	p.pubMu.Lock()
	defer p.pubMu.Unlock()
	if !p.lastPublish.IsZero() && time.Since(p.lastPublish) < cfg.WitnessPublishMinInterval {
		return 0
	}
	f, err := os.CreateTemp("", "witness-headers-*.toml")
	if err != nil {
		return 0
	}
	defer os.Remove(f.Name())
	_, werr := f.WriteString(AuditToml(rtLast, rtNew, linked))
	f.Close()
	if werr != nil {
		return 0
	}
	h, err := p.Chain.PublishWitness(ctx, f.Name())
	if err != nil {
		cfg.Logf("witness publish failed, submitting without a DA height: %v", err)
		return 0
	}
	p.lastPublish = time.Now()
	return h
}
