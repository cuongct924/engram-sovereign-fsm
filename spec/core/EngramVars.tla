--------------------------- MODULE EngramVars ---------------------------
(*
 * EngramVars — Shared Variable Declarations
 *
 * This module declares ALL variables used across the Engram specification.
 *)

(* ======================== APALACHE TYPING NOTE ==============================
 * Every type tag below is written as a fully expanded structural record
 * rather than a named type alias. Empirically (see mc/fsm/MC_FSMSafety.tla,
 * and the isolated repro kept out-of-repo during that investigation), this
 * Apalache release does not unify a named-alias record type against a
 * literal of the same shape once the literal is constructed in a different
 * module than the one declaring the alias (a "DECISION" tag on this file's
 * VARIABLES block does not unify with a hand-built record given the same
 * alias name in a driver file that merely EXTENDS this module) — plain
 * Int/Str/Set/Bool/function-type tags do not have this problem, only named
 * aliases for composite records/messages do. Since every layer's Apalache
 * driver lives in mc/, outside this file, aliases would hit this on every
 * use, so record shapes are inlined everywhere instead. `evidence` is
 * intentionally left with a narrowed placeholder type below (see its own
 * note at the VARIABLES block) — it mixes three distinct message shapes and
 * only becomes load-bearing for Apalache once the Tendermint/Server layers
 * are annotated (Task A3/A4), not for the FSM layer alone.
 *)

CONSTANTS
    \* @type: Int;
    HYSTERESIS_WAIT,    \* Consecutive safe blocks required for successful recovery
    \* @type: Int;
    DA_THRESHOLD        \* Max allowed block gap since last DA publication verification


(* ======================== TENDERMINT CORE VARIABLES ======================== *)
\* Tendermint BFT state machine variables (per-process maps over Corr).
VARIABLES
    \* @type: Str -> Int;
    round,          \* Current consensus round of each correct process
    \* @type: Str -> Str;
    step,           \* Current step: "PROPOSE" | "PREVOTE" | "PRECOMMIT" | "DECIDED"
    \* @type: Str -> { prop: { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool }, round: Int };
    decision,       \* Decided value (NilDecision if not yet decided)
    \* @type: Str -> Str;
    locked_value,   \* Value locked by the process in the last lock round
    \* @type: Str -> Int;
    locked_round,   \* Round in which locked_value was locked
    \* @type: Str -> { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool };
    valid_value,    \* Most recent valid proposal seen
    \* @type: Str -> Int;
    valid_round     \* Round in which valid_value was observed

tendermintCoreVars == <<round, step, decision, 
                        locked_value, locked_round, valid_value, valid_round>>


(* ======================== TEMPORAL / CLOCK VARIABLES ======================= *)
\* Physical and logical time tracking for clock-synchrony proofs.
VARIABLES
    \* @type: Str -> Int;
    local_clock,    \* Each correct process's local clock reading
    \* @type: Int;
    real_time,      \* Global "wall clock" (advanced by AdvanceRealTime)
    \* @type: Str -> Int;
    local_rem_time  \* Remaining timeout countdown per process

temporalVars == <<local_clock, real_time, local_rem_time>>


(* ======================== BOOKKEEPING VARIABLES ============================ *)
\* Message buffers and audit log.
VARIABLES
    \* @type: Int -> Set({ type: Str, src: Str, round: Int, proposal: { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool }, valid_round: Int });
    msgs_propose,               \* Proposal messages indexed by round
    \* @type: Int -> Set({ type: Str, src: Str, round: Int, id: { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool } });
    msgs_prevote,               \* Prevote messages indexed by round
    \* @type: Int -> Set({ type: Str, src: Str, round: Int, id: { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool } });
    msgs_precommit,             \* Precommit messages indexed by round
    \* @type: Int -> Set({ type: Str, src: Str, round: Int });
    msgs_timeout,               \* Timeout messages indexed by round
    \* @type: Set({ type: Str, src: Str, round: Int, proposal: { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool }, valid_round: Int });
    \* TODO(Apalache, Task A3/A4): `evidence` actually mixes proposal/vote/timeout message
    \* shapes (see UponQuorumOfPrevotesAny, UponQuorumOfPrecommitsAny, OnRoundCatchup,
    \* UponfPlusOneTimeoutsAny in EngramTendermint.tla). This narrowed-to-proposal-message
    \* type is only sound for modules that never exercise those actions (e.g. the FSM-only
    \* pilot, where `evidence` is mocked to a constant and left UNCHANGED). Revisit with a
    \* proper variant type when annotating EngramTendermint.tla/EngramServer.tla.
    evidence,                   \* Set of collected evidence (for accountability)
    \* @type: Str;
    action,                     \* String label of last executed action (for TLC tracing)
    \* @type: Str -> Set({ type: Str, src: Str, round: Int, proposal: { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool }, valid_round: Int });
    received_timely_proposal,   \* Per-process set of timely proposal messages
    \* @type: <<Int, Str>> -> Int;
    inspected_proposal          \* Per-(round,process) timestamp of last inspection

