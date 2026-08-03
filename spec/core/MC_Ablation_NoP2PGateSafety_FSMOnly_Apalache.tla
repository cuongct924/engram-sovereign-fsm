------------------- MODULE MC_Ablation_NoP2PGateSafety_FSMOnly_Apalache -------------------
(*
 * Apalache ablation driver: "Remove P2P Health Gate", FSM-layer-only scope.
 * Mirrors MC_Ablation_NoHysteresisSafety_FSMOnly_Apalache.tla's pattern:
 * the property under test depends only on FSM-layer variables, so this
 * skips the full Server/Tendermint stack (MC_Ablation_NoP2PGateSafety_
 * Apalache.tla, which routes through it and is much slower).
 *
 * EXTENDS EngramFSM_Ablation_NoP2PGate (dropped the IsP2PQualityHealthy
 * conjunct from IsHealthyCondition).
 *
 * NOTE on the invariant: a plain state predicate like
 * `~(state = "RECOVERING" /\ ~IsP2PQualityHealthy)` has the same flaw
 * discovered for the Hysteresis ablation -- UpdateSensors and
 * ExecuteFSMTransition are separate steps here, so sensors can go bad
 * one step before the FSM reacts, in BOTH the ablated and the real spec.
 * Instead this driver latches a ghost variable the instant
 * ExecuteFSMTransition computes state' = "RECOVERING" while P2P is
 * unhealthy -- i.e. the moment CalculateNextFSMState's own decision
 * ignores bad P2P, not a step-lag artifact. In the real (non-ablated)
 * spec this can never latch: IsHealthyCondition there still requires
 * IsP2PQualityHealthy, so no branch can produce state' = "RECOVERING"
 * while ~IsP2PQualityHealthy holds.
 *)
EXTENDS EngramFSM_Ablation_NoP2PGate, TLC

VARIABLE
    \* @type: Bool;
    illegal_p2p_recovery

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
    zk_proof_ref |-> FALSE
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

MC_FSMInit ==
    /\ FSMInit
    /\ MC_MockInit
    /\ illegal_p2p_recovery = FALSE

\* Mirrors the real commit-driven path (ServerUponProposalInPrecommitNo-
\* Decision): ExecuteFSMTransition is called unconditionally.
MC_FSMNext ==
    /\ \/ UpdateSensors /\ UNCHANGED illegal_p2p_recovery
       \/ /\ ExecuteFSMTransition(CalculateNextFSMState)
          /\ UNCHANGED <<networkSensorVars, censorshipVars>>
          /\ UNCHANGED <<reanchoring_proof_valid>>
          /\ illegal_p2p_recovery' = (state' = "RECOVERING" /\ ~IsP2PQualityHealthy)
    /\ UNCHANGED <<tendermintCoreVars, temporalVars>>
    /\ UNCHANGED <<bookkeepingVars, invariantVars>>
    /\ UNCHANGED <<certificateVars>>

\* EXPECT VIOLATION: proves an eclipsed node (bad P2P) can still enter
\* RECOVERING once the P2P health gate is ablated from IsHealthyCondition.
Sanity_NoIllegalP2PRecovery ==
    ~illegal_p2p_recovery

ApalacheCInit ==
    /\ SUSPICIOUS_THRESHOLD = 2
    /\ SOVEREIGN_THRESHOLD  = 4
    /\ DA_THRESHOLD         = 2
    /\ HYSTERESIS_WAIT      = 2
    /\ MAX_SUSPICIOUS_TIME  = 2
    /\ MIN_PEERS            = 3
    /\ MIN_SUBNET_DIVERSITY = 2
    /\ MIN_ANCHOR_PEERS     = 1
    /\ MAX_CHURN_RATE       = 10
    /\ MIN_AVG_TENURE       = 100
    /\ MAX_PEER_LATENCY     = 50

=========================================================
