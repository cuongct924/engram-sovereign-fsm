---------------------------- MODULE counterexample ----------------------------

EXTENDS MC_HealthCertificate_Apalache

(* Constant initialization state *)
ConstInit ==
  ByzantineNodes = { "n1", "n2" }
    /\ DA_THRESHOLD = 1
    /\ DELAY = 1
    /\ DOWN_HYSTERESIS_THRESHOLD = 1
    /\ HYSTERESIS_WAIT = 1
    /\ HonestNodes = { "n3", "n4" }
    /\ MAX_BTC_HEIGHT = 1
    /\ MAX_CENSORSHIP_ROUNDS = 3
    /\ MAX_CHURN_RATE = 1
    /\ MAX_DOWN_HYSTERESIS_THRESHOLD = 8
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
    /\ Proposer = SetAsFun({ <<0, "n1">>, <<1, "n2">>, <<2, "n1">> })
    /\ RESET_TIME = 2
    /\ SOVEREIGN_THRESHOLD = 2
    /\ SUSPICIOUS_HYSTERESIS_WAIT = 1
    /\ SUSPICIOUS_THRESHOLD = 1
    /\ T = 1
    /\ TIMEOUT_DURATION = 2
    /\ n1 = "n1"
    /\ n2 = "n2"
    /\ n3 = "n3"
    /\ n4 = "n4"

