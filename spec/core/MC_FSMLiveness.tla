---------------- MODULE MC_FSMLiveness ----------------
EXTENDS EngramFSM, TLC

MC_MockInit ==
    /\ round = 0 /\ step = 0 /\ decision = 0 /\ locked_value = 0 /\ locked_round = 0 /\ valid_value = 0 /\ valid_round = 0
    /\ local_clock = 0 /\ real_time = 0 /\ local_rem_time = 0
    /\ msgs_propose = 0 /\ msgs_prevote = 0 /\ msgs_precommit = 0 /\ msgs_timeout = 0 /\ evidence = 0 /\ action = "mock" /\ received_timely_proposal = 0 /\ inspected_proposal = 0
    /\ begin_round = 0 /\ end_consensus = 0 /\ last_begin_round = 0 /\ proposal_time = 0 /\ proposal_received_time = 0
    /\ forced_tx_queue = 0 /\ tx_ignored_rounds = 0
    /\ quorum_certs = {} /\ timeout_certs = {}

MC_FSMInit ==
    /\ FSMInit
    /\ MC_MockInit

\* Alias covering every EngramVars tuple untouched by the FSM-only liveness harness
MC_AllVars == <<tendermintCoreVars, temporalVars, bookkeepingVars, invariantVars,
                networkSensorVars, fsmVars, censorshipVars, certificateVars>>

MC_FSMTransition ==
    /\ state' = CalculateNextFSMState
    /\ ExecuteFSMTransition(state')
    /\  \/ state' /= state
        \/ suspicious_duration' /= suspicious_duration
        \/ safe_blocks' /= safe_blocks
    /\ UNCHANGED <<tendermintCoreVars, temporalVars, networkSensorVars>>
    /\ UNCHANGED <<bookkeepingVars, invariantVars, censorshipVars>>
    /\ UNCHANGED <<certificateVars>>

MC_FSMUpdateSensors ==
    /\ h_btc_current' \in {0, 5}
    /\ h_btc_submitted' \in {0, h_btc_current'}
    /\ h_btc_anchored' \in {0, h_btc_submitted'}
    /\ h_engram_current' \in {0, 5}
    /\ h_engram_verified' \in {0, h_engram_current'}
    /\ is_das_failed' \in BOOLEAN
    \* Simulates P2P churn/disruption.
    /\ active_peers' \in { anchor_peers, anchor_peers \cup {"honest_n1"}, {"sybil_n1"} }
    /\ peer_churn_rate' \in {0, MAX_CHURN_RATE + 1}
    /\ avg_peer_tenure' \in {0, MIN_AVG_TENURE + 1}
    /\ peer_latency'    \in {0, MAX_PEER_LATENCY + 1}
    /\ UNCHANGED <<anchor_peers, blacklisted_peers>>
    /\ UNCHANGED <<state, safe_blocks, suspicious_duration, reanchoring_proof_valid>>
    /\ UNCHANGED <<tendermintCoreVars, temporalVars, bookkeepingVars, invariantVars>>
    /\ UNCHANGED <<censorshipVars>>
    /\ UNCHANGED <<certificateVars>>

MC_GenerateZKProof ==
    /\ state = "RECOVERING"
    /\ IsHealthyCondition
    /\ reanchoring_proof_valid = FALSE
    /\ reanchoring_proof_valid' = TRUE
    /\ UNCHANGED <<state, safe_blocks, suspicious_duration>>
    /\ UNCHANGED <<btcGapSensorVars, daGapSensorVars, p2pHealthSensorVars>>
    /\ UNCHANGED <<tendermintCoreVars, temporalVars, bookkeepingVars, invariantVars, censorshipVars, certificateVars>>

MC_FSMLivenessNext ==
    \/ MC_FSMTransition
    \/ MC_FSMUpdateSensors
    \* \/ MC_GenerateZKProof

MC_FSMLivenessFairness ==
    /\ WF_fsmVars(MC_FSMTransition)
    \* /\ SF_MC_AllVars(MC_GenerateZKProof)

MC_FSMLivenessSpec ==
    /\ MC_FSMInit
    /\ [][MC_FSMLivenessNext]_MC_AllVars
    /\ MC_FSMLivenessFairness

=========================================================
