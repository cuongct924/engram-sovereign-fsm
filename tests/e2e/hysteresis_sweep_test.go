// E5 -- Hysteresis and Flapping Sensitivity (docs/EXPERIMENT.md's E5, Figure 4).
//
// Sweeps HysteresisWait over {0,1,3,5,10,20} using the same real Harness/
// BeginBlocker path as the E2 scenarios, under 5 environments matching
// docs/EXPERIMENT.md's E5 design: stable, intermittent DA receipt,
// intermittent BTC anchor, intermittent P2P churn, and combined adversarial
// oscillation (all three noisy at once). The noisy environments use a
// per-block Bernoulli disturbance (not a single one-shot flip) with a fixed
// RNG seed shared across all HysteresisWait values in the same environment,
// so the "weather" is identical and only HysteresisWait's filtering differs
// -- an earlier version of this test used a single disturbance, which never
// produced flapping at any HysteresisWait and didn't test what hysteresis is
// actually for (filtering noise, not surviving one hit).
package e2e

import (
	"encoding/csv"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper/sensors"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
)

var hysteresisSweepValues = []uint64{0, 1, 3, 5, 10, 20}

const noisyWindowBlocks = 100 // fixed window so all HysteresisWait values get a fair, identical-length shot
const noiseProbability = 0.2  // per-block chance of a 1-block disturbance in a noisy environment

type hysteresisRun struct {
	HysteresisWait   uint64
	Environment      string
	ReachedAnchored  bool // did the FSM ever reach ANCHORED within the window
	FirstAnchoredAt  int64
	FinalState       string // state at the end of the window -- "did it stick"
	FlappingCount    int
	TotalTransitions int
	AnchoredUptime   float64 // fraction of the run's blocks spent in ANCHORED -- the real "stability" metric
}

func anchoredUptime(h *Harness) float64 {
	timeline := h.Timeline()
	if len(timeline) == 0 {
		return 0
	}
	anchored := 0
	for _, row := range timeline {
		if row.State == types.StateAnchored {
			anchored++
		}
	}
	return float64(anchored) / float64(len(timeline))
}

// runStableRecovery: SOVEREIGN -> heal -> RECOVERING -> wait exactly
// HysteresisWait blocks -> submit proof -> expect ANCHORED. No noise -- the
// control group every noisy environment below is compared against.
func runStableRecovery(t *testing.T, hysteresisWait uint64) hysteresisRun {
	h := NewHarness(t)
	h.keeper.Params.HysteresisWait = hysteresisWait

	h.BTC.SetGap(h.keeper.Params.SovereignThreshold)
	h.Advance()
	require.Equal(t, types.StateSovereign, h.State())

	h.BTC.SetGap(0)
	h.Advance()
	require.Equal(t, types.StateRecovering, h.State())

	for i := uint64(0); i < hysteresisWait; i++ {
		h.Advance()
	}
	h.SetReanchoringProofValid(true)
	h.Advance()

	m := h.ComputeMetrics()
	return hysteresisRun{
		HysteresisWait:   hysteresisWait,
		Environment:      "stable",
		ReachedAnchored:  h.State() == types.StateAnchored,
		FirstAnchoredAt:  firstAnchoredHeight(h),
		FinalState:       h.State(),
		FlappingCount:    m.FlappingCount,
		TotalTransitions: m.TotalTransitions,
		AnchoredUptime:   anchoredUptime(h),
	}
}

func firstAnchoredHeight(h *Harness) int64 {
	for _, row := range h.Timeline() {
		if row.State == types.StateAnchored {
			return row.Height
		}
	}
	return -1
}

// disturbance describes which sensor(s) a noisy environment perturbs for
// exactly one block.
type disturbance func(h *Harness)

var envDisturbances = map[string]disturbance{
	"noisy_btc": func(h *Harness) { h.BTC.SetGap(h.keeper.Params.SovereignThreshold) },
	"noisy_da":  func(h *Harness) { h.DA.SetAvailable(false) },
	"noisy_p2p": func(h *Harness) {
		h.P2P.SetSnapshot(sensors.P2PSnapshot{ActiveAnchors: h.keeper.Params.MinAnchorPeers}) // clean/diversity/tenure all 0 -> unhealthy
	},
	"combined_adversarial": func(h *Harness) {
		h.BTC.SetGap(h.keeper.Params.SovereignThreshold)
		h.DA.SetAvailable(false)
		h.P2P.SetSnapshot(sensors.P2PSnapshot{ActiveAnchors: h.keeper.Params.MinAnchorPeers})
	},
}

