package e2e

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"cosmossdk.io/collections/colltest"
	log "cosmossdk.io/log/v2"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/iden3/go-merkletree-sql/v2/db/memory"

	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper/sensors"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
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

// Harness drives x/sovereignty's BeginBlocker across simulated block heights
// with mock-controlled sensors, for the E2 fault-injection scenarios in
// docs/EXPERIMENT.md. It runs entirely in-process: no CometBFT node, no
// Docker, no real network -- only cmtproto.Header{} (an empty struct) is used
// to satisfy sdk.Context's constructor signature.
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

// NewHarness builds an in-memory Keeper (colltest.MockStore + an in-memory
// SMT backing store) and three mock sensors, all starting from a healthy
// baseline (types.DefaultParams()'s thresholds, zero gaps, full P2P health).
func NewHarness(t *testing.T) *Harness {
	t.Helper()

	storeService, ctx := colltest.MockStore()
	cdc := codec.NewProtoCodec(codectypes.NewInterfaceRegistry())
	k := keeper.NewKeeper(storeService, cdc, memory.NewMemoryStorage())

	sdkCtx := sdk.NewContext(nil, cmtproto.Header{}, false, log.NewNopLogger()).WithContext(ctx)

	h := &Harness{
		t:      t,
		keeper: k,
		sdkCtx: sdkCtx,
		BTC:    sensors.NewBTCSensor(),
		DA:     sensors.NewDASensor(),
		P2P:    sensors.NewP2PSensor(),
	}

	// Start the P2P sensor at a fully-healthy snapshot (DefaultParams' minimums).
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

// SetReanchoringProofValid mirrors submitting a valid ZK recovery proof
// (normally done via MsgSubmitRecoveryProof) -- used by scenario S7 to drive
// RECOVERING -> ANCHORED once hysteresis is also satisfied.
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

// Advance runs one simulated block: composes the mock sensors into a
// PeripheralMetrics snapshot, stores it (as MsgInjectFault would), runs
// BeginBlocker, and appends a TimelineRow.
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

// daGapMetric converts the DASensor's coarse healthy/unhealthy reading into
// the raw da_gap scalar CalculateNextState's predicates expect.
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

// WriteCSV dumps the timeline to tests/e2e/results/<name>.csv, creating the
// directory if needed. Used so scripts/notebooks/analysis.ipynb-style
// tooling can consume real E2 run data instead of synthetic placeholders.
func (h *Harness) WriteCSV(name string) string {
	h.t.Helper()

	dir := filepath.Join("results")
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

// Metrics computed from the recorded timeline, matching docs/EXPERIMENT.md's
// E2 metric list (time-to-detection/fallback, recovery time, downtime proxy,
// withdrawal-blocked count, flapping count).
type E2Metrics struct {
	TimeToFallback     int64 // blocks from height 1 until first entering SOVEREIGN (-1 if never)
	RecoveryTime       int64 // blocks from entering RECOVERING until ANCHORED (-1 if never completed)
	WithdrawalBlocked  int   // number of blocks where WithdrawLocked was true
	FlappingCount      int   // number of state transitions that immediately reversed the previous one
	TotalTransitions   int
}

// ComputeMetrics derives E2Metrics from the recorded timeline.
func (h *Harness) ComputeMetrics() E2Metrics {
	var m E2Metrics
	m.TimeToFallback = -1
	m.RecoveryTime = -1

	// Seed with the FSM's actual initial default (see FSMInit in
	// spec/core/EngramFSM.tla / the zero-value fallback in abci.go's
	// BeginBlocker) so a transition that happens on the very first recorded
	// block is still counted, instead of being silently skipped.
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
		"%s: time_to_fallback=%d recovery_time=%d withdrawal_blocked_blocks=%d flapping_count=%d total_transitions=%d",
		name, m.TimeToFallback, m.RecoveryTime, m.WithdrawalBlocked, m.FlappingCount, m.TotalTransitions,
	)
}
