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
	// IsKDeep): Bitcoin confirmations required before a submitted checkpoint
	// counts as anchored (x/vigilante.AnchorTracker, Phase 7). Not part of
	// the FSM THRESHOLDS block below -- it governs the anchor-submission
	// mechanism, not the FSM's warning/critical predicates.
	KDeepFinality uint64
}

// DefaultParams returns this repo's genesis-constant proposal for a real
// (non-TLC-tractability-bounded) testnet-scale deployment -- see the
// research writeup this was derived from (constants proposal session,
// grounded in spec/core/*.cfg + live measurements against the real 4-node
// Docker testnet + bitcoin-node01/celestia-bridge). NOT the same as
// spec/core/MC_StressC1Safety.cfg's THRESHOLDS block anymore (SuspiciousThreshold=1,
// SovereignThreshold=2, DAThreshold=1, HysteresisWait=1, MaxSuspiciousTime=1,
// MinAnchorPeers=1, MinPeers=2) -- those are TLC's SMALLEST formally-verified
// config for state-space tractability, not a viable runtime default.
//
// Real, live-network bug this fixes: with the old SovereignThreshold=2 ==
// KDeepFinality=2, the FSM could NEVER reach ANCHORED with a real
// AnchorTracker wired in -- vigilante.AnchorTracker.MaybeSubmit only reports
// h_btc_anchored once a submission reaches exactly KDeepFinality
// confirmations (x/vigilante/anchor.go), so btc_gap sits at
// [KDeepFinality, KDeepFinality+1] in steady state EVEN WHEN BITCOIN IS
// PERFECTLY HEALTHY -- IsBTCGapSovereign (btc_gap >= SovereignThreshold) was
// therefore true almost always. Confirmed live against the real 4-node
// testnet: FSM state was stuck at SOVEREIGN across an entire session despite
// 100+ real, confirmed OP_RETURN anchor transactions on bitcoin-node01. This
// is not a TLC/spec bug -- EngramFSM.tla's abstract BTCNormalUpdate allows
// h_btc_anchored' = h_btc_current' (zero lag, nondeterministic), so TLC's
// liveness checks still find ANCHORED-reaching runs; the concrete
// AnchorTracker policy is strictly more conservative (always waits the full
// KDeepFinality depth) than the abstract model's best case, which is safe
// but exposes exactly this gap once SovereignThreshold isn't sized with a
// margin above KDeepFinality.
//
// SuspiciousThreshold/SovereignThreshold are now sized as KDeepFinality plus
// a margin covering AnchorTracker's real submit-then-wait-kDeepFinality-confirmations
// cycle (observed steady-state gap band of [KDeepFinality, KDeepFinality+1]
// against a 20s/block regtest miner loop); DAThreshold is sized off the
// Celestia/Engram block-time ratio (~12s vs ~9s observed) plus
// da.Publisher.MaybePublish's own up-to-one-Celestia-block async latency
// (both da_gap's real components, not directly measured live this session --
// Query.State's metrics field is documented stale/not committed, see
// preblock.go's NewPreBlocker doc -- flagged for live verification once the
// data-collection scripts land). HysteresisWait is kept small deliberately:
// docs/EXPERIMENT.md's E5 (real, measured) found anchored_uptime decreases
// MONOTONICALLY as HysteresisWait grows under sustained noise, with no
// interior sweet spot -- HysteresisWait only gates RECOVERING -> ANCHORED
// entry, never protects ANCHORED once reached (ANCHORED has no hysteresis of
// its own, and SUSPICIOUS -> ANCHORED is unconditional on IsHealthyCondition
// alone), so a larger value only delays recovery without buying stability.
// P2P thresholds reuse tests/e2e/p2p_detector_comparison_test.go's
// productionScaleParams() (already empirically validated: FPR=0%/FNR=0%
// against all 4 synthetic eclipse/Sybil attack scenarios, E4 real data),
// scaled down for this N=4 testnet's actual validator count rather than a
// larger production network's.
func DefaultParams() Params {
	return Params{
		// KDeepFinality: Bitcoin reorg-safety depth. 2 is a fast-iteration
		// regtest value (this repo's own bitcoin_miner_loop.sh mines steadily
		// every 20s); a mainnet deployment should use something closer to the
		// industry-standard 6-confirmation (~1h) settlement depth.
		KDeepFinality: 2,
		// btc_gap thresholds: KDeepFinality + margin, NOT close to
		// KDeepFinality itself -- see doc above. Comfortably above the
		// observed [2,3] steady-state band under this testnet's mining cadence.
		SuspiciousThreshold: 5,
		SovereignThreshold:  8,
		// da_gap threshold: margin above the observed ~1.3 Engram-blocks-per-
		// Celestia-block ratio plus async publish latency; not yet cross-checked
		// against a live da_gap reading (see doc above).
		DAThreshold: 4,
		// Hysteresis: deliberately small per E5's real finding (see doc
		// above) -- NOT tuned up looking for a "sweet spot" that doesn't exist.
		HysteresisWait: 2,
		// Gray-failure timeout: tolerate brief P2P/DA-only warning noise
		// without forcing a circuit-break, while still bounding how long the
		// system can sit in ambiguous SUSPICIOUS state. Reasoned default, not
		// swept/measured -- a good target for a future E5-style sweep.
		MaxSuspiciousTime: 24,
		// P2P thresholds: productionScaleParams() baseline (E4-validated),
		// MinAnchorPeers/MinPeers scaled down for N=4,f=1 (this testnet's
		// actual Byzantine-fault assumption per spec/core/MC_StressC1*.cfg) --
		// MinAnchorPeers=1 (the old default) meant losing a SINGLE anchor peer
		// already trips total-isolation critical; 2 tolerates one loss.
		MinPeers:           3,
		MinSubnetDiversity: 8,
		MinAnchorPeers:     2,
		MaxChurnRate:       5,
		MinAvgTenure:       300, // seconds (vanillaP2PHealthAdapter's real unit)
		// PeerLatency has no real RTT measurement yet on the vanilla p2p.Switch
		// transport (vanillaP2PHealthAdapter always reports 0, cmd/engramd/main.go) --
		// this threshold is inert defense today, kept at the E4-validated value
		// so it's already correctly sized once real RTT wiring lands.
		MaxPeerLatency:  200, // milliseconds
		MaxIgnoreRounds: 1,
	}
}
