------------------- MODULE MC_Ablation_NoDABackfillSafety_FSMOnly_Apalache -------------------
(*
 * Apalache ablation driver: "Remove DA backfill gate", FSM-layer-only scope.
 * Same pattern as MC_Ablation_NoP2PGateSafety_FSMOnly_Apalache.tla.
 *
 * EXTENDS EngramFSM_Ablation_NoDABackfill: a copy of the current EngramFSM
 * whose RECOVERING -> ANCHORED branch of CalculateNextFSMState drops the
 * `da_backfill_valid = TRUE` conjunct (EngramFSM.tla's exit condition).
 *
 * The check is on the transition, not on a state predicate: a ghost variable
 * latches when ExecuteFSMTransition moves RECOVERING -> ANCHORED while
 * da_backfill_valid is FALSE. This is SovereignDataPublishedSafety (S2a) as a
 * latch. In the real spec the latch can never fire.
 *)
EXTENDS EngramFSM_Ablation_NoDABackfill, TLC

VARIABLE
    \* @type: Bool;
    illegal_backfill_exit

NoProcs == {}
NoRounds == {}
NoRoundProcPairs == {}

MC_MockDA == [published_block_height |-> 0, attestation |-> FALSE]
MC_MockBTC == [checkpoint_block_height |-> 0, checkpoint_block_hash |-> <<"mock", 0>>]
MC_MockProposal == [
    value        |-> "mock",
    timestamp    |-> 0,
    round        |-> 0,
    fsm_state    |-> "mock",
    da_receipt   |-> MC_MockDA,
    btc_receipt  |-> MC_MockBTC,
    zk_proof_ref |-> FALSE,
    healthy      |-> FALSE
]
MC_MockDecision == [prop |-> MC_MockProposal, round |-> 0]

MC_MockInit ==
    /\ round = [p \in NoProcs |-> 0] /\ step = [p \in NoProcs |-> "mock"] /\ decision = [p \in NoProcs |-> MC_MockDecision]
    /\ locked_value = [p \in NoProcs |-> "mock"] /\ locked_round = [p \in NoProcs |-> 0]
    /\ valid_value = [p \in NoProcs |-> MC_MockProposal] /\ valid_round = [p \in NoProcs |-> 0]
    /\ local_clock = [p \in NoProcs |-> 0] /\ real_time = 0 /\ local_rem_time = [p \in NoProcs |-> 0]
    /\ msgs_propose = [r \in NoRounds |-> {}] /\ msgs_prevote = [r \in NoRounds |-> {}]
    /\ msgs_precommit = [r \in NoRounds |-> {}] /\ msgs_timeout = [r \in NoRounds |-> {}]
    /\ evidence = {} /\ action = "mock" /\ received_timely_proposal = [p \in NoProcs |-> {}]
    /\ inspected_proposal = [x \in NoRoundProcPairs |-> 0]
    /\ begin_round = [r \in NoRounds |-> 0] /\ end_consensus = [p \in NoProcs |-> 0]
    /\ last_begin_round = [r \in NoRounds |-> 0] /\ proposal_time = [r \in NoRounds |-> 0]
    /\ proposal_received_time = [r \in NoRounds |-> 0]
    /\ forced_tx_queue = {} /\ tx_ignored_rounds = [p \in NoProcs |-> [tx \in NoProcs |-> 0]]
    /\ quorum_certs = {} /\ timeout_certs = {}
    /\ sensor_view = [p \in NoProcs |-> [is_healthy |-> TRUE, is_warning |-> FALSE, is_critical |-> FALSE,
                                          btc_gap_obs |-> 0, da_gap_obs |-> 0]]
    /\ health_certs = {}

MC_FSMInit ==
    /\ FSMInit
    /\ MC_MockInit
    /\ illegal_backfill_exit = FALSE

\* Mirrors the real commit-driven path (ServerUponProposalInPrecommitNo-
\* Decision): ExecuteFSMTransition is called unconditionally.
MC_FSMNext ==
    /\ \/ UpdateSensors /\ UNCHANGED illegal_backfill_exit
       \/ DABackfill /\ UNCHANGED illegal_backfill_exit
       \/ /\ ExecuteFSMTransition(CalculateNextFSMState)
          /\ UNCHANGED <<networkSensorVars, censorshipVars>>
          /\ UNCHANGED <<reanchoring_proof_valid>>
          /\ illegal_backfill_exit' = (state = "RECOVERING" /\ state' = "ANCHORED" /\ ~da_backfill_valid)
    /\ UNCHANGED <<tendermintCoreVars, temporalVars>>
    /\ UNCHANGED <<bookkeepingVars, invariantVars>>
    /\ UNCHANGED <<certificateVars>>
    /\ UNCHANGED <<sensorAggVars>>

\* EXPECT VIOLATION: without the backfill conjunct the chain can leave
\* RECOVERING before the SOVEREIGN interval's data is published to DA.
Sanity_NoIllegalBackfillExit ==
    ~illegal_backfill_exit

ApalacheCInit ==
    /\ SUSPICIOUS_THRESHOLD = 2
    /\ SOVEREIGN_THRESHOLD  = 4
    /\ DA_THRESHOLD         = 2
    /\ RECOVERY_HYSTERESIS_WAIT      = 2
    /\ SUSPICIOUS_HYSTERESIS_WAIT      = 2
    /\ MAX_SUSPICIOUS_TIME  = 2
    /\ DOWN_HYSTERESIS_THRESHOLD  = 2
    /\ MAX_DOWN_HYSTERESIS_THRESHOLD  = 8
    /\ STABLE_WINDOW  = 2
    /\ MIN_PEERS            = 3
    /\ MIN_SUBNET_DIVERSITY = 2
    /\ MIN_ANCHOR_PEERS     = 1
    /\ MAX_CHURN_RATE       = 10
    /\ MIN_AVG_TENURE       = 100
    /\ MAX_PEER_LATENCY     = 50

=========================================================
