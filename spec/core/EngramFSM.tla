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
    MAX_SUSPICIOUS_TIME     \* Maximum ticks/blocks the system tolerates in SUSPICIOUS state before escalating to SOVEREIGN

ASSUME
    /\ SUSPICIOUS_THRESHOLD \in Nat
    /\ SOVEREIGN_THRESHOLD \in Nat
    /\ DA_THRESHOLD \in Nat
    /\ HYSTERESIS_WAIT \in Nat
    /\ MIN_PEERS \in Nat
    /\ MIN_SUBNET_DIVERSITY \in Nat
    /\ MIN_ANCHOR_PEERS  \in Nat
    /\ MAX_CHURN_RATE \in Nat
    /\ MIN_AVG_TENURE \in Nat
    /\ MAX_PEER_LATENCY \in Nat
    /\ SUSPICIOUS_THRESHOLD < SOVEREIGN_THRESHOLD

\* Helper: returns the smaller of two integers
\* @type: (Int, Int) => Int;
MinVal(a, b) == IF a < b THEN a ELSE b


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

\* Hard failure: BTC gap crossed threshold OR Total Loss of Anchor Peers (Complete Eclipse)
IsCriticalCondition == 
    \/ IsBTCGapSovereign
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
    /\ IsDAHealthy
    /\ IsP2PQualityHealthy


(* ======================== TYPE INVARIANT & SANITY CHECK ================================== *)
FSMTypeOK == 
    /\ state \in {"ANCHORED", "SUSPICIOUS", "SOVEREIGN", "RECOVERING"}
    /\ btc_gap >= 0
    /\ da_gap >= 0
    /\ is_das_failed \in BOOLEAN
    /\ IsFiniteSet(active_peers)
    /\ IsFiniteSet(anchor_peers)
    /\ IsFiniteSet(blacklisted_peers)
    /\ peer_churn_rate \in Nat /\ avg_peer_tenure \in Nat /\ peer_latency \in Nat
    /\ safe_blocks \in 0..HYSTERESIS_WAIT
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
    /\ UNCHANGED <<state, safe_blocks, suspicious_duration>>
    /\ UNCHANGED <<censorshipVars>>


(* ======================== FSM RULE ENGINE ================================= *)
\* Pure function: given the current sensor readings, compute the next FSM state.
\* This is called by EngramServer at every decision point.
CalculateNextFSMState == 
    CASE state = "ANCHORED"   /\ IsCriticalCondition -> "SOVEREIGN"
      [] state = "ANCHORED"   /\ IsWarningCondition /\ ~IsCriticalCondition -> "SUSPICIOUS"
      [] state = "SUSPICIOUS" /\ IsCriticalCondition -> "SOVEREIGN"
      
      \* Gray Failure Timeout. Force circuit-break if system stays suspicious too long.
      [] state = "SUSPICIOUS" /\ suspicious_duration >= MAX_SUSPICIOUS_TIME -> "SOVEREIGN"
      
      [] state = "SUSPICIOUS" /\ IsHealthyCondition  -> "ANCHORED"
      [] state = "SOVEREIGN"  /\ IsHealthyCondition  -> "RECOVERING"
      
      \* Fallback to SOVEREIGN if network degrades while in RECOVERING
      [] state = "RECOVERING" /\ ~IsHealthyCondition -> "SOVEREIGN"
      
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
                                 
    \* It resets to 0 automatically if transitioning out of RECOVERING or just entering it from SOVEREIGN
    /\ safe_blocks' = 
           IF target_state = "RECOVERING" /\ state = "RECOVERING"
           THEN MinVal(safe_blocks + 1, HYSTERESIS_WAIT)
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