// E5 -- Hysteresis and Flapping Sensitivity (E5, Figure 4). Sweeps
// HysteresisWait over {0,1,3,5,10,20} through the real Harness/BeginBlocker
// path under 5 environments (stable, noisy DA, noisy BTC, noisy P2P,
// combined). Noisy envs use a per-block Bernoulli disturbance with a fixed
// RNG seed across all HysteresisWait values, so only hysteresis filtering
// differs -- an earlier one-shot-disturbance version never produced flapping
// and didn't test what hysteresis is actually for.
package e2e

import (
	"encoding/csv"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper/sensors"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
	"github.com/stretchr/testify/require"
)

var hysteresisSweepValues = []uint64{0, 1, 3, 5, 10, 20}

const (
	noisyWindowBlocks = 100 // fixed window so all HysteresisWait values get a fair, identical-length shot
	noiseProbability  = 0.2 // per-block chance of a 1-block disturbance in a noisy environment
)

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

// runStableRecovery: SOVEREIGN -> heal -> RECOVERING -> wait HysteresisWait
// -> submit proof -> ANCHORED. No-noise control for the noisy environments.
func runStableRecovery(t *testing.T, hysteresisWait uint64) hysteresisRun {
	t.Helper()
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

// disturbance perturbs which sensor(s) for exactly one noisy block.
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

// runNoisyRecovery: enter RECOVERING with the proof valid, then run a fixed
// window where each block independently disturbs (noiseProbability) or heals.
// Fixed per-env RNG seed => identical noise across HysteresisWait values.
func runNoisyRecovery(t *testing.T, hysteresisWait uint64, env string) hysteresisRun {
	t.Helper()
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

	rng := rand.New(rand.NewSource(7)) // fixed seed: identical noise sequence across all HysteresisWait values
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
