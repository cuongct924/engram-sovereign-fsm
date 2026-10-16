package prover

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeChain is a chain that grows one header every `rate` and accepts a proof only if it continues the last accepted one, as
// SubmitRecoveryProof does (rt_last and first_height are checked against the chain at submission).
type fakeChain struct {
	mu        sync.Mutex
	start     time.Time
	rate      time.Duration
	root      string
	anchored  uint64
	submitted []string // "first-last"
	rejected  []string
	badLast   func(uint64) bool
	roundOf   func(uint64) int // commit round of a block, nil: the witness carries no round lines
	published int32
}

func newFakeChain(rate time.Duration) *fakeChain {
	return &fakeChain{start: time.Now(), rate: rate, root: "r0"}
}

func (c *fakeChain) tipLocked() uint64 { return uint64(time.Since(c.start) / c.rate) }

func (c *fakeChain) Headers(context.Context) (string, []Header, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var hs []Header
	for h := c.anchored + 1; h <= c.tipLocked(); h++ {
		hs = append(hs, Header{Height: h, FSMState: "2", WithdrawalLocked: "1", StateRoot: strconv.FormatUint(h, 10)})
	}
	return c.root, hs, nil
}

func (c *fakeChain) BoundaryWitness(_ context.Context, first, last uint64) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if first == 0 {
		first = c.anchored + 1
	}
	if c.badLast != nil && c.badLast(last) {
		return "", fmt.Errorf("no round-0 commit at %d", last)
	}
	w := fmt.Sprintf("first_height = \"%d\"\nlast_height = \"%d\"\n\n[first_block]\nh = \"x\"\n", first, last)
	if c.roundOf != nil {
		w += fmt.Sprintf("round = \"%d\"\n\n[last_block]\nround = \"%d\"\n", c.roundOf(first), c.roundOf(last))
	}
	return w, nil
}

func (c *fakeChain) Tip(context.Context) uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.tipLocked()
}

func (c *fakeChain) PublishWitness(context.Context, string) (uint64, error) {
	atomic.AddInt32(&c.published, 1)
	return 77, nil
}

func tomlValue(toml, key string) string {
	for _, l := range strings.Split(toml, "\n") {
		if v, ok := strings.CutPrefix(l, key+" = \""); ok {
			return strings.TrimSuffix(v, `"`)
		}
	}
	return ""
}

func (c *fakeChain) Submit(_ context.Context, proofPath, _ string, _ uint64) error {
	b, err := os.ReadFile(proofPath)
	if err != nil {
		return err
	}
	toml := string(b)
	first, _ := strconv.ParseUint(tomlValue(toml, "first_height"), 10, 64)
	last, _ := strconv.ParseUint(tomlValue(toml, "last_height"), 10, 64)
	c.mu.Lock()
	defer c.mu.Unlock()
	tag := fmt.Sprintf("%d-%d", first, last)
	if tomlValue(toml, "rt_last") != c.root || first != c.anchored+1 {
		c.rejected = append(c.rejected, tag)
		return fmt.Errorf("stale proof %s: rt_last %s against chain %s, first %d against %d", tag, tomlValue(toml, "rt_last"), c.root, first, c.anchored+1)
	}
	c.root = tomlValue(toml, "rt_new")
	c.anchored = last
	c.submitted = append(c.submitted, tag)
	return nil
}

type fakeLinker struct{}

func (fakeLinker) Link(_ context.Context, _ string, hs []Header) ([]Linked, string, error) {
	out := make([]Linked, len(hs))
	for i, h := range hs {
		out[i] = Linked{PrevHash: "0x01", FSMState: "0x02", WithdrawalLocked: "0x01", StateRoot: "0x" + strconv.FormatUint(h.Height, 16)}
	}
	return out, "R" + strconv.FormatUint(hs[len(hs)-1].Height, 10), nil
}

type fakeProver struct {
	d       time.Duration
	failJob map[string]bool
	cur, hi int32
}

func (p *fakeProver) Prove(ctx context.Context, job string, toml []byte) (Proof, error) {
	n := atomic.AddInt32(&p.cur, 1)
	defer atomic.AddInt32(&p.cur, -1)
	for {
		h := atomic.LoadInt32(&p.hi)
		if n <= h || atomic.CompareAndSwapInt32(&p.hi, h, n) {
			break
		}
	}
	select {
	case <-time.After(p.d):
	case <-ctx.Done():
		return Proof{}, ctx.Err()
	}
	if p.failJob[job] {
		return Proof{}, fmt.Errorf("prover failed for %s", job)
	}
	// the fake proof carries the segment's boundary values so that the chain can check it
	return Proof{Proof: append(append([]byte{}, toml...), []byte("\nfirst_height = \""+tomlValue(string(toml), "first_height")+"\"\n")...), PublicInputs: []byte("p"), Worker: "fake"}, nil
}

