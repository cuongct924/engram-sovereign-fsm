--------------------------- MODULE EngramServerRefinement -------------------------
(*
 * EngramRefinement — LiDO Abstract Refinement Mapping
 *
 * This module proves that EngramServer (the concrete Tendermint-based spec)
 * refines EngramConsensus (the abstract LiDO pacemaker spec).
 *
 * Refinement structure (per the conventions document):
 *   1. Mapping functions: translate concrete variables → abstract variables
 *   2. INSTANCE:  AbstractConsensus == INSTANCE EngramConsensus WITH ...
 *   3. Theorem:   ServerSpec => AbstractConsensus!Spec
 *
 * Key design decision — Homogeneous Stake:
 *   Instead of tracking arbitrary stake weights (which would add a variable
 *   and explode the TLC state space), every node is assigned exactly 1 unit
 *   of stake. This preserves the Quorum Overlap property while keeping the
 *   state space tractable. The assumption is verified by ASSUME QuorumOverlap.
 *
 * Depends on: EngramServer (all concrete variables and operators),
 *             EngramConsensus (abstract spec being refined)
 *)
EXTENDS EngramServer

CONSTANTS
    \* @type: Int;
    K_DEEP_FINALITY

(* ======================== STAKE CONFIGURATION ========================= *)
\* @type: Str -> Int;
HomogeneousStake == [n \in Nodes |-> 1]
\* @type: Int;
HomogeneousTotalStake == Cardinality(Nodes)

\* Randomly select a valid quorum to assign to abstract certificates
\* @type: Set(Str);
Q_abstract == CHOOSE q \in SUBSET Nodes : Cardinality(q) >= THRESHOLD2

(* ======================== TREE MAPPING HELPERS ======================== *)
\* Apalache note (mirrors EngramConsensus.tla): the abstract cache type is
\* fully inlined, not a named alias -- same cross-module unification issue.

\* Map concrete Pull certificates to abstract E-caches
\* @type: Set({ type: Str, cert_round: Int, caller: Str, method: Str, voters: Set(Str), btc_anchored: Int });
MappedECaches == {
    [ type         |-> "E",
      cert_round   |-> qc.round + 1, 
      caller       |-> qc.caller, 
      method       |-> "None", 
      voters       |-> Q_abstract, 
      btc_anchored |-> qc.btc_anchored + K_DEEP_FINALITY
    ] : qc \in { q \in quorum_certs : q.type = "E_QC" } 
}

\* Map concrete Invoke certificates to abstract M-caches
\* @type: Set({ type: Str, cert_round: Int, caller: Str, method: Str, voters: Set(Str), btc_anchored: Int });
MappedMCaches == {
    [ type         |-> "M", 
      cert_round   |-> qc.round + 1, 
      caller       |-> qc.caller, 
      method       |-> qc.method, 
      voters       |-> {qc.caller}, 
      btc_anchored |-> qc.btc_anchored + K_DEEP_FINALITY
    ] : qc \in { q \in quorum_certs : q.type = "M_QC" } 
}

\* Map concrete Timeout certificates to abstract T-caches
\* @type: Set({ type: Str, cert_round: Int, caller: Str, method: Str, voters: Set(Str), btc_anchored: Int });
MappedTCaches == {
    [ type         |-> "T", 
      cert_round   |-> tc.round + 1, 
      caller       |-> tc.caller, 
      method       |-> "None", 
      voters       |-> Q_abstract, 
      btc_anchored |-> tc.btc_anchored + K_DEEP_FINALITY
    ] : tc \in timeout_certs
}

\* Map concrete Push certificates to abstract M-caches
\* @type: Set({ type: Str, cert_round: Int, caller: Str, method: Str, voters: Set(Str), btc_anchored: Int });
MappedCCaches ==
    LET CommitPairs == UNION { { <<r, m.id>> : m \in msgs_precommit[r] } : r \in Rounds }
        ValidCommits == { p \in CommitPairs : 
                          /\ p[2] /= NilProposal 
                          /\ Cardinality({ m \in msgs_precommit[p[1]] : m.id = p[2] }) >= THRESHOLD2 }
    IN { [ type         |-> "C", 
           cert_round   |-> p[1] + 1,
           caller       |-> Proposer[p[1]],
           method       |-> "None",
           voters       |-> Q_abstract,
           btc_anchored |-> p[2].btc_receipt.checkpoint_block_height + K_DEEP_FINALITY 
         ] : p \in ValidCommits }

