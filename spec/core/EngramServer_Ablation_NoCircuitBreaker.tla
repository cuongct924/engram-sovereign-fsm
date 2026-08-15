--------------------------- MODULE EngramServer_Ablation_NoCircuitBreaker ---------------------------
(*
 * EngramServer — Concrete Protocol Integration Layer
 *
 * Bridges the abstract Tendermint BFT core (EngramTendermint) with the
 * Engram-specific application logic:
 *   - FSM-aware proposal construction (ServerInsertProposal)
 *   - LiDO certificate generation (E_QC, M_QC, T_QC)
 *   - Post-decision FSM state synchronisation (ServerUponProposalInPrecommitNoDecision)
 *   - Hybrid safety invariants (FSM <-> consensus cross-checks)
 *   - Liveness properties under GST
 *
 * The LiDO abstract refinement mapping lives in EngramServerRefinement.tla.
 *
 * Depends on: EngramFSM, EngramTendermint, Naturals, FiniteSets
 *)
EXTENDS Naturals, FiniteSets, EngramFSM, EngramTendermint_Ablation_NoCircuitBreaker

CONSTANTS
    \* @type: Set(Str);
    Nodes,      \* Set of all nodes in the abstract consensus layer
    \* @type: Set(Str);
    Method,     \* Set of valid transaction methods (e.g. {"TX_NORMAL", "TX_WITHDRAWAL"})
    \* @type: Int;
    RESET_TIME  \* Pacemaker reset time (passed through to EngramConsensus)


\* Aggregate tuple for EngramServer
serverVars ==
    <<tendermintCoreVars, temporalVars, invariantVars, 
        bookkeepingVars, certificateVars, fsmVars, networkSensorVars, censorshipVars>>


(* ======================== HELPERS ========================================= *)
\* TRUE once the network has accumulated 2f+1 matching precommits for any value.
\* Used as a guard to stop issuing new proposals after a block is closed.
\* @type: Bool;
GlobalDecisionExists ==
    \E r \in Rounds :
        \E m \in msgs_precommit[r] :
            /\ m.id /= NilProposal
            /\ Cardinality({ msg \in msgs_precommit[r] : msg.id = m.id }) >= THRESHOLD2


(* ======================== SERVER HOOKS (INTEGRATION LAYER) ================ *)

\* Hook 1: Leader builds and injects a proposal -> emits E_QC (maps to Abstract Pull).
\*
\* State-space control: all non-determinism over ValidValues, proof_search_space,
\* and validValue is resolved HERE, before entering the black-box Tendermint core.
\* @type: (Str) => Bool;
ServerInsertProposal(p) ==
    /\ ~GlobalDecisionExists
    /\ p = Proposer[round[p]]
    /\ step[p] = "PROPOSE"
    /\ \A m \in msgs_propose[round[p]] : m.src /= p
    /\ \E v \in ValidValues :
           LET
               target_state == CalculateNextFSMState

               da_receipt == [
                   published_block_height   |-> h_engram_verified,
                   attestation              |-> ~is_attestation_failed
               ]

               btc_receipt == [
                    checkpoint_block_height |-> h_btc_anchored,
                    checkpoint_block_hash   |-> ExpectedBlockHash(h_btc_anchored)  
               ]

               \* ZK proof search space: only open the TRUE branch once
               \* hysteresis is satisfied, to avoid spurious re-anchoring paths.
               proof_search_space ==
                   IF state = "RECOVERING" /\ safe_blocks >= HYSTERESIS_WAIT
                   THEN {TRUE, FALSE}
                   ELSE {FALSE}
           IN
           \E proof_found \in proof_search_space :
               LET prop ==
                       IF valid_value[p] /= NilProposal
                       THEN valid_value[p]
                       ELSE Proposal(v, local_clock[p], round[p], target_state,
                                     da_receipt, btc_receipt, proof_found,
                                     IsHealthyCondition)
               IN
               \* Inject the concrete proposal into Tendermint
               /\ InsertProposal(p, prop)

               \* Emit E_QC for the LiDO abstract pacemaker
               /\ LET new_EQC == [
                          type          |-> "E_QC",
                          round         |-> round[p],
                          caller        |-> p,
                          method        |-> "None",
                          btc_anchored  |-> h_btc_current ]
                  IN quorum_certs' = quorum_certs \cup {new_EQC}
    \* Apalache note: InsertProposal (EngramTendermint.tla) already asserts
    \* UNCHANGED for tendermintCoreVars/temporalVars/propAuditVars/
    \* censorshipVars/msgs_prevote/msgs_precommit/msgs_timeout/evidence
    \* itself -- only timeout_certs (Server-layer, InsertProposal doesn't
    \* know about it) and fsmVars/networkSensorVars are genuinely new here.
    /\ UNCHANGED <<timeout_certs>>
    /\ UNCHANGED <<fsmVars, networkSensorVars>>


