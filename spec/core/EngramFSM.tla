--------------------------- MODULE EngramFSM ---------------------------
(*
 * EngramFSM — Circuit-Breaker Finite State Machine
 *
 * Models the 4-state circuit breaker (ANCHORED -> SUSPICIOUS -> SOVEREIGN ->
 * RECOVERING -> ANCHORED) driven by three environment sensors: Bitcoin SPV
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
                                 \* before demoting -- mirrors HYSTERESIS_WAIT's role on the recovery
                                 \* edge, applied to the regression edge. A critical (hard-failure)
                                 \* reading always bypasses this and demotes immediately, same as before.

    \* @type: Int;
    MAX_DOWN_HYSTERESIS_THRESHOLD  \* Ceiling on RECOVERING's exponentially-backed-off down-hysteresis
                                    \* threshold (EffectiveDownHysteresisThreshold below) -- without a
                                    \* cap, unbounded repeated regressions would grow the grace period
                                    \* forever, itself a liveness risk after enough genuine network
                                    \* hiccups over a long run, not just a flapping attacker.

ASSUME
    /\ SUSPICIOUS_THRESHOLD \in Nat
    /\ SOVEREIGN_THRESHOLD \in Nat
    /\ DA_THRESHOLD \in Nat
    /\ HYSTERESIS_WAIT \in Nat
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
    /\ SUSPICIOUS_THRESHOLD < SOVEREIGN_THRESHOLD

\* Helper: returns the smaller of two integers
\* @type: (Int, Int) => Int;
MinVal(a, b) == IF a < b THEN a ELSE b

\* Helper: 2^n, for EffectiveDownHysteresisThreshold's exponential backoff below.
RECURSIVE Pow2(_)
\* @type: (Int) => Int;
Pow2(n) == IF n = 0 THEN 1 ELSE 2 * Pow2(n - 1)

\* Exponential backoff (docs/EXPERIMENT.md's flapping-attack hardening):
\* RECOVERING's down-hysteresis grace period doubles per consecutive
\* RECOVERING -> SOVEREIGN regression since the last successful recovery,
\* capped at MAX_DOWN_HYSTERESIS_THRESHOLD -- a repeated, precisely-timed
\* attacker faces a progressively harder bar each cycle, instead of paying
\* the same fixed DOWN_HYSTERESIS_THRESHOLD cost every time. A single
\* genuine network fault (failed_recovery_attempts = 0) still only pays the
\* plain DOWN_HYSTERESIS_THRESHOLD, unaffected.
\* @type: Int;
EffectiveDownHysteresisThreshold ==
    MinVal(DOWN_HYSTERESIS_THRESHOLD * Pow2(failed_recovery_attempts), MAX_DOWN_HYSTERESIS_THRESHOLD)


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


(* ======================== HEALTH CONDITION PREDICATES ===================== *)
\* Withdrawal guard: TRUE whenever cross-chain withdrawals must be halted
WithdrawLocked == state \in {"SOVEREIGN", "RECOVERING"}

\* Hard failure: BTC gap crossed threshold, BTC SPV/header verification failed
\* (the anchor height itself is untrustworthy, not merely stale -- same
\* severity class as IsBTCGapSovereign, unlike is_das_failed/
\* is_attestation_failed which only feed IsWarningCondition), OR Total Loss
\* of Anchor Peers (Complete Eclipse), OR a SUSPICIOUS gray-failure timeout.
IsCriticalCondition ==
    \/ IsBTCGapSovereign
    \/ is_btc_spv_failed
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
    /\ ~is_btc_spv_failed
    /\ IsDAHealthy
    /\ IsP2PQualityHealthy


(* ======================== TYPE INVARIANT & SANITY CHECK ================================== *)
FSMTypeOK == 
    /\ state \in {"ANCHORED", "SUSPICIOUS", "SOVEREIGN", "RECOVERING"}
    /\ btc_gap >= 0
    /\ da_gap >= 0
    /\ is_das_failed \in BOOLEAN
    /\ is_attestation_failed \in BOOLEAN
    /\ is_btc_spv_failed \in BOOLEAN
    /\ IsFiniteSet(active_peers)
    /\ IsFiniteSet(anchor_peers)
    /\ IsFiniteSet(blacklisted_peers)
    /\ peer_churn_rate \in Nat /\ avg_peer_tenure \in Nat /\ peer_latency \in Nat
    /\ safe_blocks \in 0..HYSTERESIS_WAIT
    /\ suspicious_safe_blocks \in 0..SUSPICIOUS_HYSTERESIS_WAIT
    /\ unhealthy_streak \in 0..DOWN_HYSTERESIS_THRESHOLD
    /\ failed_recovery_attempts \in 0..MAX_DOWN_HYSTERESIS_THRESHOLD
    /\ reanchoring_proof_valid \in BOOLEAN


(* ======================== STATE MACHINE INITIALIZATION ================================== *)
FSMInit == 
    /\ state = "ANCHORED"
    
    /\ h_btc_current = 0
    /\ h_btc_submitted = 0
    /\ h_btc_anchored = 0
    /\ is_btc_spv_failed = FALSE
    
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
    /\ failed_recovery_attempts = 0
    /\ reanchoring_proof_valid = FALSE


(* ======================== ENVIRONMENT SENSOR UPDATE ======================= *)

\* ------------------- 1. BTC FINALITY GAP SENSOR -------------------

\* Scenario 1: SPV operates normally; the anchor advances to the submitted height.
BTCNormalUpdate ==
    /\ h_btc_current' \in {h_btc_current, h_btc_current + 1}
    /\ h_btc_submitted' \in {h_btc_submitted, h_btc_current'}
    /\ is_btc_spv_failed' = FALSE
    /\ h_btc_anchored' = h_btc_submitted'   \* SPV verification passed: anchor can advance
    \* /\ UNCHANGED <<fsmVars>> 
    \* /\ UNCHANGED <<daGapSensorVars, p2pHealthSensorVars>>

\* Scenario 2: SPV fails or is under attack; the anchor is throttled (frozen).
BTCSPVFailure ==
    /\ h_btc_current' \in {h_btc_current, h_btc_current + 1}
    /\ h_btc_submitted' \in {h_btc_submitted, h_btc_current'}
    /\ is_btc_spv_failed' = TRUE
    /\ h_btc_anchored' = h_btc_anchored     \* SPV verification failed: anchor is frozen
    \* /\ UNCHANGED <<fsmVars>> 
    \* /\ UNCHANGED <<daGapSensorVars, p2pHealthSensorVars>>


