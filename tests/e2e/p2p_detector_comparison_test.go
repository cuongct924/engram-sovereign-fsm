// E4 -- P2P Eclipse/Sybil Detection (Table 6). No Docker daemon here, so
// this is a synthetic Monte Carlo comparison of the real tri-interface
// profiler (types.IsP2PQualityHealthy) against a naive peer-count-only
// baseline, across E4's 4 attack scenarios -- not a live measurement.
package e2e

import (
	"encoding/csv"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
	"github.com/stretchr/testify/require"
)

const p2pTrials = 2000 // Monte Carlo trials per (attack, detector) cell

// peerCountOnlyHealthy mirrors a naive CometBFT-style detector: raw
// CleanPeers count only, ignoring diversity/anchors/churn/tenure/latency.
func peerCountOnlyHealthy(m *types.PeripheralMetrics, p types.Params) bool {
	return m.CleanPeers >= p.MinPeers
}

// benignSnapshot generates a randomized healthy snapshot (small jitter around
// DefaultParams' minimums).
func benignSnapshot(rng *rand.Rand, p types.Params) *types.PeripheralMetrics {
	return &types.PeripheralMetrics{
		SubnetDiversity: p.MinSubnetDiversity + uint64(rng.Intn(5)),
		ActiveAnchors:   p.MinAnchorPeers + uint64(rng.Intn(3)),
		CleanPeers:      p.MinPeers + uint64(rng.Intn(10)),
		PeerChurnRate:   uint64(rng.Intn(int(p.MaxChurnRate) + 1)),
		AvgPeerTenure:   p.MinAvgTenure + uint64(rng.Intn(500)),
		PeerLatency:     uint64(rng.Intn(int(p.MaxPeerLatency) + 1)),
	}
}

// attackGenerator returns a snapshot at "intensity" in [0,1] (0 = benign,
// 1 = full attack), used both for single-shot Monte Carlo trials and for
// ramp-up detection-delay series.
type attackGenerator func(rng *rand.Rand, p types.Params, intensity float64) *types.PeripheralMetrics

var p2pAttackGenerators = map[string]attackGenerator{
	// A1 -- Peer Slot Exhaustion: floods slots with junk peers (CleanPeers
	// looks fine) that crowd out anchors and never stay connected (low
	// tenure, high churn).
	"A1_PeerSlotExhaustion": func(rng *rand.Rand, p types.Params, intensity float64) *types.PeripheralMetrics {
		m := benignSnapshot(rng, p)
		m.CleanPeers += uint64(intensity * 20) // looks MORE healthy by raw count
		if intensity > 0.3 && m.ActiveAnchors > 0 {
			m.ActiveAnchors = uint64(float64(m.ActiveAnchors) * (1 - intensity*0.9))
		}
		m.AvgPeerTenure = uint64(float64(m.AvgPeerTenure) * (1 - intensity*0.9))
		m.PeerChurnRate += uint64(intensity * float64(p.MaxChurnRate+10))
		return m
	},
	// A2 -- BGP Hijacking / Sybil: attacker peers share few subnets,
	// collapsing diversity while still presenting as individually clean peers.
	"A2_BGPHijackSybil": func(rng *rand.Rand, p types.Params, intensity float64) *types.PeripheralMetrics {
		m := benignSnapshot(rng, p)
		if p.MinSubnetDiversity > 0 {
			reduced := float64(p.MinSubnetDiversity) * (1 - intensity)
			m.SubnetDiversity = uint64(reduced)
		}
		m.CleanPeers += uint64(intensity * 15)
		return m
	},
	// A3 -- Churn-based Rotation: constant disconnect/reconnect avoids
	// tenure detection while peer count looks unchanged (snapshots can't
	// see churn).
	"A3_ChurnBasedRotation": func(rng *rand.Rand, p types.Params, intensity float64) *types.PeripheralMetrics {
		m := benignSnapshot(rng, p)
		m.PeerChurnRate += uint64(intensity * float64(p.MaxChurnRate+20))
		m.AvgPeerTenure = uint64(float64(m.AvgPeerTenure) * (1 - intensity*0.95))
		return m
	},
	// A4 -- Relay Node Attack: an intermediary relay adds latency only;
	// peer count, subnet spread, and churn unchanged.
	"A4_RelayNodeAttack": func(rng *rand.Rand, p types.Params, intensity float64) *types.PeripheralMetrics {
		m := benignSnapshot(rng, p)
		m.PeerLatency += uint64(intensity * float64(p.MaxPeerLatency+150))
		return m
	},
}

