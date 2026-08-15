// E5c -- SUSPICIOUS-exit hysteresis sensitivity on SUSPICIOUS -> ANCHORED
// (E5, sub-scenario 5c, the "Gray Failure Arbitrage" defense). Sweeps
// SuspiciousHysteresisWait over {1,2,4,6,8} with a sustained WARNING-level
// baseline interrupted by single-block healthy readings -- the exact attack
// shape (nudge sensors healthy one block to buy a free exit and reset the
// suspicious_duration clock). suspicious_duration keeps accumulating through
// absorbed blips and can itself escalate to SOVEREIGN via MaxSuspiciousTime
// during a long absorption run -- observed, not assumed away.
package e2e

import (
	"encoding/csv"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
	"github.com/stretchr/testify/require"
)

var suspiciousHysteresisSweepValues = []uint64{1, 2, 4, 6, 8}

// driveToSuspicious pushes a fresh Harness into SUSPICIOUS via sustained
// warning-level BTC gap (DownHysteresisThreshold+1 blocks worst case, bounded
// at 10 so a future default change can't hang the test).
func driveToSuspicious(t *testing.T, h *Harness) {
	t.Helper()
	for i := 0; i < 10 && h.State() != types.StateSuspicious; i++ {
		h.BTC.SetGap(h.keeper.Params.SuspiciousThreshold)
		h.Advance()
	}
	require.Equal(t, types.StateSuspicious, h.State(), "failed to reach SUSPICIOUS within 10 blocks")
}

type suspiciousExitRun struct {
	SuspiciousHysteresisWait uint64
	FinalState               string
	ReachedSovereign         bool // suspicious_duration escalated past MaxSuspiciousTime mid-run
	FlappingCount            int
	TotalTransitions         int
	AnchoredUptime           float64
	ExitCount                int // real SUSPICIOUS -> ANCHORED transitions
	AbsorbedEvents           int // healthy blips that stayed SUSPICIOUS instead of exiting
	AbsorptionRate           float64
	MaxSuspiciousDuration    uint64 // peak suspicious_duration observed -- this mechanism's actual defended quantity
}

func runNoisySuspicious(t *testing.T, suspiciousHysteresisWait uint64) suspiciousExitRun {
	t.Helper()
	h := NewHarness(t)
	h.keeper.Params.SuspiciousHysteresisWait = suspiciousHysteresisWait
	driveToSuspicious(t, h)

	rng := rand.New(rand.NewSource(13)) // distinct seed from 5a/5b
	baseHeight := len(h.Timeline())
	disturbed := make([]bool, 0, noisyWindowBlocks) // true = healthy blip this block
	for i := 0; i < noisyWindowBlocks; i++ {
		if rng.Float64() < noiseProbability {
			healSensors(h)
			disturbed = append(disturbed, true)
		} else {
			h.BTC.SetGap(h.keeper.Params.SuspiciousThreshold)
			disturbed = append(disturbed, false)
		}
		h.Advance()
	}

	m := h.ComputeMetrics()
	timeline := h.Timeline()

	prevState := types.StateSuspicious
	exits, absorbed := 0, 0
	reachedSovereign := false
	var maxDuration uint64
	for i := baseHeight; i < len(timeline); i++ {
		row := timeline[i]
		if row.SuspiciousDuration > maxDuration {
			maxDuration = row.SuspiciousDuration
		}
		if row.State == types.StateSovereign {
			reachedSovereign = true
		}
		if disturbed[i-baseHeight] && prevState == types.StateSuspicious {
			switch row.State {
			case types.StateAnchored:
				exits++
			case types.StateSuspicious:
				absorbed++
			}
		}
		prevState = row.State
	}

	absorptionRate := 0.0
	if absorbed+exits > 0 {
		absorptionRate = float64(absorbed) / float64(absorbed+exits)
	}

	return suspiciousExitRun{
		SuspiciousHysteresisWait: suspiciousHysteresisWait,
		FinalState:               h.State(),
		ReachedSovereign:         reachedSovereign,
		FlappingCount:            m.FlappingCount,
		TotalTransitions:         m.TotalTransitions,
		AnchoredUptime:           anchoredUptime(h),
		ExitCount:                exits,
		AbsorbedEvents:           absorbed,
		AbsorptionRate:           absorptionRate,
		MaxSuspiciousDuration:    maxDuration,
	}
}

func TestE5c_SuspiciousExitHysteresisSweep(t *testing.T) {
	var runs []suspiciousExitRun
	for _, shw := range suspiciousHysteresisSweepValues {
		runs = append(runs, runNoisySuspicious(t, shw))
	}

	require.NoError(t, os.MkdirAll("results", 0o755))
	path := filepath.Join("results", "e5c_suspicious_exit_sweep.csv")
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()
	_ = w.Write([]string{"suspicious_hysteresis_wait", "final_state", "reached_sovereign", "flapping_count", "total_transitions", "anchored_uptime", "exit_count", "absorbed_events", "absorption_rate", "max_suspicious_duration"})
	for _, r := range runs {
		_ = w.Write([]string{
			strconv.FormatUint(r.SuspiciousHysteresisWait, 10),
			r.FinalState,
			strconv.FormatBool(r.ReachedSovereign),
			strconv.Itoa(r.FlappingCount),
			strconv.Itoa(r.TotalTransitions),
			strconv.FormatFloat(r.AnchoredUptime, 'f', 4, 64),
			strconv.Itoa(r.ExitCount),
			strconv.Itoa(r.AbsorbedEvents),
			strconv.FormatFloat(r.AbsorptionRate, 'f', 4, 64),
			strconv.FormatUint(r.MaxSuspiciousDuration, 10),
		})
		t.Logf("SuspiciousHysteresisWait=%d final_state=%s reached_sovereign=%v flapping=%d total_transitions=%d anchored_uptime=%.2f exits=%d absorbed=%d absorption_rate=%.2f max_suspicious_duration=%d",
			r.SuspiciousHysteresisWait, r.FinalState, r.ReachedSovereign, r.FlappingCount, r.TotalTransitions, r.AnchoredUptime, r.ExitCount, r.AbsorbedEvents, r.AbsorptionRate, r.MaxSuspiciousDuration)
	}
}
