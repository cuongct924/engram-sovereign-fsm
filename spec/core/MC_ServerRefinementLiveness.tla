---------------- MODULE MC_ServerRefinementLiveness ----------------
(*
 * MC_ServerRefinementLiveness — TLC Liveness Model Checker
 *
 * Runs TLC to verify LIVENESS properties of the concrete EngramServer spec
 * against the abstract EngramConsensus (LiDO) spec via EngramServerRefinement.
 *)
EXTENDS EngramServer, EngramServerRefinement, TLC, Sequences

CONSTANTS n1, n2, n3, n4
\* CONSTANTS n1, n2, n3, n4, n5, n6, n7

ASSUME QuorumOverlap


(* ======================== NETWORK CONFIGURATION ========================== *)
MC_Nodes  == {n1, n2, n3, n4}
MC_Method == {"TX_NORMAL", "TX_WITHDRAWAL"}
MC_Byzantine == {n1}
MC_Honest   == MC_Nodes \ MC_Byzantine

\* MC_Nodes == {n1, n2, n3, n4, n5, n6, n7}
\* MC_Method == {"TX_NORMAL", "TX_WITHDRAWAL"}
\* MC_Byzantine == {n6, n7}
\* MC_Honest   == MC_Nodes \ MC_Byzantine

(* ======================== ROTATIONAL LEADER SCHEDULE ===================== *)
\* Round-robin proposer: node at position (r mod 4) + 1 in the sequence
MC_NodeSeq  == <<n1, n2, n3, n4>>
MC_Proposer == [r \in 0..5 |-> MC_NodeSeq[(r % 4) + 1]]

\* \* Round-robin proposer: node at position (r mod 7) + 1 in the sequence
\* MC_NodeSeq == <<n1, n2, n3, n4, n5, n6, n7>>
\* MC_Proposer == [r \in 0..10 |-> MC_NodeSeq[(r % 7) + 1]]

(* ======================== INIT & NEXT ==================================== *)
MC_ServerInit == ServerInit
MC_ServerNext == ServerNext


(* ======================== LIVENESS SANITY CHECKS (EXPECTED TO FAIL) ======================== *)

\* EXPECT FAIL: Proves the system eventually enters a critical condition and breaks the circuit.
Sanity_NeverBreakCircuit == 
    [] ~(IsCriticalCondition /\ state = "SOVEREIGN")

\* EXPECT FAIL: Proves the system eventually completes hysteresis and fully recovers to ANCHORED.
Sanity_NeverRecoverFully == 
    [] ~(state = "ANCHORED" /\ safe_blocks = HYSTERESIS_WAIT)

\* EXPECT FAIL: Proves the Global Stabilisation Time (GST) is actually reached in the trace.
Sanity_NeverReachGST == 
    [] ~GSTReached

\* EXPECT FAIL: Proves a transaction is eventually forced into a block due to prolonged censorship.
Sanity_NeverForceTx == 
    [] ~(\E p \in HonestNodes, tx \in Method : tx_ignored_rounds[p][tx] >= MAX_IGNORE_ROUNDS)


(* ======================== FAIRNESS CONDITIONS ============================ *)
\* Weak fairness on time advance ensures clocks always eventually tick.
\* Weak fairness on message processing ensures every enabled action eventually fires (prevents "unfair" stuttering in liveness proofs).
MC_ServerFairness ==
    /\ WF_serverVars(ServerAdvanceRealTime)
    /\ \A p \in MC_Honest : WF_serverVars(ServerMessageProcessing(p))

\* Full liveness specification: safety behaviour + fairness assumptions
MC_ServerSpec ==
    MC_ServerInit /\ [][MC_ServerNext]_serverVars /\ MC_ServerFairness


(* ======================== STATE SPACE PRUNING CONSTRAINT ================= *)
\* Bounds are tighter than Safety run to keep liveness checking tractable.
StateSpaceLimit ==
    \* Tendermint bounds
    /\ \A n \in MC_Honest : round[n] <= MAX_ROUND
    /\ real_time <= MAX_TIMESTAMP

    \* Chain height bounds (monotone by construction, but TLC needs explicit caps)
    /\ h_btc_current    <= MAX_BTC_HEIGHT
    /\ h_engram_current <= MAX_ENGRAM_HEIGHT
    /\ h_engram_verified <= h_engram_current
    /\ h_btc_submitted   <= h_btc_current
    /\ h_btc_anchored    <= h_btc_submitted

    \* P2P network size
    /\ Cardinality(active_peers) \in {2, 3}
    /\ Cardinality(anchor_peers) <= 3
    /\ Cardinality(blacklisted_peers) <= 2
    /\ peer_churn_rate <= MAX_CHURN_RATE + 2
    /\ avg_peer_tenure <= MIN_AVG_TENURE + 100
    /\ peer_latency    <= MAX_PEER_LATENCY + 10

    /\ is_btc_spv_failed \in BOOLEAN
    /\ is_das_failed \in BOOLEAN
    /\ is_attestation_failed \in BOOLEAN
    /\ state \in {"ANCHORED", "SUSPICIOUS", "SOVEREIGN", "RECOVERING"}


(* ======================== REFINEMENT PROPERTIES ========================== *)
RefinementLiveness == AbstractConsensus!Liveness
=============================================================================