UpdateBTCSensor == BTCNormalUpdate \/ BTCSPVFailure


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
    /\ UNCHANGED <<state, safe_blocks, suspicious_duration, suspicious_safe_blocks, unhealthy_streak, failed_recovery_attempts>>
    /\ UNCHANGED <<censorshipVars>>


(* ======================== FSM RULE ENGINE ================================= *)
\* Pure function: given the current sensor readings, compute the next FSM state.
\* This is called by EngramServer at every decision point.
CalculateNextFSMState ==
    CASE state = "ANCHORED"   /\ IsCriticalCondition -> "SOVEREIGN"

      \* Down-hysteresis (E5's flapping fix): a warning-only reading (never
      \* critical) only demotes ANCHORED -> SUSPICIOUS once it has recurred
      \* unhealthy_streak+1 >= DOWN_HYSTERESIS_THRESHOLD times in a row --
      \* a single noisy block is absorbed instead of triggering an immediate
      \* drop. Critical conditions above are NEVER softened this way.
      [] state = "ANCHORED"   /\ IsWarningCondition /\ ~IsCriticalCondition
                               /\ unhealthy_streak + 1 >= DOWN_HYSTERESIS_THRESHOLD -> "SUSPICIOUS"
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

      \* Down-hysteresis + leak (E5's flapping fix), exponentially backed off
      \* per repeated failed attempt (flapping-attack hardening): a
      \* non-critical unhealthy reading only demotes RECOVERING -> SOVEREIGN
      \* once it has recurred unhealthy_streak+1 >=
      \* EffectiveDownHysteresisThreshold times in a row -- doubling each
      \* time a prior RECOVERING attempt was knocked back, capped at
      \* MAX_DOWN_HYSTERESIS_THRESHOLD. ExecuteFSMTransition leaks
      \* safe_blocks by 1 (not a hard reset to 0) while absorbing the noise
      \* below, preserving partial progress.
      [] state = "RECOVERING" /\ ~IsHealthyCondition
                               /\ unhealthy_streak + 1 >= EffectiveDownHysteresisThreshold -> "SOVEREIGN"
      [] state = "RECOVERING" /\ ~IsHealthyCondition -> "RECOVERING"

      \* Exit condition when hysteresis and ZK proof are both satisfied
      [] state = "RECOVERING" /\ IsHealthyCondition  /\ safe_blocks = HYSTERESIS_WAIT /\ reanchoring_proof_valid = TRUE -> "ANCHORED"

      \* Catch-all for remaining in RECOVERING (covers safe_blocks < HYSTERESIS_WAIT and pending ZK proofs)
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

    \* failed_recovery_attempts backs off RECOVERING's down-hysteresis
    \* exponentially (EffectiveDownHysteresisThreshold above): +1 on every
    \* real RECOVERING -> SOVEREIGN regression (critical or down-hysteresis-
    \* exhausted alike -- both are a failed attempt), reset to 0 on a
    \* successful RECOVERING -> ANCHORED, saturating once growing further
    \* wouldn't change the capped effective threshold (keeps this a finite
    \* Nat, matching FSMTypeOK's 0..MAX_DOWN_HYSTERESIS_THRESHOLD bound,
    \* instead of counting attempts forever past the point they matter).
    \* Unchanged for every other transition (ANCHORED/SUSPICIOUS activity,
    \* or simply staying in RECOVERING).
    /\ failed_recovery_attempts' =
           IF state = "RECOVERING" /\ target_state = "SOVEREIGN"
           THEN IF DOWN_HYSTERESIS_THRESHOLD * Pow2(failed_recovery_attempts) >= MAX_DOWN_HYSTERESIS_THRESHOLD
                THEN failed_recovery_attempts
                ELSE failed_recovery_attempts + 1
           ELSE IF state = "RECOVERING" /\ target_state = "ANCHORED"
                THEN 0
                ELSE failed_recovery_attempts

    \* It resets to 0 automatically if transitioning out of RECOVERING or just entering it from SOVEREIGN.
    \* While staying in RECOVERING: a healthy block increments toward
    \* HYSTERESIS_WAIT as before; a non-critical unhealthy block being
    \* absorbed by down-hysteresis LEAKS one unit instead of hard-resetting
    \* to 0 -- E5's flapping fix keeps partial progress through sporadic
    \* noise rather than discarding a whole streak on one bad reading.
    /\ safe_blocks' =
           IF target_state = "RECOVERING" /\ state = "RECOVERING"
           THEN IF IsHealthyCondition
                THEN MinVal(safe_blocks + 1, HYSTERESIS_WAIT)
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

