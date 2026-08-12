package types

import "fmt"

// Params mirrors the CONSTANTS declared in spec/core/EngramFSM.tla.
type Params struct {
	SuspiciousThreshold uint64 // BTC gap threshold for Gray Failure warning
	SovereignThreshold  uint64 // BTC gap threshold for Hard Failure (circuit-break)
	DAThreshold         uint64 // da_gap threshold for DA health
	HysteresisWait      uint64 // safe_blocks required before RECOVERING -> ANCHORED
	MinPeers            uint64 // minimum clean (non-blacklisted) peers required
	MinSubnetDiversity  uint64 // minimum distinct subnets required
	MinAnchorPeers      uint64 // minimum active anchor/bootstrap peers required
	MaxChurnRate        uint64 // maximum allowed peer disconnects/reconnects per epoch
	MinAvgTenure        uint64 // minimum average peer tenure
	MaxPeerLatency      uint64 // maximum allowed peer latency
	MaxSuspiciousTime   uint64 // max blocks tolerated in SUSPICIOUS before escalating to SOVEREIGN
	MaxIgnoreRounds     uint64 // rounds a forced tx can be ignored before IsCensoring trips

	// DownHysteresisThreshold gates the downward transitions ANCHORED->SUSPICIOUS
	// and RECOVERING->SOVEREIGN on a non-critical (warning-only) reading: it
	// only fires once UnhealthyStreak+1 reaches this value, absorbing shorter
	// runs of noise instead of demoting on the very first bad block (E5's
	// flapping fix, docs/EXPERIMENT.md). A critical condition always bypasses
	// this and demotes immediately. 1 reproduces the pre-fix immediate-demote
	// behavior exactly (streak+1 >= 1 is always true).
	DownHysteresisThreshold uint64

	// MaxDownHysteresisThreshold caps RECOVERING's exponentially-backed-off
	// down-hysteresis threshold (EffectiveDownHysteresisThreshold,
	// circuit_breaker.go): a repeated, precisely-timed flapping attack
	// (docs/EXPERIMENT.md) doubles the effective grace period on every
	// consecutive RECOVERING->SOVEREIGN regression, up to this ceiling --
	// without a cap, unbounded regressions would grow the grace period
	// forever, itself a liveness risk after enough genuine network hiccups
	// over a long run, not just an attacker. Must be >= DownHysteresisThreshold.
	MaxDownHysteresisThreshold uint64

	// SuspiciousHysteresisWait gates SUSPICIOUS's exit to ANCHORED on
	// SuspiciousSafeBlocks+1 consecutive healthy blocks, instead of a single
	// one (Gray Failure Arbitrage fix, docs/EXPERIMENT.md): without this, an
	// attacker who can nudge sensors healthy for exactly one block right
	// before MaxSuspiciousTime resets suspicious_duration to 0 and repeats
	// forever, never letting the network escalate to SOVEREIGN. 1 reproduces
	// the pre-fix immediate-exit behavior exactly (streak+1 >= 1 always holds).
	SuspiciousHysteresisWait uint64

	// KDeepFinality is K_DEEP_FINALITY (spec/core/EngramConsensus.tla's
	// IsKDeep): Bitcoin confirmations required before AnchorTracker treats a
	// submission as anchored. Governs anchor submission, not FSM thresholds.
	KDeepFinality uint64

	// MaxUnprovenTailBlocks bounds how far RealProofSubmittedHeight may
	// trail the current tip and still count as valid
	// (refreshReanchoringProofValid). Every block up to
	// tip-MaxUnprovenTailBlocks still requires a real verified proof
	// (SubmitRecoveryProof) -- this only relaxes an exact tip match to a
	// bounded gap, mirroring KDeepFinality's own "K-deep, not exact tip"
	// pattern.
	MaxUnprovenTailBlocks uint64

	// MaxPeersPerSubnet bounds same-/24 (or /48 IPv6) connected peers before
	// FilterPeerByAddr (peer_filter.go) rejects further connections from
	// that subnet outright -- the active counterpart to the passive
	// SubnetDiversity metric IsP2PQualityHealthy reads.
	MaxPeersPerSubnet uint64
}

