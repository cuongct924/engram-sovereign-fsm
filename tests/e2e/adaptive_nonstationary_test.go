// E5c -- adaptive vs. static down-hysteresis under NON-STATIONARY noise (quiet -> noisy -> quiet), in-process.
//
// Under stationary jitter a well-chosen static threshold already removes flapping (live sweep, docs/EXPERIMENT.md), so
// the only case for the adaptive threshold is a network whose noise changes: it should react fast when quiet, resist
// when noisy, and recover its responsiveness after the noise ends. This measures exactly that, with the real keeper
// (BeginBlocker -> CalculateNextState/NextFlapLevel), many seeds, and the SAME noise schedule for every configuration
// (paired comparison).
//
// Per seed and configuration:
//   - the noisy phase: flaps (a transition that reverses the previous one), demotions, share of blocks in ANCHORED;
//   - detection delay of a sustained DA outage injected (a) cold, before any noise, and (b) after the noise has ended
//     and the chain has been healthy in ANCHORED for `offset` blocks -- the time the adaptive level needs to decay.
//
// Run: ENGRAM_E5C=1 go test ./tests/e2e -run TestE5c -count=1 -timeout 30m
// Summary (mean and 95% t-intervals): python3 scripts/e5_hysteresis_flapping/summarize_nonstationary.py
package e2e

import (
	"encoding/csv"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
	"github.com/stretchr/testify/require"
)

type hysteresisConfig struct {
	Name         string
	Down, Max    uint64
	StableWindow uint64
}

const (
	nsQuietBlocks    = 150  // phase 1: quiet, ~3 min at 1.2 s
	nsNoisyBlocks    = 250  // phase 2: noisy, ~5 min (the live runs are 300 s)
	nsBurstStartProb = 0.03 // per block; bursts last 1-3 blocks, so ~2/3 of them reach a threshold of 2
	nsFaultCap       = 80   // blocks to wait for a detection after the outage starts
	nsRegainCap      = 200  // blocks to wait for ANCHORED after the noise ends
)

var nsOffsets = []int{0, 16, 48, 96, 200}

func nsConfigs() []hysteresisConfig {
	var cs []hysteresisConfig
	for _, k := range []uint64{2, 3, 4, 5, 6, 8} {
		cs = append(cs, hysteresisConfig{Name: fmt.Sprintf("static%d", k), Down: k, Max: k, StableWindow: 16})
	}
	for _, km := range [][2]uint64{{2, 8}, {2, 6}, {3, 8}} {
		for _, w := range []uint64{16, 32, 64, 128, 256} {
			cs = append(cs, hysteresisConfig{Name: fmt.Sprintf("adaptive%d-%d_w%d", km[0], km[1], w), Down: km[0], Max: km[1], StableWindow: w})
		}
	}
	return cs
}

// nsNoise returns which blocks of the noisy phase are disturbed: bursts of 1-3 consecutive warning-level blocks.
func nsNoise(seed int64, blocks int) []bool {
	rng := rand.New(rand.NewSource(seed))
	out := make([]bool, blocks)
	for i := 0; i < blocks; {
		if rng.Float64() < nsBurstStartProb {
			length := 1 + rng.Intn(3)
			for j := 0; j < length && i < blocks; j++ {
				out[i] = true
				i++
			}
		} else {
			i++
		}
	}
	return out
}

func nsHarness(t *testing.T, c hysteresisConfig) *Harness {
	h := NewHarness(t)
	h.keeper.Params.DownHysteresisThreshold = c.Down
	h.keeper.Params.MaxDownHysteresisThreshold = c.Max
	h.keeper.Params.StableWindow = c.StableWindow
	healSensors(h)
	h.Advance()
	require.Equal(t, types.StateAnchored, h.State())
	return h
}

func nsQuiet(h *Harness, n int) {
	for i := 0; i < n; i++ {
		healSensors(h)
		h.Advance()
	}
}

// nsDetect starts a sustained DA outage and returns the blocks until the FSM first leaves ANCHORED, or -1.
func nsDetect(h *Harness) int {
	healSensors(h)
	h.DA.SetAvailable(false)
	for i := 1; i <= nsFaultCap; i++ {
		h.Advance()
		if h.State() != types.StateAnchored {
			return i
		}
	}
	return -1
}

type nsRow struct {
	Config        string
	Seed          int64
	NoisyFlaps    int
	NoisyDemotion int
	NoisyUptime   float64
	DelayCold     int
	DelayAfter    map[int]int // offset -> blocks; -1 not detected, -2 never regained ANCHORED
}