(* ======================== THE NEXT-STATE ACTION (FOR UNIT TEST) ============ *)
FSMNext == 
    \/ /\ UpdateSensors
    \/ /\ state' = CalculateNextFSMState 
       /\ state' /= state
       /\ ExecuteFSMTransition(state')
       /\ UNCHANGED <<networkSensorVars, censorshipVars>>
       /\ UNCHANGED <<reanchoring_proof_valid>>


FSMSpec == FSMInit /\ [][FSMNext]_fsmVars


(* ======================== SAFETY PROPERTIES ============================== *)
\* Safety 1: Withdrawal lock is active if and only if state is SOVEREIGN or RECOVERING
CircuitBreakerSafety ==
    WithdrawLocked <=> (state \in {"SOVEREIGN", "RECOVERING"})

\* Safety 2: The only way to exit RECOVERING is after full hysteresis + valid ZK proof
HysteresisSafety ==
    [][ (state = "RECOVERING" /\ state' = "ANCHORED")
        => (safe_blocks = HYSTERESIS_WAIT /\ reanchoring_proof_valid) ]_fsmVars

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
    (state = "RECOVERING" /\ reanchoring_proof_valid /\ IsHealthyCondition)
    ~> (state = "ANCHORED" \/ ~IsHealthyCondition \/ ~reanchoring_proof_valid)

\* Liveness 4: ZK proof must eventually be generated during recovery under healthy conditions.
ZKProofGenerationLiveness == 
    (state = "RECOVERING" /\ IsHealthyCondition) ~> (reanchoring_proof_valid = TRUE)

\* Liveness 5: Persistent network anomalies must eventually trigger a circuit-break or recovery.
PersistentEclipseResolutionLiveness == 
    ([]<> ~IsP2PQualityHealthy) ~> (state \in {"SOVEREIGN", "ANCHORED"})

=============================================================================