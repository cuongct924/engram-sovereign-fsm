---------------------------- MODULE counterexample ----------------------------

EXTENDS MC_Ablation_NoCircuitBreakerSafety_Apalache

(* Constant initialization state *)
ConstInit ==
  ByzantineNodes = {"n1"}
    /\ DA_THRESHOLD = 1
    /\ DELAY = 1
    /\ HYSTERESIS_WAIT = 1
    /\ HonestNodes = { "n2", "n3", "n4" }
    /\ MAX_BTC_HEIGHT = 1
    /\ MAX_CHURN_RATE = 1
    /\ MAX_ENGRAM_HEIGHT = 1
    /\ MAX_IGNORE_ROUNDS = 1
    /\ MAX_PEER_LATENCY = 1
    /\ MAX_ROUND = 2
    /\ MAX_SUSPICIOUS_TIME = 1
    /\ MAX_TIMESTAMP = 2
    /\ MIN_ANCHOR_PEERS = 1
    /\ MIN_AVG_TENURE = 1
    /\ MIN_PEERS = 2
    /\ MIN_SUBNET_DIVERSITY = 2
    /\ MIN_TIMESTAMP = 0
    /\ Method = { "TX_NORMAL", "TX_WITHDRAWAL" }
    /\ N = 4
    /\ Nodes = { "n1", "n2", "n3", "n4" }
    /\ PRECISION = 1
    /\ Proposer = SetAsFun({ <<0, "n2">>, <<1, "n1">>, <<2, "n2">> })
    /\ RESET_TIME = 2
    /\ SOVEREIGN_THRESHOLD = 2
    /\ SUSPICIOUS_THRESHOLD = 1
    /\ T = 1
    /\ TIMEOUT_DURATION = 2
    /\ n1 = "n1"
    /\ n2 = "n2"
    /\ n3 = "n3"
    /\ n4 = "n4"

