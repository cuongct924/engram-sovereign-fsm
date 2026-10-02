--------------------------- MODULE EngramFSM ---------------------------
(*
 * EngramFSM — Circuit-Breaker Finite State Machine
 *
 * Models the 4-state circuit breaker (ANCHORED -> SUSPICIOUS -> SOVEREIGN ->
 * RECOVERING -> ANCHORED) driven by three environment sensors: Bitcoin anchor re-verification
 * gap, DA availability, and P2P network quality.
 *
 * Depends on: EngramVars (variables), Integers, FiniteSets
 *)

EXTENDS Integers, FiniteSets, EngramVars

(* ======================== CONSTANTS & ASSUMPTIONS ========================= *)
CONSTANTS
    \* @type: Int;
    SUSPICIOUS_THRESHOLD,   \* BTC gap threshold for Gray Failure warning
    \* @type: Int;
    SOVEREIGN_THRESHOLD,    \* BTC gap threshold for Hard Failure (circuit-break)

    \* @type: Int;
    MIN_PEERS,              \* Minimum clean peers required to avoid isolation
    \* @type: Int;
    MIN_SUBNET_DIVERSITY,   \* Minimum distinct subnets required
    \* @type: Int;
    MIN_ANCHOR_PEERS,       \* Minimum active anchor/bootstrap peers required
    \* @type: Int;
    MAX_CHURN_RATE,         \* Maximum allowed peer disconnects/reconnects per epoch
    \* @type: Int;
    MIN_AVG_TENURE,         \* Minimum average age of connections in the routing table
    \* @type: Int;
    MAX_PEER_LATENCY,       \* Maximum allowable delay for heartbeat/block propagation

    \* @type: Int;
    MAX_SUSPICIOUS_TIME,    \* Maximum ticks/blocks the system tolerates in SUSPICIOUS state before escalating to SOVEREIGN

    \* @type: Int;
    DOWN_HYSTERESIS_THRESHOLD,  \* Consecutive warning-only blocks tolerated in ANCHORED/RECOVERING
                                 \* before demoting -- mirrors RECOVERY_HYSTERESIS_WAIT's role on the recovery
                                 \* edge, applied to the regression edge. A critical (hard-failure)
                                 \* reading always bypasses this and demotes immediately, same as before.

    \* @type: Int;
    MAX_DOWN_HYSTERESIS_THRESHOLD  \* Ceiling on RECOVERING's exponentially-backed-off down-hysteresis
                                    \* threshold (EffectiveDownHysteresisThreshold below) -- without a
                                    \* cap, unbounded repeated regressions would grow the grace period
                                    \* forever, itself a liveness risk after enough genuine network
                                    \* hiccups over a long run, not just a flapping attacker.

    \* @type: Int;
    STABLE_WINDOW  \* Consecutive healthy ANCHORED blocks that decay flap_level by 1 -- the
                    \* forgiveness half of adaptive hysteresis: every hysteresis threshold
                    \* is backed off by flap_level and relaxes again once the system is stable.

ASSUME
    /\ SUSPICIOUS_THRESHOLD \in Nat
    /\ SOVEREIGN_THRESHOLD \in Nat
    /\ DA_THRESHOLD \in Nat
    /\ RECOVERY_HYSTERESIS_WAIT \in Nat
    /\ SUSPICIOUS_HYSTERESIS_WAIT \in Nat
    /\ MIN_PEERS \in Nat
    /\ MIN_SUBNET_DIVERSITY \in Nat
    /\ MIN_ANCHOR_PEERS  \in Nat
    /\ MAX_CHURN_RATE \in Nat
    /\ MIN_AVG_TENURE \in Nat
    /\ MAX_PEER_LATENCY \in Nat
    /\ DOWN_HYSTERESIS_THRESHOLD \in Nat
    /\ MAX_DOWN_HYSTERESIS_THRESHOLD \in Nat
    /\ MAX_DOWN_HYSTERESIS_THRESHOLD >= DOWN_HYSTERESIS_THRESHOLD
    /\ STABLE_WINDOW \in Nat
    /\ STABLE_WINDOW >= 1
    /\ DOWN_HYSTERESIS_THRESHOLD >= 1
    /\ SUSPICIOUS_THRESHOLD < SOVEREIGN_THRESHOLD

\* Helper: returns the smaller of two integers
\* @type: (Int, Int) => Int;
MinVal(a, b) == IF a < b THEN a ELSE b