func nsRun(t *testing.T, c hysteresisConfig, seed int64) nsRow {
	noise := nsNoise(seed, nsNoisyBlocks)
	row := nsRow{Config: c.Name, Seed: seed, DelayAfter: map[int]int{}}

	// Cold: the same quiet phase, then the outage, no noise in between.
	h := nsHarness(t, c)
	nsQuiet(h, nsQuietBlocks)
	row.DelayCold = nsDetect(h)

	for _, off := range nsOffsets {
		h := nsHarness(t, c)
		nsQuiet(h, nsQuietBlocks)
		for _, bad := range noise {
			healSensors(h)
			if bad {
				h.BTC.SetGap(h.keeper.Params.SuspiciousThreshold) // warning level, never critical
			}
			h.Advance()
		}
		if off == nsOffsets[0] { // the noisy-phase metrics do not depend on the later offset: take them once
			tl := h.Timeline()
			seg := tl[1+nsQuietBlocks : 1+nsQuietBlocks+nsNoisyBlocks]
			prev, last := types.StateAnchored, ""
			anchored := 0
			for _, r := range seg {
				if r.State == types.StateAnchored {
					anchored++
				}
				if r.State != prev {
					tr := prev + ">" + r.State
					if last == r.State+">"+prev {
						row.NoisyFlaps++
					}
					if prev == types.StateAnchored && r.State == types.StateSuspicious {
						row.NoisyDemotion++
					}
					last = tr
				}
				prev = r.State
			}
			row.NoisyUptime = float64(anchored) / float64(len(seg))
		}
		regained := h.State() == types.StateAnchored
		for i := 0; i < nsRegainCap && !regained; i++ {
			healSensors(h)
			h.Advance()
			regained = h.State() == types.StateAnchored
		}
		if !regained {
			row.DelayAfter[off] = -2
			continue
		}
		nsQuiet(h, off)
		if h.State() != types.StateAnchored {
			row.DelayAfter[off] = -2
			continue
		}
		row.DelayAfter[off] = nsDetect(h)
	}
	return row
}

func TestE5c_AdaptiveNonStationary(t *testing.T) {
	if os.Getenv("ENGRAM_E5C") == "" {
		t.Skip("set ENGRAM_E5C=1 to run the (about a minute long) non-stationary hysteresis simulation")
	}
	seeds := 200
	if v, err := strconv.Atoi(os.Getenv("ENGRAM_E5C_SEEDS")); err == nil && v > 0 {
		seeds = v
	}

	require.NoError(t, os.MkdirAll("results", 0o755))
	f, err := os.Create(filepath.Join("results", "e5c_adaptive_nonstationary.csv"))
	require.NoError(t, err)
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	header := []string{"config", "seed", "noisy_flaps", "noisy_demotions", "noisy_anchored_share", "delay_cold"}
	for _, off := range nsOffsets {
		header = append(header, "delay_after_"+strconv.Itoa(off))
	}
	require.NoError(t, w.Write(header))

	for _, c := range nsConfigs() {
		for s := 1; s <= seeds; s++ {
			r := nsRun(t, c, int64(s))
			rec := []string{c.Name, strconv.Itoa(s), strconv.Itoa(r.NoisyFlaps), strconv.Itoa(r.NoisyDemotion),
				strconv.FormatFloat(r.NoisyUptime, 'f', 4, 64), strconv.Itoa(r.DelayCold)}
			for _, off := range nsOffsets {
				rec = append(rec, strconv.Itoa(r.DelayAfter[off]))
			}
			require.NoError(t, w.Write(rec))
		}
	}
}

// E5c' -- repeated-trigger adversary. The adversary can read the committed flap level, hence the effective threshold,
// and sends a warning-level burst exactly that long whenever the chain is ANCHORED, then stops until it is ANCHORED
// again. What it spends is bad blocks; what it gets is demotions and time outside ANCHORED. A static threshold costs
// it the same k bad blocks every time, the adaptive one a growing number up to k_max (and a naive fixed-length burst
// of k0 blocks stops working once the level has risen).
type adversaryRow struct {
	Config           string
	Strategy         string
	BadBlocks        int
	Demotions        int
	OutsideAnchored  int
	BadPerDemotion   float64
	OutsideShare     float64
	DemotionsPer100B float64
}

func advRun(t *testing.T, c hysteresisConfig, strategy string, total int) adversaryRow {
	h := nsHarness(t, c)
	bad, demotions := 0, 0
	burstLeft, cooldown := 0, 0
	prev := types.StateAnchored
	outside := 0
	for i := 0; i < total; i++ {
		healSensors(h)
		level, _ := h.keeper.FlapLevel.Get(h.sdkCtx)
		if burstLeft == 0 && cooldown == 0 && h.State() == types.StateAnchored {
			switch strategy {
			case "tailored": // reads the committed level, sends exactly the effective threshold
				burstLeft = int(keeper.EffectiveDownHysteresisThreshold(level, h.keeper.Params))
			case "fixed_k0": // always k0 bad blocks, one healthy block between bursts
				burstLeft = int(h.keeper.Params.DownHysteresisThreshold)
			case "patient": // waits until the level has decayed to 0, then sends k0 bad blocks
				if level == 0 {
					burstLeft = int(h.keeper.Params.DownHysteresisThreshold)
				}
			}
		}
		if cooldown > 0 {
			cooldown--
		}
		if burstLeft > 0 {
			h.BTC.SetGap(h.keeper.Params.SuspiciousThreshold)
			burstLeft--
			bad++
			if burstLeft == 0 && strategy != "tailored" {
				cooldown = 1
			}
		}
		h.Advance()
		s := h.State()
		if s != types.StateAnchored {
			outside++
			burstLeft = 0 // the adversary waits for ANCHORED again
		}
		if prev == types.StateAnchored && s == types.StateSuspicious {
			demotions++
		}
		prev = s
	}
	r := adversaryRow{Config: c.Name, Strategy: strategy, BadBlocks: bad, Demotions: demotions, OutsideAnchored: outside,
		OutsideShare: float64(outside) / float64(total)}
	if demotions > 0 {
		r.BadPerDemotion = float64(bad) / float64(demotions)
	}
	if bad > 0 {
		r.DemotionsPer100B = 100 * float64(demotions) / float64(bad)
	}
	return r
}

