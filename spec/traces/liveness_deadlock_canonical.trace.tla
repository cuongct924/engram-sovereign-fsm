---- MODULE MC_ServerRefinementLiveness_TTrace_1785752047 ----
EXTENDS Sequences, MC_ServerRefinementLiveness, TLCExt, Toolbox, Naturals, TLC

_expression ==
    LET MC_ServerRefinementLiveness_TEExpression == INSTANCE MC_ServerRefinementLiveness_TEExpression
    IN MC_ServerRefinementLiveness_TEExpression!expression
----

_trace ==
    LET MC_ServerRefinementLiveness_TETrace == INSTANCE MC_ServerRefinementLiveness_TETrace
    IN MC_ServerRefinementLiveness_TETrace!trace
----

_prop ==
    ~<>[](
        safe_blocks = (0)
        /\
        evidence = ({})
        /\
        local_clock = ((n2 :> 0 @@ n3 :> 0 @@ n4 :> 0))
        /\
        real_time = (0)
        /\
        last_begin_round = ((0 :> 0 @@ 1 :> -1 @@ 2 :> -1 @@ 3 :> -1 @@ 4 :> -1))
        /\
        timeout_certs = ({})
        /\
        is_das_failed = (FALSE)
        /\
        h_engram_current = (0)
        /\
        forced_tx_queue = ({"TX_NORMAL"})
        /\
        avg_peer_tenure = (1)
        /\
        local_rem_time = ((n2 :> 2 @@ n3 :> 2 @@ n4 :> 2))
        /\
        peer_latency = (0)
        /\
        is_btc_spv_failed = (FALSE)
        /\
        is_attestation_failed = (FALSE)
        /\
        begin_round = ((0 :> 0 @@ 1 :> 12 @@ 2 :> 12 @@ 3 :> 12 @@ 4 :> 12))
        /\
        h_engram_verified = (0)
        /\
        locked_value = ((n2 :> "NIL_TX" @@ n3 :> "NIL_TX" @@ n4 :> "NIL_TX"))
        /\
        action = ("Init")
        /\
        h_btc_current = (0)
        /\
        inspected_proposal = ((<<0, n2>> :> -1 @@ <<0, n3>> :> -1 @@ <<0, n4>> :> -1 @@ <<1, n2>> :> -1 @@ <<1, n3>> :> -1 @@ <<1, n4>> :> -1 @@ <<2, n2>> :> -1 @@ <<2, n3>> :> -1 @@ <<2, n4>> :> -1 @@ <<3, n2>> :> -1 @@ <<3, n3>> :> -1 @@ <<3, n4>> :> -1 @@ <<4, n2>> :> -1 @@ <<4, n3>> :> -1 @@ <<4, n4>> :> -1))
        /\
        state = ("ANCHORED")
        /\
        reanchoring_proof_valid = (FALSE)
        /\
        msgs_timeout = ((0 :> {[round |-> 0, id |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], src |-> n1, type |-> "TIMEOUT", proposal |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], valid_round |-> -1]} @@ 1 :> {[round |-> 1, id |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], src |-> n1, type |-> "TIMEOUT", proposal |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], valid_round |-> -1]} @@ 2 :> {[round |-> 2, id |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], src |-> n1, type |-> "TIMEOUT", proposal |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], valid_round |-> -1]} @@ 3 :> {[round |-> 3, id |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], src |-> n1, type |-> "TIMEOUT", proposal |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], valid_round |-> -1]} @@ 4 :> {[round |-> 4, id |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], src |-> n1, type |-> "TIMEOUT", proposal |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], valid_round |-> -1]}))
        /\
        msgs_propose = ((0 :> {} @@ 1 :> {} @@ 2 :> {} @@ 3 :> {} @@ 4 :> {}))
        /\
        quorum_certs = ({[round |-> 0, type |-> "E_QC", caller |-> n1, method |-> "None", btc_anchored |-> 0]})
        /\
        valid_value = ((n2 :> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1] @@ n3 :> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1] @@ n4 :> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1]))
        /\
        anchor_peers = ({"anchor_n1", "anchor_n2", "anchor_n3"})
        /\
        locked_round = ((n2 :> -1 @@ n3 :> -1 @@ n4 :> -1))
        /\
        msgs_precommit = ((0 :> {[round |-> 0, id |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], src |-> n1, type |-> "PRECOMMIT", proposal |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], valid_round |-> -1]} @@ 1 :> {[round |-> 1, id |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], src |-> n1, type |-> "PRECOMMIT", proposal |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], valid_round |-> -1]} @@ 2 :> {[round |-> 2, id |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], src |-> n1, type |-> "PRECOMMIT", proposal |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], valid_round |-> -1]} @@ 3 :> {[round |-> 3, id |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], src |-> n1, type |-> "PRECOMMIT", proposal |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], valid_round |-> -1]} @@ 4 :> {[round |-> 4, id |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], src |-> n1, type |-> "PRECOMMIT", proposal |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], valid_round |-> -1]}))
        /\
        msgs_prevote = ((0 :> {[round |-> 0, id |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], src |-> n1, type |-> "PREVOTE", proposal |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], valid_round |-> -1]} @@ 1 :> {[round |-> 1, id |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], src |-> n1, type |-> "PREVOTE", proposal |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], valid_round |-> -1]} @@ 2 :> {[round |-> 2, id |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], src |-> n1, type |-> "PREVOTE", proposal |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], valid_round |-> -1]} @@ 3 :> {[round |-> 3, id |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], src |-> n1, type |-> "PREVOTE", proposal |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], valid_round |-> -1]} @@ 4 :> {[round |-> 4, id |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], src |-> n1, type |-> "PREVOTE", proposal |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], valid_round |-> -1]}))
        /\
        peer_churn_rate = (0)
        /\
        decision = ((n2 :> [round |-> -1, prop |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1]] @@ n3 :> [round |-> -1, prop |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1]] @@ n4 :> [round |-> -1, prop |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1]]))
        /\
        h_btc_submitted = (0)
        /\
        tx_ignored_rounds = ((n2 :> [TX_NORMAL |-> 0, TX_WITHDRAWAL |-> 0] @@ n3 :> [TX_NORMAL |-> 0, TX_WITHDRAWAL |-> 0] @@ n4 :> [TX_NORMAL |-> 0, TX_WITHDRAWAL |-> 0]))
        /\
        received_timely_proposal = ((n2 :> {} @@ n3 :> {} @@ n4 :> {}))
        /\
        proposal_received_time = ((0 :> -1 @@ 1 :> -1 @@ 2 :> -1 @@ 3 :> -1 @@ 4 :> -1))
        /\
        round = ((n2 :> 0 @@ n3 :> 0 @@ n4 :> 0))
        /\
        valid_round = ((n2 :> -1 @@ n3 :> -1 @@ n4 :> -1))
        /\
        end_consensus = ((n2 :> -1 @@ n3 :> -1 @@ n4 :> -1))
        /\
        active_peers = ({"anchor_n1", "anchor_n2", "anchor_n3"})
        /\
        blacklisted_peers = ({})
        /\
        step = ((n2 :> "PROPOSE" @@ n3 :> "PROPOSE" @@ n4 :> "PROPOSE"))
        /\
        suspicious_duration = (0)
        /\
        h_btc_anchored = (0)
        /\
        proposal_time = ((0 :> -1 @@ 1 :> -1 @@ 2 :> -1 @@ 3 :> -1 @@ 4 :> -1))
    )