\* Adaptive down-hysteresis: flap_level scales the down threshold of both
\* demotion edges (ANCHORED -> SUSPICIOUS, RECOVERING -> SOVEREIGN).
\* flap_level rises on every demotion caused by accumulated warning-level
\* noise and decays by 1 per STABLE_WINDOW healthy ANCHORED blocks, so the
\* threshold backs off under repeated flapping (WAN noise or a precisely-
\* timed attacker) and relaxes once the system is stable. A fresh or long-
\* stable system (flap_level = 0) pays only the plain DOWN_HYSTERESIS_THRESHOLD.
\* The up-hysteresis waits (RECOVERY_/SUSPICIOUS_HYSTERESIS_WAIT) stay fixed.
\* Uses `^` (Naturals, via EXTENDS Integers) rather than a user RECURSIVE
\* operator -- Apalache does not support RECURSIVE at all (fails every check
\* transitively EXTENDS-ing this module), while `^` is a native built-in both
\* TLC and Apalache handle directly.
\* @type: Int;
EffectiveDownHysteresisThreshold ==
    MinVal(DOWN_HYSTERESIS_THRESHOLD * (2 ^ flap_level), MAX_DOWN_HYSTERESIS_THRESHOLD)


(* ======================== P2P HEALTH SENSOR (Tri-interface Profiler) =============================== *)
\* @type: (Str) => Str;
SubnetOf(p) ==
    CASE p = "anchor_n1"                             -> "subnet_A"
      [] p = "anchor_n2"                             -> "subnet_B"
      [] p = "anchor_n3"                             -> "subnet_C"
      [] p \in {"sybil_n1", "sybil_n2", "sybil_n3"}  -> "subnet_malicious"
      [] p = "honest_node_1"                         -> "subnet_D"
      [] p = "honest_node_2"                         -> "subnet_E"
      [] OTHER                                       -> "unknown_subnet"

\* Number of distinct subnets represented in active_peers
SubnetDiversity == Cardinality({SubnetOf(p) : p \in active_peers})

\* Subset of anchor_peers that are currently active
ActiveAnchors == active_peers \intersect anchor_peers

\* Active peers that have not been blacklisted
CleanPeers == active_peers \ blacklisted_peers

\* Composite P2P health predicate
IsP2PQualityHealthy ==
    /\ SubnetDiversity            >= MIN_SUBNET_DIVERSITY   \* Not concentrated on a single IP block
    /\ Cardinality(ActiveAnchors) >= MIN_ANCHOR_PEERS       \* Maintains connection with root nodes
    /\ Cardinality(CleanPeers)    >= MIN_PEERS              \* Sufficient non-malicious peers
    /\ peer_churn_rate            <= MAX_CHURN_RATE         \* The routing table is not constantly being shuffled.
    /\ avg_peer_tenure            >= MIN_AVG_TENURE         \* All peers are "long-lived" nodes.
    /\ peer_latency               <= MAX_PEER_LATENCY       \* No indication of routing through the Relay node.


(* ======================== DATA AVAILABILITY SENSOR ======================================= *)
\* Data Availability gap: unverified Engram blocks since last DA proof
da_gap == h_engram_current - h_engram_verified

\* DA layer is publishing proofs within the allowed gap
IsDAHealthy == (da_gap < DA_THRESHOLD) /\ ~is_das_failed /\ ~is_attestation_failed


(* ======================== BTC FINALITY GAP SENSOR ========================= *)
\* Bitcoin settlement gap: distance from current tip to last confirmed anchor
btc_gap == h_btc_current - MinVal(h_btc_submitted, h_btc_anchored)

IsBTCGapSuspicious == 
    /\ SUSPICIOUS_THRESHOLD <= btc_gap 
    /\ (btc_gap < SOVEREIGN_THRESHOLD)

IsBTCGapSovereign == btc_gap >= SOVEREIGN_THRESHOLD

\* BTC layer is within both gap thresholds and its anchor re-verification hasn't
\* failed -- the BTC-side analog of IsDAHealthy above, gating
\* IsValidProposal's Settlement Monotonicity & BTC Light Client Hash Check
\* (EngramTendermint.tla) the same way IsDAHealthy gates its DA Pipeline
\* Check.
IsBTCHealthy == ~IsBTCGapSuspicious /\ ~IsBTCGapSovereign /\ ~is_anchor_reverification_failed


(* ======================== HEALTH CONDITION PREDICATES ===================== *)
\* Withdrawal guard: TRUE whenever cross-chain withdrawals must be halted
WithdrawLocked == state \in {"SOVEREIGN", "RECOVERING"}

\* Hard failure: BTC gap crossed threshold, BTC anchor re-verification failed
\* (the anchor height itself is untrustworthy, not merely stale -- same
\* severity class as IsBTCGapSovereign, unlike is_das_failed/
\* is_attestation_failed which only feed IsWarningCondition), OR Total Loss
\* of Anchor Peers (Complete Eclipse), OR a SUSPICIOUS gray-failure timeout.
IsCriticalCondition ==
    \/ IsBTCGapSovereign
    \/ is_anchor_reverification_failed
    \/ Cardinality(ActiveAnchors) = 0
    \/ suspicious_duration >= MAX_SUSPICIOUS_TIME

\* Soft warning: BTC gap is elevated, or DA/P2P shows degradation
IsWarningCondition == 
    \/ IsBTCGapSuspicious
    \/ ~IsDAHealthy
    \/ ~IsP2PQualityHealthy

