------------------- MODULE MC_Ablation_NoHysteresisSafety_FSMOnly_Apalache -------------------
(*
 * Apalache ablation driver: "Remove Hysteresis", FSM-layer-only scope.
 *
 * Sanity_NeverFlapInRecovering depends only on state/safe_blocks/
 * IsHealthyCondition -- pure FSM-layer variables. Going through the full
 * Server/Tendermint stack (as MC_Ablation_NoHysteresisSafety_Apalache.tla
 * does) adds ~30-45 irrelevant transitions per step Apalache must still
 * evaluate, most "disabled", which is what made that route slow. This
 * driver mirrors MC_FSMSafety_Apalache.tla's mocking pattern instead,
 * dropping Tendermint/Server entirely.
 *
 * EXTENDS EngramFSM_Ablation_NoHysteresis (dropped `safe_blocks =
 * HYSTERESIS_WAIT` from CalculateNextFSMState's RECOVERING -> ANCHORED
 * branch). Expected to violate Sanity_NeverFlapInRecovering.
 *
 * NOTE on MC_FSMNext: the real FSMNext (EngramFSM.tla) guards its
 * transition branch with `state' /= state`, so it can never fire while
 * REMAINING in RECOVERING -- meaning safe_blocks can never even be
 * incremented past 0 by that toy relation, independent of --length. The
 * real commit path (EngramServer.tla's ServerUponProposalInPrecommitNo-
 * Decision) calls ExecuteFSMTransition unconditionally on every commit,
 * with no such guard. MC_FSMNext below mirrors that real path instead of
 * reusing FSMNext, so this driver can actually reach the ablated bug.
 *
 * NOTE on the invariant: an earlier version of this driver checked
 * `~(state = "RECOVERING" /\ safe_blocks > 0 /\ ~IsHealthyCondition)`.
 * A control run against the REAL (non-ablated) EngramFSM.tla with the
 * identical MC_FSMNext found the same violation at the same depth --
 * that predicate was an artifact of UpdateSensors and ExecuteFSMTransition
 * being separate steps (sensors can go unhealthy one step before the FSM
 * reacts), present in both specs equally, not a consequence of ablating
 * hysteresis. The real property being ablated is HysteresisSafety
 * (EngramFSM.tla): `[][(state="RECOVERING" /\ state'="ANCHORED") =>
 * (safe_blocks=HYSTERESIS_WAIT /\ reanchoring_proof_valid)]_fsmVars`.
 * Apalache's --inv only checks current-state predicates, not this kind
 * of action/step formula, so `illegal_early_exit` below is a ghost
 * variable (declared only in this driver, mirroring nothing in core/)
 * that latches TRUE the instant ExecuteFSMTransition performs a
 * RECOVERING -> ANCHORED move without the real guard holding --
 * checking `illegal_early_exit` as a plain state invariant is then
 * equivalent to checking HysteresisSafety itself.
 *)
EXTENDS EngramFSM_Ablation_NoHysteresis, TLC

VARIABLE
    \* @type: Bool;
    illegal_early_exit

\* @type: Set(Str);
NoProcs == {}
\* @type: Set(Int);
NoRounds == {}
\* @type: Set(<<Int, Str>>);
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
    /\ illegal_early_exit = FALSE

\* Mirrors the real commit-driven path (ServerUponProposalInPrecommitNo-
\* Decision): ExecuteFSMTransition is called unconditionally, with no
\* `state' /= state` guard, so remaining in RECOVERING can still advance
\* safe_blocks.
MC_FSMNext ==
    /\ \/ UpdateSensors /\ UNCHANGED illegal_early_exit
       \/ /\ ExecuteFSMTransition(CalculateNextFSMState)
          /\ UNCHANGED <<networkSensorVars, censorshipVars>>
          /\ UNCHANGED <<reanchoring_proof_valid>>
          /\ illegal_early_exit' =
                 (state = "RECOVERING" /\ state' = "ANCHORED"
                  /\ ~(safe_blocks = HYSTERESIS_WAIT /\ reanchoring_proof_valid))
    /\ UNCHANGED <<tendermintCoreVars, temporalVars>>
    /\ UNCHANGED <<bookkeepingVars, invariantVars>>
    /\ UNCHANGED <<certificateVars>>

\* EXPECT VIOLATION: equivalent to checking the real HysteresisSafety --
\* proves a RECOVERING -> ANCHORED move can happen without the hysteresis
\* wait / valid ZK proof once the guard is ablated.
Sanity_NoIllegalHysteresisExit ==
    ~illegal_early_exit

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
