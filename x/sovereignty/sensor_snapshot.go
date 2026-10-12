package sovereignty

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/cuongct220020/engram-sovereign-fsm/x/da"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Snapshot path of ExtendVote: Bitcoin/Celestia RPC runs in two background
// loops and ExtendVote only composes the latest result, so its duration no
// longer depends on external RPC latency. The composition mirrors
// RefreshMetrics (sensors_refresh.go) field for field; the difference is the
// freshness rule below. Nothing here touches committed state: the result is
// still only a signed observation that AggregateHealthCertificates certifies.
//
// Freshness rule (deterministic, local): an observation older than maxAge, or
// not yet taken, is "unknown" and reads as the worst case already used for an
// RPC error -- maximal btc_gap, DA failed -- never as the last healthy value.

const (
	defaultSensorInterval = 500 * time.Millisecond
	defaultSensorMaxAge   = 5 * time.Second
)

func envMillis(name string, def time.Duration) time.Duration {
	if v, err := strconv.Atoi(os.Getenv(name)); err == nil && v > 0 {
		return time.Duration(v) * time.Millisecond
	}
	return def
}

// syncSensorsFromEnv reports whether ENGRAM_SENSOR_MODE=sync selects the
// previous behaviour (ExtendVote reads RPC inline); kept as the measured
// "before" arm of the critical-path experiment.
func syncSensorsFromEnv() bool { return os.Getenv("ENGRAM_SENSOR_MODE") == "sync" }

// emptySensorsFromEnv reports whether ENGRAM_SENSOR_MODE=empty makes ExtendVote sign an
// all-zero observation without reading any sensor: the cost of the vote-extension mechanism
// alone, a baseline arm for the critical-path experiment, never a production setting.
func emptySensorsFromEnv() bool { return os.Getenv("ENGRAM_SENSOR_MODE") == "empty" }

// readerObs is what the reader loop measures over RPC.
type readerObs struct {
	btcReachable bool
	btcHeightOK  bool
	btcHeight    uint64
	spvFailed    bool
	daProbe      bool
}

// actorObs is the local bookkeeping that the submit/publish loop updates;
// its accessors can block behind an in-flight RPC, so they are read there.
type actorObs struct {
	confirmedAnchor uint64
	daVerified      uint64
	daHasVerified   bool
	daContiguous    uint64
	daFailed        bool
}

type snapshotInputs struct {
	height    int64
	hAnchored uint64
}

type sensorSnapshotCache struct {
	mu       sync.RWMutex
	in       snapshotInputs
	rd       readerObs
	rdAt     time.Time
	act      actorObs
	actAt    time.Time
	once     sync.Once
	interval time.Duration
	maxAge   time.Duration
}

func newSensorSnapshotCache() *sensorSnapshotCache {
	return &sensorSnapshotCache{
		interval: envMillis("ENGRAM_SENSOR_INTERVAL_MS", defaultSensorInterval),
		maxAge:   envMillis("ENGRAM_SENSOR_MAX_AGE_MS", defaultSensorMaxAge),
	}
}

func (c *sensorSnapshotCache) setInputs(in snapshotInputs) {
	c.mu.Lock()
	c.in = in
	c.mu.Unlock()
}

func (c *sensorSnapshotCache) inputs() snapshotInputs {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.in
}

func (c *sensorSnapshotCache) start(s *Sensors) {
	c.once.Do(func() {
		go c.readerLoop(s)
		go c.actorLoop(s)
	})
}

func (c *sensorSnapshotCache) readerLoop(s *Sensors) {
	t := time.NewTicker(c.interval)
	defer t.Stop()
	for ; ; <-t.C {
		in := c.inputs()
		if in.height == 0 {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), c.maxAge)
		obs := readExternal(ctx, s, in)
		cancel()
		c.mu.Lock()
		c.rd, c.rdAt = obs, time.Now()
		c.mu.Unlock()
	}
}

func readExternal(ctx context.Context, s *Sensors, in snapshotInputs) readerObs {
	var obs readerObs
	if src := s.BTC.Source(); src != nil {
		obs.btcReachable = src.Reachable(ctx)
		if obs.btcReachable {
			h, err := src.CurrentHeight(ctx)
			obs.btcHeightOK, obs.btcHeight = err == nil, h
			if obs.btcHeightOK && s.Anchor != nil && in.hAnchored > 0 {
				deep, derr := s.Anchor.IsAnchorDeepEnough(ctx, in.hAnchored)
				if derr != nil {
					obs.spvFailed = true
				} else if deep {
					ok, verr := s.Anchor.VerifyAnchor(ctx, in.hAnchored)
					obs.spvFailed = verr != nil || !ok
				}
			}
		}
	}
	if src := s.DA.Source(); src != nil {
		obs.daProbe = src.ProbeHealthy(ctx)
	}
	return obs
}