(* Initial state [_transition(0)] *)
State0 ==
  ByzantineNodes = { "n1", "n2" }
    /\ DA_THRESHOLD = 1
    /\ DELAY = 1
    /\ DOWN_HYSTERESIS_THRESHOLD = 1
    /\ HYSTERESIS_WAIT = 1
    /\ HonestNodes = { "n3", "n4" }
    /\ MAX_BTC_HEIGHT = 1
    /\ MAX_CENSORSHIP_ROUNDS = 3
    /\ MAX_CHURN_RATE = 1
    /\ MAX_DOWN_HYSTERESIS_THRESHOLD = 8
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
    /\ Proposer = SetAsFun({ <<0, "n1">>, <<1, "n2">>, <<2, "n1">> })
    /\ RESET_TIME = 2
    /\ SOVEREIGN_THRESHOLD = 2
    /\ SUSPICIOUS_HYSTERESIS_WAIT = 1
    /\ SUSPICIOUS_THRESHOLD = 1
    /\ T = 1
    /\ TIMEOUT_DURATION = 2
    /\ action = "Init"
    /\ active_peers = { "anchor_n1", "anchor_n2", "anchor_n3" }
    /\ anchor_peers = { "anchor_n1", "anchor_n2", "anchor_n3" }
    /\ avg_peer_tenure = 1
    /\ begin_round = SetAsFun({ <<0, 0>>, <<1, 2>>, <<2, 2>> })
    /\ blacklisted_peers = {}
    /\ da_backfill_valid = FALSE
    /\ decision
      = SetAsFun({ <<
          "n3", [prop |->
              [btc_receipt |->
                  [checkpoint_block_hash |-> <<"NIL", -1>>,
                    checkpoint_block_height |-> -1],
                da_receipt |->
                  [attestation |-> FALSE, published_block_height |-> -1],
                fsm_state |-> "NONE",
                healthy |-> FALSE,
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
                healthy |-> FALSE,
                round |-> -1,
                timestamp |-> -1,
                value |-> "NIL_TX",
                zk_proof_ref |-> FALSE],
            round |-> -1]
        >> })
    /\ end_consensus = SetAsFun({ <<"n3", -1>>, <<"n4", -1>> })
    /\ evidence = {}
    /\ failed_recovery_attempts = 0
    /\ forced_tx_queue = {"TX_NORMAL"}
    /\ h_btc_anchored = 0
    /\ h_btc_current = 0
    /\ h_btc_submitted = 0
    /\ h_engram_current = 0
    /\ h_engram_verified = 0
    /\ health_certs = {}
    /\ inspected_proposal
      = SetAsFun({ <<<<0, "n3">>, -1>>,
        <<<<0, "n4">>, -1>>,
        <<<<1, "n3">>, -1>>,
        <<<<1, "n4">>, -1>>,
        <<<<2, "n3">>, -1>>,
        <<<<2, "n4">>, -1>> })
    /\ is_anchor_reverification_failed = FALSE
    /\ is_attestation_failed = FALSE
    /\ is_das_failed = FALSE
    /\ last_begin_round = SetAsFun({ <<0, 1>>, <<1, -1>>, <<2, -1>> })
    /\ local_clock = SetAsFun({ <<"n3", 0>>, <<"n4", 1>> })
    /\ local_rem_time = SetAsFun({ <<"n3", 2>>, <<"n4", 2>> })
    /\ locked_round = SetAsFun({ <<"n3", -1>>, <<"n4", -1>> })
    /\ locked_value = SetAsFun({ <<"n3", "NIL_TX">>, <<"n4", "NIL_TX">> })
    /\ msgs_precommit
      = SetAsFun({ <<
          0, { [id |->
                [btc_receipt |->
                    [checkpoint_block_hash |-> <<"NIL", -1>>,
                      checkpoint_block_height |-> -1],
                  da_receipt |->
                    [attestation |-> FALSE, published_block_height |-> -1],
                  fsm_state |-> "NONE",
                  healthy |-> FALSE,
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
                  healthy |-> FALSE,
                  round |-> -1,
                  timestamp |-> -1,
                  value |-> "NIL_TX",
                  zk_proof_ref |-> FALSE],
              round |-> 0,
              src |-> "n1",
              type |-> "PRECOMMIT",
              valid_round |-> -1],
            [id |->
                [btc_receipt |->
                    [checkpoint_block_hash |-> <<"NIL", -1>>,
                      checkpoint_block_height |-> -1],
                  da_receipt |->
                    [attestation |-> FALSE, published_block_height |-> -1],
                  fsm_state |-> "NONE",
                  healthy |-> FALSE,
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
                  healthy |-> FALSE,
                  round |-> -1,
                  timestamp |-> -1,
                  value |-> "NIL_TX",
                  zk_proof_ref |-> FALSE],
              round |-> 0,
              src |-> "n2",
              type |-> "PRECOMMIT",
              valid_round |-> -1] }
        >>,
        <<
          1, { [id |->
                [btc_receipt |->
                    [checkpoint_block_hash |-> <<"NIL", -1>>,
                      checkpoint_block_height |-> -1],
                  da_receipt |->
                    [attestation |-> FALSE, published_block_height |-> -1],
                  fsm_state |-> "NONE",
                  healthy |-> FALSE,
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
                  healthy |-> FALSE,
                  round |-> -1,
                  timestamp |-> -1,
                  value |-> "NIL_TX",
                  zk_proof_ref |-> FALSE],
              round |-> 1,
              src |-> "n1",
              type |-> "PRECOMMIT",
              valid_round |-> -1],
            [id |->
                [btc_receipt |->
                    [checkpoint_block_hash |-> <<"NIL", -1>>,
                      checkpoint_block_height |-> -1],
                  da_receipt |->
                    [attestation |-> FALSE, published_block_height |-> -1],
                  fsm_state |-> "NONE",
                  healthy |-> FALSE,
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
                  healthy |-> FALSE,
                  round |-> -1,
                  timestamp |-> -1,
                  value |-> "NIL_TX",
                  zk_proof_ref |-> FALSE],
              round |-> 1,
              src |-> "n2",
              type |-> "PRECOMMIT",
              valid_round |-> -1] }
        >>,
        <<
          2, { [id |->
                [btc_receipt |->
                    [checkpoint_block_hash |-> <<"NIL", -1>>,
                      checkpoint_block_height |-> -1],
                  da_receipt |->
                    [attestation |-> FALSE, published_block_height |-> -1],
                  fsm_state |-> "NONE",
                  healthy |-> FALSE,
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
                  healthy |-> FALSE,
                  round |-> -1,
                  timestamp |-> -1,
                  value |-> "NIL_TX",
                  zk_proof_ref |-> FALSE],
              round |-> 2,
              src |-> "n1",
              type |-> "PRECOMMIT",
              valid_round |-> -1],
            [id |->
                [btc_receipt |->
                    [checkpoint_block_hash |-> <<"NIL", -1>>,
                      checkpoint_block_height |-> -1],
                  da_receipt |->
                    [attestation |-> FALSE, published_block_height |-> -1],
                  fsm_state |-> "NONE",
                  healthy |-> FALSE,
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
                  healthy |-> FALSE,
                  round |-> -1,
                  timestamp |-> -1,
                  value |-> "NIL_TX",
                  zk_proof_ref |-> FALSE],
              round |-> 2,
              src |-> "n2",
              type |-> "PRECOMMIT",
              valid_round |-> -1] }
        >> })
    /\ msgs_prevote
      = SetAsFun({ <<
          0, { [id |->
                [btc_receipt |->
                    [checkpoint_block_hash |-> <<"NIL", -1>>,
                      checkpoint_block_height |-> -1],
                  da_receipt |->
                    [attestation |-> FALSE, published_block_height |-> -1],
                  fsm_state |-> "NONE",
                  healthy |-> FALSE,
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
                  healthy |-> FALSE,
                  round |-> -1,
                  timestamp |-> -1,
                  value |-> "NIL_TX",
                  zk_proof_ref |-> FALSE],
              round |-> 0,
              src |-> "n1",
              type |-> "PREVOTE",
              valid_round |-> -1],
            [id |->
                [btc_receipt |->
                    [checkpoint_block_hash |-> <<"NIL", -1>>,
                      checkpoint_block_height |-> -1],
                  da_receipt |->
                    [attestation |-> FALSE, published_block_height |-> -1],
                  fsm_state |-> "NONE",
                  healthy |-> FALSE,
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
                  healthy |-> FALSE,
                  round |-> -1,
                  timestamp |-> -1,
                  value |-> "NIL_TX",
                  zk_proof_ref |-> FALSE],
              round |-> 0,
              src |-> "n2",
              type |-> "PREVOTE",
              valid_round |-> -1] }
        >>,
        <<
          1, { [id |->
                [btc_receipt |->
                    [checkpoint_block_hash |-> <<"NIL", -1>>,
                      checkpoint_block_height |-> -1],
                  da_receipt |->
                    [attestation |-> FALSE, published_block_height |-> -1],
                  fsm_state |-> "NONE",
                  healthy |-> FALSE,
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
                  healthy |-> FALSE,
                  round |-> -1,
                  timestamp |-> -1,
                  value |-> "NIL_TX",
                  zk_proof_ref |-> FALSE],
              round |-> 1,
              src |-> "n1",
              type |-> "PREVOTE",
              valid_round |-> -1],
            [id |->
                [btc_receipt |->
                    [checkpoint_block_hash |-> <<"NIL", -1>>,
                      checkpoint_block_height |-> -1],
                  da_receipt |->
                    [attestation |-> FALSE, published_block_height |-> -1],
                  fsm_state |-> "NONE",
                  healthy |-> FALSE,
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
                  healthy |-> FALSE,
                  round |-> -1,
                  timestamp |-> -1,
                  value |-> "NIL_TX",
                  zk_proof_ref |-> FALSE],
              round |-> 1,
              src |-> "n2",
              type |-> "PREVOTE",
              valid_round |-> -1] }
        >>,
        <<
          2, { [id |->
                [btc_receipt |->
                    [checkpoint_block_hash |-> <<"NIL", -1>>,
                      checkpoint_block_height |-> -1],
                  da_receipt |->
                    [attestation |-> FALSE, published_block_height |-> -1],
                  fsm_state |-> "NONE",
                  healthy |-> FALSE,
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
                  healthy |-> FALSE,
                  round |-> -1,
                  timestamp |-> -1,
                  value |-> "NIL_TX",
                  zk_proof_ref |-> FALSE],
              round |-> 2,
              src |-> "n1",
              type |-> "PREVOTE",
              valid_round |-> -1],
            [id |->
                [btc_receipt |->
                    [checkpoint_block_hash |-> <<"NIL", -1>>,
                      checkpoint_block_height |-> -1],
                  da_receipt |->
                    [attestation |-> FALSE, published_block_height |-> -1],
                  fsm_state |-> "NONE",
                  healthy |-> FALSE,
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
                  healthy |-> FALSE,
                  round |-> -1,
                  timestamp |-> -1,
                  value |-> "NIL_TX",
                  zk_proof_ref |-> FALSE],
              round |-> 2,
              src |-> "n2",
              type |-> "PREVOTE",
              valid_round |-> -1] }
        >> })
    /\ msgs_propose = SetAsFun({ <<0, {}>>, <<1, {}>>, <<2, {}>> })
    /\ msgs_timeout
      = SetAsFun({ <<
          0, { [id |->
                [btc_receipt |->
                    [checkpoint_block_hash |-> <<"NIL", -1>>,
                      checkpoint_block_height |-> -1],
                  da_receipt |->
                    [attestation |-> FALSE, published_block_height |-> -1],
                  fsm_state |-> "NONE",
                  healthy |-> FALSE,
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
                  healthy |-> FALSE,
                  round |-> -1,
                  timestamp |-> -1,
                  value |-> "NIL_TX",
                  zk_proof_ref |-> FALSE],
              round |-> 0,
              src |-> "n1",
              type |-> "TIMEOUT",
              valid_round |-> -1],
            [id |->
                [btc_receipt |->
                    [checkpoint_block_hash |-> <<"NIL", -1>>,
                      checkpoint_block_height |-> -1],
                  da_receipt |->
                    [attestation |-> FALSE, published_block_height |-> -1],
                  fsm_state |-> "NONE",
                  healthy |-> FALSE,
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
                  healthy |-> FALSE,
                  round |-> -1,
                  timestamp |-> -1,
                  value |-> "NIL_TX",
                  zk_proof_ref |-> FALSE],
              round |-> 0,
              src |-> "n2",
              type |-> "TIMEOUT",
              valid_round |-> -1] }
        >>,
        <<
          1, { [id |->
                [btc_receipt |->
                    [checkpoint_block_hash |-> <<"NIL", -1>>,
                      checkpoint_block_height |-> -1],
                  da_receipt |->
                    [attestation |-> FALSE, published_block_height |-> -1],
                  fsm_state |-> "NONE",
                  healthy |-> FALSE,
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
                  healthy |-> FALSE,
                  round |-> -1,
                  timestamp |-> -1,
                  value |-> "NIL_TX",
                  zk_proof_ref |-> FALSE],
              round |-> 1,
              src |-> "n1",
              type |-> "TIMEOUT",
              valid_round |-> -1],
            [id |->
                [btc_receipt |->
                    [checkpoint_block_hash |-> <<"NIL", -1>>,
                      checkpoint_block_height |-> -1],
                  da_receipt |->
                    [attestation |-> FALSE, published_block_height |-> -1],
                  fsm_state |-> "NONE",
                  healthy |-> FALSE,
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
                  healthy |-> FALSE,
                  round |-> -1,
                  timestamp |-> -1,
                  value |-> "NIL_TX",
                  zk_proof_ref |-> FALSE],
              round |-> 1,
              src |-> "n2",
              type |-> "TIMEOUT",
              valid_round |-> -1] }
        >>,
        <<
          2, { [id |->
                [btc_receipt |->
                    [checkpoint_block_hash |-> <<"NIL", -1>>,
                      checkpoint_block_height |-> -1],
                  da_receipt |->
                    [attestation |-> FALSE, published_block_height |-> -1],
                  fsm_state |-> "NONE",
                  healthy |-> FALSE,
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
                  healthy |-> FALSE,
                  round |-> -1,
                  timestamp |-> -1,
                  value |-> "NIL_TX",
                  zk_proof_ref |-> FALSE],
              round |-> 2,
              src |-> "n1",
              type |-> "TIMEOUT",
              valid_round |-> -1],
            [id |->
                [btc_receipt |->
                    [checkpoint_block_hash |-> <<"NIL", -1>>,
                      checkpoint_block_height |-> -1],
                  da_receipt |->
                    [attestation |-> FALSE, published_block_height |-> -1],
                  fsm_state |-> "NONE",
                  healthy |-> FALSE,
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
                  healthy |-> FALSE,
                  round |-> -1,
                  timestamp |-> -1,
                  value |-> "NIL_TX",
                  zk_proof_ref |-> FALSE],
              round |-> 2,
              src |-> "n2",
              type |-> "TIMEOUT",
              valid_round |-> -1] }
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
    /\ received_timely_proposal = SetAsFun({ <<"n3", {}>>, <<"n4", {}>> })
    /\ round = SetAsFun({ <<"n3", 0>>, <<"n4", 0>> })
    /\ safe_blocks = 0
    /\ sensor_view
      = SetAsFun({ <<
          "n1", [btc_gap_obs |-> 3,
            da_gap_obs |-> 1,
            is_critical |-> FALSE,
            is_healthy |-> FALSE,
            is_warning |-> FALSE]
        >>,
        <<
          "n2", [btc_gap_obs |-> 3,
            da_gap_obs |-> 1,
            is_critical |-> FALSE,
            is_healthy |-> FALSE,
            is_warning |-> FALSE]
        >>,
        <<
          "n3", [btc_gap_obs |-> 2,
            da_gap_obs |-> 3,
            is_critical |-> FALSE,
            is_healthy |-> FALSE,
            is_warning |-> FALSE]
        >>,
        <<
          "n4", [btc_gap_obs |-> 2,
            da_gap_obs |-> 1,
            is_critical |-> TRUE,
            is_healthy |-> TRUE,
            is_warning |-> TRUE]
        >> })
    /\ state = "ANCHORED"
    /\ step = SetAsFun({ <<"n3", "PROPOSE">>, <<"n4", "PROPOSE">> })
    /\ suspicious_duration = 0
    /\ suspicious_safe_blocks = 0
    /\ timeout_certs = {}
    /\ tx_ignored_rounds
      = SetAsFun({ <<
          "n3", SetAsFun({ <<"TX_NORMAL", 0>>, <<"TX_WITHDRAWAL", 0>> })
        >>,
        <<"n4", SetAsFun({ <<"TX_NORMAL", 0>>, <<"TX_WITHDRAWAL", 0>> })>> })
    /\ unhealthy_streak = 0
    /\ valid_round = SetAsFun({ <<"n3", -1>>, <<"n4", -1>> })
    /\ valid_value
      = SetAsFun({ <<
          "n3", [btc_receipt |->
              [checkpoint_block_hash |-> <<"NIL", -1>>,
                checkpoint_block_height |-> -1],
            da_receipt |->
              [attestation |-> FALSE, published_block_height |-> -1],
            fsm_state |-> "NONE",
            healthy |-> FALSE,
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
            healthy |-> FALSE,
            round |-> -1,
            timestamp |-> -1,
            value |-> "NIL_TX",
            zk_proof_ref |-> FALSE]
        >> })

(* The following formula holds true in the last state and violates the invariant *)
InvariantViolation ==
  Skolem((\E members_2 \in SUBSET (HonestNodes \union ByzantineNodes):
    ConstCardinality((Cardinality(members_2) >= 3))
      /\ (LET honest_si_2 ==
        { t_l_1 \in members_2: ~(t_l_1 \in ByzantineNodes) }
      IN
      LET cert_si_2 ==
        [is_healthy |->
            2
              * Cardinality({
                m_11 \in members_2:
                  sensor_view[m_11]["is_healthy"]
              })
              > Cardinality(members_2),
          is_warning |->
            2
              * Cardinality({
                m_12 \in members_2:
                  sensor_view[m_12]["is_warning"]
              })
              > Cardinality(members_2),
          is_critical |->
            2
              * Cardinality({
                m_13 \in members_2:
                  sensor_view[m_13]["is_critical"]
              })
              > Cardinality(members_2),
          btc_gap_obs |->
            Cardinality({
              x_5 \in 0 .. 3:
                2
                  * Cardinality({
                    m_14 \in members_2:
                      [
                        m_15 \in members_2 |->
                          sensor_view[m_15]["btc_gap_obs"]
                      ][
                        m_14
                      ]
                        <= x_5
                  })
                  <= Cardinality(members_2)
            }),
          da_gap_obs |->
            Cardinality({
              x_6 \in 0 .. 3:
                2
                  * Cardinality({
                    m_16 \in members_2:
                      [ m_17 \in members_2 |-> sensor_view[m_17]["da_gap_obs"] ][
                        m_16
                      ]
                        <= x_6
                  })
                  <= Cardinality(members_2)
            }),
          quorum_weight |-> Cardinality(members_2)]
      IN
      \A lo_2 \in honest_si_2:
        \A hi_2 \in honest_si_2:
          sensor_view[lo_2]["btc_gap_obs"] > (cert_si_2)["btc_gap_obs"]
            \/ (cert_si_2)["btc_gap_obs"] > sensor_view[hi_2]["btc_gap_obs"])))

================================================================================
(* Created by Apalache on Fri Oct 02 17:57:33 ICT 2026 *)
(* https://github.com/apalache-mc/apalache *)
