// E9 -- Trace-Driven Stress Test (E9, Figure 2). Replays one continuous
// combined-failure trace through the real Harness/BeginBlocker path: BTC
// congestion ramps, DA outage overlaps it, P2P churn overlaps both, then all
// three heal in sequence and the chain recovers. Unlike S1-S7 (one fault per
// scenario), faults overlap here.
package e2e

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper/sensors"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
	"github.com/stretchr/testify/require"
)

type traceRow struct {
	TimelineRow
	ProofSubmitted bool
}

func TestE9_TraceDrivenCombinedFailure(t *testing.T) {
	h := NewHarness(t)
	p := h.keeper.Params
	var rows []traceRow
	proofSubmitted := false

	record := func() {
		h.Advance()
		last := h.Timeline()[len(h.Timeline())-1]
		rows = append(rows, traceRow{TimelineRow: last, ProofSubmitted: proofSubmitted})
	}

	// Phase 1 (blocks 1-15): baseline healthy.
	for i := 0; i < 15; i++ {
		record()
	}
	require.Equal(t, types.StateAnchored, h.State(), "phase 1: must start healthy")

	// Phase 2 (blocks 16-25): BTC congestion ramps up past SovereignThreshold.
	btcRamp := []uint64{0, 0, 1, 1, p.SuspiciousThreshold + 1, p.SovereignThreshold, p.SovereignThreshold, p.SovereignThreshold, p.SovereignThreshold, p.SovereignThreshold}
	for _, gap := range btcRamp {
		h.BTC.SetGap(gap)
		record()
	}
	require.Equal(t, types.StateSovereign, h.State(), "phase 2: sustained BTC congestion must reach SOVEREIGN")

	// Phase 3 (blocks 26-35): DA outage overlaps the active BTC failure
	// (S6's combined failure, mid-trace).
	h.DA.SetAvailable(false)
	for i := 0; i < 10; i++ {
		record()
	}
	require.Equal(t, types.StateSovereign, h.State(), "phase 3: combined BTC+DA failure must stay SOVEREIGN")

	// Phase 4 (blocks 36-40): P2P churn spike overlaps both (triple combined
	// failure).
	h.P2P.SetSnapshot(sensors.P2PSnapshot{
		ActiveAnchors:   p.MinAnchorPeers,
		CleanPeers:      0,
		SubnetDiversity: 0,
		ChurnRate:       p.MaxChurnRate + 10,
		AvgTenure:       0,
		Latency:         p.MaxPeerLatency + 10,
	})
	for i := 0; i < 5; i++ {
		record()
	}
	require.Equal(t, types.StateSovereign, h.State(), "phase 4: triple combined failure must remain SOVEREIGN, not halt")

	// Phase 5 (blocks 41-45): peripherals heal one by one -- P2P first, then DA.
	h.P2P.SetSnapshot(sensors.P2PSnapshot{
		ActiveAnchors:   p.MinAnchorPeers,
		CleanPeers:      p.MinPeers,
		SubnetDiversity: p.MinSubnetDiversity,
		AvgTenure:       p.MinAvgTenure,
	})
	record()
	h.DA.SetAvailable(true)
	for i := 0; i < 4; i++ {
		record()
	}

	// Phase 6: BTC heals last -> RECOVERING, then wait hysteresis and submit
	// the reanchoring proof.
	h.BTC.SetGap(0)
	record()
	require.Equal(t, types.StateRecovering, h.State(), "phase 6: all peripherals healed must move SOVEREIGN -> RECOVERING")

	for i := uint64(0); i < p.HysteresisWait; i++ {
		record()
	}
	h.SetReanchoringProofValid(true)
	proofSubmitted = true
	record()
	require.Equal(t, types.StateAnchored, h.State(), "trace must fully recover to ANCHORED once all peripherals heal and hysteresis+proof are satisfied")

	writeTraceCSV(t, rows)
}

func writeTraceCSV(t *testing.T, rows []traceRow) {
	t.Helper()
	require.NoError(t, os.MkdirAll("results", 0o755))
	path := filepath.Join("results", "e9_trace_driven.csv")
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()
	_ = w.Write([]string{"height", "state", "btc_gap", "da_healthy", "p2p_healthy", "withdraw_locked", "proof_submitted", "block_committed"})
	for _, r := range rows {
		_ = w.Write([]string{
			strconv.FormatInt(r.Height, 10),
			r.State,
			strconv.FormatUint(r.BtcGap, 10),
			strconv.FormatBool(r.DAHealthy),
			strconv.FormatBool(r.P2PHealthy),
			strconv.FormatBool(r.WithdrawLocked),
			strconv.FormatBool(r.ProofSubmitted),
			"true", // every Advance() in this harness commits a block -- see harness.go doc
		})
	}
	t.Logf("wrote %d rows to %s", len(rows), path)
}