func (c *sensorSnapshotCache) actorLoop(s *Sensors) {
	t := time.NewTicker(c.interval)
	defer t.Stop()
	for ; ; <-t.C {
		in := c.inputs()
		if in.height == 0 {
			continue
		}
		ctx := context.Background()
		if s.Anchor != nil && in.height > 1 {
			_ = s.Anchor.MaybeSubmit(ctx, uint64(in.height)-1)
		}
		if s.DAPublisher != nil {
			_ = s.DAPublisher.MaybePublish(ctx, uint64(in.height), da.HeightMarker(uint64(in.height)))
		}
		var a actorObs
		if s.Anchor != nil {
			a.confirmedAnchor, _ = s.Anchor.ConfirmedAnchorHeight()
		}
		if s.DAPublisher != nil {
			a.daVerified, a.daHasVerified = s.DAPublisher.VerifiedHeight()
			a.daContiguous = s.DAPublisher.ContiguousHeight()
			a.daFailed = s.DAPublisher.Failed()
		} else if src := s.DA.Source(); src != nil {
			a.daVerified, a.daHasVerified = src.VerifiedHeight()
			a.daFailed = src.Failed()
		}
		c.mu.Lock()
		c.act, c.actAt = a, time.Now()
		c.mu.Unlock()
	}
}

// snapshotMetrics is RefreshMetrics' counterpart for ExtendVote: no RPC, no
// lock held by an RPC, so its latency is bounded by local work. degraded
// reports that at least one observation was missing or older than maxAge.
func snapshotMetrics(ctx sdk.Context, k *keeper.Keeper, s *Sensors, c *sensorSnapshotCache) (m *types.PeripheralMetrics, degraded bool, err error) {
	hAnchored, _ := k.HBtcAnchored.Get(ctx)
	c.setInputs(snapshotInputs{height: ctx.BlockHeight(), hAnchored: hAnchored})
	c.start(s)

	c.mu.RLock()
	rd, rdAt, act, actAt := c.rd, c.rdAt, c.act, c.actAt
	c.mu.RUnlock()
	now := time.Now()
	rdFresh := !rdAt.IsZero() && now.Sub(rdAt) <= c.maxAge
	actFresh := !actAt.IsZero() && now.Sub(actAt) <= c.maxAge

	var btcGap, btcCurrent uint64
	var spvFailed bool
	if s.BTC.Source() == nil {
		if btcGap, err = s.BTC.GetMetric(ctx); err != nil {
			return nil, false, err
		}
	} else {
		degraded = degraded || !rdFresh
		switch {
		case !rdFresh || !rd.btcReachable || !rd.btcHeightOK:
			btcGap = k.Params.SovereignThreshold
		case rd.btcHeight <= hAnchored:
			btcGap, btcCurrent, spvFailed = 0, rd.btcHeight, rd.spvFailed
		default:
			btcGap, btcCurrent, spvFailed = rd.btcHeight-hAnchored, rd.btcHeight, rd.spvFailed
		}
	}

	var daGap uint64
	var dasFailed, attestationFailed bool
	if s.DA.Source() == nil {
		if s.DA.IsHealthy() {
			dasFailed, attestationFailed = s.DA.DasFailed(), s.DA.AttestationFailed()
		} else {
			daGap, dasFailed, attestationFailed = k.Params.DAThreshold, s.DA.DasFailed(), s.DA.AttestationFailed()
		}
	} else {
		degraded = degraded || !rdFresh || !actFresh
		hCurrent := uint64(ctx.BlockHeight())
		if err = k.HEngramCurrent.Set(ctx, hCurrent); err != nil {
			return nil, false, err
		}
		failed := !rdFresh || !actFresh || !rd.daProbe || act.daFailed
		dasFailed, attestationFailed = failed, failed
		switch {
		case !act.daHasVerified:
			daGap = k.Params.DAThreshold
		case hCurrent > act.daVerified:
			daGap = hCurrent - act.daVerified
		}
	}

	p2p, err := s.P2P.GetSnapshot(ctx)
	if err != nil {
		return nil, false, err
	}
	daContiguous := uint64(ctx.BlockHeight())
	if s.DAPublisher != nil {
		daContiguous = act.daContiguous
	}

	m = &types.PeripheralMetrics{
		BtcGap:                       btcGap,
		IsAnchorReverificationFailed: spvFailed,
		DaGap:                        daGap,
		IsDasFailed:                  dasFailed,
		IsAttestationFailed:          attestationFailed,
		SubnetDiversity:              p2p.SubnetDiversity,
		ActiveAnchors:                p2p.ActiveAnchors,
		CleanPeers:                   p2p.CleanPeers,
		PeerChurnRate:                p2p.ChurnRate,
		AvgPeerTenure:                p2p.AvgTenure,
		PeerLatency:                  p2p.Latency,
		ConfirmedAnchorHeight:        act.confirmedAnchor,
		DaVerifiedHeight:             act.daVerified,
		DaContiguousHeight:           daContiguous,
		BtcCurrentHeight:             btcCurrent,
	}
	fmt.Printf("engramd: sensor_snapshot height=%d btc_gap=%d anchor_reverification_failed=%t da_gap=%d da_failed=%t attestation_failed=%t active_anchors=%d clean_peers=%d churn_rate=%d avg_tenure=%d peer_latency=%d snapshot_degraded=%t\n",
		ctx.BlockHeight(), m.BtcGap, m.IsAnchorReverificationFailed, m.DaGap, m.IsDasFailed, m.IsAttestationFailed, m.ActiveAnchors, m.CleanPeers, m.PeerChurnRate, m.AvgPeerTenure, m.PeerLatency, degraded)
	return m, degraded, nil
}