func healSensors(h *Harness) {
	h.BTC.SetGap(0)
	h.DA.SetAvailable(true)
	p := h.keeper.Params
	h.P2P.SetSnapshot(sensors.P2PSnapshot{
		ActiveAnchors:   p.MinAnchorPeers,
		CleanPeers:      p.MinPeers,
		SubnetDiversity: p.MinSubnetDiversity,
		AvgTenure:       p.MinAvgTenure,
	})
}

// runNoisyRecovery: enters RECOVERING with the proof already valid, then runs
// a fixed window of blocks where each block independently has a
// noiseProbability chance of a 1-block disturbance (env-specific), otherwise
// healed. Uses a fixed per-environment RNG seed so every HysteresisWait value
// sees the identical noise sequence -- isolating HysteresisWait's effect.
func runNoisyRecovery(t *testing.T, hysteresisWait uint64, env string) hysteresisRun {
	h := NewHarness(t)
	h.keeper.Params.HysteresisWait = hysteresisWait
	disturb := envDisturbances[env]

	h.BTC.SetGap(h.keeper.Params.SovereignThreshold)
	h.Advance()
	require.Equal(t, types.StateSovereign, h.State())

	healSensors(h)
	h.Advance()
	require.Equal(t, types.StateRecovering, h.State(), "environment %s", env)
	h.SetReanchoringProofValid(true)

	rng := rand.New(rand.NewSource(7)) // fixed seed, identical noise sequence across all HysteresisWait values
	for i := 0; i < noisyWindowBlocks; i++ {
		if rng.Float64() < noiseProbability {
			disturb(h)
		} else {
			healSensors(h)
		}
		h.Advance()
	}

	m := h.ComputeMetrics()
	return hysteresisRun{
		HysteresisWait:   hysteresisWait,
		Environment:      env,
		ReachedAnchored:  firstAnchoredHeight(h) != -1,
		FirstAnchoredAt:  firstAnchoredHeight(h),
		FinalState:       h.State(),
		FlappingCount:    m.FlappingCount,
		TotalTransitions: m.TotalTransitions,
		AnchoredUptime:   anchoredUptime(h),
	}
}

func TestE5_HysteresisSweep(t *testing.T) {
	var runs []hysteresisRun
	for _, w := range hysteresisSweepValues {
		runs = append(runs, runStableRecovery(t, w))
		for _, env := range []string{"noisy_btc", "noisy_da", "noisy_p2p", "combined_adversarial"} {
			runs = append(runs, runNoisyRecovery(t, w, env))
		}
	}

	require.NoError(t, os.MkdirAll("results", 0o755))
	path := filepath.Join("results", "e5_hysteresis_sweep.csv")
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()
	_ = w.Write([]string{"hysteresis_wait", "environment", "reached_anchored", "first_anchored_at", "final_state", "flapping_count", "total_transitions", "anchored_uptime"})
	for _, r := range runs {
		_ = w.Write([]string{
			strconv.FormatUint(r.HysteresisWait, 10),
			r.Environment,
			strconv.FormatBool(r.ReachedAnchored),
			strconv.FormatInt(r.FirstAnchoredAt, 10),
			r.FinalState,
			strconv.Itoa(r.FlappingCount),
			strconv.Itoa(r.TotalTransitions),
			strconv.FormatFloat(r.AnchoredUptime, 'f', 4, 64),
		})
		t.Logf("HysteresisWait=%d env=%s reached_anchored=%v first_anchored_at=%d final_state=%s flapping=%d total_transitions=%d anchored_uptime=%.2f",
			r.HysteresisWait, r.Environment, r.ReachedAnchored, r.FirstAnchoredAt, r.FinalState, r.FlappingCount, r.TotalTransitions, r.AnchoredUptime)
	}

	// Stable (no noise) must always reach and stay ANCHORED regardless of HysteresisWait.
	for _, r := range runs {
		if r.Environment == "stable" {
			require.True(t, r.ReachedAnchored, "HysteresisWait=%d stable: must reach ANCHORED", r.HysteresisWait)
			require.Equal(t, types.StateAnchored, r.FinalState, "HysteresisWait=%d stable: must end ANCHORED", r.HysteresisWait)
		}
	}
}