func TestE5c_RepeatedTriggerAdversary(t *testing.T) {
	if os.Getenv("ENGRAM_E5C") == "" {
		t.Skip("set ENGRAM_E5C=1 to run the repeated-trigger adversary simulation")
	}
	require.NoError(t, os.MkdirAll("results", 0o755))
	f, err := os.Create(filepath.Join("results", "e5c_repeated_trigger.csv"))
	require.NoError(t, err)
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	require.NoError(t, w.Write([]string{"config", "strategy", "blocks", "bad_blocks", "demotions", "bad_per_demotion", "demotions_per_100_bad", "outside_anchored_share"}))
	const total = 1200
	for _, c := range nsConfigs() {
		for _, strategy := range []string{"tailored", "fixed_k0", "patient"} {
			r := advRun(t, c, strategy, total)
			require.NoError(t, w.Write([]string{c.Name, strategy, strconv.Itoa(total), strconv.Itoa(r.BadBlocks), strconv.Itoa(r.Demotions),
				strconv.FormatFloat(r.BadPerDemotion, 'f', 2, 64), strconv.FormatFloat(r.DemotionsPer100B, 'f', 2, 64),
				strconv.FormatFloat(r.OutsideShare, 'f', 4, 64)}))
		}
	}
}

// E8/B9 -- can b Byzantine proposers hide a real DA outage? Placement sweep with the real FSM.
//
// The proposer rotation is a fixed cycle of 7. What each block's certificate says follows from the weighted median over
// 7 vote extensions (b lying "healthy", 7-b truthfully reporting the outage) and from which quorum of 5 the proposer
// keeps (a Byzantine proposer picks the healthiest 5, an honest one aggregates all of them):
//
//	b <= 2   never healthy (lies stay below the median even in the biased quorum: Lemma 1)
//	b == 3   healthy only on blocks a Byzantine node proposes (3 lies of 5 in its quorum, 3 of 7 otherwise)
//	b >= 4   healthy on every block
//
// For every placement of b Byzantine slots in the cycle and each down threshold k, the outage starts at an ANCHORED
// chain; the test records the blocks until the first non-ANCHORED state and the share of ANCHORED blocks afterwards.
func TestE8_HideOutagePlacement(t *testing.T) {
	if os.Getenv("ENGRAM_E5C") == "" {
		t.Skip("set ENGRAM_E5C=1")
	}
	require.NoError(t, os.MkdirAll("results", 0o755))
	f, err := os.Create(filepath.Join("results", "e8_hide_outage_placement.csv"))
	require.NoError(t, err)
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	require.NoError(t, w.Write([]string{"b", "k", "placement", "max_honest_run", "blocks_to_detection", "anchored_share_after"}))

	const cycle, blocks = 7, 300
	for _, b := range []int{2, 3, 4} {
		for mask := 0; mask < 1<<cycle; mask++ {
			pop := 0
			for i := 0; i < cycle; i++ {
				pop += (mask >> i) & 1
			}
			if pop != b {
				continue
			}
			// longest run of consecutive honest slots in the cyclic order
			maxRun := 0
			for start := 0; start < cycle; start++ {
				run := 0
				for j := 0; j < cycle && (mask>>((start+j)%cycle))&1 == 0; j++ {
					run++
				}
				if run > maxRun {
					maxRun = run
				}
			}
			for _, k := range []uint64{2, 3, 4} {
				h := nsHarness(t, hysteresisConfig{Name: "k", Down: k, Max: k, StableWindow: 16})
				detected, anchoredAfter, total := -1, 0, 0
				for i := 0; i < blocks; i++ {
					byzProposer := (mask>>(i%cycle))&1 == 1
					healthy := b >= 4 || (b == 3 && byzProposer)
					healSensors(h)
					if !healthy {
						h.DA.SetAvailable(false)
					}
					h.Advance()
					if detected < 0 && h.State() != types.StateAnchored {
						detected = i + 1
					}
					if detected >= 0 {
						total++
						if h.State() == types.StateAnchored {
							anchoredAfter++
						}
					}
				}
				share := 0.0
				if total > 0 {
					share = float64(anchoredAfter) / float64(total)
				}
				require.NoError(t, w.Write([]string{strconv.Itoa(b), strconv.FormatUint(k, 10), fmt.Sprintf("%07b", mask),
					strconv.Itoa(maxRun), strconv.Itoa(detected), strconv.FormatFloat(share, 'f', 3, 64)}))
			}
		}
	}
}
