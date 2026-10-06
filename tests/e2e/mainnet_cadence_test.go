// E5e -- how the FSM behaves when Bitcoin runs at mainnet cadence, using the real FSM (the e2e Harness) and a model of the
// checkpoint cycle of x/anchor.MaybeSubmit.
//
// The testbed mines a regtest block every 20 s; mainnet averages 600 s, with exponential gaps. Thresholds are in Bitcoin
// blocks, so wall-clock detection time scales with the cadence while the Engram block time (about 1.2 s) does not. The
// model of the checkpoint cycle:
//   - one checkpoint is pending at a time; it is included in the next Bitcoin block;
//   - it counts as confirmed once it has KDeepFinality+1 confirmations (the including block counts as one);
//   - the tracker submits the next one at once, unless submission is paused;
//   - the committed h_btc_anchored follows the tracker after `lag` Engram blocks (certification and polling delay).
// `lag` is the one free parameter. It is set from the testbed: the healthy btc_gap sits at 5 for about two Engram blocks
// (scripts/e9_composite_fault sensor logs), and with that value the model has to reproduce the quiet-phase share of
// ANCHORED measured live (about 0.8 at k=2, 1.0 at k=4) and the 117-142 s to SOVEREIGN of the checkpoint-pause fault.
//
// Run: ENGRAM_E5E=1 go test ./tests/e2e -run TestE5e -count=1 -v
package e2e

import (
	"encoding/csv"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"

	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
	"github.com/stretchr/testify/require"
)

const (
	e5eEngramBlockS = 1.2 // seconds per Engram block, as on the testbed
	e5eKDeep        = 2
)

// e5eMaxSuspiciousTime is the live genesis value (24 Engram blocks); TestE5e_MaxSuspiciousTimeSweep varies it.
var e5eMaxSuspiciousTime uint64 = 24

type e5eCadence struct {
	Name    string
	MeanS   float64
	Poisson bool
}

type e5eRun struct {
	h        *Harness
	rng      *rand.Rand
	cad      e5eCadence
	lag      int
	now      float64
	nextBtc  float64
	btcH     int
	anchored int
	pending  bool
	incl     int
	paused   bool
	queue    []e5eUpdate // committed anchored height updates, due at an Engram block index
	block    int
	state    string
}

type e5eUpdate struct {
	due, height int
}

func newE5eRun(t *testing.T, k uint64, cad e5eCadence, lag int, seed int64) *e5eRun {
	h := NewHarness(t)
	h.keeper.Params.DownHysteresisThreshold = k
	h.keeper.Params.MaxDownHysteresisThreshold = k
	// the live genesis (.env): SuspiciousHysteresisWait 8, MaxSuspiciousTime 24 Engram blocks
	h.keeper.Params.SuspiciousHysteresisWait = 8
	h.keeper.Params.MaxSuspiciousTime = e5eMaxSuspiciousTime
	r := &e5eRun{h: h, rng: rand.New(rand.NewSource(seed)), cad: cad, lag: lag, btcH: 1000}
	r.anchored = r.btcH - e5eKDeep
	r.nextBtc = r.interval()
	healSensors(h)
	return r
}

func (r *e5eRun) interval() float64 {
	if r.cad.Poisson {
		return r.rng.ExpFloat64() * r.cad.MeanS
	}
	return r.cad.MeanS
}

// step advances one Engram block and returns the FSM state afterwards.
func (r *e5eRun) step() string {
	r.now += e5eEngramBlockS
	r.block++
	for r.now >= r.nextBtc {
		r.btcH++
		r.nextBtc += r.interval()
		if r.pending && r.incl == 0 {
			r.incl = r.btcH
		}
	}
	// tracker: confirm the pending checkpoint, then submit the next one (MaybeSubmit)
	if r.pending && r.incl > 0 && r.btcH-r.incl+1 >= e5eKDeep+1 {
		r.queue = append(r.queue, e5eUpdate{due: r.block + r.lag, height: r.incl})
		r.pending = false
	}
	if !r.pending && !r.paused {
		r.pending = true
		r.incl = 0
	}
	for len(r.queue) > 0 && r.queue[0].due <= r.block {
		r.anchored = r.queue[0].height
		r.queue = r.queue[1:]
	}
	gap := r.btcH - r.anchored
	if gap < 0 {
		gap = 0
	}
	healSensors(r.h)
	r.h.BTC.SetGap(uint64(gap))
	r.h.Advance()
	r.state = r.h.State()
	return r.state
}