// DefaultParams returns this repo's tuned defaults for the real N=4 Docker
// testnet -- NOT spec/core/MC_StressC1Safety.cfg's THRESHOLDS block, which
// is TLC's smallest tractable config for state-space size, not a viable
// runtime default.
func DefaultParams() Params {
	return Params{
		// Regtest reorg-safety depth; mainnet should use ~6 confirmations.
		KDeepFinality: 2,
		// 2x the real observed steady-state proof-tail gap (max 4, under the
		// N_MAX=256 circuit) -- revisit if proof latency or N_MAX changes.
		MaxUnprovenTailBlocks: 8,
		// KDeepFinality + margin: AnchorTracker only reports h_btc_anchored
		// once a submission reaches KDeepFinality confirmations, so btc_gap
		// sits at [KDeepFinality, KDeepFinality+1] even when Bitcoin is
		// healthy -- these must clear that band, not sit close to it.
		SuspiciousThreshold: 5,
		SovereignThreshold:  8,
		// Sized off the real Engram:Celestia block-time ratio plus
		// da.Publisher's async submit latency -- cadence-dependent, revisit
		// if Engram's block time changes materially.
		DAThreshold: 30,
		// Kept deliberately small: E5 (docs/EXPERIMENT.md) found
		// anchored_uptime decreases monotonically as HysteresisWait grows
		// under sustained noise, with no interior sweet spot.
		HysteresisWait: 2,
		// Smallest value that actually grants a 1-block grace period (see
		// the field doc) -- candidate for the same E5-style sweep.
		DownHysteresisThreshold: 2,
		// 8 (=2*2^2): allows 2 backoff doublings (2 -> 4 -> 8) before
		// capping -- enough to meaningfully harden against a repeated
		// attacker without letting a long run of genuine faults strand
		// RECOVERING behind an ever-growing wall.
		MaxDownHysteresisThreshold: 8,
		// Same reasoning as HysteresisWait above -- smallest value granting a
		// genuine 1-block grace period, candidate for the same E5-style sweep.
		SuspiciousHysteresisWait: 2,
		// Reasoned default, not measured -- candidate for a future E5-style sweep.
		MaxSuspiciousTime: 24,
		// productionScaleParams() baseline (E4-validated), MinAnchorPeers/
		// MinPeers scaled down for this testnet's N=4,f=1 assumption.
		MinPeers: 3,
		// 2, not productionScaleParams()'s 8: each validator reaches the
		// other 3 over a dedicated pairwise-link subnet (docker/
		// engram-validator-cluster.yml, cmd/engramd/main.go's
		// pairwiseLinkPeerIP), so a healthy node's real SubnetDiversity is
		// 3 -- 2 leaves one peer's worth of tolerance for a single link
		// being down/reconnecting, not the trivially-always-true floor of 1.
		MinSubnetDiversity: 2,
		MinAnchorPeers:     2,
		MaxChurnRate:       5,
		MinAvgTenure:       300, // seconds
		MaxPeerLatency:     200, // milliseconds (real RTT via p2p.Peer.RTT())
		MaxIgnoreRounds:    1,
		// Each validator's 3 honest peers now arrive on 3 SEPARATE /29
		// pairwise-link subnets (1 peer each), not one shared /24 -- this
		// threshold only needs headroom for engram-net-side traffic
		// (attacker swarm, reanchoring-prover), not co-located honest peers.
		MaxPeersPerSubnet: 8,
	}
}

