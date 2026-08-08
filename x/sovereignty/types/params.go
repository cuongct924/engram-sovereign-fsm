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

	// MaxUnprovenTailBlocks bounds how far behind the current tip a real ZK
	// re-anchoring proof's checkpoint (RealProofSubmittedHeight) is allowed
	// to trail and still count as valid (sensors_refresh.go's
	// refreshReanchoringProofValid). NOT a bypass of the "RECOVERING ->
	// ANCHORED must go through a real verified proof" invariant -- every
	// block up to tip-MaxUnprovenTailBlocks still requires a real, verified
	// N-header proof (x/sovereignty/keeper/msg_server.go's
	// SubmitRecoveryProof); this only relaxes "the proof's checkpoint must
	// equal the tip EXACTLY" to "must be within this many blocks of it".
	//
	// Real liveness bug this fixes, found by actually running the rolling-
	// checkpoint prover against a live, fast-producing testnet: requiring
	// an EXACT match makes ANCHORED unreachable whenever block production
	// is faster than one full query+prove+submit round trip, since the tip
	// is a moving target that a real proof (built from a snapshot taken
	// before it lands) can only ever equal by coincidence -- confirmed
	// live, dozens of real proofs landing successfully (checkpoint
	// advancing every time) without ever once satisfying the exact-match
	// check, RECOVERING never completing despite the network being
	// genuinely healthy the whole time. This mirrors KDeepFinality's own
	// "accept once K confirmations deep, not literally at the current
	// tip" precedent already used elsewhere in this same file for Bitcoin
	// settlement, and is the same pattern production ZK-rollups use
	// (sequencer-confirmed "soft" state vs L1-proven "hard" state trailing
	// behind it by a bounded, continuously-shrinking backlog) -- not an
	// invented workaround.
	//
	// Originally set to N=4 (the circuit's fixed per-proof header count) on
	// the theoretical reasoning that a single proof can only ever advance
	// the checkpoint by N at a time, so a tighter bound wouldn't change
	// what's achievable. That theory was WRONG, found by actually
	// measuring the real steady-state gap against a live, continuously-
	// producing testnet after deploying it: the full query + witness-link
	// + nargo execute + bb prove + broadcast + confirm round trip takes
	// long enough in wall-clock time that ~9-10 new headers accumulate
	// during a SINGLE proof cycle, while each accepted proof only removes
	// N=4 -- so the gap oscillates around 9-10 in steady state (real
	// histogram from one run: 12/21 samples at exactly 9, none at or below
	// 4), meaning N=4 as the bound was structurally unsatisfiable, not just
	// unlucky. Set to 16 -- comfortable margin above the observed 9-10
	// steady-state band (covers normal variance) plus headroom for a
	// round-skip stall (observed up to 50s+) briefly widening the gap
	// further without permanently blocking the transition. Revisit if the
	// proof pipeline's wall-clock latency changes materially (e.g. if
	// prove_and_submit.sh's round trip is later optimized to be faster
	// than N blocks' worth of real time, at which point N=4 itself would
	// become achievable again).
	MaxUnprovenTailBlocks uint64

	// MaxPeersPerSubnet bounds how many currently-connected peers may share
	// the same /24 (IPv4) or /48 (IPv6) subnet (types.SubnetOf) before the
	// ingress peer filter (x/sovereignty/keeper/peer_filter.go's
	// FilterPeerByAddr, wired via baseapp.SetAddrPeerFilter) starts
	// rejecting further same-subnet connection attempts outright. This is
	// the ACTIVE counterpart to the passive SubnetDiversity metric
	// IsP2PQualityHealthy already reads: that metric only degrades the FSM
	// AFTER enough same-subnet peers have already connected (a Sybil/eclipse
	// condition already in effect); this stops the next one from connecting
	// at all, closing a real design-vs-implementation gap found by
	// re-reading this project's own early P2P design notes (an active
	// ingress filter was designed but never built -- only passive detection
	// existed until this field).
	MaxPeersPerSubnet uint64
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
// larger production network's -- EXCEPT MinSubnetDiversity, see its own
// field comment below: a real, structural bug in an earlier version of this
// function, found by actually running it against the live 4-node testnet
// and watching SOVEREIGN never transition to RECOVERING no matter how long
// BTC/DA stayed healthy.
func DefaultParams() Params {
	return Params{
		// KDeepFinality: Bitcoin reorg-safety depth. 2 is a fast-iteration
		// regtest value (this repo's own bitcoin_miner_loop.sh mines steadily
		// every 20s); a mainnet deployment should use something closer to the
		// industry-standard 6-confirmation (~1h) settlement depth.
		KDeepFinality: 2,
		// MaxUnprovenTailBlocks: 16 -- see the field's own doc on
		// x/sovereignty/types.Params for the full reasoning. NOT N=4 (tried
		// first, found live to be structurally unsatisfiable: the real
		// proof round trip takes long enough that the steady-state gap
		// oscillates around 9-10, never at or below 4).
		MaxUnprovenTailBlocks: 16,
		// btc_gap thresholds: KDeepFinality + margin, NOT close to
		// KDeepFinality itself -- see doc above. Comfortably above the
		// observed [2,3] steady-state band under this testnet's mining cadence.
		SuspiciousThreshold: 5,
		SovereignThreshold:  8,
		// da_gap threshold: WAS 4, sized off an assumed ~1.3
		// Engram-blocks-per-Celestia-block ratio that held when this was first
		// written -- a real bug, found live AFTER the Tolerance round-skip fix
		// (x/vigilante/verify.go) landed: removing the guaranteed round-skips
		// sped Engram block time up roughly 30-40x (from ~9-10s/block to
		// ~0.3s/block, confirmed live), while da.Publisher.MaybePublish's
		// underlying celestia-bridge blob.Submit call is still bound by
		// Celestia's own real ~12s block time -- so the Engram:Celestia block
		// ratio this threshold needs to absorb grew by the same 30-40x,
		// completely invalidating the old estimate. Confirmed via live debug
		// instrumentation: da_gap oscillated in a real [9,16] band once Engram
		// was running at its new (fixed) speed, with SUSPICIOUS/SOVEREIGN BTC
		// thresholds and P2P conditions all simultaneously healthy -- da_gap
		// alone was keeping IsHealthyCondition false. 30 covers the observed
		// band with real margin; this is fundamentally a function of the
		// current Engram block cadence, so it should be revisited again if
		// that cadence changes materially (e.g. real CometBFT timeout_commit
		// tuning, not just round-skip elimination).
		DAThreshold: 30,
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
		MinPeers: 3,
		// MinSubnetDiversity: WAS 8 (productionScaleParams()'s value,
		// copied verbatim without translating it for this testnet's real
		// topology) -- a real bug, found live: all 4 engram-nodeNN
		// containers sit on the SAME engram-net Docker bridge subnet
		// (172.28.0.0/24, docker/engram-validator-node0N.yml's
		// ipv4_address entries), so vanillaP2PHealthAdapter's real
		// SubnetDiversity computation (cmd/engramd/main.go, masks each
		// connected peer's IP to /24 and counts distinct results) can
		// only ever report 1 from any node's point of view, no matter how
		// healthy the network genuinely is -- MinSubnetDiversity=8 made
		// IsP2PQualityHealthy, and therefore IsHealthyCondition, and
		// therefore SOVEREIGN -> RECOVERING, structurally unreachable on
		// this deployment regardless of BTC/DA state. Confirmed by
		// running the 4-node testnet at MinSubnetDiversity=8 and
		// observing FSM state stay SOVEREIGN indefinitely despite a
		// healthy BTC anchor pipeline and clean AppHash convergence.
		// 1 is the correct value for THIS single-Docker-network
		// deployment (this metric only becomes meaningful once
		// validators genuinely span multiple L3 subnets/ASNs -- a real
		// multi-host or multi-region deployment should raise this back
		// toward productionScaleParams()'s 8).
		MinSubnetDiversity: 1,
		MinAnchorPeers:     2,
		MaxChurnRate:       5,
		MinAvgTenure:       300, // seconds (vanillaP2PHealthAdapter's real unit)
		// PeerLatency has no real RTT measurement yet on the vanilla p2p.Switch
		// transport (vanillaP2PHealthAdapter always reports 0, cmd/engramd/main.go) --
		// this threshold is inert defense today, kept at the E4-validated value
		// so it's already correctly sized once real RTT wiring lands.
		MaxPeerLatency:  200, // milliseconds
		MaxIgnoreRounds: 1,
		// MaxPeersPerSubnet: same same-subnet-topology constraint as
		// MinSubnetDiversity's doc above applies here too -- all 4 real
		// engram-nodeNN validators sit on ONE /24 (172.28.0.0/24), so this
		// MUST exceed 4 or the ingress filter (FilterPeerByAddr) would
		// reject the honest validators' own mesh connections to each other,
		// not just an attacker's. 8 gives comfortable margin above the 4
		// known-honest peers (reconnects, brief overlap during a rolling
		// restart) while still meaningfully capping a same-subnet Sybil/
		// slot-exhaustion swarm (docs/EXPERIMENT.md's E4/E8 A1/A2) -- revisit
		// once Part 2's live attacker-swarm test measures real behavior
		// against this value, same "reasoned default, then live-tuned"
		// precedent as MaxUnprovenTailBlocks/DAThreshold above.
		MaxPeersPerSubnet: 8,
	}
}
