// E5b -- Down-hysteresis sensitivity on ANCHORED -> SUSPICIOUS (E5,
// sub-scenario 5b). Sweeps DownHysteresisThreshold over {1,2,4,6,8} using
// WARNING-level noise only (btc_gap at SuspiciousThreshold, never critical --
// a critical reading bypasses absorption by design, see CalculateNextState's
// ANCHORED branch), unlike the deliberately critical-level 5a environments.
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

var downHysteresisSweepValues = []uint64{1, 2, 4, 6, 8}

// warningDisturbances perturbs a healthy baseline at WARNING level only
// (IsCriticalCondition never true) -- the level the ANCHORED/SUSPICIOUS
// branches gate on. noisy_da/noisy_p2p are reused from envDisturbances: both
// are warning-level by construction (DA has no critical path; the P2P blip
// keeps ActiveAnchors at MinAnchorPeers, never 0).
var warningDisturbances = map[string]disturbance{
	// No-noise control: healSensors on every block, disturbed slot or not --
	// the real baseline for Figure 4's panel (b), not a derived/assumed line.
	"stable":      healSensors,
	"warning_btc": func(h *Harness) { h.BTC.SetGap(h.keeper.Params.SuspiciousThreshold) },
	"noisy_da":    envDisturbances["noisy_da"],
	"noisy_p2p":   envDisturbances["noisy_p2p"],
	"combined_warning": func(h *Harness) {
		h.BTC.SetGap(h.keeper.Params.SuspiciousThreshold)
		h.DA.SetAvailable(false)
		h.P2P.SetSnapshot(sensors.P2PSnapshot{ActiveAnchors: h.keeper.Params.MinAnchorPeers})
	},
}

type downHysteresisRun struct {
	DownHysteresisThreshold uint64
	Environment             string
	FinalState              string
	FlappingCount           int
	TotalTransitions        int
	WithdrawalBlocked       int
	AnchoredUptime          float64
	TimeOutsideAnchored     int // blocks where State != ANCHORED
	DemotionCount           int // real ANCHORED -> SUSPICIOUS transitions
	AbsorbedEvents          int // noisy blocks that stayed ANCHORED instead of demoting
	AbsorptionRate          float64
}

// runNoisyAnchored starts at default ANCHORED and runs a fixed noisy window,
// tracking per-block disturbances so AbsorbedEvents/DemotionCount are
// attributed to the ANCHORED -> SUSPICIOUS edge, not FlappingCount alone.
func runNoisyAnchored(t *testing.T, downHysteresisThreshold uint64, env string) downHysteresisRun {
	t.Helper()
	h := NewHarness(t)
	h.keeper.Params.DownHysteresisThreshold = downHysteresisThreshold
	disturb := warningDisturbances[env]

	healSensors(h)
	h.Advance()
	require.Equal(t, types.StateAnchored, h.State(), "environment %s: must start ANCHORED", env)

	rng := rand.New(rand.NewSource(11)) // distinct seed from 5a's up-hysteresis sweep
	disturbed := []bool{false}          // aligns with the initial healSensors/Advance block above
	for i := 0; i < noisyWindowBlocks; i++ {
		if rng.Float64() < noiseProbability {
			disturb(h)
			disturbed = append(disturbed, true)
		} else {
			healSensors(h)
			disturbed = append(disturbed, false)
		}
		h.Advance()
	}

	m := h.ComputeMetrics()
	timeline := h.Timeline()

	prevState := types.StateAnchored
	outsideAnchored, demotions, absorbed := 0, 0, 0
	for i, row := range timeline {
		if row.State != types.StateAnchored {
			outsideAnchored++
		}
		if disturbed[i] && prevState == types.StateAnchored {
			switch row.State {
			case types.StateSuspicious:
				demotions++
			case types.StateAnchored:
				absorbed++
			}
		}
		prevState = row.State
	}

	absorptionRate := 0.0
	if absorbed+demotions > 0 {
		absorptionRate = float64(absorbed) / float64(absorbed+demotions)
	}

	return downHysteresisRun{
		DownHysteresisThreshold: downHysteresisThreshold,
		Environment:             env,
		FinalState:              h.State(),
		FlappingCount:           m.FlappingCount,
		TotalTransitions:        m.TotalTransitions,
		WithdrawalBlocked:       m.WithdrawalBlocked,
		AnchoredUptime:          anchoredUptime(h),
		TimeOutsideAnchored:     outsideAnchored,
		DemotionCount:           demotions,
		AbsorbedEvents:          absorbed,
		AbsorptionRate:          absorptionRate,
	}
}

func TestE5b_DownHysteresisSweep(t *testing.T) {
	var runs []downHysteresisRun
	for _, dht := range downHysteresisSweepValues {
		for _, env := range []string{"stable", "warning_btc", "noisy_da", "noisy_p2p", "combined_warning"} {
			runs = append(runs, runNoisyAnchored(t, dht, env))
		}
	}

	require.NoError(t, os.MkdirAll("results", 0o755))
	path := filepath.Join("results", "e5b_down_hysteresis_sweep.csv")
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()
	_ = w.Write([]string{"down_hysteresis_threshold", "environment", "final_state", "flapping_count", "total_transitions", "withdrawal_blocked", "anchored_uptime", "time_outside_anchored", "demotion_count", "absorbed_events", "absorption_rate"})
	for _, r := range runs {
		_ = w.Write([]string{
			strconv.FormatUint(r.DownHysteresisThreshold, 10),
			r.Environment,
			r.FinalState,
			strconv.Itoa(r.FlappingCount),
			strconv.Itoa(r.TotalTransitions),
			strconv.Itoa(r.WithdrawalBlocked),
			strconv.FormatFloat(r.AnchoredUptime, 'f', 4, 64),
			strconv.Itoa(r.TimeOutsideAnchored),
			strconv.Itoa(r.DemotionCount),
			strconv.Itoa(r.AbsorbedEvents),
			strconv.FormatFloat(r.AbsorptionRate, 'f', 4, 64),
		})
		t.Logf("DownHysteresisThreshold=%d env=%s final_state=%s flapping=%d total_transitions=%d withdrawal_blocked=%d anchored_uptime=%.2f time_outside_anchored=%d demotions=%d absorbed=%d absorption_rate=%.2f",
			r.DownHysteresisThreshold, r.Environment, r.FinalState, r.FlappingCount, r.TotalTransitions, r.WithdrawalBlocked, r.AnchoredUptime, r.TimeOutsideAnchored, r.DemotionCount, r.AbsorbedEvents, r.AbsorptionRate)
	}

	// SUSPICIOUS alone never locks withdrawals (CircuitBreakerSafety only locks
	// SOVEREIGN/RECOVERING), and warning-level noise never escalates past
	// SUSPICIOUS -- so no run may report WithdrawalBlocked > 0.
	for _, r := range runs {
		require.Equal(t, 0, r.WithdrawalBlocked, "DownHysteresisThreshold=%d env=%s: warning-level noise must never lock withdrawals", r.DownHysteresisThreshold, r.Environment)
	}
}