func runPipeline(t *testing.T, ch *fakeChain, pr *fakeProver, cfg Config) (Stats, []string) {
	t.Helper()
	var mu sync.Mutex
	var lines []string
	cfg.ProofLog = func(l string) { mu.Lock(); lines = append(lines, l); mu.Unlock() }
	cfg.Logf = func(f string, a ...any) { t.Logf(f, a...) }
	p := &Pipeline{Cfg: cfg, Chain: ch, Linker: fakeLinker{}, Prover: pr}
	st := p.Run(context.Background())
	mu.Lock()
	defer mu.Unlock()
	return st, append([]string(nil), lines...)
}

func baseCfg() Config {
	return Config{Provers: 3, Stagger: 60 * time.Millisecond, Poll: 10 * time.Millisecond, MinBatch: 1, MaxChecks: 150, WitnessPublishMinInterval: time.Hour}
}

func TestPipelineSubmitsInOrderAndInParallel(t *testing.T) {
	ch := newFakeChain(10 * time.Millisecond)
	pr := &fakeProver{d: 150 * time.Millisecond}
	st, lines := runPipeline(t, ch, pr, baseCfg())
	ch.mu.Lock()
	defer ch.mu.Unlock()
	if len(ch.rejected) != 0 {
		t.Fatalf("the chain rejected proofs: %v", ch.rejected)
	}
	if len(ch.submitted) < 4 || st.Submitted < 4 {
		t.Fatalf("submitted %v, stats %+v", ch.submitted, st)
	}
	if st.Failed != 0 {
		t.Fatalf("failures: %+v", st)
	}
	if pr.hi < 2 || pr.hi > 3 {
		t.Fatalf("proofs in flight at most %d, want 2 or 3", pr.hi)
	}
	if len(lines) != len(ch.submitted) || !strings.HasPrefix(lines[0], "PROOFLOG,") {
		t.Fatalf("log lines %d for %d proofs: %v", len(lines), len(ch.submitted), lines)
	}
	if f := strings.Split(lines[0], ","); len(f) != 17 || f[16] != "fake" {
		t.Fatalf("PROOFLOG fields: %v", f)
	}
}

func TestPipelineRestartsFromTheChainAfterAFailure(t *testing.T) {
	ch := newFakeChain(10 * time.Millisecond)
	pr := &fakeProver{d: 120 * time.Millisecond, failJob: map[string]bool{"seg2": true}}
	st, _ := runPipeline(t, ch, pr, baseCfg())
	ch.mu.Lock()
	defer ch.mu.Unlock()
	if st.Failed < 1 {
		t.Fatalf("the failure was not reported: %+v", st)
	}
	if len(ch.rejected) != 0 {
		t.Fatalf("the chain rejected proofs after the restart: %v", ch.rejected)
	}
	if len(ch.submitted) < 3 {
		t.Fatalf("the pipeline did not continue after the failure: %v", ch.submitted)
	}
}

func TestPipelineEndsASegmentEarlierWhenTheBoundaryHasNoCommit(t *testing.T) {
	ch := newFakeChain(10 * time.Millisecond)
	ch.badLast = func(h uint64) bool { return h%4 == 0 }
	st, _ := runPipeline(t, ch, &fakeProver{d: 100 * time.Millisecond}, baseCfg())
	ch.mu.Lock()
	defer ch.mu.Unlock()
	if len(ch.rejected) != 0 || st.Submitted < 3 {
		t.Fatalf("rejected %v, submitted %v, stats %+v", ch.rejected, ch.submitted, st)
	}
	for _, s := range ch.submitted {
		last, _ := strconv.Atoi(strings.Split(s, "-")[1])
		if last%4 == 0 {
			t.Fatalf("a segment ends at %d, a height without a usable commit", last)
		}
	}
}

func TestPipelineStartsNothingBelowMinBatch(t *testing.T) {
	ch := newFakeChain(time.Hour) // no header ever
	cfg := baseCfg()
	cfg.MaxChecks = 20
	st, lines := runPipeline(t, ch, &fakeProver{d: time.Millisecond}, cfg)
	if st.Submitted != 0 || len(lines) != 0 {
		t.Fatalf("stats %+v lines %v", st, lines)
	}
}

func TestWitnessIsPublishedAtMostOnceInTheInterval(t *testing.T) {
	ch := newFakeChain(10 * time.Millisecond)
	_, _ = runPipeline(t, ch, &fakeProver{d: 100 * time.Millisecond}, baseCfg())
	if n := atomic.LoadInt32(&ch.published); n != 1 {
		t.Fatalf("published %d times with a one-hour interval, want 1", n)
	}
}