// productionScaleParams is NOT DefaultParams(): DefaultParams's smallest
// TLC-verified thresholds make every synthetic attack trivially separable
// (degenerate 0%/100% FPR/FNR); this uses realistic values instead.
func productionScaleParams() types.Params {
	p := types.DefaultParams()
	p.MinSubnetDiversity = 8
	p.MinAnchorPeers = 4
	p.MinPeers = 20
	p.MaxChurnRate = 5
	p.MinAvgTenure = 300
	p.MaxPeerLatency = 200
	return p
}

type detectorResult struct {
	ParamSet         string
	Attack           string
	Detector         string
	FPR              float64 // benign snapshots flagged unhealthy
	FNR              float64 // full-intensity attack snapshots flagged healthy
	DetectionDelay   float64 // avg. #snapshots (of 10, ramping intensity 0.1..1.0) until first flagged unhealthy; -1 if never
	DetectionDelayOK int     // count of ramp trials that were ever detected (out of trials)
}

func runP2PDetectorComparison(t *testing.T, paramSetName string, p types.Params, attack string, gen attackGenerator, detectorName string, healthy func(*types.PeripheralMetrics, types.Params) bool) detectorResult {
	t.Helper()
	rng := rand.New(rand.NewSource(42)) // fixed seed: reproducible synthetic trials

	falsePositives := 0
	for i := 0; i < p2pTrials; i++ {
		m := benignSnapshot(rng, p)
		if !healthy(m, p) {
			falsePositives++
		}
	}

	falseNegatives := 0
	for i := 0; i < p2pTrials; i++ {
		m := gen(rng, p, 1.0) // full-intensity attack
		if healthy(m, p) {
			falseNegatives++
		}
	}

	// Detection delay: ramp intensity 0.1..1.0 over 10 synthetic snapshots,
	// find the first snapshot the detector flags unhealthy.
	const rampSteps = 10
	const delayTrials = 500
	totalDelay := 0.0
	detected := 0
	for i := 0; i < delayTrials; i++ {
		for step := 1; step <= rampSteps; step++ {
			intensity := float64(step) / rampSteps
			m := gen(rng, p, intensity)
			if !healthy(m, p) {
				totalDelay += float64(step)
				detected++
				break
			}
		}
	}
	avgDelay := -1.0
	if detected > 0 {
		avgDelay = totalDelay / float64(detected)
	}

	return detectorResult{
		ParamSet:         paramSetName,
		Attack:           attack,
		Detector:         detectorName,
		FPR:              float64(falsePositives) / float64(p2pTrials),
		FNR:              float64(falseNegatives) / float64(p2pTrials),
		DetectionDelay:   avgDelay,
		DetectionDelayOK: detected,
	}
}

func TestE4_P2PDetectorComparison(t *testing.T) {
	paramSets := map[string]types.Params{
		"default_verification_config": types.DefaultParams(),
		"production_scale":            productionScaleParams(),
	}

	var results []detectorResult
	for setName, p := range paramSets {
		for attack, gen := range p2pAttackGenerators {
			results = append(results, runP2PDetectorComparison(t, setName, p, attack, gen, "peer_count_only", peerCountOnlyHealthy))
			results = append(results, runP2PDetectorComparison(t, setName, p, attack, gen, "tri_interface", types.IsP2PQualityHealthy))
		}
	}

	require.NoError(t, os.MkdirAll("results", 0o755))
	path := filepath.Join("results", "e4_p2p_detector_comparison.csv")
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()
	_ = w.Write([]string{"param_set", "attack", "detector", "fpr", "fnr", "avg_detection_delay_snapshots", "detected_out_of_trials"})
	for _, r := range results {
		_ = w.Write([]string{
			r.ParamSet, r.Attack, r.Detector,
			strconv.FormatFloat(r.FPR, 'f', 4, 64),
			strconv.FormatFloat(r.FNR, 'f', 4, 64),
			strconv.FormatFloat(r.DetectionDelay, 'f', 2, 64),
			strconv.Itoa(r.DetectionDelayOK),
		})
		t.Logf("[%s] %s / %s: FPR=%.4f FNR=%.4f avg_delay=%.2f (detected %d/500 ramp trials)",
			r.ParamSet, r.Attack, r.Detector, r.FPR, r.FNR, r.DetectionDelay, r.DetectionDelayOK)
	}

	// Sanity: tri-interface FNR must never exceed peer-count-only's for any
	// attack it's designed to catch, under either parameter set.
	byKey := map[string]detectorResult{}
	for _, r := range results {
		byKey[r.ParamSet+"/"+r.Attack+"/"+r.Detector] = r
	}
	for setName := range paramSets {
		for attack := range p2pAttackGenerators {
			triFNR := byKey[setName+"/"+attack+"/tri_interface"].FNR
			peerFNR := byKey[setName+"/"+attack+"/peer_count_only"].FNR
			require.LessOrEqualf(t, triFNR, peerFNR, "[%s] %s: tri-interface FNR (%.4f) must not exceed peer-count-only FNR (%.4f)", setName, attack, triFNR, peerFNR)
		}
	}
}

