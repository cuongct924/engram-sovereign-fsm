package e2e

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper/sensors"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"

	"cosmossdk.io/collections/colltest"
	log "cosmossdk.io/log/v2"

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// TimelineRow is one logged block-height sample, matching the CSV schema
// consumed by scripts/notebooks/analysis.ipynb-style tooling.
type TimelineRow struct {
	Height             int64
	State              string
	BtcGap             uint64
	DAHealthy          bool
	P2PHealthy         bool
	SafeBlocks         uint64
	SuspiciousDuration uint64
	WithdrawLocked     bool
}

// Harness drives x/sovereignty's BeginBlocker across simulated heights with
// mock sensors, for E2's fault-injection scenarios -- fully in-process (no
// CometBFT/Docker/network; cmtproto.Header{} only satisfies sdk.Context's ctor).
type Harness struct {
	t      *testing.T
	keeper *keeper.Keeper
	sdkCtx sdk.Context

	BTC *sensors.BTCSensor
	DA  *sensors.DASensor
	P2P *sensors.P2PSensor

	height   int64
	timeline []TimelineRow
}

// NewHarness builds an in-memory Keeper and three mock sensors from a healthy
// baseline (DefaultParams thresholds, zero gaps, full P2P health).
func NewHarness(t *testing.T) *Harness {
	t.Helper()

	storeService, ctx := colltest.MockStore()
	cdc := codec.NewProtoCodec(codectypes.NewInterfaceRegistry())
	k := keeper.NewKeeper(storeService, cdc)

	sdkCtx := sdk.NewContext(nil, cmtproto.Header{}, false, log.NewNopLogger()).WithContext(ctx)

	h := &Harness{
		t:      t,
		keeper: k,
		sdkCtx: sdkCtx,
		BTC:    sensors.NewBTCSensor(),
		DA:     sensors.NewDASensor(),
		P2P:    sensors.NewP2PSensor(),
	}

	// Start the P2P sensor at a fully-healthy snapshot.
	p := k.Params
	h.P2P.SetSnapshot(sensors.P2PSnapshot{
		ActiveAnchors:   p.MinAnchorPeers,
		CleanPeers:      p.MinPeers,
		SubnetDiversity: p.MinSubnetDiversity,
		ChurnRate:       0,
		AvgTenure:       p.MinAvgTenure,
		Latency:         0,
	})

	return h
}

// SetReanchoringProofValid mirrors MsgSubmitRecoveryProof -- S7 uses it to
// drive RECOVERING -> ANCHORED once hysteresis is also satisfied.
func (h *Harness) SetReanchoringProofValid(valid bool) {
	require := h.t
	if err := h.keeper.ReanchoringProofValid.Set(h.sdkCtx, valid); err != nil {
		require.Fatalf("SetReanchoringProofValid: %v", err)
	}
}

// State returns the current FSM state.
func (h *Harness) State() string {
	s, err := h.keeper.FSMState.Get(h.sdkCtx)
	if err != nil {
		return types.StateAnchored
	}
	return s
}

// Advance runs one simulated block: compose mock sensors into a
// PeripheralMetrics snapshot (as MsgInjectFault would), run BeginBlocker, log a row.
func (h *Harness) Advance() {
	h.height++
	h.sdkCtx = h.sdkCtx.WithBlockHeight(h.height)

	btcGap, _ := h.BTC.GetMetric(h.sdkCtx)
	daHealthy := h.DA.IsHealthy()
	p2pSnap, _ := h.P2P.GetSnapshot(h.sdkCtx)

	metrics := &types.PeripheralMetrics{
		BtcGap:              btcGap,
		DaGap:               daGapMetric(daHealthy),
		IsDasFailed:         false,
		IsAttestationFailed: false,
		SubnetDiversity:     p2pSnap.SubnetDiversity,
		ActiveAnchors:       p2pSnap.ActiveAnchors,
		CleanPeers:          p2pSnap.CleanPeers,
		PeerChurnRate:       p2pSnap.ChurnRate,
		AvgPeerTenure:       p2pSnap.AvgTenure,
		PeerLatency:         p2pSnap.Latency,
	}
	if err := h.keeper.Metrics.Set(h.sdkCtx, metrics); err != nil {
		h.t.Fatalf("Metrics.Set: %v", err)
	}

	if err := sovereignty.BeginBlocker(h.sdkCtx, h.keeper); err != nil {
		h.t.Fatalf("BeginBlocker: %v", err)
	}

	state := h.State()
	safeBlocks, _ := h.keeper.SafeBlocks.Get(h.sdkCtx)
	suspiciousDuration, _ := h.keeper.SuspiciousDuration.Get(h.sdkCtx)
	p2pHealthy := types.IsP2PQualityHealthy(metrics, h.keeper.Params)

	h.timeline = append(h.timeline, TimelineRow{
		Height:             h.height,
		State:              state,
		BtcGap:             btcGap,
		DAHealthy:          daHealthy,
		P2PHealthy:         p2pHealthy,
		SafeBlocks:         safeBlocks,
		SuspiciousDuration: suspiciousDuration,
		WithdrawLocked:     types.WithdrawLocked(state),
	})
}