func TestSSHPoolOneProofPerWorkerAndTarParsed(t *testing.T) {
	var mu sync.Mutex
	busy := map[string]int{}
	overlap := false
	used := map[string]bool{}
	tarBytes := func() []byte {
		var b bytes.Buffer
		w := newTar(&b)
		w("proof", bytes.Repeat([]byte("p"), 100))
		w("public_inputs", bytes.Repeat([]byte("i"), 224))
		return b.Bytes()
	}()
	pool := NewSSHPool([]string{"ubuntu@w1", "ubuntu@w2"}, "/k", "img", "/kh")
	pool.Run = func(ctx context.Context, stdin ioReader, name string, args ...string) ([]byte, string, error) {
		host := args[len(args)-2]
		mu.Lock()
		busy[host]++
		if busy[host] > 1 {
			overlap = true
		}
		used[host] = true
		mu.Unlock()
		time.Sleep(40 * time.Millisecond)
		mu.Lock()
		busy[host]--
		mu.Unlock()
		return tarBytes, "", nil
	}
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			p, err := pool.Prove(context.Background(), fmt.Sprintf("j%d", i), []byte("toml"))
			if err != nil || len(p.Proof) != 100 || len(p.PublicInputs) != 224 || p.Worker == "" {
				t.Errorf("job %d: %+v %v", i, p, err)
			}
		}(i)
	}
	wg.Wait()
	if overlap {
		t.Fatal("a worker ran two proofs at once")
	}
	if len(used) != 2 {
		t.Fatalf("workers used: %v", used)
	}
}

func TestSSHPoolCancelRemovesTheContainer(t *testing.T) {
	var mu sync.Mutex
	var removed []string
	pool := NewSSHPool([]string{"ubuntu@w1"}, "/k", "img", "/kh")
	pool.Run = func(ctx context.Context, stdin ioReader, name string, args ...string) ([]byte, string, error) {
		cmd := args[len(args)-1]
		if strings.HasPrefix(cmd, "docker rm -f") {
			mu.Lock()
			removed = append(removed, cmd)
			mu.Unlock()
			return nil, "", nil
		}
		<-ctx.Done()
		return nil, "", ctx.Err()
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := pool.Prove(ctx, "segX", []byte("t")); done <- err }()
	time.Sleep(50 * time.Millisecond)
	t0 := time.Now()
	cancel()
	if err := <-done; err == nil {
		t.Fatal("a canceled proof must return an error")
	}
	if time.Since(t0) > time.Second {
		t.Fatal("the cancel waited")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(removed) != 1 || !strings.Contains(removed[0], "engram-prove-segX") {
		t.Fatalf("removed %v", removed)
	}
	// the worker is free again
	pool.Run = func(context.Context, ioReader, string, ...string) ([]byte, string, error) {
		return nil, "", fmt.Errorf("boom")
	}
	if _, err := pool.Prove(context.Background(), "segY", nil); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("worker not reusable or error lost: %v", err)
	}
}

func TestBoundaryRoundsReadsBothTables(t *testing.T) {
	w := "first_height = \"5\"\n\n[validators]\nx = \"1\"\n\n[first_block]\nround = \"0\"\nps_total = \"1\"\n\n[last_block]\nround = \"2\"\n"
	if f, l := BoundaryRounds(w); f != 0 || l != 2 {
		t.Fatalf("rounds %d, %d", f, l)
	}
	if f, l := BoundaryRounds("[first_block]\nh = \"x\"\n"); f != -1 || l != -1 {
		t.Fatalf("a table without a round line must give -1, got %d, %d", f, l)
	}
}

func TestPipelineEndsOnlyAtTheWantedCommitRound(t *testing.T) {
	ch := newFakeChain(10 * time.Millisecond)
	ch.roundOf = func(h uint64) int {
		if h%5 == 0 {
			return 2
		}
		return 0
	}
	cfg := baseCfg()
	cfg.ControlDir = t.TempDir()
	if err := os.WriteFile(cfg.ControlDir+"/want_round", []byte("2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	st, _ := runPipeline(t, ch, &fakeProver{d: 100 * time.Millisecond}, cfg)
	ch.mu.Lock()
	defer ch.mu.Unlock()
	if len(ch.rejected) != 0 || st.Submitted < 2 {
		t.Fatalf("rejected %v, submitted %v, stats %+v", ch.rejected, ch.submitted, st)
	}
	for _, s := range ch.submitted {
		last, _ := strconv.Atoi(strings.Split(s, "-")[1])
		if last%5 != 0 {
			t.Fatalf("a segment ends at %d, which did not commit in round 2", last)
		}
	}
}

func TestPipelineArchivesAndHoldsTheProofWhileTheHoldFileExists(t *testing.T) {
	ch := newFakeChain(10 * time.Millisecond)
	cfg := baseCfg()
	cfg.Provers, cfg.MaxChecks = 1, 150
	cfg.ControlDir = t.TempDir()
	hold := cfg.ControlDir + "/hold"
	if err := os.WriteFile(hold, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	go func() {
		time.Sleep(300 * time.Millisecond)
		_ = os.Remove(hold)
	}()
	start := time.Now()
	st, _ := runPipeline(t, ch, &fakeProver{d: 20 * time.Millisecond}, cfg)
	if st.Submitted < 1 || time.Since(start) < 300*time.Millisecond {
		t.Fatalf("the first proof was submitted while the hold file existed: %+v after %v", st, time.Since(start))
	}
	ents, _ := os.ReadDir(cfg.ControlDir + "/archive")
	if len(ents) < 3 {
		t.Fatalf("expected the proof, public inputs and DA height archived, got %d files", len(ents))
	}
}
