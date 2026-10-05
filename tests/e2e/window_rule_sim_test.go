// E5d -- would demoting on the SHARE of bad blocks in a window, instead of a consecutive run, beat the static and
// adaptive thresholds? A cheap pre-test before any change to the spec.
//
// The candidate is a reduced model of the ANCHORED <-> SUSPICIOUS edge (the only edge the experiments below exercise):
// the same recovery rule as the keeper (SuspiciousHysteresisWait healthy blocks, leak on noise), but a different
// demotion rule. The model is validated first: with the consecutive-run rules it must reproduce the keeper-based
// results of TestE5c_AdaptiveNonStationary and TestE8_HideOutagePlacement on the same 200 noise schedules.
//
// Run: ENGRAM_E5C=1 go test ./tests/e2e -run TestE5d -count=1
package e2e

import (
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

type miniRule struct {
	Name string
	K    int // consecutive-run rule when K > 0
	M, W int // window rule (>= M bad of the last W blocks) when K == 0
	// Levels, when set, make the rule adaptive: Levels[flapLevel] is the rule in force; a demotion raises the level
	// (capped at the last), StableWindow healthy ANCHORED blocks lower it.
	Levels       []miniRule
	StableWindow int
}

// base is the threshold a naive adversary sends: K blocks, or M for a window rule (the first level when adaptive).
func (r miniRule) base() int {
	if len(r.Levels) > 0 {
		return r.Levels[0].base()
	}
	if r.K > 0 {
		return r.K
	}
	return r.M
}

const miniRecoveryWait = 2 // DefaultParams.SuspiciousHysteresisWait, as in the keeper-based simulation

type miniFSM struct {
	rule    miniRule
	level   int
	stable  int
	inS     bool
	streak  int
	safe    int
	history []bool
	demoted int
}

func (m *miniFSM) current() miniRule {
	if len(m.rule.Levels) > 0 {
		return m.rule.Levels[m.level]
	}
	return m.rule
}

func (m *miniFSM) trigger(bad bool) bool {
	r := m.current()
	if r.K > 0 {
		return bad && m.streak+1 >= r.K
	}
	m.history = append(m.history, bad)
	if len(m.history) > r.W {
		m.history = m.history[1:]
	}
	n := 0
	for _, b := range m.history {
		if b {
			n++
		}
	}
	return bad && n >= r.M
}

// step consumes one block and returns whether the state is SUSPICIOUS afterwards.
func (m *miniFSM) step(bad bool) bool {
	if !m.inS {
		if m.trigger(bad) {
			m.inS, m.streak, m.safe, m.history, m.stable = true, 0, 0, nil, 0
			m.demoted++
			if len(m.rule.Levels) > 0 && m.level < len(m.rule.Levels)-1 {
				m.level++
			}
		} else if bad {
			m.streak++
			if m.stable > 0 { // an absorbed warning block leaks the stable counter, as NextStableBlocks does
				m.stable--
			}
		} else {
			m.streak = 0
			if m.level > 0 {
				m.stable++
				if m.stable >= m.rule.StableWindow {
					m.level--
					m.stable = 0
				}
			}
		}
		return m.inS
	}
	if bad {
		if m.safe > 0 {
			m.safe--
		}
		return true
	}
	if m.safe+1 >= miniRecoveryWait {
		m.inS, m.safe, m.streak, m.history = false, 0, 0, nil
	} else {
		m.safe++
	}
	return m.inS
}

func miniFlaps(states []bool) int {
	prev, last, flaps := false, "", 0
	for _, s := range states {
		if s != prev {
			tr := fmt.Sprintf("%v>%v", prev, s)
			if last == fmt.Sprintf("%v>%v", s, prev) {
				flaps++
			}
			last = tr
		}
		prev = s
	}
	return flaps
}

func miniNoisy(rule miniRule, seed int64) int {
	noise := nsNoise(seed, nsNoisyBlocks)
	m := &miniFSM{rule: rule}
	for i := 0; i < nsQuietBlocks; i++ {
		m.step(false)
	}
	var seg []bool
	for _, bad := range noise {
		seg = append(seg, m.step(bad))
	}
	return miniFlaps(seg)
}

func miniCold(rule miniRule) int {
	m := &miniFSM{rule: rule}
	for i := 0; i < nsQuietBlocks; i++ {
		m.step(false)
	}
	for i := 1; i <= nsFaultCap; i++ {
		if m.step(true) {
			return i
		}
	}
	return -1
}

// miniAfterNoise: detection delay of an outage that starts `quiet` healthy blocks after the noisy phase ended and the
// chain is ANCHORED again.
func miniAfterNoise(rule miniRule, seed int64, quiet int) int {
	noise := nsNoise(seed, nsNoisyBlocks)
	m := &miniFSM{rule: rule}
	for i := 0; i < nsQuietBlocks; i++ {
		m.step(false)
	}
	for _, bad := range noise {
		m.step(bad)
	}
	for i := 0; i < nsRegainCap && m.inS; i++ {
		m.step(false)
	}
	for i := 0; i < quiet; i++ {
		m.step(false)
	}
	for i := 1; i <= nsFaultCap; i++ {
		if m.step(true) {
			return i
		}
	}
	return -1
}

func meanAfter(r miniRule, quiet int) float64 {
	var sum float64
	n := 0
	for s := 1; s <= 200; s++ {
		if d := miniAfterNoise(r, int64(s), quiet); d >= 0 {
			sum += float64(d)
			n++
		}
	}
	if n == 0 {
		return -1
	}
	return sum / float64(n)
}

// miniHidden counts the placements (of 35) of three Byzantine proposers in the 7-slot cycle under which a real outage
// is never detected: honest proposers' blocks report it, Byzantine ones do not. `level` is the initial flap level.
func miniHidden(rule miniRule, level int) int {
	hidden := 0
	for mask := 0; mask < 1<<7; mask++ {
		pop := 0
		for i := 0; i < 7; i++ {
			pop += (mask >> i) & 1
		}
		if pop != 3 {
			continue
		}
		m := &miniFSM{rule: rule, level: level}
		detected := false
		for i := 0; i < 300 && !detected; i++ {
			detected = m.step((mask>>(i%7))&1 == 0)
		}
		if !detected {
			hidden++
		}
	}
	return hidden
}

// miniNaive: bursts of the rule's base length with one healthy block in between, only while ANCHORED, 1200 blocks.
func miniNaive(rule miniRule) int {
	m := &miniFSM{rule: rule}
	burst, gap := 0, 0
	base := rule.base()
	for i := 0; i < 1200; i++ {
		bad := false
		if m.inS {
			burst, gap = 0, 0
		} else if gap > 0 {
			gap--
		} else {
			if burst == 0 {
				burst = base
			}
			bad = true
			burst--
			if burst == 0 {
				gap = 1
			}
		}
		m.step(bad)
	}
	return m.demoted
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func TestE5d_WindowRule(t *testing.T) {
	if os.Getenv("ENGRAM_E5C") == "" {
		t.Skip("set ENGRAM_E5C=1")
	}
	rules := []miniRule{{Name: "static2", K: 2}, {Name: "static3", K: 3}, {Name: "static4", K: 4}, {Name: "static5", K: 5}}
	for _, mw := range [][2]int{{2, 3}, {3, 5}, {4, 6}, {4, 8}, {5, 8}} {
		rules = append(rules, miniRule{Name: fmt.Sprintf("window%d-of-%d", mw[0], mw[1]), M: mw[0], W: mw[1]})
	}
	// the adaptive logic is validated against the keeper-based simulation (adaptive 2->4, T=64) before the new rule
	rules = append(rules,
		miniRule{Name: "adaptive2-4_T64", Levels: []miniRule{{K: 2}, {K: 4}}, StableWindow: 64},
		miniRule{Name: "adaptive-window_T64", Levels: []miniRule{{M: 2, W: 3}, {M: 3, W: 5}, {M: 4, W: 8}}, StableWindow: 64})

	require.NoError(t, os.MkdirAll("results", 0o755))
	f, err := os.Create(filepath.Join("results", "e5d_window_rule.csv"))
	require.NoError(t, err)
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	require.NoError(t, w.Write([]string{"rule", "flaps_mean", "flaps_ci95", "cold_detection_blocks", "after_noise_detect_0", "after_noise_detect_96",
		"hidden_quiet_of_35", "hidden_raised_of_35", "naive_demotions_1200"}))
	for _, r := range rules {
		var sum, sumsq float64
		const seeds = 200
		for s := 1; s <= seeds; s++ {
			v := float64(miniNoisy(r, int64(s)))
			sum += v
			sumsq += v * v
		}
		mean := sum / seeds
		sd := math.Sqrt((sumsq - seeds*mean*mean) / (seeds - 1))
		require.NoError(t, w.Write([]string{r.Name, strconv.FormatFloat(mean, 'f', 2, 64), strconv.FormatFloat(1.96*sd/math.Sqrt(seeds), 'f', 2, 64),
			strconv.Itoa(miniCold(r)), strconv.FormatFloat(meanAfter(r, 0), 'f', 2, 64), strconv.FormatFloat(meanAfter(r, 96), 'f', 2, 64),
			strconv.Itoa(miniHidden(r, 0)), strconv.Itoa(miniHidden(r, maxInt(len(r.Levels)-1, 0))), strconv.Itoa(miniNaive(r))}))
	}
}
