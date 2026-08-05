// E4 -- P2P Eclipse/Sybil Detection (docs/EXPERIMENT.md's E4, Table 6).
//
// Live Pumba/Docker chaos injection against a real multi-node network is not
// available in this environment (no Docker daemon running -- see CLAUDE.md's
// M7 status). Instead, this is a synthetic Monte Carlo comparison of the real
// tri-interface profiler (types.IsP2PQualityHealthy, spec/core/EngramFSM.tla:
// 76-81) against a naive peer-count-only baseline (CometBFT's default: just
// CleanPeers), driven by randomized peer-snapshot generators for each of the
// 4 attack scenarios in docs/EXPERIMENT.md's E4 table. This measures real
// FPR/FNR/detection-delay of the two REAL detector predicates against
// synthetic (not live-network) data -- the methodology is documented here so
// results aren't mistaken for a live-network measurement.
package e2e

import (
	"encoding/csv"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
)

const p2pTrials = 2000 // Monte Carlo trials per (attack, detector) cell

// peerCountOnlyHealthy mirrors a naive CometBFT-style detector: only look at
// the raw clean-peer count, ignoring subnet diversity, anchors, churn,
// tenure, and latency.
func peerCountOnlyHealthy(m *types.PeripheralMetrics, p types.Params) bool {
	return m.CleanPeers >= p.MinPeers
}

// benignSnapshot generates a randomized but genuinely healthy peer snapshot
// (small natural jitter around DefaultParams' minimums).
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
	// A1 -- Peer Slot Exhaustion: attacker floods connection slots with
	// junk peers that aren't individually blacklisted (CleanPeers stays
	// high/normal) but crowd out real anchor peers and never stay connected
	// long (low tenure, high churn).
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
	// A2 -- BGP Hijacking / Sybil: attacker-controlled peers share a small
	// number of subnets/ASNs, collapsing subnet diversity while still
	// presenting as individually clean, well-connected peers.
	"A2_BGPHijackSybil": func(rng *rand.Rand, p types.Params, intensity float64) *types.PeripheralMetrics {
		m := benignSnapshot(rng, p)
		if p.MinSubnetDiversity > 0 {
			reduced := float64(p.MinSubnetDiversity) * (1 - intensity)
			m.SubnetDiversity = uint64(reduced)
		}
		m.CleanPeers += uint64(intensity * 15)
		return m
	},
	// A3 -- Churn-based Rotation: attacker continuously disconnects/
	// reconnects to avoid tenure-based detection; instantaneous peer COUNT
	// looks unchanged (a snapshot-only counter can't see churn at all).
	"A3_ChurnBasedRotation": func(rng *rand.Rand, p types.Params, intensity float64) *types.PeripheralMetrics {
		m := benignSnapshot(rng, p)
		m.PeerChurnRate += uint64(intensity * float64(p.MaxChurnRate+20))
		m.AvgPeerTenure = uint64(float64(m.AvgPeerTenure) * (1 - intensity*0.95))
		return m
	},
	// A4 -- Relay Node Attack: an intermediary relay adds latency without
	// changing peer count, subnet spread, or churn at all.
	"A4_RelayNodeAttack": func(rng *rand.Rand, p types.Params, intensity float64) *types.PeripheralMetrics {
		m := benignSnapshot(rng, p)
		m.PeerLatency += uint64(intensity * float64(p.MaxPeerLatency+150))
		return m
	},
}

// productionScaleParams is NOT DefaultParams() -- DefaultParams (see
// x/sovereignty/types/params.go) intentionally uses the smallest thresholds
// formally verified by TLC (tiny integers, for state-space tractability),
// which makes every synthetic attack signature below trivially separable
// from benign traffic (a degenerate, uninformative 0%/100% FPR/FNR split).
// This variant uses larger, more realistic threshold values so the
// comparison actually says something about detector sensitivity, not just
// "the verification-config thresholds are tiny." Both are reported.
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

	// Sanity: the tri-interface profiler must never do WORSE than peer-count-only
	// on false negatives for any attack it's specifically designed to catch,
	// under either parameter set.
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