----

_init ==
    /\ action = _TETrace[1].action
    /\ proposal_received_time = _TETrace[1].proposal_received_time
    /\ msgs_prevote = _TETrace[1].msgs_prevote
    /\ proposal_time = _TETrace[1].proposal_time
    /\ h_btc_anchored = _TETrace[1].h_btc_anchored
    /\ peer_churn_rate = _TETrace[1].peer_churn_rate
    /\ inspected_proposal = _TETrace[1].inspected_proposal
    /\ h_btc_current = _TETrace[1].h_btc_current
    /\ real_time = _TETrace[1].real_time
    /\ msgs_precommit = _TETrace[1].msgs_precommit
    /\ valid_round = _TETrace[1].valid_round
    /\ anchor_peers = _TETrace[1].anchor_peers
    /\ quorum_certs = _TETrace[1].quorum_certs
    /\ avg_peer_tenure = _TETrace[1].avg_peer_tenure
    /\ step = _TETrace[1].step
    /\ locked_value = _TETrace[1].locked_value
    /\ reanchoring_proof_valid = _TETrace[1].reanchoring_proof_valid
    /\ is_attestation_failed = _TETrace[1].is_attestation_failed
    /\ round = _TETrace[1].round
    /\ msgs_timeout = _TETrace[1].msgs_timeout
    /\ local_clock = _TETrace[1].local_clock
    /\ safe_blocks = _TETrace[1].safe_blocks
    /\ tx_ignored_rounds = _TETrace[1].tx_ignored_rounds
    /\ blacklisted_peers = _TETrace[1].blacklisted_peers
    /\ is_das_failed = _TETrace[1].is_das_failed
    /\ h_engram_current = _TETrace[1].h_engram_current
    /\ timeout_certs = _TETrace[1].timeout_certs
    /\ evidence = _TETrace[1].evidence
    /\ suspicious_duration = _TETrace[1].suspicious_duration
    /\ begin_round = _TETrace[1].begin_round
    /\ local_rem_time = _TETrace[1].local_rem_time
    /\ decision = _TETrace[1].decision
    /\ msgs_propose = _TETrace[1].msgs_propose
    /\ valid_value = _TETrace[1].valid_value
    /\ end_consensus = _TETrace[1].end_consensus
    /\ locked_round = _TETrace[1].locked_round
    /\ h_btc_submitted = _TETrace[1].h_btc_submitted
    /\ active_peers = _TETrace[1].active_peers
    /\ last_begin_round = _TETrace[1].last_begin_round
    /\ state = _TETrace[1].state
    /\ is_btc_spv_failed = _TETrace[1].is_btc_spv_failed
    /\ peer_latency = _TETrace[1].peer_latency
    /\ received_timely_proposal = _TETrace[1].received_timely_proposal
    /\ h_engram_verified = _TETrace[1].h_engram_verified
    /\ forced_tx_queue = _TETrace[1].forced_tx_queue