// daGapMetric maps the DASensor's coarse healthy/unhealthy reading onto the
// da_gap scalar CalculateNextState's predicates expect.
func daGapMetric(healthy bool) uint64 {
	if healthy {
		return 0
	}
	return types.DefaultParams().DAThreshold // >= DAThreshold => not healthy
}

// Timeline returns the recorded per-height samples so far.
func (h *Harness) Timeline() []TimelineRow {
	return h.timeline
}

// WriteCSV dumps the timeline to tests/e2e/results/<name>.csv for
// scripts/notebooks/analysis.ipynb-style tooling.
func (h *Harness) WriteCSV(name string) string {
	h.t.Helper()

	dir := "results"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		h.t.Fatalf("mkdir results: %v", err)
	}
	path := filepath.Join(dir, name+".csv")

	f, err := os.Create(path)
	if err != nil {
		h.t.Fatalf("create %s: %v", path, err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	_ = w.Write([]string{"height", "state", "btc_gap", "da_healthy", "p2p_healthy", "safe_blocks", "suspicious_duration", "withdraw_locked"})
	for _, row := range h.timeline {
		_ = w.Write([]string{
			strconv.FormatInt(row.Height, 10),
			row.State,
			strconv.FormatUint(row.BtcGap, 10),
			strconv.FormatBool(row.DAHealthy),
			strconv.FormatBool(row.P2PHealthy),
			strconv.FormatUint(row.SafeBlocks, 10),
			strconv.FormatUint(row.SuspiciousDuration, 10),
			strconv.FormatBool(row.WithdrawLocked),
		})
	}
	return path
}

// E2Metrics holds the metrics computed from the recorded timeline, matching
// docs/EXPERIMENT.md's E2 metric list (time-to-detection/fallback, recovery
// time, downtime proxy, withdrawal-blocked count, flapping count).
type E2Metrics struct {
	TimeToDetection   int64 // blocks from height 1 until first leaving ANCHORED, any state (-1 if never)
	TimeToFallback    int64 // blocks from height 1 until first entering SOVEREIGN (-1 if never)
	RecoveryTime      int64 // blocks from entering RECOVERING until ANCHORED (-1 if never completed)
	WithdrawalBlocked int   // number of blocks where WithdrawLocked was true
	FlappingCount     int   // number of state transitions that immediately reversed the previous one
	TotalTransitions  int
}

// ComputeMetrics derives E2Metrics from the recorded timeline.
func (h *Harness) ComputeMetrics() E2Metrics {
	var m E2Metrics
	m.TimeToDetection = -1
	m.TimeToFallback = -1
	m.RecoveryTime = -1

	// Seed with the FSM's real initial default (FSMInit in EngramFSM.tla /
	// abci.go's BeginBlocker fallback) so a transition on the first recorded
	// block is still counted, not silently skipped.
	prevState := types.StateAnchored
	var recoveringSince int64 = -1
	var lastTransition string

	for _, row := range h.timeline {
		if row.WithdrawLocked {
			m.WithdrawalBlocked++
		}

		if row.State != prevState {
			m.TotalTransitions++
			transition := prevState + "->" + row.State
			reverse := row.State + "->" + prevState
			if lastTransition == reverse {
				m.FlappingCount++
			}
			lastTransition = transition

			if row.State != types.StateAnchored && m.TimeToDetection == -1 {
				m.TimeToDetection = row.Height
			}
			if row.State == types.StateSovereign && m.TimeToFallback == -1 {
				m.TimeToFallback = row.Height
			}
			if row.State == types.StateRecovering {
				recoveringSince = row.Height
			}
			if row.State == types.StateAnchored && recoveringSince != -1 && m.RecoveryTime == -1 {
				m.RecoveryTime = row.Height - recoveringSince
			}
		}
		prevState = row.State
	}

	return m
}

func fmtMetrics(name string, m E2Metrics) string {
	return fmt.Sprintf(
		"%s: time_to_detection=%d time_to_fallback=%d recovery_time=%d withdrawal_blocked_blocks=%d flapping_count=%d total_transitions=%d",
		name, m.TimeToDetection, m.TimeToFallback, m.RecoveryTime, m.WithdrawalBlocked, m.FlappingCount, m.TotalTransitions,
	)
}