// evasionThreshold binary-searches for the intensity at which a detector
// stops flagging gen's output unhealthy at least half the time -- an
// oracle-guided adaptive search, not a fixed a-priori ramp. Returns the
// highest intensity at which the adversary still evades.
func evasionThreshold(rng *rand.Rand, p types.Params, gen attackGenerator, healthy func(*types.PeripheralMetrics, types.Params) bool, probesPerLevel int) float64 {
	evades := func(intensity float64) bool {
		evadeCount := 0
		for i := 0; i < probesPerLevel; i++ {
			if healthy(gen(rng, p, intensity), p) {
				evadeCount++
			}
		}
		return evadeCount*2 > probesPerLevel // majority still reads healthy
	}

	lo, hi := 0.0, 1.0 // invariant: evades(lo) == true (or lo==0), evades(hi) == false (or hi==1 and still evades)
	if !evades(lo) {
		return 0.0 // detector catches even a near-benign snapshot at this param set
	}
	if evades(hi) {
		return 1.0 // adversary evades even at full intensity -- detector has no ceiling
	}
	for i := 0; i < 12; i++ { // ~2^-12 resolution
		mid := (lo + hi) / 2
		if evades(mid) {
			lo = mid
		} else {
			hi = mid
		}
	}
	return lo
}

// TestE4_AdaptiveAdversary closes the "needs an adaptive adversary" gap:
// runP2PDetectorComparison's FPR/FNR use a fixed a-priori attack shape;
// here the search uses the detector's own live pass/fail feedback.
// Reports each detector's evasion threshold per attack -- lower is better.
func TestE4_AdaptiveAdversary(t *testing.T) {
	rng := rand.New(rand.NewSource(4242))
	p := productionScaleParams() // DefaultParams' thresholds are trivially separable (see its own doc)

	require.NoError(t, os.MkdirAll("results", 0o755))
	path := filepath.Join("results", "e4_adaptive_adversary.csv")
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	_ = w.Write([]string{"attack", "detector", "evasion_threshold_intensity"})

	type thresholds struct{ peerCount, triInterface float64 }
	byAttack := map[string]thresholds{}
	for attack, gen := range p2pAttackGenerators {
		peerT := evasionThreshold(rng, p, gen, peerCountOnlyHealthy, 20)
		triT := evasionThreshold(rng, p, gen, types.IsP2PQualityHealthy, 20)
		byAttack[attack] = thresholds{peerCount: peerT, triInterface: triT}
		_ = w.Write([]string{attack, "peer_count_only", strconv.FormatFloat(peerT, 'f', 4, 64)})
		_ = w.Write([]string{attack, "tri_interface", strconv.FormatFloat(triT, 'f', 4, 64)})
		t.Logf("[adaptive] %s: peer_count_only evades up to intensity=%.4f, tri_interface evades up to intensity=%.4f",
			attack, peerT, triT)
	}

	// Same claim as the FNR sanity check above, restated against an
	// adaptive adversary's own best evasion point rather than a single
	// fixed-intensity sample: tri-interface must not be easier to evade.
	for attack, th := range byAttack {
		require.LessOrEqualf(t, th.triInterface, th.peerCount,
			"%s: tri-interface's evasion threshold (%.4f) must not exceed peer-count-only's (%.4f) -- "+
				"an adaptive adversary must not find tri-interface easier to evade", attack, th.triInterface, th.peerCount)
	}
}

// intensityShape maps a normalized ramp step in [0,1] to an attack
// intensity in [0,1] -- TestE4_DistributionSensitivity's 3 shapes.
type intensityShape struct {
	name string
	f    func(step float64) float64
}

var p2pIntensityShapes = []intensityShape{
	// The existing runP2PDetectorComparison ramp: constant-rate escalation.
	{"linear", func(step float64) float64 { return step }},
	// Front-loaded: attacker escalates aggressively early, testing whether
	// detectors tuned for a gradual ramp miss a fast-onset attack.
	{"front_loaded", math.Sqrt},
	// Stealthy: attacker escalates slowly, spending most of the ramp near
	// benign, testing whether a slow creep evades longer than linear.
	{"stealthy_slow_ramp", func(step float64) float64 { return step * step }},
}