----

_next ==
    /\ \E i,j \in DOMAIN _TETrace:
        /\ \/ /\ j = i + 1
              /\ i = TLCGet("level")
        /\ action  = _TETrace[i].action
        /\ action' = _TETrace[j].action
        /\ proposal_received_time  = _TETrace[i].proposal_received_time
        /\ proposal_received_time' = _TETrace[j].proposal_received_time
        /\ msgs_prevote  = _TETrace[i].msgs_prevote
        /\ msgs_prevote' = _TETrace[j].msgs_prevote
        /\ proposal_time  = _TETrace[i].proposal_time
        /\ proposal_time' = _TETrace[j].proposal_time
        /\ h_btc_anchored  = _TETrace[i].h_btc_anchored
        /\ h_btc_anchored' = _TETrace[j].h_btc_anchored
        /\ peer_churn_rate  = _TETrace[i].peer_churn_rate
        /\ peer_churn_rate' = _TETrace[j].peer_churn_rate
        /\ inspected_proposal  = _TETrace[i].inspected_proposal
        /\ inspected_proposal' = _TETrace[j].inspected_proposal
        /\ h_btc_current  = _TETrace[i].h_btc_current
        /\ h_btc_current' = _TETrace[j].h_btc_current
        /\ real_time  = _TETrace[i].real_time
        /\ real_time' = _TETrace[j].real_time
        /\ msgs_precommit  = _TETrace[i].msgs_precommit
        /\ msgs_precommit' = _TETrace[j].msgs_precommit
        /\ valid_round  = _TETrace[i].valid_round
        /\ valid_round' = _TETrace[j].valid_round
        /\ anchor_peers  = _TETrace[i].anchor_peers
        /\ anchor_peers' = _TETrace[j].anchor_peers
        /\ quorum_certs  = _TETrace[i].quorum_certs
        /\ quorum_certs' = _TETrace[j].quorum_certs
        /\ avg_peer_tenure  = _TETrace[i].avg_peer_tenure
        /\ avg_peer_tenure' = _TETrace[j].avg_peer_tenure
        /\ step  = _TETrace[i].step
        /\ step' = _TETrace[j].step
        /\ locked_value  = _TETrace[i].locked_value
        /\ locked_value' = _TETrace[j].locked_value
        /\ reanchoring_proof_valid  = _TETrace[i].reanchoring_proof_valid
        /\ reanchoring_proof_valid' = _TETrace[j].reanchoring_proof_valid
        /\ is_attestation_failed  = _TETrace[i].is_attestation_failed
        /\ is_attestation_failed' = _TETrace[j].is_attestation_failed
        /\ round  = _TETrace[i].round
        /\ round' = _TETrace[j].round
        /\ msgs_timeout  = _TETrace[i].msgs_timeout
        /\ msgs_timeout' = _TETrace[j].msgs_timeout
        /\ local_clock  = _TETrace[i].local_clock
        /\ local_clock' = _TETrace[j].local_clock
        /\ safe_blocks  = _TETrace[i].safe_blocks
        /\ safe_blocks' = _TETrace[j].safe_blocks
        /\ tx_ignored_rounds  = _TETrace[i].tx_ignored_rounds
        /\ tx_ignored_rounds' = _TETrace[j].tx_ignored_rounds
        /\ blacklisted_peers  = _TETrace[i].blacklisted_peers
        /\ blacklisted_peers' = _TETrace[j].blacklisted_peers
        /\ is_das_failed  = _TETrace[i].is_das_failed
        /\ is_das_failed' = _TETrace[j].is_das_failed
        /\ h_engram_current  = _TETrace[i].h_engram_current
        /\ h_engram_current' = _TETrace[j].h_engram_current
        /\ timeout_certs  = _TETrace[i].timeout_certs
        /\ timeout_certs' = _TETrace[j].timeout_certs
        /\ evidence  = _TETrace[i].evidence
        /\ evidence' = _TETrace[j].evidence
        /\ suspicious_duration  = _TETrace[i].suspicious_duration
        /\ suspicious_duration' = _TETrace[j].suspicious_duration
        /\ begin_round  = _TETrace[i].begin_round
        /\ begin_round' = _TETrace[j].begin_round
        /\ local_rem_time  = _TETrace[i].local_rem_time
        /\ local_rem_time' = _TETrace[j].local_rem_time
        /\ decision  = _TETrace[i].decision
        /\ decision' = _TETrace[j].decision
        /\ msgs_propose  = _TETrace[i].msgs_propose
        /\ msgs_propose' = _TETrace[j].msgs_propose
        /\ valid_value  = _TETrace[i].valid_value
        /\ valid_value' = _TETrace[j].valid_value
        /\ end_consensus  = _TETrace[i].end_consensus
        /\ end_consensus' = _TETrace[j].end_consensus
        /\ locked_round  = _TETrace[i].locked_round
        /\ locked_round' = _TETrace[j].locked_round
        /\ h_btc_submitted  = _TETrace[i].h_btc_submitted
        /\ h_btc_submitted' = _TETrace[j].h_btc_submitted
        /\ active_peers  = _TETrace[i].active_peers
        /\ active_peers' = _TETrace[j].active_peers
        /\ last_begin_round  = _TETrace[i].last_begin_round
        /\ last_begin_round' = _TETrace[j].last_begin_round
        /\ state  = _TETrace[i].state
        /\ state' = _TETrace[j].state
        /\ is_btc_spv_failed  = _TETrace[i].is_btc_spv_failed
        /\ is_btc_spv_failed' = _TETrace[j].is_btc_spv_failed
        /\ peer_latency  = _TETrace[i].peer_latency
        /\ peer_latency' = _TETrace[j].peer_latency
        /\ received_timely_proposal  = _TETrace[i].received_timely_proposal
        /\ received_timely_proposal' = _TETrace[j].received_timely_proposal
        /\ h_engram_verified  = _TETrace[i].h_engram_verified
        /\ h_engram_verified' = _TETrace[j].h_engram_verified
        /\ forced_tx_queue  = _TETrace[i].forced_tx_queue
        /\ forced_tx_queue' = _TETrace[j].forced_tx_queue