// Validate enforces the cross-field constraints documented on
// DefaultParams above -- called from InitChain (app/app.go) on whatever
// genesis actually supplies, whether that's DefaultParams() or a
// genesis-time ENGRAM_PARAM_* override (cmd/engramd/main.go), so a bad
// override fails the chain at genesis rather than silently producing
// unreachable states or self-locking the ingress filter.
func (p Params) Validate() error {
	if p.KDeepFinality == 0 {
		return fmt.Errorf("params: KDeepFinality must be >= 1 (0 confirmations defeats reorg safety)")
	}
	// btc_gap sits at [KDeepFinality, KDeepFinality+1] even when Bitcoin is
	// healthy (AnchorTracker only reports h_btc_anchored once KDeepFinality
	// confirmations land) -- both thresholds must clear that band, or the
	// circuit breaker trips on a perfectly healthy chain.
	floor := p.KDeepFinality + 1
	if p.SuspiciousThreshold <= floor {
		return fmt.Errorf("params: SuspiciousThreshold (%d) must be > KDeepFinality+1 (%d)", p.SuspiciousThreshold, floor)
	}
	if p.SovereignThreshold <= floor {
		return fmt.Errorf("params: SovereignThreshold (%d) must be > KDeepFinality+1 (%d)", p.SovereignThreshold, floor)
	}
	// IsWarningCondition's band is [SuspiciousThreshold, SovereignThreshold)
	// (predicates.go) -- if SovereignThreshold doesn't exceed
	// SuspiciousThreshold, SUSPICIOUS is never reachable at all.
	if p.SovereignThreshold <= p.SuspiciousThreshold {
		return fmt.Errorf("params: SovereignThreshold (%d) must be > SuspiciousThreshold (%d)", p.SovereignThreshold, p.SuspiciousThreshold)
	}
	if p.DAThreshold == 0 {
		return fmt.Errorf("params: DAThreshold must be >= 1")
	}
	if p.MaxPeersPerSubnet == 0 {
		return fmt.Errorf("params: MaxPeersPerSubnet must be >= 1 (0 rejects every peer, including honest validators)")
	}
	if p.MaxDownHysteresisThreshold < p.DownHysteresisThreshold {
		return fmt.Errorf("params: MaxDownHysteresisThreshold (%d) must be >= DownHysteresisThreshold (%d)",
			p.MaxDownHysteresisThreshold, p.DownHysteresisThreshold)
	}
	return nil
}

// ToGenesisParams converts to the genesis wire format (GenesisParams,
// genesis.proto) -- see that message's doc for why genesis, not an env var
// read per-process, is the safe place to configure these.
func (p Params) ToGenesisParams() *GenesisParams {
	return &GenesisParams{
		SuspiciousThreshold:        p.SuspiciousThreshold,
		SovereignThreshold:         p.SovereignThreshold,
		DaThreshold:                p.DAThreshold,
		HysteresisWait:             p.HysteresisWait,
		MinPeers:                   p.MinPeers,
		MinSubnetDiversity:         p.MinSubnetDiversity,
		MinAnchorPeers:             p.MinAnchorPeers,
		MaxChurnRate:               p.MaxChurnRate,
		MinAvgTenure:               p.MinAvgTenure,
		MaxPeerLatency:             p.MaxPeerLatency,
		MaxSuspiciousTime:          p.MaxSuspiciousTime,
		MaxIgnoreRounds:            p.MaxIgnoreRounds,
		KDeepFinality:              p.KDeepFinality,
		MaxUnprovenTailBlocks:      p.MaxUnprovenTailBlocks,
		MaxPeersPerSubnet:          p.MaxPeersPerSubnet,
		DownHysteresisThreshold:    p.DownHysteresisThreshold,
		MaxDownHysteresisThreshold: p.MaxDownHysteresisThreshold,
		SuspiciousHysteresisWait:   p.SuspiciousHysteresisWait,
	}
}

// ToParams converts a genesis-supplied GenesisParams back to the runtime
// Params type. Returns DefaultParams() if gp is nil (genesis predates this
// field, or a test/harness never set it).
func (gp *GenesisParams) ToParams() Params {
	if gp == nil {
		return DefaultParams()
	}
	return Params{
		SuspiciousThreshold:        gp.SuspiciousThreshold,
		SovereignThreshold:         gp.SovereignThreshold,
		DAThreshold:                gp.DaThreshold,
		HysteresisWait:             gp.HysteresisWait,
		MinPeers:                   gp.MinPeers,
		MinSubnetDiversity:         gp.MinSubnetDiversity,
		MinAnchorPeers:             gp.MinAnchorPeers,
		MaxChurnRate:               gp.MaxChurnRate,
		MinAvgTenure:               gp.MinAvgTenure,
		MaxPeerLatency:             gp.MaxPeerLatency,
		MaxSuspiciousTime:          gp.MaxSuspiciousTime,
		MaxIgnoreRounds:            gp.MaxIgnoreRounds,
		KDeepFinality:              gp.KDeepFinality,
		MaxUnprovenTailBlocks:      gp.MaxUnprovenTailBlocks,
		MaxPeersPerSubnet:          gp.MaxPeersPerSubnet,
		DownHysteresisThreshold:    gp.DownHysteresisThreshold,
		MaxDownHysteresisThreshold: gp.MaxDownHysteresisThreshold,
		SuspiciousHysteresisWait:   gp.SuspiciousHysteresisWait,
	}
}