\* Small group 1: messsage broadcast
msgsBroadcastVars == <<msgs_propose, msgs_prevote, msgs_precommit, msgs_timeout>>

\* Small group 2: Auditing proposal
propAuditVars == <<received_timely_proposal, inspected_proposal>>

\* Small group 3: trace and evidence
traceVars == <<evidence, action>>

bookkeepingVars == <<msgsBroadcastVars, propAuditVars, traceVars>>

(* ======================== INVARIANT SUPPORT VARIABLES ====================== *)
\* Ghost variables used exclusively to express timing invariants.
\* These are never read by the protocol logic itself.
VARIABLES
    \* @type: Int -> Int;
    begin_round,            \* Earliest local clock when any process entered round r
    \* @type: Str -> Int;
    end_consensus,          \* Local clock when process p decided
    \* @type: Int -> Int;
    last_begin_round,       \* Latest local clock when any process entered round r
    \* @type: Int -> Int;
    proposal_time,          \* Real time at which the proposal for round r was broadcast
    \* @type: Int -> Int;
    proposal_received_time  \* Real time at which the first timely proposal was received

invariantVars == 
    <<begin_round, end_consensus, last_begin_round, 
        proposal_time, proposal_received_time>>


(* ======================== P2P HEALTH / DA GAP / BTC GAP SENSOR ================================ *)
VARIABLES
    \* @type: Set(Str);
    active_peers,            \* Set of currently connected peers
    \* @type: Set(Str);
    anchor_peers,            \* Statically configured bootstrap/anchor peer set
    \* @type: Set(Str);
    blacklisted_peers,       \* Peers identified as malicious and blacklisted
    \* @type: Int;
    peer_churn_rate,         \* Interference/disconnection rate in the routing table
    \* @type: Int;
    avg_peer_tenure,         \* Average age of current connections
    \* @type: Int;
    peer_latency             \* Average block/heartbeat transmission latency

p2pHealthSensorVars == 
    <<active_peers, anchor_peers, blacklisted_peers, 
        peer_churn_rate, avg_peer_tenure, peer_latency>>


VARIABLES
    \* @type: Int;
    h_engram_current,           \* Latest Engram chain block height
    \* @type: Int;
    h_engram_verified,          \* Last DA-verified Engram block height
    \* @type: Bool;
    is_attestation_failed,      \* DA attestation failure flag from Blobstream
    \* @type: Bool;
    is_das_failed               \* Data availability sampling failure flag

daGapSensorVars == <<h_engram_current, h_engram_verified, is_attestation_failed, is_das_failed>>


VARIABLES
    \* @type: Int;
    h_btc_current,              \* Latest observed Bitcoin block height
    \* @type: Int;
    h_btc_submitted,            \* Height at which the ZK re-anchoring proof was submitted
    \* @type: Int;
    h_btc_anchored,             \* Last confirmed Engram checkpoint height on Bitcoin
    \* @type: Bool;
    is_btc_spv_failed           \* OP_RETURN inclusion check & Block header verification failure flag

btcGapSensorVars == <<h_btc_current, h_btc_submitted, h_btc_anchored, is_btc_spv_failed>>

\* All environmental sensors
networkSensorVars == <<p2pHealthSensorVars, daGapSensorVars, btcGapSensorVars>>


(* ======================== FSM VARIABLES ====================== *)
\* Circuit-breaker FSM state
VARIABLES
    \* @type: Str;
    state,                   \* FSM state: "ANCHORED"|"SUSPICIOUS"|"SOVEREIGN"|"RECOVERING"
    \* @type: Int;
    safe_blocks,             \* Consecutive healthy blocks counted during RECOVERING
    \* @type: Int;
    suspicious_duration,     \* Count the number of system blocks/ticks stuck in SUSPICIOUS
    \* @type: Bool;
    reanchoring_proof_valid  \* Boolean: ZK re-anchoring proof confirmed on-chain

\* Top-level FSM tuple consumed by EngramTendermint actions
fsmVars == <<state, safe_blocks, suspicious_duration, reanchoring_proof_valid>>


(* ======================== CENSORSHIP VARIABLES ======================= *)
VARIABLES
    \* @type: Set(Str);
    forced_tx_queue,         \* Transactions pending forced inclusion (censorship resistance)
    \* @type: Str -> (Str -> Int);
    tx_ignored_rounds        \* Per-(process,tx) counter of rounds where tx was ignored

censorshipVars == <<forced_tx_queue, tx_ignored_rounds>>


(* ======================== LIDO CERTIFICATE VARIABLES ============== *)
\* Abstract pacemaker certificates used by EngramServer and the LiDO refinement.
VARIABLES
    \* @type: Set({ type: Str, round: Int, caller: Str, method: Str, btc_anchored: Int });
    quorum_certs,       \* Set of Quorum Certificates (E_QC, M_QC)
    \* @type: Set({ type: Str, round: Int, caller: Str, btc_anchored: Int });
    timeout_certs       \* Set of Timeout Certificates (T_QC)

\* Tuple of consensus certificates (LiDO Certificates)
certificateVars == <<quorum_certs, timeout_certs>>

=========================================================================