// TestE4_DistributionSensitivity closes the "sensitivity to distribution"
// gap: runP2PDetectorComparison always ramps linearly. Sweeps 3 differently-
// shaped intensity trajectories per attack/detector, so detection delay
// isn't reported for only the shape that looks best.
func TestE4_DistributionSensitivity(t *testing.T) {
	rng := rand.New(rand.NewSource(99))
	p := productionScaleParams()
	const rampSteps = 10
	const trialsPerShape = 300

	require.NoError(t, os.MkdirAll("results", 0o755))
	path := filepath.Join("results", "e4_distribution_sensitivity.csv")
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	_ = w.Write([]string{"attack", "detector", "shape", "avg_detection_delay_steps", "detected_out_of_trials"})

	detectors := map[string]func(*types.PeripheralMetrics, types.Params) bool{
		"peer_count_only": peerCountOnlyHealthy,
		"tri_interface":   types.IsP2PQualityHealthy,
	}

	for attack, gen := range p2pAttackGenerators {
		for detName, healthy := range detectors {
			for _, shape := range p2pIntensityShapes {
				totalDelay, detected := 0.0, 0
				for i := 0; i < trialsPerShape; i++ {
					for step := 1; step <= rampSteps; step++ {
						intensity := shape.f(float64(step) / rampSteps)
						if !healthy(gen(rng, p, intensity), p) {
							totalDelay += float64(step)
							detected++
							break
						}
					}
				}
				avgDelay := -1.0
				if detected > 0 {
					avgDelay = totalDelay / float64(detected)
				}
				_ = w.Write([]string{
					attack, detName, shape.name,
					strconv.FormatFloat(avgDelay, 'f', 2, 64),
					strconv.Itoa(detected),
				})
				t.Logf("[sensitivity] %s/%s/%s: avg_delay=%.2f steps (detected %d/%d)",
					attack, detName, shape.name, avgDelay, detected, trialsPerShape)
			}
		}
	}
}

// TestE4_CorrelatedDomainFailure closes the "correlated sensor failures"
// gap: both P2P detectors are pure functions of P2P fields alone, so their
// accuracy is structurally invariant to simultaneous BTC/DA degradation.
// Asserts that invariant holds for the real code, not just the spec.
func TestE4_CorrelatedDomainFailure(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	p := productionScaleParams()

	for attack, gen := range p2pAttackGenerators {
		for i := 0; i < 200; i++ {
			intensity := rng.Float64()
			isolated := gen(rng, p, intensity)
			// Same P2P fields as isolated -- field-by-field, never a struct
			// copy: *PeripheralMetrics embeds a sync.Mutex (CLAUDE.md), so
			// copying the struct value trips go vet's lock-copy check.
			correlated := &types.PeripheralMetrics{
				SubnetDiversity: isolated.SubnetDiversity,
				ActiveAnchors:   isolated.ActiveAnchors,
				CleanPeers:      isolated.CleanPeers,
				PeerChurnRate:   isolated.PeerChurnRate,
				AvgPeerTenure:   isolated.AvgPeerTenure,
				PeerLatency:     isolated.PeerLatency,
			}
			// Layer independent BTC/DA degradation on top of the SAME P2P
			// attack snapshot -- a real multi-vector adversary manipulating
			// its own peers while BTC/DA are also (independently) unhealthy.
			correlated.BtcGap = p.SovereignThreshold + uint64(rng.Intn(10))
			correlated.IsBtcSpvFailed = true
			correlated.DaGap = p.DAThreshold + uint64(rng.Intn(10))
			correlated.IsDasFailed = true

			require.Equal(t, types.IsP2PQualityHealthy(isolated, p), types.IsP2PQualityHealthy(correlated, p),
				"%s: P2P detector's verdict must not change when BTC/DA are ALSO degraded (cross-domain leakage would be a real bug)", attack)
			require.Equal(t, peerCountOnlyHealthy(isolated, p), peerCountOnlyHealthy(correlated, p),
				"%s: peer-count-only detector's verdict must not change when BTC/DA are ALSO degraded", attack)
		}
	}

	// The full FSM-level predicate, by contrast, MUST catch a P2P attack
	// even when BTC/DA happen to be healthy at the same time.
	for attack, gen := range p2pAttackGenerators {
		m := gen(rng, p, 1.0) // full-intensity P2P attack
		m.BtcGap = 0
		m.DaGap = 0
		require.Falsef(t, types.IsHealthyCondition(m, p),
			"%s: IsHealthyCondition must reflect the P2P attack even though BTC/DA read healthy", attack)
	}
}