(* Initial state [_transition(0)] *)
State0 ==
  ByzantineNodes = {"n1"}
    /\ DA_THRESHOLD = 1
    /\ DELAY = 1
    /\ HYSTERESIS_WAIT = 1
    /\ HonestNodes = { "n2", "n3", "n4" }
    /\ MAX_BTC_HEIGHT = 1
    /\ MAX_CHURN_RATE = 1
    /\ MAX_ENGRAM_HEIGHT = 1
    /\ MAX_IGNORE_ROUNDS = 1
    /\ MAX_PEER_LATENCY = 1
    /\ MAX_ROUND = 2
    /\ MAX_SUSPICIOUS_TIME = 1
    /\ MAX_TIMESTAMP = 2
    /\ MIN_ANCHOR_PEERS = 1
    /\ MIN_AVG_TENURE = 1
    /\ MIN_PEERS = 2
    /\ MIN_SUBNET_DIVERSITY = 2
    /\ MIN_TIMESTAMP = 0
    /\ Method = { "TX_NORMAL", "TX_WITHDRAWAL" }
    /\ N = 4
    /\ Nodes = { "n1", "n2", "n3", "n4" }
    /\ PRECISION = 1
    /\ Proposer = SetAsFun({ <<0, "n2">>, <<1, "n1">>, <<2, "n2">> })
    /\ RESET_TIME = 2
    /\ SOVEREIGN_THRESHOLD = 2
    /\ SUSPICIOUS_THRESHOLD = 1
    /\ T = 1
    /\ TIMEOUT_DURATION = 2
    /\ action = "Init"
    /\ active_peers = { "anchor_n1", "anchor_n2", "anchor_n3" }
    /\ anchor_peers = { "anchor_n1", "anchor_n2", "anchor_n3" }
    /\ avg_peer_tenure = 1
    /\ begin_round = SetAsFun({ <<0, 1>>, <<1, 2>>, <<2, 2>> })
    /\ blacklisted_peers = {}
    /\ decision
      = SetAsFun({ <<
          "n2", [prop |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            round |-> -1]
        >>,
        <<
          "n3", [prop |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            round |-> -1]
        >>,
        <<
          "n4", [prop |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            round |-> -1]
        >> })
    /\ end_consensus = SetAsFun({ <<"n2", -1>>, <<"n3", -1>>, <<"n4", -1>> })
    /\ evidence = {}
    /\ forced_tx_queue = {"TX_NORMAL"}
    /\ h_btc_anchored = 0
    /\ h_btc_current = 0
    /\ h_btc_submitted = 0
    /\ h_engram_current = 0
    /\ h_engram_verified = 0
    /\ inspected_proposal
      = SetAsFun({ <<<<0, "n2">>, -1>>,
        <<<<0, "n3">>, -1>>,
        <<<<0, "n4">>, -1>>,
        <<<<1, "n2">>, -1>>,
        <<<<1, "n3">>, -1>>,
        <<<<1, "n4">>, -1>>,
        <<<<2, "n2">>, -1>>,
        <<<<2, "n3">>, -1>>,
        <<<<2, "n4">>, -1>> })
    /\ is_attestation_failed = FALSE
    /\ is_btc_spv_failed = FALSE
    /\ is_das_failed = FALSE
    /\ last_begin_round = SetAsFun({ <<0, 1>>, <<1, -1>>, <<2, -1>> })
    /\ local_clock = SetAsFun({ <<"n2", 1>>, <<"n3", 1>>, <<"n4", 1>> })
    /\ local_rem_time = SetAsFun({ <<"n2", 2>>, <<"n3", 2>>, <<"n4", 2>> })
    /\ locked_round = SetAsFun({ <<"n2", -1>>, <<"n3", -1>>, <<"n4", -1>> })
    /\ locked_value
      = SetAsFun({ <<"n2", "NIL_TX">>, <<"n3", "NIL_TX">>, <<"n4", "NIL_TX">> })
    /\ msgs_precommit
      = SetAsFun({ <<
          0, {[id |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            proposal |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            round |-> 0,
            src |-> "n1",
            type |-> "PRECOMMIT",
            valid_round |-> -1]}
        >>,
        <<
          1, {[id |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            proposal |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            round |-> 1,
            src |-> "n1",
            type |-> "PRECOMMIT",
            valid_round |-> -1]}
        >>,
        <<
          2, {[id |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            proposal |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            round |-> 2,
            src |-> "n1",
            type |-> "PRECOMMIT",
            valid_round |-> -1]}
        >> })
    /\ msgs_prevote
      = SetAsFun({ <<
          0, {[id |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            proposal |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            round |-> 0,
            src |-> "n1",
            type |-> "PREVOTE",
            valid_round |-> -1]}
        >>,
        <<
          1, {[id |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            proposal |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            round |-> 1,
            src |-> "n1",
            type |-> "PREVOTE",
            valid_round |-> -1]}
        >>,
        <<
          2, {[id |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            proposal |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            round |-> 2,
            src |-> "n1",
            type |-> "PREVOTE",
            valid_round |-> -1]}
        >> })
    /\ msgs_propose = SetAsFun({ <<0, {}>>, <<1, {}>>, <<2, {}>> })
    /\ msgs_timeout
      = SetAsFun({ <<
          0, {[id |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            proposal |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            round |-> 0,
            src |-> "n1",
            type |-> "TIMEOUT",
            valid_round |-> -1]}
        >>,
        <<
          1, {[id |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            proposal |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            round |-> 1,
            src |-> "n1",
            type |-> "TIMEOUT",
            valid_round |-> -1]}
        >>,
        <<
          2, {[id |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            proposal |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            round |-> 2,
            src |-> "n1",
            type |-> "TIMEOUT",
            valid_round |-> -1]}
        >> })
    /\ n1 = "n1"
    /\ n2 = "n2"
    /\ n3 = "n3"
    /\ n4 = "n4"
    /\ peer_churn_rate = 0
    /\ peer_latency = 0
    /\ proposal_received_time = SetAsFun({ <<0, -1>>, <<1, -1>>, <<2, -1>> })
    /\ proposal_time = SetAsFun({ <<0, -1>>, <<1, -1>>, <<2, -1>> })
    /\ quorum_certs = {}
    /\ real_time = 0
    /\ reanchoring_proof_valid = FALSE
    /\ received_timely_proposal
      = SetAsFun({ <<"n2", {}>>, <<"n3", {}>>, <<"n4", {}>> })
    /\ round = SetAsFun({ <<"n2", 0>>, <<"n3", 0>>, <<"n4", 0>> })
    /\ safe_blocks = 0
    /\ state = "ANCHORED"
    /\ step
      = SetAsFun({ <<"n2", "PROPOSE">>, <<"n3", "PROPOSE">>, <<"n4", "PROPOSE">>
      })
    /\ suspicious_duration = 0
    /\ timeout_certs = {}
    /\ tx_ignored_rounds
      = SetAsFun({ <<
          "n2", SetAsFun({ <<"TX_NORMAL", 0>>, <<"TX_WITHDRAWAL", 0>> })
        >>,
        <<"n3", SetAsFun({ <<"TX_NORMAL", 0>>, <<"TX_WITHDRAWAL", 0>> })>>,
        <<"n4", SetAsFun({ <<"TX_NORMAL", 0>>, <<"TX_WITHDRAWAL", 0>> })>> })
    /\ valid_round = SetAsFun({ <<"n2", -1>>, <<"n3", -1>>, <<"n4", -1>> })
    /\ valid_value
      = SetAsFun({ <<
          "n2", [btc_receipt |->
              [checkpoint_block_hash |-> <<"NIL", -1>>,
                checkpoint_block_height |-> -1],
            da_receipt |->
              [attestation |-> FALSE, published_block_height |-> -1],
            fsm_state |-> "NONE",
            round |-> -1,
            timestamp |-> -1,
            value |-> "NIL_TX",
            zk_proof_ref |-> FALSE]
        >>,
        <<
          "n3", [btc_receipt |->
              [checkpoint_block_hash |-> <<"NIL", -1>>,
                checkpoint_block_height |-> -1],
            da_receipt |->
              [attestation |-> FALSE, published_block_height |-> -1],
            fsm_state |-> "NONE",
            round |-> -1,
            timestamp |-> -1,
            value |-> "NIL_TX",
            zk_proof_ref |-> FALSE]
        >>,
        <<
          "n4", [btc_receipt |->
              [checkpoint_block_hash |-> <<"NIL", -1>>,
                checkpoint_block_height |-> -1],
            da_receipt |->
              [attestation |-> FALSE, published_block_height |-> -1],
            fsm_state |-> "NONE",
            round |-> -1,
            timestamp |-> -1,
            value |-> "NIL_TX",
            zk_proof_ref |-> FALSE]
        >> })

