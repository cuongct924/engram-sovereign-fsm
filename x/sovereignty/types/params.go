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

	// DownHysteresisThreshold gates ANCHORED->SUSPICIOUS and
	// RECOVERING->SOVEREIGN on UnhealthyStreak+1 reaching this value (E5's
	// flapping fix). 1 = pre-fix immediate-demote behavior.
	DownHysteresisThreshold uint64

	// MaxDownHysteresisThreshold caps RECOVERING's backoff-doubled
	// down-hysteresis threshold -- without it, repeated regressions would
	// grow the grace period into a liveness risk. Must be >= DownHysteresisThreshold.
	MaxDownHysteresisThreshold uint64

	// SuspiciousHysteresisWait gates SUSPICIOUS->ANCHORED on
	// SuspiciousSafeBlocks+1 consecutive healthy blocks (Gray Failure
	// Arbitrage fix). 1 = pre-fix immediate-exit behavior.
	SuspiciousHysteresisWait uint64

	// KDeepFinality is K_DEEP_FINALITY (spec/core/EngramConsensus.tla's
	// IsKDeep): BTC confirmations before a submission counts as anchored.
	KDeepFinality uint64

	// MaxUnprovenTailBlocks bounds how far RealProofSubmittedHeight may trail
	// the tip and still validate -- a bounded-gap relax of an exact tip match.
	MaxUnprovenTailBlocks uint64

	// MaxPeersPerSubnet bounds same-subnet peers before FilterPeerByAddr
	// rejects -- the active counterpart to the passive SubnetDiversity.
	MaxPeersPerSubnet uint64

	// MaxSuspiciousForcedTxQueue caps ForcedTxQueue admission while
	// SUSPICIOUS (app/ante.go). Concrete-only, no spec line.
	MaxSuspiciousForcedTxQueue uint64
}

// DefaultParams returns this repo's tuned defaults for the real N=4 Docker
// testnet -- NOT spec/core/MC_StressC1Safety.cfg's THRESHOLDS block (that's
// TLC's tractable config, not a viable runtime default).
func DefaultParams() Params {
	return Params{
		// Regtest reorg-safety depth; mainnet ~6 confirmations.
		KDeepFinality: 2,
		// 2x the real steady-state proof-tail gap (max 4, under N_MAX=256).
		MaxUnprovenTailBlocks: 8,
		// Must clear the healthy [KDeepFinality, KDeepFinality+1] btc_gap band.
		SuspiciousThreshold: 5,
		SovereignThreshold:  8,
		// Sized off the Engram:Celestia block-time ratio plus DA submit latency.
		DAThreshold: 30,
		// Deliberately small -- E5 found uptime falls as HysteresisWait grows.
		HysteresisWait: 2,
		// Smallest value granting a genuine 1-block grace period.
		DownHysteresisThreshold: 2,
		// 8 (=2*2^2): allows 2 backoff doublings (2->4->8) before capping.
		MaxDownHysteresisThreshold: 8,
		// Same reasoning as HysteresisWait -- smallest value granting a
		// genuine 1-block grace period.
		SuspiciousHysteresisWait: 2,
		// Reasoned default, not measured -- candidate for an E5-style sweep.
		MaxSuspiciousTime: 24,
		// productionScaleParams() baseline (E4-validated), scaled for N=4,f=1.
		MinPeers: 3,
		// 2, not productionScaleParams()'s 8: healthy SubnetDiversity is 3
		// (one peer per pairwise-link subnet), so 2 tolerates one link down.
		MinSubnetDiversity: 2,
		MinAnchorPeers:     2,
		MaxChurnRate:       5,
		MinAvgTenure:       300, // seconds
		MaxPeerLatency:     200, // milliseconds (real RTT via p2p.Peer.RTT())
		MaxIgnoreRounds:    1,
		// Headroom only needed for engram-net-side traffic (attacker swarm,
		// reanchoring prover), not co-located peers.
		MaxPeersPerSubnet: 8,
		// Reasoned default -- candidate for a future E8-style flood-vs-
		// censorship-resistance sweep, not run by this change.
		MaxSuspiciousForcedTxQueue: 8,
	}
}

// Validate enforces the cross-field constraints documented on DefaultParams,
// called from InitChain on the genesis params -- a bad override fails at
// genesis instead of producing unreachable states or self-locking ingress.
func (p Params) Validate() error {
	if p.KDeepFinality == 0 {
		return fmt.Errorf("params: KDeepFinality must be >= 1 (0 confirmations defeats reorg safety)")
	}
	// btc_gap sits at [KDeepFinality, KDeepFinality+1] even when healthy, so
	// both thresholds must clear that band or the breaker trips spuriously.
	floor := p.KDeepFinality + 1
	if p.SuspiciousThreshold <= floor {
		return fmt.Errorf("params: SuspiciousThreshold (%d) must be > KDeepFinality+1 (%d)", p.SuspiciousThreshold, floor)
	}
	if p.SovereignThreshold <= floor {
		return fmt.Errorf("params: SovereignThreshold (%d) must be > KDeepFinality+1 (%d)", p.SovereignThreshold, floor)
	}
	// IsWarningCondition's band is [SuspiciousThreshold, SovereignThreshold)
	// -- equal values make SUSPICIOUS unreachable.
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

// ToGenesisParams converts to the genesis wire format (genesis.proto) -- see
// that message's doc for why genesis, not an env var, configures these.
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
		MaxSuspiciousForcedTxQueue: p.MaxSuspiciousForcedTxQueue,
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
		MaxSuspiciousForcedTxQueue: gp.MaxSuspiciousForcedTxQueue,
	}
}