\* Uncomment the ASSUME below to write the states of the error trace
\* to the given file in Json format. Note that you can pass any tuple
\* to `JsonSerialize`. For example, a sub-sequence of _TETrace.
    \* ASSUME
    \*     LET J == INSTANCE Json
    \*         IN J!JsonSerialize("MC_ServerRefinementLiveness_TTrace_1785752047.json", _TETrace)

=============================================================================

 Note that you can extract this module `MC_ServerRefinementLiveness_TEExpression`
  to a dedicated file to reuse `expression` (the module in the 
  dedicated `MC_ServerRefinementLiveness_TEExpression.tla` file takes precedence 
  over the module `MC_ServerRefinementLiveness_TEExpression` below).

---- MODULE MC_ServerRefinementLiveness_TEExpression ----
EXTENDS Sequences, MC_ServerRefinementLiveness, TLCExt, Toolbox, Naturals, TLC

expression == 
    [
        \* To hide variables of the `MC_ServerRefinementLiveness` spec from the error trace,
        \* remove the variables below.  The trace will be written in the order
        \* of the fields of this record.
        action |-> action
        ,proposal_received_time |-> proposal_received_time
        ,msgs_prevote |-> msgs_prevote
        ,proposal_time |-> proposal_time
        ,h_btc_anchored |-> h_btc_anchored
        ,peer_churn_rate |-> peer_churn_rate
        ,inspected_proposal |-> inspected_proposal
        ,h_btc_current |-> h_btc_current
        ,real_time |-> real_time
        ,msgs_precommit |-> msgs_precommit
        ,valid_round |-> valid_round
        ,anchor_peers |-> anchor_peers
        ,quorum_certs |-> quorum_certs
        ,avg_peer_tenure |-> avg_peer_tenure
        ,step |-> step
        ,locked_value |-> locked_value
        ,reanchoring_proof_valid |-> reanchoring_proof_valid
        ,is_attestation_failed |-> is_attestation_failed
        ,round |-> round
        ,msgs_timeout |-> msgs_timeout
        ,local_clock |-> local_clock
        ,safe_blocks |-> safe_blocks
        ,tx_ignored_rounds |-> tx_ignored_rounds
        ,blacklisted_peers |-> blacklisted_peers
        ,is_das_failed |-> is_das_failed
        ,h_engram_current |-> h_engram_current
        ,timeout_certs |-> timeout_certs
        ,evidence |-> evidence
        ,suspicious_duration |-> suspicious_duration
        ,begin_round |-> begin_round
        ,local_rem_time |-> local_rem_time
        ,decision |-> decision
        ,msgs_propose |-> msgs_propose
        ,valid_value |-> valid_value
        ,end_consensus |-> end_consensus
        ,locked_round |-> locked_round
        ,h_btc_submitted |-> h_btc_submitted
        ,active_peers |-> active_peers
        ,last_begin_round |-> last_begin_round
        ,state |-> state
        ,is_btc_spv_failed |-> is_btc_spv_failed
        ,peer_latency |-> peer_latency
        ,received_timely_proposal |-> received_timely_proposal
        ,h_engram_verified |-> h_engram_verified
        ,forced_tx_queue |-> forced_tx_queue
        
        \* Put additional constant-, state-, and action-level expressions here:
        \* ,_stateNumber |-> _TEPosition
        \* ,_actionUnchanged |-> action = action'
        
        \* Format the `action` variable as Json value.
        \* ,_actionJson |->
        \*     LET J == INSTANCE Json
        \*     IN J!ToJson(action)
        
        \* Lastly, you may build expressions over arbitrary sets of states by
        \* leveraging the _TETrace operator.  For example, this is how to
        \* count the number of times a spec variable changed up to the current
        \* state in the trace.
        \* ,_actionModCount |->
        \*     LET F[s \in DOMAIN _TETrace] ==
        \*         IF s = 1 THEN 0
        \*         ELSE IF _TETrace[s].action # _TETrace[s-1].action
        \*             THEN 1 + F[s-1] ELSE F[s-1]
        \*     IN F[_TEPosition - 1]
    ]