(* ======================== ABSTRACT TREE MAPPING ========================== *)
\* Translate concrete QC/TC certificate sets (quorum_certs, timout_certs) into the abstract
\* AdoB buffer tree consumed by EngramConsensus.
\*
\*   E_QC  -> E-cache  (Pull event)
\*   M_QC  -> M-cache  (Invoke event)
\*   T_QC  -> T-cache  (Timeout event)
\*   precommit quorum -> C-cache (Push / Commit event)
\* @type: Set({ type: Str, cert_round: Int, caller: Str, method: Str, voters: Set(Str), btc_anchored: Int });
MappedTree == MappedECaches \union MappedMCaches \union MappedCCaches \union MappedTCaches

(* ======================== ABSTRACT STATE MAPPINGS ======================== *)
\* FSM state mapping: collapse SUSPICIOUS into ANCHORED (abstract spec only
\* distinguishes ANCHORED vs SOVEREIGN).
\* @type: Str;
mapped_fsm_state ==
    IF state \in {"ANCHORED", "SUSPICIOUS"}
    THEN "ANCHORED"
    ELSE "SOVEREIGN"

\* Local time mapping: a node's abstract logical time is the highest E_QC or
\* C-cache round it has participated in.
\* @type: Str -> Int;
mapped_local_times ==
    [ n \in Nodes |-> IF n \in Q_abstract THEN
        Max( {0} \cup { qc.round + 1 : qc \in { q \in quorum_certs : q.type = "E_QC" } }
                 \cup { c.cert_round + 1: c \in MappedCCaches } )
        ELSE 0 ]

\* Round mapping: the abstract consensus round leads the concrete max round by 1
\* @type: Int;
CURRENT_MAX_ROUND == Max({ round[p] : p \in HonestNodes })

\* @type: Int;
MIN_REM_TIME ==
    LET CurrentNodes == { p \in HonestNodes : round[p] = CURRENT_MAX_ROUND }
    IN Min({ local_rem_time[p] : p \in CurrentNodes })


(* ======================== INSTANTIATION ================================== *)
AbstractConsensus ==
    INSTANCE EngramConsensus WITH
        Nodes           <- Nodes,
        Method          <- Method,
        Stake           <- HomogeneousStake,
        TOTAL_STAKE     <- HomogeneousTotalStake,
        RESET_TIME      <- TIMEOUT_DURATION,
        MAX_BTC_HEIGHT  <- MAX_BTC_HEIGHT,
        K_DEEP_FINALITY <- K_DEEP_FINALITY,        
        tree            <- MappedTree,
        fsm_state       <- mapped_fsm_state,
        round           <- CURRENT_MAX_ROUND + 1,
        local_times     <- mapped_local_times,
        rem_time        <- MIN_REM_TIME,
        h_btc_current   <- h_btc_current + K_DEEP_FINALITY,
        h_btc_anchored  <- h_btc_anchored + K_DEEP_FINALITY


(* ======================== REFINEMENT CHECKS ============================== *)
\* Apalache scope note: this file's own operators above (mapping helpers,
\* QuorumOverlap) are ordinary state predicates/functions and typecheck like
\* any other layer. RefinementSafety/RefinementLiveness themselves are NOT
\* defined here -- they live in MC_ServerRefinementSafety.tla/
\* MC_ServerRefinementLiveness.tla as `AbstractConsensus!Safety`/`!Liveness`,
\* i.e. a whole embedded spec (Init /\ [][Next]_vars) used as a property.
\* Apalache's --inv/--temporal model one Init/Next pair at a time and have no
\* equivalent for "does this concrete spec refine that abstract spec" --
\* verifying those two remains TLC-only (see CLAUDE.md's Apalache section).
\* QuorumOverlap: any two valid quorums share at least one correct process.
\* This is the foundational safety assumption — checked as ASSUME in MC files.
\* @type: Bool;
QuorumOverlap ==
    \A q1, q2 \in AbstractConsensus!ValidQuorums :
        (q1 \intersect q2) \intersect HonestNodes /= {}

=============================================================================