(* State1 [_transition(9)] *)
State1 ==
  ByzantineNodes = {"n1"}
    /\ DA_THRESHOLD = 1
    /\ DELAY = 1
    /\ HYSTERESIS_WAIT = 1
    /\ HonestNodes = { "n2", "n3", "n4" }
    /\ MAX_BTC_HEIGHT = 1
    /\ MAX_CHURN_RATE = 1
    /\ MAX_ENGRAM_HEIGHT = 1
    /\ MAX_IGNORE_ROUNDS = 1
    /\ MAX_PEER_LATENCY = 1
    /\ MAX_ROUND = 2
    /\ MAX_SUSPICIOUS_TIME = 1
    /\ MAX_TIMESTAMP = 2
    /\ MIN_ANCHOR_PEERS = 1
    /\ MIN_AVG_TENURE = 1
    /\ MIN_PEERS = 2
    /\ MIN_SUBNET_DIVERSITY = 2
    /\ MIN_TIMESTAMP = 0
    /\ Method = { "TX_NORMAL", "TX_WITHDRAWAL" }
    /\ N = 4
    /\ Nodes = { "n1", "n2", "n3", "n4" }
    /\ PRECISION = 1
    /\ Proposer = SetAsFun({ <<0, "n2">>, <<1, "n1">>, <<2, "n2">> })
    /\ RESET_TIME = 2
    /\ SOVEREIGN_THRESHOLD = 2
    /\ SUSPICIOUS_THRESHOLD = 1
    /\ T = 1
    /\ TIMEOUT_DURATION = 2
    /\ action = "UpdateEnvironment"
    /\ active_peers = { "sybil_n1", "sybil_n2", "sybil_n3" }
    /\ anchor_peers = { "anchor_n1", "anchor_n2", "anchor_n3" }
    /\ avg_peer_tenure = 1
    /\ begin_round = SetAsFun({ <<0, 1>>, <<1, 2>>, <<2, 2>> })
    /\ blacklisted_peers = {}
    /\ decision
      = SetAsFun({ <<
          "n2", [prop |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            round |-> -1]
        >>,
        <<
          "n3", [prop |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            round |-> -1]
        >>,
        <<
          "n4", [prop |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            round |-> -1]
        >> })
    /\ end_consensus = SetAsFun({ <<"n2", -1>>, <<"n3", -1>>, <<"n4", -1>> })
    /\ evidence = {}
    /\ forced_tx_queue = {"TX_NORMAL"}
    /\ h_btc_anchored = 0
    /\ h_btc_current = 0
    /\ h_btc_submitted = 0
    /\ h_engram_current = 1
    /\ h_engram_verified = 1
    /\ inspected_proposal
      = SetAsFun({ <<<<0, "n2">>, -1>>,
        <<<<0, "n3">>, -1>>,
        <<<<0, "n4">>, -1>>,
        <<<<1, "n2">>, -1>>,
        <<<<1, "n3">>, -1>>,
        <<<<1, "n4">>, -1>>,
        <<<<2, "n2">>, -1>>,
        <<<<2, "n3">>, -1>>,
        <<<<2, "n4">>, -1>> })
    /\ is_attestation_failed = FALSE
    /\ is_btc_spv_failed = TRUE
    /\ is_das_failed = FALSE
    /\ last_begin_round = SetAsFun({ <<0, 1>>, <<1, -1>>, <<2, -1>> })
    /\ local_clock = SetAsFun({ <<"n2", 1>>, <<"n3", 1>>, <<"n4", 1>> })
    /\ local_rem_time = SetAsFun({ <<"n2", 2>>, <<"n3", 2>>, <<"n4", 2>> })
    /\ locked_round = SetAsFun({ <<"n2", -1>>, <<"n3", -1>>, <<"n4", -1>> })
    /\ locked_value
      = SetAsFun({ <<"n2", "NIL_TX">>, <<"n3", "NIL_TX">>, <<"n4", "NIL_TX">> })
    /\ msgs_precommit
      = SetAsFun({ <<
          0, {[id |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            proposal |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            round |-> 0,
            src |-> "n1",
            type |-> "PRECOMMIT",
            valid_round |-> -1]}
        >>,
        <<
          1, {[id |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            proposal |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            round |-> 1,
            src |-> "n1",
            type |-> "PRECOMMIT",
            valid_round |-> -1]}
        >>,
        <<
          2, {[id |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            proposal |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            round |-> 2,
            src |-> "n1",
            type |-> "PRECOMMIT",
            valid_round |-> -1]}
        >> })
    /\ msgs_prevote
      = SetAsFun({ <<
          0, {[id |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            proposal |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            round |-> 0,
            src |-> "n1",
            type |-> "PREVOTE",
            valid_round |-> -1]}
        >>,
        <<
          1, {[id |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            proposal |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            round |-> 1,
            src |-> "n1",
            type |-> "PREVOTE",
            valid_round |-> -1]}
        >>,
        <<
          2, {[id |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            proposal |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            round |-> 2,
            src |-> "n1",
            type |-> "PREVOTE",
            valid_round |-> -1]}
        >> })
    /\ msgs_propose = SetAsFun({ <<0, {}>>, <<1, {}>>, <<2, {}>> })
    /\ msgs_timeout
      = SetAsFun({ <<
          0, {[id |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            proposal |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            round |-> 0,
            src |-> "n1",
            type |-> "TIMEOUT",
            valid_round |-> -1]}
        >>,
        <<
          1, {[id |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            proposal |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            round |-> 1,
            src |-> "n1",
            type |-> "TIMEOUT",
            valid_round |-> -1]}
        >>,
        <<
          2, {[id |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            proposal |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            round |-> 2,
            src |-> "n1",
            type |-> "TIMEOUT",
            valid_round |-> -1]}
        >> })
    /\ n1 = "n1"
    /\ n2 = "n2"
    /\ n3 = "n3"
    /\ n4 = "n4"
    /\ peer_churn_rate = 0
    /\ peer_latency = 0
    /\ proposal_received_time = SetAsFun({ <<0, -1>>, <<1, -1>>, <<2, -1>> })
    /\ proposal_time = SetAsFun({ <<0, -1>>, <<1, -1>>, <<2, -1>> })
    /\ quorum_certs = {}
    /\ real_time = 0
    /\ reanchoring_proof_valid = FALSE
    /\ received_timely_proposal
      = SetAsFun({ <<"n2", {}>>, <<"n3", {}>>, <<"n4", {}>> })
    /\ round = SetAsFun({ <<"n2", 0>>, <<"n3", 0>>, <<"n4", 0>> })
    /\ safe_blocks = 0
    /\ state = "ANCHORED"
    /\ step
      = SetAsFun({ <<"n2", "PROPOSE">>, <<"n3", "PROPOSE">>, <<"n4", "PROPOSE">>
      })
    /\ suspicious_duration = 0
    /\ timeout_certs = {}
    /\ tx_ignored_rounds
      = SetAsFun({ <<
          "n2", SetAsFun({ <<"TX_NORMAL", 0>>, <<"TX_WITHDRAWAL", 0>> })
        >>,
        <<"n3", SetAsFun({ <<"TX_NORMAL", 0>>, <<"TX_WITHDRAWAL", 0>> })>>,
        <<"n4", SetAsFun({ <<"TX_NORMAL", 0>>, <<"TX_WITHDRAWAL", 0>> })>> })
    /\ valid_round = SetAsFun({ <<"n2", -1>>, <<"n3", -1>>, <<"n4", -1>> })
    /\ valid_value
      = SetAsFun({ <<
          "n2", [btc_receipt |->
              [checkpoint_block_hash |-> <<"NIL", -1>>,
                checkpoint_block_height |-> -1],
            da_receipt |->
              [attestation |-> FALSE, published_block_height |-> -1],
            fsm_state |-> "NONE",
            round |-> -1,
            timestamp |-> -1,
            value |-> "NIL_TX",
            zk_proof_ref |-> FALSE]
        >>,
        <<
          "n3", [btc_receipt |->
              [checkpoint_block_hash |-> <<"NIL", -1>>,
                checkpoint_block_height |-> -1],
            da_receipt |->
              [attestation |-> FALSE, published_block_height |-> -1],
            fsm_state |-> "NONE",
            round |-> -1,
            timestamp |-> -1,
            value |-> "NIL_TX",
            zk_proof_ref |-> FALSE]
        >>,
        <<
          "n4", [btc_receipt |->
              [checkpoint_block_hash |-> <<"NIL", -1>>,
                checkpoint_block_height |-> -1],
            da_receipt |->
              [attestation |-> FALSE, published_block_height |-> -1],
            fsm_state |-> "NONE",
            round |-> -1,
            timestamp |-> -1,
            value |-> "NIL_TX",
            zk_proof_ref |-> FALSE]
        >> })

