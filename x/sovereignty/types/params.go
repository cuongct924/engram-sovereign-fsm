package types

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
		// Reasoned default, not measured -- candidate for a future E5-style sweep.
		MaxSuspiciousTime: 24,
		// productionScaleParams() baseline (E4-validated), MinAnchorPeers/
		// MinPeers scaled down for this testnet's N=4,f=1 assumption.
		MinPeers: 3,
		// 1, not productionScaleParams()'s 8: all 4 validators share one
		// Docker /24 subnet here, so real SubnetDiversity can never exceed
		// 1 regardless of network health. Raise toward 8 for a genuine
		// multi-subnet/multi-region deployment.
		MinSubnetDiversity: 1,
		MinAnchorPeers:     2,
		MaxChurnRate:       5,
		MinAvgTenure:       300, // seconds
		MaxPeerLatency:     200, // milliseconds (real RTT via p2p.Peer.RTT())
		MaxIgnoreRounds:    1,
		// Must exceed the 4 known-honest same-subnet validators (see
		// MinSubnetDiversity above) or the ingress filter would reject
		// their own mesh connections, not just an attacker's.
		MaxPeersPerSubnet: 8,
	}
}