\* All sensors are green and thresholds are satisfied
IsHealthyCondition ==
    /\ ~IsBTCGapSovereign
    /\ ~IsBTCGapSuspicious
    /\ ~is_anchor_reverification_failed
    /\ IsDAHealthy
    /\ IsP2PQualityHealthy


(* ======================== SENSOR AGGREGATION / HEALTH CERTIFICATE ========= *)
\* Numeric observations are clipped to 0..ObsCap so the model stays finite;
\* the cap only has to exceed the thresholds' relative order, not their value.
ObsCap == 3
ObsDomain == 0..ObsCap

\* @type: (Int) => Int;
ClipObs(x) == IF x > ObsCap THEN ObsCap ELSE x

\* WeightedMedianOf -- uniform-stake port of vote_extension.go's
\* weightedMedianUint64: the smallest x with 2 * |{m : vals[m] <= x}| > |members|,
\* which equals the Go "cum > total/2" rule for every |members| (integer
\* halving included). The predicate is monotone in x, so that smallest x is
\* the number of values in ObsDomain for which it fails -- no CHOOSE or
\* recursion, which keeps the operator Apalache-compatible.
\* @type: (Set(Str), Str -> Int) => Int;
WeightedMedianOf(members, vals) ==
    Cardinality({ x \in ObsDomain :
        2 * Cardinality({ m \in members : vals[m] <= x }) <= Cardinality(members) })