type e5eHealthy struct {
	AnchoredShare   float64
	DemotionsPerDay float64
	SovereignPerDay float64
	GapShare5       float64
}

func e5eHealthyRun(t *testing.T, k uint64, cad e5eCadence, lag int, hours float64, seed int64) e5eHealthy {
	r := newE5eRun(t, k, cad, lag, seed)
	warm := int(3600 / e5eEngramBlockS)
	for i := 0; i < warm; i++ {
		r.step()
	}
	n := int(hours * 3600 / e5eEngramBlockS)
	anchored, demotions, sovereign, atFive := 0, 0, 0, 0
	prev := r.state
	for i := 0; i < n; i++ {
		s := r.step()
		if s == types.StateAnchored {
			anchored++
		}
		if prev == types.StateAnchored && s != types.StateAnchored {
			demotions++
		}
		if prev != types.StateSovereign && s == types.StateSovereign {
			sovereign++
		}
		if r.btcH-r.anchored >= 5 {
			atFive++
		}
		prev = s
	}
	days := hours / 24
	return e5eHealthy{float64(anchored) / float64(n), float64(demotions) / days, float64(sovereign) / days, float64(atFive) / float64(n)}
}

type e5eFault struct {
	Seconds   float64
	BtcBlocks int
	Engram    int
}

// e5eFaultRun pauses checkpoint submission at a random phase and returns the time to SOVEREIGN.
func e5eFaultRun(t *testing.T, k uint64, cad e5eCadence, lag int, seed int64) (e5eFault, bool) {
	r := newE5eRun(t, k, cad, lag, seed)
	warm := int(3600/e5eEngramBlockS) + r.rng.Intn(int(2*cad.MeanS/e5eEngramBlockS)+1)
	for i := 0; i < warm; i++ {
		r.step()
	}
	r.paused = true
	t0, b0, blk0 := r.now, r.btcH, r.block
	limit := int(40 * cad.MeanS / e5eEngramBlockS)
	for i := 0; i < limit; i++ {
		if r.step() == types.StateSovereign {
			return e5eFault{r.now - t0, r.btcH - b0, r.block - blk0}, true
		}
	}
	return e5eFault{}, false
}

func e5ePercentile(v []float64, p float64) float64 {
	if len(v) == 0 {
		return math.NaN()
	}
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	return s[int(math.Min(float64(len(s)-1), math.Round(p*float64(len(s)-1))))]
}

