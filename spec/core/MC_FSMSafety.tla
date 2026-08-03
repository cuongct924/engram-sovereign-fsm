------------------- MODULE MC_FSMSafety -------------------
EXTENDS EngramFSM, TLC

\* Structurally well-typed placeholder values for variables the FSM layer
\* never touches (Tendermint/Server/certificate bookkeeping) — needed so
\* both TLC and Apalache see closed, domain-consistent Init values instead
\* of bare 0s, which no longer match the real record/function types
\* declared on these VARIABLES in EngramVars.tla. The empty domains below
\* are given an explicit Apalache type tag since it cannot infer an
\* element type from a bare set literal on its own.

\* @type: Set(Str);
NoProcs == {}

\* @type: Set(Int);
NoRounds == {}

\* @type: Set(<<Int, Str>>);
NoRoundProcPairs == {}

\* Left unannotated: Snowcat infers these structurally from the literal, and
\* that structural type unifies fine against decision/valid_value's expanded
\* (alias-free) type tags in EngramVars.tla — see the typing note there.
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

MC_FSMNext ==
    /\ FSMNext
    /\ UNCHANGED <<tendermintCoreVars, temporalVars>>
    /\ UNCHANGED <<bookkeepingVars, invariantVars>>
    /\ UNCHANGED <<certificateVars>>

StateSpaceLimit == 
    /\ h_btc_current < 10 
    /\ h_engram_current < 10

=========================================================