\* AggregateHealthCertificate -- weighted median (numeric) / weighted majority
\* (Boolean) aggregation over `members` (mirrors x/sovereignty/
\* vote_extension.go's AggregateHealthCertificates + weightedMedianUint64 +
\* weightedMajorityBool: a Boolean field is TRUE in the aggregate iff strictly
\* more than half of `members` reported TRUE for it).
\* `members` is the caller-selected quorum subset (EngramServer.tla decides
\* whether Cardinality(members) clears the real 2f+1 threshold via
\* EngramTendermint.tla's THRESHOLD2) -- this operator itself is pure and
\* knows nothing about quorum sizing, matching EngramFSM.tla's role as the
\* lowest layer (see spec/CLAUDE.md's four-layer refinement hierarchy).
\* @type: (Set(Str), Str -> { is_healthy: Bool, is_warning: Bool, is_critical: Bool, btc_gap_obs: Int, da_gap_obs: Int }) => { is_healthy: Bool, is_warning: Bool, is_critical: Bool, btc_gap_obs: Int, da_gap_obs: Int, quorum_weight: Int };
AggregateHealthCertificate(members, view) ==
    [ is_healthy    |-> 2 * Cardinality({ m \in members : view[m].is_healthy })  > Cardinality(members),
      is_warning    |-> 2 * Cardinality({ m \in members : view[m].is_warning })  > Cardinality(members),
      is_critical   |-> 2 * Cardinality({ m \in members : view[m].is_critical }) > Cardinality(members),
      btc_gap_obs   |-> WeightedMedianOf(members, [m \in members |-> view[m].btc_gap_obs]),
      da_gap_obs    |-> WeightedMedianOf(members, [m \in members |-> view[m].da_gap_obs]),
      quorum_weight |-> Cardinality(members) ]

\* The signed observation a correct, non-eclipsed validator reports for
\* itself (ServerExtendVote's honest branch, EngramServer.tla) -- equal to
\* the true global predicates and gaps it can currently see.
\* @type: { is_healthy: Bool, is_warning: Bool, is_critical: Bool, btc_gap_obs: Int, da_gap_obs: Int };
TrueSensorReading ==
    [ is_healthy  |-> IsHealthyCondition,
      is_warning  |-> IsWarningCondition,
      is_critical |-> IsCriticalCondition,
      btc_gap_obs |-> ClipObs(btc_gap),
      da_gap_obs  |-> ClipObs(da_gap) ]

\* What an honest validator whose P2P layer is eclipsed may report: it sees its
\* peer set as degraded while the Bitcoin/DA gaps still come from the shared
\* external chains (EngramVars.tla's sensorAggVars note). Modelling choice:
\* eclipse can only make a node look worse (never healthier) -- it may or may
\* not also escalate to is_critical (e.g. zero reachable anchors).
\* @type: Set({ is_healthy: Bool, is_warning: Bool, is_critical: Bool, btc_gap_obs: Int, da_gap_obs: Int });
EclipsedReadings ==
    { [ is_healthy  |-> FALSE,
        is_warning  |-> TRUE,
        is_critical |-> c,
        btc_gap_obs |-> ClipObs(btc_gap),
        da_gap_obs  |-> ClipObs(da_gap) ] : c \in BOOLEAN }


(* ======================== TYPE INVARIANT & SANITY CHECK ================================== *)
FSMTypeOK == 
    /\ state \in {"ANCHORED", "SUSPICIOUS", "SOVEREIGN", "RECOVERING"}
    /\ btc_gap >= 0
    /\ da_gap >= 0
    /\ is_das_failed \in BOOLEAN
    /\ is_attestation_failed \in BOOLEAN
    /\ is_anchor_reverification_failed \in BOOLEAN
    /\ IsFiniteSet(active_peers)
    /\ IsFiniteSet(anchor_peers)
    /\ IsFiniteSet(blacklisted_peers)
    /\ peer_churn_rate \in Nat /\ avg_peer_tenure \in Nat /\ peer_latency \in Nat
    /\ safe_blocks \in 0..RECOVERY_HYSTERESIS_WAIT
    /\ suspicious_safe_blocks \in 0..SUSPICIOUS_HYSTERESIS_WAIT
    /\ unhealthy_streak \in 0..MAX_DOWN_HYSTERESIS_THRESHOLD
    /\ flap_level \in 0..MAX_DOWN_HYSTERESIS_THRESHOLD
    /\ stable_blocks \in 0..STABLE_WINDOW
    /\ reanchoring_proof_valid \in BOOLEAN
    /\ da_backfill_valid \in BOOLEAN


(* ======================== STATE MACHINE INITIALIZATION ================================== *)
FSMInit == 
    /\ state = "ANCHORED"
    
    /\ h_btc_current = 0
    /\ h_btc_submitted = 0
    /\ h_btc_anchored = 0
    /\ is_anchor_reverification_failed = FALSE
    
    /\ h_engram_current = 0
    /\ h_engram_verified = 0
    /\ is_attestation_failed = FALSE
    /\ is_das_failed = FALSE
    
    /\ anchor_peers = {"anchor_n1", "anchor_n2", "anchor_n3"}
    /\ active_peers = anchor_peers
    /\ blacklisted_peers = {}
    /\ peer_churn_rate = 0 
    /\ avg_peer_tenure = MIN_AVG_TENURE 
    /\ peer_latency = 0
    
    /\ safe_blocks = 0
    /\ suspicious_duration = 0
    /\ suspicious_safe_blocks = 0
    /\ unhealthy_streak = 0
    /\ flap_level = 0
    /\ stable_blocks = 0
    /\ reanchoring_proof_valid = FALSE
    /\ da_backfill_valid = FALSE


(* ======================== ENVIRONMENT SENSOR UPDATE ======================= *)

\* ------------------- 1. BTC FINALITY GAP SENSOR -------------------

\* Scenario 1: anchor re-verification operates normally; the anchor advances to the submitted height.
BTCNormalUpdate ==
    /\ h_btc_current' \in {h_btc_current, h_btc_current + 1}
    /\ h_btc_submitted' \in {h_btc_submitted, h_btc_current'}
    /\ is_anchor_reverification_failed' = FALSE
    /\ h_btc_anchored' = h_btc_submitted'   \* anchor re-verification passed: anchor can advance
    \* /\ UNCHANGED <<fsmVars>> 
    \* /\ UNCHANGED <<daGapSensorVars, p2pHealthSensorVars>>

\* Scenario 2: anchor re-verification fails or is under attack; the anchor is throttled (frozen).
BTCReverificationFailure ==
    /\ h_btc_current' \in {h_btc_current, h_btc_current + 1}
    /\ h_btc_submitted' \in {h_btc_submitted, h_btc_current'}
    /\ is_anchor_reverification_failed' = TRUE
    /\ h_btc_anchored' = h_btc_anchored     \* anchor re-verification failed: anchor is frozen
    \* /\ UNCHANGED <<fsmVars>> 
    \* /\ UNCHANGED <<daGapSensorVars, p2pHealthSensorVars>>


UpdateBTCSensor == BTCNormalUpdate \/ BTCReverificationFailure


\* ------------------- 2. DATA AVAILABILITY SENSOR -------------------

\* Scenario 1: DA Layer is healthy; attestation succeeds.
DANormalUpdate ==
    /\ h_engram_current' \in {h_engram_current, h_engram_current + 1}
    /\ is_attestation_failed' = FALSE
    /\ is_das_failed' = FALSE
    /\ h_engram_verified' = h_engram_current' \* DA attestation passed: allowed to update to current height
    \* /\ UNCHANGED <<state, safe_blocks, suspicious_duration>> 
    \* /\ UNCHANGED <<btcGapSensorVars, p2pHealthSensorVars>>

\* Scenario 2: DA Layer reports failure (Data Withholding attack or Blobstream disconnect).
DAFailure ==
    /\ h_engram_current' \in {h_engram_current, h_engram_current + 1}
    /\ is_attestation_failed' \in BOOLEAN
    /\ is_das_failed' \in BOOLEAN
    /\ \/ is_attestation_failed' = TRUE
       \/ is_das_failed' = TRUE
    /\ h_engram_verified' = h_engram_verified \* DA failure: verification is throttled (frozen)
    \* /\ UNCHANGED <<state, safe_blocks, suspicious_duration>> 
    \* /\ UNCHANGED <<btcGapSensorVars, p2pHealthSensorVars>>


UpdateDASensor == DANormalUpdate \/ DAFailure

\* ------------------- 3. P2P HEALTH SENSOR -------------------

\* Update P2P Health Sensor
\* The node is connected to a healthy mix of anchor peers and honest nodes.
\* P2PNormalUpdate ==
\*     /\ active_peers' \in { anchor_peers, anchor_peers \cup {"honest_n1"} }
\*     /\ peer_churn_rate' \in {0, MAX_CHURN_RATE}
\*     /\ avg_peer_tenure' \in {MIN_AVG_TENURE, MIN_AVG_TENURE + 10}
\*     /\ peer_latency'    \in {0, MAX_PEER_LATENCY}

P2PNormalUpdate ==
    /\ active_peers' = anchor_peers \cup {"honest_node_1"}
    /\ peer_churn_rate' = 0
    /\ avg_peer_tenure' = MIN_AVG_TENURE + 10
    /\ peer_latency'    = 0


\* Attack Scenario 1: Relay-node latency injection
\* The adversary inserts a proxy/relay node into the routing path to intercept messages.
\* This physically forces the peer latency to spike beyond the acceptable threshold.
RelayNodeAttack == 
    /\ peer_latency' = MAX_PEER_LATENCY + 10  
    /\ UNCHANGED <<active_peers, peer_churn_rate, avg_peer_tenure>>


\* Attack Scenario 2: BGP Hijacking / Connection Hijacking
\* The adversary manipulates BGP routes to isolate the victim at the infrastructure level.
\* The victim's active peer set is entirely replaced by Sybil nodes from a single ASN/subnet.
BGPHijackingAttack == 
    /\ active_peers' = {"sybil_n1", "sybil_n2", "sybil_n3"} 
    /\ UNCHANGED <<peer_latency, peer_churn_rate, avg_peer_tenure>>


\* Attack Scenario 3: Churn-based IP rotation (Dynamic Peer Replacement)
\* The adversary continuously rotates malicious IP addresses to evade static firewalls.
\* This triggers a high network churn rate and reduces the average peer tenure to zero.
ChurnBasedRotationAttack == 
    /\ peer_churn_rate' = MAX_CHURN_RATE + 5  
    /\ avg_peer_tenure' = 0                   
    /\ UNCHANGED <<active_peers, peer_latency>>


P2PAdversaryAttack ==         
    \/ RelayNodeAttack 
    \/ BGPHijackingAttack
    \/ ChurnBasedRotationAttack

\* P2PAdversaryAttack ==
\*     \* ADVERSARY EXPLOITS THE "WEAKEST LINK" (DEFENSE-IN-DEPTH TEST)
\*     \* The adversary non-deterministically chooses to eclipse 1 of the 3 external/internal network interfaces.
\*     /\ \E target_network \in {"ENGRAM_P2P", "BTC_SPV", "CELESTIA_DA"} : 
\*         /\ \E p \in active_peers : 
\*             \* Abstraction: Regardless of the targeted interface, the "Test-before-evict" mechanism strictly protects Anchor peers.
\*             /\ p \notin ActiveAnchors  
\*             /\ active_peers' = (active_peers \ {p}) \cup {"sybil_n1"}
            
\*     \* Aggregated consequences reflected on the Holistic Monitor (Cross-interface P2P Health Sensor)
\*     /\ peer_churn_rate' = MAX_CHURN_RATE + 1    \* Triggers network churn alarm (Dynamic Replacement / Handover)
\*     /\ avg_peer_tenure' = 0                     \* Triggers Sybil alarm (Adversary nodes have 0 tenure)
\*     /\ peer_latency' = MAX_PEER_LATENCY + 10    \* Simulates latency spikes caused by Relay Nodes or BGP Hijacking


UpdateP2PHealthSensor == 
    /\ (P2PNormalUpdate \/ P2PAdversaryAttack)
    /\ anchor_peers' = anchor_peers
    /\ blacklisted_peers' = blacklisted_peers
    \* /\ UNCHANGED <<state, safe_blocks, suspicious_duration>>
    \* /\ UNCHANGED <<btcGapSensorVars, daGapSensorVars>>

\* Non-deterministic environment update that simulates the real network.
UpdateSensors ==
    /\ UpdateBTCSensor
    /\ UpdateDASensor
    /\ UpdateP2PHealthSensor

    \* ZK re-anchoring proof validity
    \* Proof becomes valid only once the Bitcoin anchor has caught up to the submission height (i.e., the OP_RETURN tx is confirmed).
    /\ reanchoring_proof_valid' =
           IF state = "RECOVERING"
              /\ h_btc_anchored' >= h_btc_submitted'
              /\ h_btc_submitted' > 0
           THEN TRUE
           ELSE FALSE
    /\ UNCHANGED <<state, safe_blocks, suspicious_duration, suspicious_safe_blocks, unhealthy_streak, flap_level, stable_blocks, da_backfill_valid>>
    /\ UNCHANGED <<censorshipVars>>


(* ======================== FSM RULE ENGINE ================================= *)
\* Pure function: given the current sensor readings, compute the next FSM state.
\* This is called by EngramServer at every decision point.
CalculateNextFSMState ==
    CASE state = "ANCHORED"   /\ IsCriticalCondition -> "SOVEREIGN"

      \* Down-hysteresis (E5's flapping fix): a warning-only reading (never
      \* critical) only demotes ANCHORED -> SUSPICIOUS once it has recurred
      \* unhealthy_streak+1 >= EffectiveDownHysteresisThreshold times in a row --
      \* a single noisy block is absorbed instead of triggering an immediate
      \* drop. Critical conditions above are NEVER softened this way.
      [] state = "ANCHORED"   /\ IsWarningCondition /\ ~IsCriticalCondition
                               /\ unhealthy_streak + 1 >= EffectiveDownHysteresisThreshold -> "SUSPICIOUS"
      [] state = "ANCHORED"   /\ IsWarningCondition /\ ~IsCriticalCondition -> "ANCHORED"

      [] state = "SUSPICIOUS" /\ IsCriticalCondition -> "SOVEREIGN"

      \* Gray Failure Timeout. Force circuit-break if system stays suspicious too long.
      [] state = "SUSPICIOUS" /\ suspicious_duration >= MAX_SUSPICIOUS_TIME -> "SOVEREIGN"

      \* Up-hysteresis on the exit edge (Gray Failure Arbitrage fix): a single
      \* healthy block used to exit SUSPICIOUS -> ANCHORED immediately, hard-
      \* resetting suspicious_duration to 0 (ExecuteFSMTransition below) --
      \* letting an attacker who can nudge sensors healthy for exactly one
      \* block, right before MAX_SUSPICIOUS_TIME, restart the clock forever
      \* without ever reaching SOVEREIGN. Now requires
      \* suspicious_safe_blocks+1 >= SUSPICIOUS_HYSTERESIS_WAIT consecutive
      \* healthy blocks while STILL in SUSPICIOUS -- suspicious_duration keeps
      \* accumulating throughout (target_state stays "SUSPICIOUS" during
      \* absorption, so its formula below is unaffected), so a short healthy
      \* blip no longer buys the attacker a free reset.
      [] state = "SUSPICIOUS" /\ IsHealthyCondition
                               /\ suspicious_safe_blocks + 1 >= SUSPICIOUS_HYSTERESIS_WAIT -> "ANCHORED"
      [] state = "SUSPICIOUS" /\ IsHealthyCondition -> "SUSPICIOUS"

      [] state = "SOVEREIGN"  /\ IsHealthyCondition  -> "RECOVERING"

      \* Critical failure while RECOVERING always falls back immediately --
      \* down-hysteresis below only ever softens a non-critical (warning-only
      \* or merely-not-yet-fully-healthy) reading, never a hard failure.
      [] state = "RECOVERING" /\ IsCriticalCondition -> "SOVEREIGN"

      \* Down-hysteresis + leak (E5's flapping fix), adaptively backed off by
      \* flap_level (flapping-attack hardening): a non-critical unhealthy
      \* reading only demotes RECOVERING -> SOVEREIGN once it has recurred
      \* unhealthy_streak+1 >= EffectiveDownHysteresisThreshold times in a
      \* row -- doubling per flap_level, capped at
      \* MAX_DOWN_HYSTERESIS_THRESHOLD. ExecuteFSMTransition leaks
      \* safe_blocks by 1 (not a hard reset to 0) while absorbing the noise
      \* below, preserving partial progress.
      [] state = "RECOVERING" /\ ~IsHealthyCondition
                               /\ unhealthy_streak + 1 >= EffectiveDownHysteresisThreshold -> "SOVEREIGN"
      [] state = "RECOVERING" /\ ~IsHealthyCondition -> "RECOVERING"

      \* Exit condition when hysteresis, ZK proof and DA backfill are all satisfied.
      \* A recovery proof shows the SOVEREIGN interval is valid, not that its
      \* data is available, so exit also needs the DA backfill certificate.
      [] state = "RECOVERING" /\ IsHealthyCondition  /\ safe_blocks = RECOVERY_HYSTERESIS_WAIT /\ reanchoring_proof_valid = TRUE
                               /\ da_backfill_valid = TRUE -> "ANCHORED"

      \* Catch-all for remaining in RECOVERING (covers safe_blocks < RECOVERY_HYSTERESIS_WAIT, pending ZK proofs and pending DA backfill)
      [] state = "RECOVERING" /\ IsHealthyCondition  -> "RECOVERING"

      [] OTHER -> state


\* Action: write the FSM transition and update the hysteresis counter.
\* @type: (Str) => Bool;
ExecuteFSMTransition(target_state) ==
    /\ state' = target_state

    /\ suspicious_duration' =
           IF target_state = "SUSPICIOUS"
           THEN MinVal(suspicious_duration + 1, MAX_SUSPICIOUS_TIME + 1)
           ELSE 0

    \* unhealthy_streak accumulates only while absorbing a non-critical
    \* warning/unhealthy reading without yet demoting (i.e. staying in the
    \* same state that CalculateNextFSMState's down-hysteresis branches
    \* just chose not to leave) -- resets to 0 the instant a real transition
    \* fires (demotion or a fully healthy reading), same reset pattern as
    \* suspicious_duration above.
    /\ unhealthy_streak' =
           IF \/ (state = "ANCHORED"   /\ target_state = "ANCHORED"   /\ IsWarningCondition /\ ~IsCriticalCondition)
              \/ (state = "RECOVERING" /\ target_state = "RECOVERING" /\ ~IsHealthyCondition /\ ~IsCriticalCondition)
           THEN unhealthy_streak + 1
           ELSE 0

    \* flap_level (adaptive hysteresis, see EffectiveDownHysteresisThreshold):
    \* +1 on every demotion driven by accumulated warning-level noise --
    \* ANCHORED -> SUSPICIOUS, and RECOVERING -> SOVEREIGN only when it was
    \* NOT a critical reading (a critical failure is a genuine fault, not
    \* flapping, so it never raises the level). Saturates once growing
    \* further wouldn't change the capped effective threshold (keeps this a
    \* finite Nat within FSMTypeOK's bound). -1 once STABLE_WINDOW healthy
    \* ANCHORED blocks have accumulated in stable_blocks. A successful
    \* recovery does not reset it: forgiveness is earned by sustained
    \* stability. Unchanged otherwise.
    /\ flap_level' =
           IF \/ (state = "ANCHORED" /\ target_state = "SUSPICIOUS")
              \/ (state = "RECOVERING" /\ target_state = "SOVEREIGN" /\ ~IsCriticalCondition)
           THEN IF DOWN_HYSTERESIS_THRESHOLD * (2 ^ flap_level) >= MAX_DOWN_HYSTERESIS_THRESHOLD
                THEN flap_level
                ELSE flap_level + 1
           ELSE IF state = "ANCHORED" /\ target_state = "ANCHORED" /\ IsHealthyCondition
                   /\ flap_level >= 1 /\ stable_blocks + 1 >= STABLE_WINDOW
                THEN flap_level - 1
                ELSE flap_level

    \* stable_blocks counts healthy blocks spent in ANCHORED while
    \* flap_level > 0: a healthy block increments it (restarting at 0 once it
    \* decays flap_level), and an absorbed warning block leaks 1 instead of
    \* hard-resetting, mirroring safe_blocks, so sporadic WAN noise doesn't
    \* discard all progress toward forgiveness. Resets to 0 on any transition
    \* out of (or into) ANCHORED.
    /\ stable_blocks' =
           IF state = "ANCHORED" /\ target_state = "ANCHORED" /\ flap_level >= 1
           THEN IF IsHealthyCondition
                THEN IF stable_blocks + 1 >= STABLE_WINDOW THEN 0 ELSE stable_blocks + 1
                ELSE IF stable_blocks >= 1 THEN stable_blocks - 1 ELSE 0
           ELSE 0

    \* da_backfill_valid is earned only by DABackfill during RECOVERING and
    \* survives while the state stays RECOVERING; any transition into or out of
    \* RECOVERING clears it, so each SOVEREIGN interval needs its own backfill.
    /\ da_backfill_valid' =
           IF target_state = "RECOVERING" /\ state = "RECOVERING"
           THEN da_backfill_valid
           ELSE FALSE

    \* It resets to 0 automatically if transitioning out of RECOVERING or just entering it from SOVEREIGN.
    \* While staying in RECOVERING: a healthy block increments toward
    \* RECOVERY_HYSTERESIS_WAIT as before; a non-critical unhealthy block being
    \* absorbed by down-hysteresis LEAKS one unit instead of hard-resetting
    \* to 0 -- E5's flapping fix keeps partial progress through sporadic
    \* noise rather than discarding a whole streak on one bad reading.
    /\ safe_blocks' =
           IF target_state = "RECOVERING" /\ state = "RECOVERING"
           THEN IF IsHealthyCondition
                THEN MinVal(safe_blocks + 1, RECOVERY_HYSTERESIS_WAIT)
                ELSE IF safe_blocks >= 1 THEN safe_blocks - 1 ELSE 0
           ELSE 0

    \* suspicious_safe_blocks mirrors safe_blocks' leak semantics, applied to
    \* SUSPICIOUS's own exit edge (Gray Failure Arbitrage fix): a healthy
    \* block while still SUSPICIOUS increments toward SUSPICIOUS_HYSTERESIS_WAIT;
    \* a non-healthy block being absorbed leaks one unit instead of hard-
    \* resetting, so sporadic noise during the healthy streak doesn't fully
    \* discard prior progress. Resets to 0 on any real transition out of (or
    \* into) SUSPICIOUS.
    /\ suspicious_safe_blocks' =
           IF target_state = "SUSPICIOUS" /\ state = "SUSPICIOUS"
           THEN IF IsHealthyCondition
                THEN MinVal(suspicious_safe_blocks + 1, SUSPICIOUS_HYSTERESIS_WAIT)
                ELSE IF suspicious_safe_blocks >= 1 THEN suspicious_safe_blocks - 1 ELSE 0
           ELSE 0

\* DA backfill: while RECOVERING, once the DA layer is healthy, the queued block
\* data of the SOVEREIGN interval is published and becomes retrievable. Enabled
\* only in RECOVERING and only until it completes; it changes nothing else.
DABackfill ==
    /\ state = "RECOVERING"
    /\ da_backfill_valid = FALSE
    /\ IsDAHealthy
    /\ da_backfill_valid' = TRUE
    /\ UNCHANGED <<state, safe_blocks, suspicious_duration, suspicious_safe_blocks, unhealthy_streak, flap_level, stable_blocks, reanchoring_proof_valid>>
    /\ UNCHANGED <<networkSensorVars, censorshipVars>>

(* ======================== THE NEXT-STATE ACTION (FOR UNIT TEST) ============ *)
FSMNext == 
    \/ /\ UpdateSensors
    \/ /\ state' = CalculateNextFSMState 
       /\ state' /= state
       /\ ExecuteFSMTransition(state')
       /\ UNCHANGED <<networkSensorVars, censorshipVars>>
       /\ UNCHANGED <<reanchoring_proof_valid>>
    \/ DABackfill


FSMSpec == FSMInit /\ [][FSMNext]_fsmVars


(* ======================== SAFETY PROPERTIES ============================== *)
\* Safety 1: Withdrawal lock is active if and only if state is SOVEREIGN or RECOVERING
CircuitBreakerSafety ==
    WithdrawLocked <=> (state \in {"SOVEREIGN", "RECOVERING"})

\* Safety 2: The only way to exit RECOVERING is after full hysteresis + valid ZK proof
HysteresisSafety ==
    [][ (state = "RECOVERING" /\ state' = "ANCHORED")
        => (safe_blocks = RECOVERY_HYSTERESIS_WAIT /\ reanchoring_proof_valid) ]_fsmVars

\* Safety 2a: The only way to exit RECOVERING is also after the DA backfill
\* certificate -- the SOVEREIGN interval's data is available before withdrawals
\* reopen (P_avail).
SovereignDataPublishedSafety ==
    [][ (state = "RECOVERING" /\ state' = "ANCHORED") => da_backfill_valid ]_fsmVars

\* Safety 2b: The only way to exit SUSPICIOUS to ANCHORED is after full
\* hysteresis (Gray Failure Arbitrage fix) -- a single healthy block can
\* never do it alone.
SuspiciousHysteresisSafety ==
    [][ (state = "SUSPICIOUS" /\ state' = "ANCHORED")
        => (suspicious_safe_blocks = SUSPICIOUS_HYSTERESIS_WAIT - 1) ]_fsmVars

\* Safety 3: Prevents any illegal or out-of-order FSM state transitions.
StrictFSMTransitionSafety == 
    [][ state /= state' => 
        \/ (state = "ANCHORED"   /\ state' \in {"SUSPICIOUS", "SOVEREIGN"})
        \/ (state = "SUSPICIOUS" /\ state' \in {"ANCHORED", "SOVEREIGN"})
        \/ (state = "SOVEREIGN"  /\ state' = "RECOVERING")
        \/ (state = "RECOVERING" /\ state' \in {"ANCHORED", "SOVEREIGN"})
      ]_fsmVars


(* ======================== LIVENESS PROPERTIES ============================ *)
\* Liveness 1: Critical condition must eventually cause transition to SOVEREIGN
CircuitBreakerLiveness ==
    IsCriticalCondition ~> (state = "SOVEREIGN" \/ ~IsCriticalCondition)

\* Liveness 2: Recovery attempt must eventually be initiated once network heals
RecoveryAttemptLiveness ==
    (state = "SOVEREIGN" /\ IsHealthyCondition)
    ~> (state = "RECOVERING" \/ ~IsHealthyCondition)

\* Liveness 3: Recovery must eventually complete once proof is ready
CompleteRecoveryLiveness ==
    (state = "RECOVERING" /\ reanchoring_proof_valid /\ da_backfill_valid /\ IsHealthyCondition)
    ~> (state = "ANCHORED" \/ ~IsHealthyCondition \/ ~reanchoring_proof_valid \/ ~da_backfill_valid)

\* Liveness 3b: DA backfill must eventually complete during recovery under healthy conditions.
DABackfillLiveness ==
    (state = "RECOVERING" /\ IsHealthyCondition) ~> (da_backfill_valid = TRUE \/ state /= "RECOVERING")

\* Liveness 4: ZK proof must eventually be generated during recovery under healthy conditions.
ZKProofGenerationLiveness == 
    (state = "RECOVERING" /\ IsHealthyCondition) ~> (reanchoring_proof_valid = TRUE)

\* Liveness 5: Persistent network anomalies must eventually trigger a circuit-break or recovery.
PersistentEclipseResolutionLiveness == 
    ([]<> ~IsP2PQualityHealthy) ~> (state \in {"SOVEREIGN", "ANCHORED"})

=============================================================================