func TestE5e_MainnetCadence(t *testing.T) {
	if os.Getenv("ENGRAM_E5E") == "" {
		t.Skip("set ENGRAM_E5E=1")
	}
	cads := []e5eCadence{
		{"testbed_steady_20s", 20, false},
		{"mainnet_steady_600s", 600, false},
		{"mainnet_poisson_600s", 600, true},
	}
	lags := []int{1, 2, 3, 4}
	require.NoError(t, os.MkdirAll("results", 0o755))
	f, err := os.Create(filepath.Join("results", "e5e_mainnet_cadence.csv"))
	require.NoError(t, err)
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	require.NoError(t, w.Write([]string{"cadence", "k", "lag_blocks", "healthy_hours", "anchored_share", "demotions_per_day", "sovereign_per_day",
		"gap_ge5_share", "fault_runs", "fault_detected", "t_sov_s_mean", "t_sov_s_p05", "t_sov_s_p95", "btc_blocks_mean", "engram_blocks_mean"}))
	for _, cad := range cads {
		hours := 6.0
		runs := 200
		if cad.MeanS > 100 {
			hours = 24 * 7
			runs = 40
		}
		if os.Getenv("ENGRAM_E5E_FAST") != "" {
			hours, runs = hours/8, runs/5
		}
		for _, k := range []uint64{2, 4} {
			for _, lag := range lags {
				hl := e5eHealthyRun(t, k, cad, lag, hours, 1)
				var secs, bb, eb []float64
				for s := int64(1); s <= int64(runs); s++ {
					if fr, ok := e5eFaultRun(t, k, cad, lag, s); ok {
						secs = append(secs, fr.Seconds)
						bb = append(bb, float64(fr.BtcBlocks))
						eb = append(eb, float64(fr.Engram))
					}
				}
				mean := func(v []float64) float64 {
					s := 0.0
					for _, x := range v {
						s += x
					}
					return s / float64(len(v))
				}
				row := []string{cad.Name, strconv.Itoa(int(k)), strconv.Itoa(lag), fmt.Sprintf("%.0f", hours),
					fmt.Sprintf("%.3f", hl.AnchoredShare), fmt.Sprintf("%.2f", hl.DemotionsPerDay), fmt.Sprintf("%.3f", hl.SovereignPerDay),
					fmt.Sprintf("%.3f", hl.GapShare5), strconv.Itoa(runs), strconv.Itoa(len(secs)),
					fmt.Sprintf("%.1f", mean(secs)), fmt.Sprintf("%.1f", e5ePercentile(secs, 0.05)), fmt.Sprintf("%.1f", e5ePercentile(secs, 0.95)),
					fmt.Sprintf("%.2f", mean(bb)), fmt.Sprintf("%.0f", mean(eb))}
				require.NoError(t, w.Write(row))
				w.Flush()
				fmt.Println(row)
			}
		}
	}
}

// e5eHazard estimates how often a healthy chain is escalated to SOVEREIGN at a given Bitcoin cadence: independent runs
// each end at their first SOVEREIGN or after maxHours, and the rate is events per exposed day. The harness never leaves
// SOVEREIGN without a recovery proof, so a single run cannot count repeats.
func TestE5e_FalseSovereignHazard(t *testing.T) {
	if os.Getenv("ENGRAM_E5E") == "" {
		t.Skip("set ENGRAM_E5E=1")
	}
	cads := []e5eCadence{
		{"testbed_steady_20s", 20, false},
		{"mainnet_steady_600s", 600, false},
		{"mainnet_poisson_600s", 600, true},
	}
	require.NoError(t, os.MkdirAll("results", 0o755))
	f, err := os.Create(filepath.Join("results", "e5e_false_sovereign_hazard.csv"))
	require.NoError(t, err)
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	require.NoError(t, w.Write([]string{"cadence", "k", "lag_blocks", "runs", "max_hours", "events", "exposed_days", "events_per_day",
		"demotions_per_day_before_first_sovereign"}))
	seeds, maxHours := 30, 72.0
	if os.Getenv("ENGRAM_E5E_FAST") != "" {
		seeds, maxHours = 6, 24
	}
	for _, cad := range cads {
		for _, k := range []uint64{2, 4} {
			for _, lag := range []int{2, 3} {
				events, demotions := 0, 0
				exposed := 0.0
				for s := int64(1); s <= int64(seeds); s++ {
					r := newE5eRun(t, k, cad, lag, 1000+s)
					for i := 0; i < int(3600/e5eEngramBlockS); i++ {
						r.step()
					}
					start := r.now
					prev := r.state
					n := int(maxHours * 3600 / e5eEngramBlockS)
					for i := 0; i < n; i++ {
						st := r.step()
						if prev == types.StateAnchored && st != types.StateAnchored {
							demotions++
						}
						if st == types.StateSovereign {
							events++
							break
						}
						prev = st
					}
					exposed += (r.now - start) / 86400
				}
				row := []string{cad.Name, strconv.Itoa(int(k)), strconv.Itoa(lag), strconv.Itoa(seeds), fmt.Sprintf("%.0f", maxHours),
					strconv.Itoa(events), fmt.Sprintf("%.1f", exposed), fmt.Sprintf("%.3f", float64(events)/exposed),
					fmt.Sprintf("%.1f", float64(demotions)/exposed)}
				require.NoError(t, w.Write(row))
				w.Flush()
				fmt.Println(row)
			}
		}
	}
}

