---------------- MODULE MC_ServerRefinementSafety ----------------
(*
 * MC_ServerRefinementSafety — TLC Safety Model Checker
 *
 * Runs TLC to verify SAFETY properties of the concrete EngramServer spec
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


(* ======================== SAFETY SANITY CHECKS (EXPECTED TO FAIL) ======================== *)

\* EXPECT FAIL: Proves the FSM successfully explores the SUSPICIOUS warning state.
Sanity_NeverSuspicious == 
    \A p \in HonestNodes : 
        decision[p] /= NilDecision => decision[p].prop.fsm_state /= "SUSPICIOUS"

\* EXPECT FAIL: Proves the Circuit Breaker is triggered and reaches SOVEREIGN.
Sanity_NeverSovereign == 
    \A p \in HonestNodes : 
        decision[p] /= NilDecision => decision[p].prop.fsm_state /= "SOVEREIGN"

\* EXPECT FAIL: Proves the system enters the RECOVERING phase.
Sanity_NeverRecovering == 
    \A p \in HonestNodes : 
        decision[p] /= NilDecision => decision[p].prop.fsm_state /= "RECOVERING"

\* EXPECT FAIL: Proves the Byzantine Data Withholding attack actually fires (E_QC without M_QC).
Sanity_NoDataWithholding == 
    \A eqc \in {q \in quorum_certs : q.type = "E_QC"} :
        \E mqc \in quorum_certs : 
            /\ mqc.type = "M_QC" 
            /\ mqc.round = eqc.round 
            /\ mqc.caller = eqc.caller

\* EXPECT FAIL: Proves the censorship-resistance counter increments under network stress.
Sanity_NoCensorship == 
    \A p \in HonestNodes, tx \in Method : tx_ignored_rounds[p][tx] = 0


(* ======================== ABLATION SANITY CHECKS (EXPECTED TO FAIL) ======================== *)

\* EXPECT FAIL: Proves the attacker attempts a withdrawal while in SOVEREIGN or RECOVERING state.
Sanity_NeverAttemptWithdrawalLeakage ==
    \A r \in Rounds : \A m \in msgs_propose[r] :
        (m.proposal.fsm_state \in {"SOVEREIGN", "RECOVERING"}) => (m.proposal.value /= "TX_WITHDRAWAL")

\* EXPECT FAIL: Proves the attacker attempts to propose a block with withheld DA (attestation = FALSE).
Sanity_NeverProposeWithheldData ==
    \A r \in Rounds : \A m \in msgs_propose[r] :
        m.proposal.da_receipt.attestation = TRUE

\* EXPECT FAIL: Proves the attacker attempts a cross-mode double spend using a stale BTC checkpoint.
Sanity_NeverAttemptCrossModeDoubleSpend ==
    \A r \in Rounds : \A m \in msgs_propose[r] :
        (m.proposal.fsm_state = "SOVEREIGN") => (m.proposal.btc_receipt.checkpoint_block_height > h_btc_anchored)

\* EXPECT FAIL: Proves the system experiences a network degradation while accumulating safe_blocks (flapping trigger).
Sanity_NeverFlapInRecovering ==
    ~(state = "RECOVERING" /\ safe_blocks > 0 /\ ~IsHealthyCondition)


(* ======================== STATE SPACE PRUNING CONSTRAINT ================= *)
\* Bounds are deliberately tighter than Liveness to keep safety runs tractable.
StateSpaceLimit ==
    \* Tendermint bounds
    /\ \A n \in MC_Honest : round[n] <= MAX_ROUND
    /\ real_time <= MAX_TIMESTAMP
    /\ h_btc_current <= MAX_BTC_HEIGHT
    /\ h_engram_current <= MAX_ENGRAM_HEIGHT

    \* Chain height bounds (monotone by construction, but TLC needs explicit caps)
    /\ h_engram_verified <= h_engram_current
    /\ h_btc_submitted <= h_btc_current
    /\ h_btc_anchored <= h_btc_submitted

    \* P2P network size
    /\ Cardinality(active_peers) \in {2, 3}
    /\ Cardinality(anchor_peers) <= 3
    /\ Cardinality(blacklisted_peers) <= 2
    /\ peer_churn_rate \in {0, MAX_CHURN_RATE} 
    /\ avg_peer_tenure \in {MIN_AVG_TENURE, MIN_AVG_TENURE + 2} 
    /\ peer_latency \in {0, MAX_PEER_LATENCY}

    /\ is_btc_spv_failed \in BOOLEAN
    /\ is_das_failed \in BOOLEAN
    /\ is_attestation_failed \in BOOLEAN
    /\ state \in {"ANCHORED", "SUSPICIOUS", "SOVEREIGN", "RECOVERING"}



(* ======================== TRACE LOG FILTER (FLAT MINIMALIST) ======================== *)
MyTraceView == 
    [
        _1_action |-> action,
        _2_fsm    |-> state,
        
        _3_sensors |-> [ 
            btc_gap    |-> h_btc_current - h_btc_anchored, 
            spv_failed |-> is_btc_spv_failed,
            da_failed  |-> is_das_failed, 
            peers      |-> active_peers 
        ],

        _4_proposals |-> { [
            round           |-> m.round,
            src             |-> m.src, 
            da_valid        |-> m.proposal.da_receipt.attestation, 
            btc_anchored    |-> m.proposal.btc_receipt.checkpoint_block_height
        ] : m \in UNION {msgs_propose[r] : r \in 0..MAX_ROUND} },

        _5_decisions |-> [p \in HonestNodes |-> 
                            IF decision[p] = NilDecision THEN "PENDING"
                            ELSE decision[p].prop.value],

        _6_prevotes  |-> { [
            round |-> m.round, 
            src   |-> m.src, 
            vote  |-> m.id.value
        ] : m \in UNION {msgs_prevote[r] : r \in 0..MAX_ROUND} }
    ]

(* ======================== REFINEMENT PROPERTIES ========================== *)
RefinementSafety   == AbstractConsensus!Safety

=============================================================================