(* State2 [_transition(34)] *)
State2 ==
  ByzantineNodes = {"n1"}
    /\ DA_THRESHOLD = 1
    /\ DELAY = 1
    /\ HYSTERESIS_WAIT = 1
    /\ HonestNodes = { "n2", "n3", "n4" }
    /\ MAX_BTC_HEIGHT = 1
    /\ MAX_CHURN_RATE = 1
    /\ MAX_ENGRAM_HEIGHT = 1
    /\ MAX_IGNORE_ROUNDS = 1
    /\ MAX_PEER_LATENCY = 1
    /\ MAX_ROUND = 2
    /\ MAX_SUSPICIOUS_TIME = 1
    /\ MAX_TIMESTAMP = 2
    /\ MIN_ANCHOR_PEERS = 1
    /\ MIN_AVG_TENURE = 1
    /\ MIN_PEERS = 2
    /\ MIN_SUBNET_DIVERSITY = 2
    /\ MIN_TIMESTAMP = 0
    /\ Method = { "TX_NORMAL", "TX_WITHDRAWAL" }
    /\ N = 4
    /\ Nodes = { "n1", "n2", "n3", "n4" }
    /\ PRECISION = 1
    /\ Proposer = SetAsFun({ <<0, "n2">>, <<1, "n1">>, <<2, "n2">> })
    /\ RESET_TIME = 2
    /\ SOVEREIGN_THRESHOLD = 2
    /\ SUSPICIOUS_THRESHOLD = 1
    /\ T = 1
    /\ TIMEOUT_DURATION = 2
    /\ action = "InsertProposal"
    /\ active_peers = { "sybil_n1", "sybil_n2", "sybil_n3" }
    /\ anchor_peers = { "anchor_n1", "anchor_n2", "anchor_n3" }
    /\ avg_peer_tenure = 1
    /\ begin_round = SetAsFun({ <<0, 1>>, <<1, 2>>, <<2, 2>> })
    /\ blacklisted_peers = {}
    /\ decision
      = SetAsFun({ <<
          "n2", [prop |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            round |-> -1]
        >>,
        <<
          "n3", [prop |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            round |-> -1]
        >>,
        <<
          "n4", [prop |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            round |-> -1]
        >> })
    /\ end_consensus = SetAsFun({ <<"n2", -1>>, <<"n3", -1>>, <<"n4", -1>> })
    /\ evidence = {}
    /\ forced_tx_queue = {"TX_NORMAL"}
    /\ h_btc_anchored = 0
    /\ h_btc_current = 0
    /\ h_btc_submitted = 0
    /\ h_engram_current = 1
    /\ h_engram_verified = 1
    /\ inspected_proposal
      = SetAsFun({ <<<<0, "n2">>, -1>>,
        <<<<0, "n3">>, -1>>,
        <<<<0, "n4">>, -1>>,
        <<<<1, "n2">>, -1>>,
        <<<<1, "n3">>, -1>>,
        <<<<1, "n4">>, -1>>,
        <<<<2, "n2">>, -1>>,
        <<<<2, "n3">>, -1>>,
        <<<<2, "n4">>, -1>> })
    /\ is_attestation_failed = FALSE
    /\ is_btc_spv_failed = TRUE
    /\ is_das_failed = FALSE
    /\ last_begin_round = SetAsFun({ <<0, 1>>, <<1, -1>>, <<2, -1>> })
    /\ local_clock = SetAsFun({ <<"n2", 1>>, <<"n3", 1>>, <<"n4", 1>> })
    /\ local_rem_time = SetAsFun({ <<"n2", 2>>, <<"n3", 2>>, <<"n4", 2>> })
    /\ locked_round = SetAsFun({ <<"n2", -1>>, <<"n3", -1>>, <<"n4", -1>> })
    /\ locked_value
      = SetAsFun({ <<"n2", "NIL_TX">>, <<"n3", "NIL_TX">>, <<"n4", "NIL_TX">> })
    /\ msgs_precommit
      = SetAsFun({ <<
          0, {[id |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            proposal |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            round |-> 0,
            src |-> "n1",
            type |-> "PRECOMMIT",
            valid_round |-> -1]}
        >>,
        <<
          1, {[id |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            proposal |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            round |-> 1,
            src |-> "n1",
            type |-> "PRECOMMIT",
            valid_round |-> -1]}
        >>,
        <<
          2, {[id |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            proposal |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            round |-> 2,
            src |-> "n1",
            type |-> "PRECOMMIT",
            valid_round |-> -1]}
        >> })
    /\ msgs_prevote
      = SetAsFun({ <<
          0, {[id |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            proposal |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            round |-> 0,
            src |-> "n1",
            type |-> "PREVOTE",
            valid_round |-> -1]}
        >>,
        <<
          1, {[id |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            proposal |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            round |-> 1,
            src |-> "n1",
            type |-> "PREVOTE",
            valid_round |-> -1]}
        >>,
        <<
          2, {[id |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            proposal |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            round |-> 2,
            src |-> "n1",
            type |-> "PREVOTE",
            valid_round |-> -1]}
        >> })
    /\ msgs_propose
      = SetAsFun({ <<
          0, {[id |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            proposal |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"BTC_BLOCK", 0>>,
                    checkpoint_block_height |-> 0],
                da_receipt |->
                  [attestation |-> TRUE, published_block_height |-> 1],
                fsm_state |-> "SOVEREIGN",
                round |-> 0,
                timestamp |-> 1,
                value |-> "TX_WITHDRAWAL",
                zk_proof_ref |-> FALSE],
            round |-> 0,
            src |-> "n2",
            type |-> "PROPOSAL",
            valid_round |-> -1]}
        >>,
        <<1, {}>>,
        <<2, {}>> })
    /\ msgs_timeout
      = SetAsFun({ <<
          0, {[id |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            proposal |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            round |-> 0,
            src |-> "n1",
            type |-> "TIMEOUT",
            valid_round |-> -1]}
        >>,
        <<
          1, {[id |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            proposal |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            round |-> 1,
            src |-> "n1",
            type |-> "TIMEOUT",
            valid_round |-> -1]}
        >>,
        <<
          2, {[id |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            proposal |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            round |-> 2,
            src |-> "n1",
            type |-> "TIMEOUT",
            valid_round |-> -1]}
        >> })
    /\ n1 = "n1"
    /\ n2 = "n2"
    /\ n3 = "n3"
    /\ n4 = "n4"
    /\ peer_churn_rate = 0
    /\ peer_latency = 0
    /\ proposal_received_time = SetAsFun({ <<0, -1>>, <<1, -1>>, <<2, -1>> })
    /\ proposal_time = SetAsFun({ <<0, 0>>, <<1, -1>>, <<2, -1>> })
    /\ quorum_certs
      = {[btc_anchored |-> 0,
        caller |-> "n2",
        method |-> "None",
        round |-> 0,
        type |-> "E_QC"]}
    /\ real_time = 0
    /\ reanchoring_proof_valid = FALSE
    /\ received_timely_proposal
      = SetAsFun({ <<"n2", {}>>, <<"n3", {}>>, <<"n4", {}>> })
    /\ round = SetAsFun({ <<"n2", 0>>, <<"n3", 0>>, <<"n4", 0>> })
    /\ safe_blocks = 0
    /\ state = "ANCHORED"
    /\ step
      = SetAsFun({ <<"n2", "PROPOSE">>, <<"n3", "PROPOSE">>, <<"n4", "PROPOSE">>
      })
    /\ suspicious_duration = 0
    /\ timeout_certs = {}
    /\ tx_ignored_rounds
      = SetAsFun({ <<
          "n2", SetAsFun({ <<"TX_NORMAL", 0>>, <<"TX_WITHDRAWAL", 0>> })
        >>,
        <<"n3", SetAsFun({ <<"TX_NORMAL", 0>>, <<"TX_WITHDRAWAL", 0>> })>>,
        <<"n4", SetAsFun({ <<"TX_NORMAL", 0>>, <<"TX_WITHDRAWAL", 0>> })>> })
    /\ valid_round = SetAsFun({ <<"n2", -1>>, <<"n3", -1>>, <<"n4", -1>> })
    /\ valid_value
      = SetAsFun({ <<
          "n2", [btc_receipt |->
              [checkpoint_block_hash |-> <<"NIL", -1>>,
                checkpoint_block_height |-> -1],
            da_receipt |->
              [attestation |-> FALSE, published_block_height |-> -1],
            fsm_state |-> "NONE",
            round |-> -1,
            timestamp |-> -1,
            value |-> "NIL_TX",
            zk_proof_ref |-> FALSE]
        >>,
        <<
          "n3", [btc_receipt |->
              [checkpoint_block_hash |-> <<"NIL", -1>>,
                checkpoint_block_height |-> -1],
            da_receipt |->
              [attestation |-> FALSE, published_block_height |-> -1],
            fsm_state |-> "NONE",
            round |-> -1,
            timestamp |-> -1,
            value |-> "NIL_TX",
            zk_proof_ref |-> FALSE]
        >>,
        <<
          "n4", [btc_receipt |->
              [checkpoint_block_hash |-> <<"NIL", -1>>,
                checkpoint_block_height |-> -1],
            da_receipt |->
              [attestation |-> FALSE, published_block_height |-> -1],
            fsm_state |-> "NONE",
            round |-> -1,
            timestamp |-> -1,
            value |-> "NIL_TX",
            zk_proof_ref |-> FALSE]
        >> })

(* The following formula holds true in the last state and violates the invariant *)
InvariantViolation ==
  Skolem((\E r_44 \in 0 .. 2:
    Skolem((\E m_45 \in msgs_propose[r_44]:
      m_45["proposal"]["fsm_state"] \in { "SOVEREIGN", "RECOVERING" }
        /\ m_45["proposal"]["value"] = "TX_WITHDRAWAL"))))

================================================================================
(* Created by Apalache on Mon Aug 03 14:28:39 ICT 2026 *)
(* https://github.com/apalache-mc/apalache *)