// TestE5e_MaxSuspiciousTimeSweep asks how long the SUSPICIOUS timeout must be at mainnet cadence for a healthy chain to
// stop escalating, and what that costs a real fault in time to SOVEREIGN.
func TestE5e_MaxSuspiciousTimeSweep(t *testing.T) {
	if os.Getenv("ENGRAM_E5E") == "" {
		t.Skip("set ENGRAM_E5E=1")
	}
	defer func() { e5eMaxSuspiciousTime = 24 }()
	cad := e5eCadence{"mainnet_poisson_600s", 600, true}
	require.NoError(t, os.MkdirAll("results", 0o755))
	f, err := os.Create(filepath.Join("results", "e5e_max_suspicious_time.csv"))
	require.NoError(t, err)
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	require.NoError(t, w.Write([]string{"max_suspicious_time_blocks", "seconds", "k", "lag_blocks", "runs", "max_hours", "false_events", "exposed_days",
		"false_events_per_day", "fault_runs", "t_sov_min_mean", "t_sov_min_p05", "t_sov_min_p95", "btc_blocks_mean"}))
	seeds, maxHours, faults := 30, 72.0, 40
	if os.Getenv("ENGRAM_E5E_FAST") != "" {
		seeds, maxHours, faults = 6, 24, 10
	}
	for _, mst := range []uint64{24, 250, 500, 750, 1000, 1500, 2500} {
		e5eMaxSuspiciousTime = mst
		for _, k := range []uint64{4} {
			lag := 3
			events := 0
			exposed := 0.0
			for s := int64(1); s <= int64(seeds); s++ {
				r := newE5eRun(t, k, cad, lag, 1000+s)
				for i := 0; i < int(3600/e5eEngramBlockS); i++ {
					r.step()
				}
				start := r.now
				for i := 0; i < int(maxHours*3600/e5eEngramBlockS); i++ {
					if r.step() == types.StateSovereign {
						events++
						break
					}
				}
				exposed += (r.now - start) / 86400
			}
			var ts, bs []float64
			for s := int64(1); s <= int64(faults); s++ {
				fr, ok := e5eFaultRun(t, k, cad, lag, 5000+s)
				if ok {
					ts = append(ts, fr.Seconds/60)
					bs = append(bs, float64(fr.BtcBlocks))
				}
			}
			row := []string{strconv.Itoa(int(mst)), fmt.Sprintf("%.0f", float64(mst)*e5eEngramBlockS), strconv.Itoa(int(k)), strconv.Itoa(lag),
				strconv.Itoa(seeds), fmt.Sprintf("%.0f", maxHours), strconv.Itoa(events), fmt.Sprintf("%.1f", exposed),
				fmt.Sprintf("%.3f", float64(events)/exposed), strconv.Itoa(len(ts)), fmt.Sprintf("%.1f", e5eMean(ts)),
				fmt.Sprintf("%.1f", e5ePercentile(ts, 0.05)), fmt.Sprintf("%.1f", e5ePercentile(ts, 0.95)), fmt.Sprintf("%.1f", e5eMean(bs))}
			require.NoError(t, w.Write(row))
			w.Flush()
			fmt.Println(row)
		}
	}
}

func e5eMean(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := 0.0
	for _, x := range v {
		s += x
	}
	return s / float64(len(v))
}