=============================================================================



Parsing and semantic processing can take forever if the trace below is long.
 In this case, it is advised to uncomment the module below to deserialize the
 trace from a generated binary file.

\*
\*---- MODULE MC_ServerRefinementLiveness_TETrace ----
\*EXTENDS IOUtils, MC_ServerRefinementLiveness, TLC
\*
\*trace == IODeserialize("MC_ServerRefinementLiveness_TTrace_1785752047.bin", TRUE)
\*
\*=============================================================================
\*

---- MODULE MC_ServerRefinementLiveness_TETrace ----
EXTENDS MC_ServerRefinementLiveness, TLC

trace == 
    <<
    ([safe_blocks |-> 0,evidence |-> {},local_clock |-> (n2 :> 0 @@ n3 :> 0 @@ n4 :> 0),real_time |-> 0,last_begin_round |-> (0 :> 0 @@ 1 :> -1 @@ 2 :> -1 @@ 3 :> -1 @@ 4 :> -1),timeout_certs |-> {},is_das_failed |-> FALSE,h_engram_current |-> 0,forced_tx_queue |-> {"TX_NORMAL"},avg_peer_tenure |-> 1,local_rem_time |-> (n2 :> 2 @@ n3 :> 2 @@ n4 :> 2),peer_latency |-> 0,is_btc_spv_failed |-> FALSE,is_attestation_failed |-> FALSE,begin_round |-> (0 :> 0 @@ 1 :> 12 @@ 2 :> 12 @@ 3 :> 12 @@ 4 :> 12),h_engram_verified |-> 0,locked_value |-> (n2 :> "NIL_TX" @@ n3 :> "NIL_TX" @@ n4 :> "NIL_TX"),action |-> "Init",h_btc_current |-> 0,inspected_proposal |-> (<<0, n2>> :> -1 @@ <<0, n3>> :> -1 @@ <<0, n4>> :> -1 @@ <<1, n2>> :> -1 @@ <<1, n3>> :> -1 @@ <<1, n4>> :> -1 @@ <<2, n2>> :> -1 @@ <<2, n3>> :> -1 @@ <<2, n4>> :> -1 @@ <<3, n2>> :> -1 @@ <<3, n3>> :> -1 @@ <<3, n4>> :> -1 @@ <<4, n2>> :> -1 @@ <<4, n3>> :> -1 @@ <<4, n4>> :> -1),state |-> "ANCHORED",reanchoring_proof_valid |-> FALSE,msgs_timeout |-> (0 :> {[round |-> 0, id |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], src |-> n1, type |-> "TIMEOUT", proposal |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], valid_round |-> -1]} @@ 1 :> {[round |-> 1, id |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], src |-> n1, type |-> "TIMEOUT", proposal |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], valid_round |-> -1]} @@ 2 :> {[round |-> 2, id |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], src |-> n1, type |-> "TIMEOUT", proposal |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], valid_round |-> -1]} @@ 3 :> {[round |-> 3, id |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], src |-> n1, type |-> "TIMEOUT", proposal |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], valid_round |-> -1]} @@ 4 :> {[round |-> 4, id |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], src |-> n1, type |-> "TIMEOUT", proposal |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], valid_round |-> -1]}),msgs_propose |-> (0 :> {} @@ 1 :> {} @@ 2 :> {} @@ 3 :> {} @@ 4 :> {}),quorum_certs |-> {},valid_value |-> (n2 :> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1] @@ n3 :> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1] @@ n4 :> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1]),anchor_peers |-> {"anchor_n1", "anchor_n2", "anchor_n3"},locked_round |-> (n2 :> -1 @@ n3 :> -1 @@ n4 :> -1),msgs_precommit |-> (0 :> {[round |-> 0, id |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], src |-> n1, type |-> "PRECOMMIT", proposal |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], valid_round |-> -1]} @@ 1 :> {[round |-> 1, id |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], src |-> n1, type |-> "PRECOMMIT", proposal |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], valid_round |-> -1]} @@ 2 :> {[round |-> 2, id |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], src |-> n1, type |-> "PRECOMMIT", proposal |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], valid_round |-> -1]} @@ 3 :> {[round |-> 3, id |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], src |-> n1, type |-> "PRECOMMIT", proposal |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], valid_round |-> -1]} @@ 4 :> {[round |-> 4, id |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], src |-> n1, type |-> "PRECOMMIT", proposal |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], valid_round |-> -1]}),msgs_prevote |-> (0 :> {[round |-> 0, id |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], src |-> n1, type |-> "PREVOTE", proposal |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], valid_round |-> -1]} @@ 1 :> {[round |-> 1, id |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], src |-> n1, type |-> "PREVOTE", proposal |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], valid_round |-> -1]} @@ 2 :> {[round |-> 2, id |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], src |-> n1, type |-> "PREVOTE", proposal |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], valid_round |-> -1]} @@ 3 :> {[round |-> 3, id |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], src |-> n1, type |-> "PREVOTE", proposal |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], valid_round |-> -1]} @@ 4 :> {[round |-> 4, id |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], src |-> n1, type |-> "PREVOTE", proposal |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], valid_round |-> -1]}),peer_churn_rate |-> 0,decision |-> (n2 :> [round |-> -1, prop |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1]] @@ n3 :> [round |-> -1, prop |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1]] @@ n4 :> [round |-> -1, prop |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1]]),h_btc_submitted |-> 0,tx_ignored_rounds |-> (n2 :> [TX_NORMAL |-> 0, TX_WITHDRAWAL |-> 0] @@ n3 :> [TX_NORMAL |-> 0, TX_WITHDRAWAL |-> 0] @@ n4 :> [TX_NORMAL |-> 0, TX_WITHDRAWAL |-> 0]),received_timely_proposal |-> (n2 :> {} @@ n3 :> {} @@ n4 :> {}),proposal_received_time |-> (0 :> -1 @@ 1 :> -1 @@ 2 :> -1 @@ 3 :> -1 @@ 4 :> -1),round |-> (n2 :> 0 @@ n3 :> 0 @@ n4 :> 0),valid_round |-> (n2 :> -1 @@ n3 :> -1 @@ n4 :> -1),end_consensus |-> (n2 :> -1 @@ n3 :> -1 @@ n4 :> -1),active_peers |-> {"anchor_n1", "anchor_n2", "anchor_n3"},blacklisted_peers |-> {},step |-> (n2 :> "PROPOSE" @@ n3 :> "PROPOSE" @@ n4 :> "PROPOSE"),suspicious_duration |-> 0,h_btc_anchored |-> 0,proposal_time |-> (0 :> -1 @@ 1 :> -1 @@ 2 :> -1 @@ 3 :> -1 @@ 4 :> -1)]),
    ([safe_blocks |-> 0,evidence |-> {},local_clock |-> (n2 :> 0 @@ n3 :> 0 @@ n4 :> 0),real_time |-> 0,last_begin_round |-> (0 :> 0 @@ 1 :> -1 @@ 2 :> -1 @@ 3 :> -1 @@ 4 :> -1),timeout_certs |-> {},is_das_failed |-> FALSE,h_engram_current |-> 0,forced_tx_queue |-> {"TX_NORMAL"},avg_peer_tenure |-> 1,local_rem_time |-> (n2 :> 2 @@ n3 :> 2 @@ n4 :> 2),peer_latency |-> 0,is_btc_spv_failed |-> FALSE,is_attestation_failed |-> FALSE,begin_round |-> (0 :> 0 @@ 1 :> 12 @@ 2 :> 12 @@ 3 :> 12 @@ 4 :> 12),h_engram_verified |-> 0,locked_value |-> (n2 :> "NIL_TX" @@ n3 :> "NIL_TX" @@ n4 :> "NIL_TX"),action |-> "Init",h_btc_current |-> 0,inspected_proposal |-> (<<0, n2>> :> -1 @@ <<0, n3>> :> -1 @@ <<0, n4>> :> -1 @@ <<1, n2>> :> -1 @@ <<1, n3>> :> -1 @@ <<1, n4>> :> -1 @@ <<2, n2>> :> -1 @@ <<2, n3>> :> -1 @@ <<2, n4>> :> -1 @@ <<3, n2>> :> -1 @@ <<3, n3>> :> -1 @@ <<3, n4>> :> -1 @@ <<4, n2>> :> -1 @@ <<4, n3>> :> -1 @@ <<4, n4>> :> -1),state |-> "ANCHORED",reanchoring_proof_valid |-> FALSE,msgs_timeout |-> (0 :> {[round |-> 0, id |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], src |-> n1, type |-> "TIMEOUT", proposal |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], valid_round |-> -1]} @@ 1 :> {[round |-> 1, id |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], src |-> n1, type |-> "TIMEOUT", proposal |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], valid_round |-> -1]} @@ 2 :> {[round |-> 2, id |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], src |-> n1, type |-> "TIMEOUT", proposal |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], valid_round |-> -1]} @@ 3 :> {[round |-> 3, id |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], src |-> n1, type |-> "TIMEOUT", proposal |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], valid_round |-> -1]} @@ 4 :> {[round |-> 4, id |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], src |-> n1, type |-> "TIMEOUT", proposal |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], valid_round |-> -1]}),msgs_propose |-> (0 :> {} @@ 1 :> {} @@ 2 :> {} @@ 3 :> {} @@ 4 :> {}),quorum_certs |-> {[round |-> 0, type |-> "E_QC", caller |-> n1, method |-> "None", btc_anchored |-> 0]},valid_value |-> (n2 :> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1] @@ n3 :> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1] @@ n4 :> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1]),anchor_peers |-> {"anchor_n1", "anchor_n2", "anchor_n3"},locked_round |-> (n2 :> -1 @@ n3 :> -1 @@ n4 :> -1),msgs_precommit |-> (0 :> {[round |-> 0, id |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], src |-> n1, type |-> "PRECOMMIT", proposal |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], valid_round |-> -1]} @@ 1 :> {[round |-> 1, id |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], src |-> n1, type |-> "PRECOMMIT", proposal |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], valid_round |-> -1]} @@ 2 :> {[round |-> 2, id |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], src |-> n1, type |-> "PRECOMMIT", proposal |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], valid_round |-> -1]} @@ 3 :> {[round |-> 3, id |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], src |-> n1, type |-> "PRECOMMIT", proposal |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], valid_round |-> -1]} @@ 4 :> {[round |-> 4, id |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], src |-> n1, type |-> "PRECOMMIT", proposal |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], valid_round |-> -1]}),msgs_prevote |-> (0 :> {[round |-> 0, id |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], src |-> n1, type |-> "PREVOTE", proposal |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], valid_round |-> -1]} @@ 1 :> {[round |-> 1, id |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], src |-> n1, type |-> "PREVOTE", proposal |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], valid_round |-> -1]} @@ 2 :> {[round |-> 2, id |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], src |-> n1, type |-> "PREVOTE", proposal |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], valid_round |-> -1]} @@ 3 :> {[round |-> 3, id |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], src |-> n1, type |-> "PREVOTE", proposal |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], valid_round |-> -1]} @@ 4 :> {[round |-> 4, id |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], src |-> n1, type |-> "PREVOTE", proposal |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1], valid_round |-> -1]}),peer_churn_rate |-> 0,decision |-> (n2 :> [round |-> -1, prop |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1]] @@ n3 :> [round |-> -1, prop |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1]] @@ n4 :> [round |-> -1, prop |-> [round |-> -1, da_receipt |-> [published_block_height |-> -1, attestation |-> FALSE], btc_receipt |-> [checkpoint_block_height |-> -1, checkpoint_block_hash |-> <<"NIL", -1>>], value |-> "NIL_TX", fsm_state |-> "NONE", zk_proof_ref |-> FALSE, timestamp |-> -1]]),h_btc_submitted |-> 0,tx_ignored_rounds |-> (n2 :> [TX_NORMAL |-> 0, TX_WITHDRAWAL |-> 0] @@ n3 :> [TX_NORMAL |-> 0, TX_WITHDRAWAL |-> 0] @@ n4 :> [TX_NORMAL |-> 0, TX_WITHDRAWAL |-> 0]),received_timely_proposal |-> (n2 :> {} @@ n3 :> {} @@ n4 :> {}),proposal_received_time |-> (0 :> -1 @@ 1 :> -1 @@ 2 :> -1 @@ 3 :> -1 @@ 4 :> -1),round |-> (n2 :> 0 @@ n3 :> 0 @@ n4 :> 0),valid_round |-> (n2 :> -1 @@ n3 :> -1 @@ n4 :> -1),end_consensus |-> (n2 :> -1 @@ n3 :> -1 @@ n4 :> -1),active_peers |-> {"anchor_n1", "anchor_n2", "anchor_n3"},blacklisted_peers |-> {},step |-> (n2 :> "PROPOSE" @@ n3 :> "PROPOSE" @@ n4 :> "PROPOSE"),suspicious_duration |-> 0,h_btc_anchored |-> 0,proposal_time |-> (0 :> -1 @@ 1 :> -1 @@ 2 :> -1 @@ 3 :> -1 @@ 4 :> -1)])
    >>