\* Hook 2: Proposer votes for its own proposal -> emits M_QC (maps to Abstract Invoke).
\* @type: (Str) => Bool;
ServerProposerVotes(p) ==
    /\ \/ UponProposalInPropose(p)
       \/ UponProposalInProposeAndPrevote(p)
    /\ IF p = Proposer[round[p]]
          /\ \E m \in msgs_propose[round[p]] : m.src = p
       THEN
           LET
               prop  == (CHOOSE m \in msgs_propose[round[p]] : m.src = p).proposal
               new_MQC == [
                   type         |-> "M_QC",
                   round        |-> round[p],
                   caller       |-> p,
                   method       |-> prop.value,
                   btc_anchored |-> h_btc_current ]
           IN
           /\ quorum_certs' = quorum_certs \cup {new_MQC}
           /\ timeout_certs' = timeout_certs
       ELSE
           /\ quorum_certs' = quorum_certs
           /\ timeout_certs' = timeout_certs


\* Hook 3: Intercept the decision moment -> trigger FSM transition + state sync.
\*
\* On every block commit, the decided proposal's FSM state, BTC anchor, and
\* DA receipt are written back into the local sensor variables so that the
\* next proposal reflects the globally agreed-upon chain view.
\* @type: (Str) => Bool;
ServerUponProposalInPrecommitNoDecision(p) ==
    \* Step 1: Execute core Tendermint decision logic
    /\ UponProposalInPrecommitNoDecision(p)

    \* Step 2: Extract the just-decided proposal (the majority's agreed truth)
    /\ LET
           r    == round[p]
           msg  == CHOOSE m \in msgs_propose[r] :
                       m.src = Proposer[r] /\ m.type = "PROPOSAL"
           prop == msg.proposal
       IN
           \* Step 3: Drive FSM transition and update anchored heights
           /\ ExecuteFSMTransition(prop.fsm_state)
           /\ h_btc_anchored'    = prop.btc_receipt.checkpoint_block_height
           /\ h_engram_verified' = prop.da_receipt.published_block_height

           \* Step 4: ZK proof submission tracking.
           \* Mark proof as submitted (pending Bitcoin confirmation).
           /\ IF prop.fsm_state = "RECOVERING" /\ prop.zk_proof_ref = TRUE
              THEN
                  /\ h_btc_submitted' = h_btc_current
                  /\ reanchoring_proof_valid' = FALSE   \* Awaiting Bitcoin confirmation
              ELSE
                  /\ h_btc_submitted' = h_btc_submitted
                  /\ reanchoring_proof_valid' = reanchoring_proof_valid

           \* Step 5: Force-sync local sensors when ANCHORED.
           \* If the network majority is in ANCHORED, suppress any local false alarms.
           /\ IF state = "RECOVERING" /\ prop.fsm_state = "ANCHORED"
              THEN
                  /\ h_btc_current' = prop.btc_receipt.checkpoint_block_height
                  /\ h_engram_current' = prop.da_receipt.published_block_height
                  /\ is_das_failed' = FALSE
                  /\ is_attestation_failed' = FALSE
                  /\ is_btc_spv_failed' = FALSE
              ELSE
                  /\ UNCHANGED <<h_btc_current, is_btc_spv_failed>> 
                  /\ UNCHANGED <<h_engram_current, is_das_failed, is_attestation_failed>>

           \* ExecuteFSMTransition already fully determines safe_blocks'/
           \* suspicious_duration' via formula -- do not also assert them
           \* UNCHANGED here (an unsatisfiable contradiction whenever the
           \* hysteresis counter actually needs to change).
           /\ UNCHANGED <<p2pHealthSensorVars>>

    \* Step 6: Keep pacemaker certificates unchanged (censorshipVars is
    \* already covered by UponProposalInPrecommitNoDecision's own UNCHANGED
    \* -- see the Apalache note on ServerUpdateEnvironment above).
    /\ UNCHANGED <<certificateVars>>

\* Hook 4: 2f+1 timeout votes -> emit T_QC (maps to Abstract Timeout)
\* @type: (Str) => Bool;
ServerUponTimeoutCert(p) ==
    \* 1. Check timeout quorum
    /\  LET unique_senders == { m.src : m \in msgs_timeout[round[p]] }
        IN Cardinality(unique_senders) >= THRESHOLD2
    /\ ~\E tqc \in timeout_certs : tqc.round = round[p]
    \* 2. Emit T_QC for the LiDO abstract pacemaker
    /\  LET new_TQC == [
               type         |-> "T_QC",
               round        |-> round[p],
               caller       |-> p,
               btc_anchored |-> h_btc_current ]
        IN timeout_certs' = timeout_certs \cup {new_TQC}
    /\ UNCHANGED <<quorum_certs, fsmVars, networkSensorVars, censorshipVars>>
    /\ UNCHANGED <<tendermintCoreVars, temporalVars, bookkeepingVars, invariantVars>>


(* ======================== ACTION AGGREGATION ============================== *)
\* Apalache note: the 11 pass-through branches below were flattened from a
\* separate ServerPassThrough(p) operator (wrapped with one shared
\* UNCHANGED conjunct) into one level, distributing UNCHANGED into each
\* leaf directly -- Apalache's assignment analysis cannot resolve the
\* resulting 2-level-nested disjunction (each of the 4 server hooks passes
\* its own assignment analysis in isolation, but combining all 5 branches
\* fails as "spurious"); TLC has no such requirement, mirroring
\* TendermintNext/MC_TendermintNext's own flat structure.
\* @type: (Str) => Bool;
ServerMessageProcessing(p) ==
    \* 1. Pass-through actions (formerly ServerPassThrough(p))
    \/ ReceiveProposal(p) /\ UNCHANGED <<certificateVars, fsmVars, networkSensorVars>>
    \/ UponProposalInPropose(p) /\ UNCHANGED <<certificateVars, fsmVars, networkSensorVars>>
    \/ UponProposalInProposeAndPrevote(p) /\ UNCHANGED <<certificateVars, fsmVars, networkSensorVars>>
    \/ UponQuorumOfPrevotesAny(p) /\ UNCHANGED <<certificateVars, fsmVars, networkSensorVars>>
    \/ UponProposalInPrevoteOrCommitAndPrevote(p) /\ UNCHANGED <<certificateVars, fsmVars, networkSensorVars>>
    \/ /\ UponQuorumOfPrecommitsAny(p)
       /\ ~GlobalDecisionExists
       /\ UNCHANGED <<certificateVars, fsmVars, networkSensorVars>>
    \/ OnTimeoutPropose(p) /\ UNCHANGED <<certificateVars, fsmVars, networkSensorVars>>
    \/ OnQuorumOfNilPrevotes(p) /\ UNCHANGED <<certificateVars, fsmVars, networkSensorVars>>
    \/ OnRoundCatchup(p) /\ UNCHANGED <<certificateVars, fsmVars, networkSensorVars>>
    \/ UponfPlusOneTimeoutsAny(p) /\ UNCHANGED <<certificateVars, fsmVars, networkSensorVars>>
    \/ OnLocalTimerExpire(p) /\ UNCHANGED <<certificateVars, fsmVars, networkSensorVars>>

    \* 2. Hook 1: leader creates block (emits E_QC). ServerInsertProposal
    \* already asserts UNCHANGED <<fsmVars, networkSensorVars>> itself --
    \* repeating it here was the actual root cause of the Apalache
    \* assignment-analysis failure noted above.
    \/ ServerInsertProposal(p)

    \* 3. Hook 2: leader votes for itself (emits M_QC)
    \/ /\ ServerProposerVotes(p)
       /\ UNCHANGED <<fsmVars, networkSensorVars>>

    \* 4. Hook 3: block decision (emits C_QC, syncs FSM)
    \/ ServerUponProposalInPrecommitNoDecision(p)

    \* 5. Hook 4: timeout (emits T_QC)
    \/ ServerUponTimeoutCert(p)


(* ======================== SPECIFICATION (INIT & NEXT) ===================== *)
\* @type: Bool;
ServerInit ==
    /\ TendermintInit
    /\ FSMInit
    /\ quorum_certs = {} 
    /\ timeout_certs = {}

\* Pure clock tick: advances real_time/local_clock/local_rem_time only, leaving
\* every sensor/FSM variable untouched. Split from ServerUpdateEnvironment
\* below so each concrete step maps to exactly one abstract LiDO action under
\* the refinement in EngramServerRefinement.tla — a single step doing both
\* (the original ServerAdvanceRealTime, which optionally ran UpdateSensors in
\* the same transition as the clock tick) was found via RefinementSafety to
\* violate [][Next]_vars at depth 2: no single abstract action (Elapse/
\* TimeoutStartNext vs. UpdateEnv) permits changing both rem_time and
\* h_btc_current at once, since rem_time <- MIN_REM_TIME tracks the clock
\* tick and h_btc_current tracks the sensor update.
\* Guard mirrors the abstract Elapse precondition exactly: block the clock
\* while an E_QC or M_QC (E-cache/M-cache once mapped, regardless of caller —
\* Elapse's own guard does not distinguish honest from Byzantine) already
\* exists for the current round, i.e. Pull or Invoke has started and the
\* pacemaker must wait for Push before elapsing further. The previous guard
\* only blocked on a Byzantine E_QC lacking a matching Byzantine M_QC, which
\* is a strictly narrower (and, per RefinementSafety, incorrect) condition:
\* it let the clock advance past an honest OR a fully-formed Byzantine E_QC+
\* M_QC pair at the current round, which Elapse itself forbids.
\* @type: Bool;
ServerAdvanceRealTime ==
    /\ AdvanceRealTime
    /\ LET current_max_round == Max({round[p] : p \in HonestNodes}) IN
       ~\E qc \in quorum_certs : qc.type \in {"E_QC", "M_QC"} /\ qc.round = current_max_round
    /\ UNCHANGED <<fsmVars, networkSensorVars>>

\* Pure environment/sensor update: leaves the clock (and everything else
\* AdvanceRealTime would touch) unchanged. See note above ServerAdvanceRealTime.
\* @type: Bool;
\* Apalache note: censorshipVars is not repeated in this UNCHANGED --
\* UpdateSensors (EngramFSM.tla) already asserts UNCHANGED <<censorshipVars>>
\* itself. TLC tolerates the redundant conjunct fine, but Apalache's
\* assignment analysis flags a second assignment to the same variable
\* within one branch as an error ("spurious").
ServerUpdateEnvironment ==
    /\ UpdateSensors
    /\ UNCHANGED <<tendermintCoreVars, temporalVars>>
    /\ UNCHANGED <<msgsBroadcastVars, propAuditVars, evidence>>
    /\ UNCHANGED <<invariantVars, certificateVars>>
    /\ action' = "UpdateEnvironment"

\* Apalache note: quorum_certs is a single Set, so every member (E_QC and
\* M_QC alike) must share one uniform record shape -- new_EQC below adds
\* `method |-> "None"` for that reason, matching ServerInsertProposal's E_QC.
\* Harmless: MappedECaches (EngramServerRefinement.tla) hardcodes
\* `method |-> "None"` for every E_QC regardless of what's stored here, and
\* no other operator reads `.method` off a type = "E_QC" record.
\* @type: Bool;
ServerByzantinePull ==
    \E r \in Rounds :
        /\ Proposer[r] \in ByzantineNodes
        /\ msgs_propose[r] = {}
        /\ ~\E q \in quorum_certs : q.type = "E_QC" /\ q.round = r /\ q.caller = Proposer[r]
        /\ LET new_EQC == [
                type |-> "E_QC",
                round |-> r,
                caller |-> Proposer[r],
                method |-> "None",
                btc_anchored |-> h_btc_current
            ]
           IN quorum_certs' = quorum_certs \cup {new_EQC}
        /\ UNCHANGED <<tendermintCoreVars, timeout_certs>>
        /\ UNCHANGED <<temporalVars, bookkeepingVars, invariantVars>>
        /\ UNCHANGED <<fsmVars, networkSensorVars, censorshipVars>>


\* @type: Bool;
ServerByzantineDataWithholding ==
    /\ ByzantineDataWithholding
    /\ LET r == CHOOSE rnd \in Rounds : msgs_propose[rnd] /= msgs_propose'[rnd]
           m == CHOOSE msg \in msgs_propose'[r] : msg.src = Proposer[r]
       IN 
       \* LiDO's math requires an E_QC from step 1 before this can proceed.
       /\ \E eqc \in quorum_certs : eqc.type = "E_QC" /\ eqc.round = r /\ eqc.caller = Proposer[r]
       \* Emits M_QC to complete the record.
       /\ LET new_MQC == [ 
                type |-> "M_QC", 
                round |-> r, 
                caller |-> Proposer[r], 
                method |-> m.proposal.value, 
                btc_anchored |-> h_btc_current 
            ]
          IN quorum_certs' = quorum_certs \cup {new_MQC}
    /\ UNCHANGED <<timeout_certs>> 
    /\ UNCHANGED <<fsmVars, networkSensorVars>>


\* @type: Bool;
ServerNext ==
    \/ ServerAdvanceRealTime
    \/ ServerUpdateEnvironment
    \/ /\ SynchronizedLocalClocks
       /\ \E p \in HonestNodes : ServerMessageProcessing(p)
    \/ ServerByzantinePull
    \/ ServerByzantineDataWithholding

\* @type: Bool;
ServerSpec == ServerInit /\ [][ServerNext]_serverVars


(* ======================== MONOTONICITY SAFETY ======================== *)
\* Chain heights and real time must monotonically increase or remain constant.
\* This temporal property ensures the model is immune to time-travel or 
\* chain rollback anomalies, preventing Long-Range Attacks.
\* @type: Bool;
MonotonicitySafety ==
    [][ /\ h_btc_current'    >= h_btc_current
        /\ h_btc_anchored'   >= h_btc_anchored
        /\ h_engram_current' >= h_engram_current
        /\ real_time'        >= real_time 
      ]_serverVars

(* ======================== HYBRID INVARIANTS =============================== *)
\* Cross-layer consistency checks: every decided proposal must agree with the
\* current FSM and sensor state. These are checked in addition to CoreTendermintInvariant.

\* Decided FSM state must match the current circuit-breaker state
\* @type: Bool;
FSMStateConsistency ==
    \A p \in HonestNodes :
        decision[p] /= NilDecision => decision[p].prop.fsm_state = state

\* DA attestation must be present in any decided ANCHORED or RECOVERING block
\* @type: Bool;
DAReceiptConsistency ==
    \A p \in HonestNodes :
        (decision[p] /= NilDecision /\
        (decision[p].prop.fsm_state \in {"ANCHORED", "RECOVERING"} \/ IsDAHealthy))
        => decision[p].prop.da_receipt.attestation = TRUE

\* BTC anchor height in decided proposal must match the current anchored height
\* @type: Bool;
BTCConsistency ==
    \A p \in HonestNodes :
        decision[p] /= NilDecision
        => decision[p].prop.btc_receipt.checkpoint_block_height = h_btc_anchored

\* ZK proof must be present in any RECOVERING block that completed hysteresis
\* @type: Bool;
ZKProofConsistency ==
    \A p \in HonestNodes :
        (decision[p] /= NilDecision
         /\ decision[p].prop.fsm_state = "RECOVERING"
         /\ safe_blocks = HYSTERESIS_WAIT)
        => decision[p].prop.zk_proof_ref = TRUE

\* Decided proposal's healthy claim must match the current sensor state --
\* mirrors x/sovereignty/proposal.go:299-303's cross-check.
\* @type: Bool;
HealthyConsistency ==
    \A p \in HonestNodes :
        decision[p] /= NilDecision => decision[p].prop.healthy = IsHealthyCondition

\* Master hybrid invariant — checked together with CoreTendermintInvariant in TLC
\* @type: Bool;
HybridTendermintInvariant ==
    /\ FSMStateConsistency
    /\ DAReceiptConsistency
    /\ BTCConsistency
    /\ ZKProofConsistency
    /\ HealthyConsistency


(* ======================== LIVENESS PROPERTIES ============================ *)
\* At least one honest node (process) eventually decides
\* @type: Bool;
ServerEventualDecisionLiveness ==
    <>(\E p \in HonestNodes : step[p] = "DECIDED")

\* All three FSM liveness properties from EngramFSM hold end-to-end
\* @type: Bool;
ServerFSMLiveness ==
    /\ CircuitBreakerLiveness
    /\ RecoveryAttemptLiveness
    /\ CompleteRecoveryLiveness

\* Every tx that is repeatedly proposed must eventually be decided
\* @type: Bool;
ForcedInclusionLiveness ==
    \A tx \in ValidValues :
        ([]<>(\E r \in Rounds, p \in HonestNodes :
                  \E m \in msgs_propose[r] : m.src = p /\ m.proposal.value = tx))
        => <>(\E p \in HonestNodes :
                  decision[p] /= NilDecision /\ decision[p].prop.value = tx)

\* Global Stabilisation Time predicate: clocks sync + enough peers + ANCHORED
\* @type: Bool;
GSTReached ==
    /\ SynchronizedLocalClocks
    /\ Cardinality(active_peers) >= MIN_PEERS
    /\ state = "ANCHORED"

\* Under repeated GST, the system must eventually reach a decision
\* @type: Bool;
EventualDecisionUnderGSTLiveness ==
    ([]<> GSTReached) ~> (\E p \in HonestNodes : step[p] = "DECIDED")

===================================================================