----


=============================================================================

---- CONFIG MC_ServerRefinementLiveness_TTrace_1785752047 ----
CONSTANTS
    n1 = n1
    n2 = n2
    n3 = n3
    n4 = n4
    Nodes <- MC_Nodes
    Method <- MC_Method
    ByzantineNodes <- MC_Byzantine
    HonestNodes <- MC_Honest
    Proposer <- MC_Proposer
    MAX_ROUND = 4
    MAX_BTC_HEIGHT = 3
    MAX_ENGRAM_HEIGHT = 3
    MAX_TIMESTAMP = 12
    MAX_IGNORE_ROUNDS = 1
    RESET_TIME = 2
    SUSPICIOUS_THRESHOLD = 1
    SOVEREIGN_THRESHOLD = 2
    DA_THRESHOLD = 2
    HYSTERESIS_WAIT = 1
    MAX_SUSPICIOUS_TIME = 2
    MIN_PEERS = 2
    MIN_SUBNET_DIVERSITY = 2
    MIN_ANCHOR_PEERS = 1
    MAX_CHURN_RATE = 1
    MIN_AVG_TENURE = 1
    MAX_PEER_LATENCY = 1
    N = 4
    T = 1
    MIN_TIMESTAMP = 0
    PRECISION = 1
    DELAY = 1
    TIMEOUT_DURATION = 2
    K_DEEP_FINALITY = 2
    n1 = n1
    n3 = n3
    n4 = n4
    n2 = n2

PROPERTY
    _prop

CHECK_DEADLOCK
    \* CHECK_DEADLOCK off because of PROPERTY or INVARIANT above.
    FALSE

INIT
    _init

NEXT
    _next

CONSTANT
    _TETrace <- _trace

ALIAS
    _expression
=============================================================================
\* Generated on Mon Aug 03 17:16:16 ICT 2026