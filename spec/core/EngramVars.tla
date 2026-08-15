--------------------------- MODULE EngramVars ---------------------------
(*
 * EngramVars — Shared Variable Declarations
 *
 * This module declares ALL variables used across the Engram specification.
 *)

(* ======================== APALACHE TYPING NOTE ==============================
 * Every type tag below is written as a fully expanded structural record
 * rather than a named type alias. Empirically (see core/MC_FSMSafety.tla,
 * and the isolated repro kept out-of-repo during that investigation), this
 * Apalache release does not unify a named-alias record type against a
 * literal of the same shape once the literal is constructed in a different
 * module than the one declaring the alias (a "DECISION" tag on this file's
 * VARIABLES block does not unify with a hand-built record given the same
 * alias name in a driver file that merely EXTENDS this module) — plain
 * Int/Str/Set/Bool/function-type tags do not have this problem, only named
 * aliases for composite records/messages do. Since every layer's Apalache
 * driver EXTENDS this module from a separate MC_*_Apalache.tla file, aliases
 * would hit this on every use, so record shapes are inlined everywhere
 * instead. `evidence` and the 4 msgs_* buffers all share one unified 6-field
 * message record (type/src/round/proposal/valid_round/id), with Nil
 * sentinels on the fields a given message kind doesn't use (Task A3, see
 * EngramTendermint.tla's Broadcast*/Faulty* constructors) — this is what
 * lets Apalache's Snowcat type-check `\union`s across message kinds without
 * needing a Variant (sum) type.
 *)

CONSTANTS
    \* @type: Int;
    HYSTERESIS_WAIT,    \* Consecutive safe blocks required for successful recovery
    \* @type: Int;
    DA_THRESHOLD,       \* Max allowed block gap since last DA publication verification
    \* @type: Int;
    SUSPICIOUS_HYSTERESIS_WAIT  \* Consecutive healthy blocks required to exit SUSPICIOUS ->
                                 \* ANCHORED -- mirrors HYSTERESIS_WAIT's role on the
                                 \* RECOVERING -> ANCHORED edge, applied to SUSPICIOUS's own
                                 \* exit edge (closes the single-block gray-failure-arbitrage gap)


(* ======================== TENDERMINT CORE VARIABLES ======================== *)
\* Tendermint BFT state machine variables (per-process maps over Corr).
VARIABLES
    \* @type: Str -> Int;
    round,          \* Current consensus round of each correct process
    \* @type: Str -> Str;
    step,           \* Current step: "PROPOSE" | "PREVOTE" | "PRECOMMIT" | "DECIDED"
    \* @type: Str -> { prop: { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool, healthy: Bool }, round: Int };
    decision,       \* Decided value (NilDecision if not yet decided)
    \* @type: Str -> Str;
    locked_value,   \* Value locked by the process in the last lock round
    \* @type: Str -> Int;
    locked_round,   \* Round in which locked_value was locked
    \* @type: Str -> { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool, healthy: Bool };
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
\* Apalache note (resolved Task A3): all 4 message kinds (propose/prevote/
\* precommit/timeout) are annotated with the SAME 6-field record shape
\* (type/src/round/proposal/valid_round/id), each message using Nil sentinels
\* on the fields its own kind doesn't populate (see EngramTendermint.tla's
\* Broadcast*/Faulty* constructors). This lets Apalache's Snowcat unify sets
\* built from different message kinds (e.g. msgs_propose[r] \union
\* msgs_prevote[r] \union msgs_precommit[r] in OnRoundCatchup, or `evidence`
\* below) without a Variant type.
VARIABLES
    \* @type: Int -> Set({ type: Str, src: Str, round: Int, proposal: { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool, healthy: Bool }, valid_round: Int, id: { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool, healthy: Bool } });
    msgs_propose,               \* Proposal messages indexed by round
    \* @type: Int -> Set({ type: Str, src: Str, round: Int, proposal: { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool, healthy: Bool }, valid_round: Int, id: { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool, healthy: Bool } });
    msgs_prevote,               \* Prevote messages indexed by round
    \* @type: Int -> Set({ type: Str, src: Str, round: Int, proposal: { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool, healthy: Bool }, valid_round: Int, id: { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool, healthy: Bool } });
    msgs_precommit,             \* Precommit messages indexed by round
    \* @type: Int -> Set({ type: Str, src: Str, round: Int, proposal: { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool, healthy: Bool }, valid_round: Int, id: { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool, healthy: Bool } });
    msgs_timeout,               \* Timeout messages indexed by round
    \* @type: Set({ type: Str, src: Str, round: Int, proposal: { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool, healthy: Bool }, valid_round: Int, id: { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool, healthy: Bool } });
    evidence,                   \* Set of collected evidence (for accountability)
    \* @type: Str;
    action,                     \* String label of last executed action (for TLC tracing)
    \* @type: Str -> Set({ type: Str, src: Str, round: Int, proposal: { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool, healthy: Bool }, valid_round: Int, id: { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool, healthy: Bool } });
    received_timely_proposal,   \* Per-process set of timely proposal messages
    \* @type: <<Int, Str>> -> Int;
    inspected_proposal          \* Per-(round,process) timestamp of last inspection

\* Small group 1: messsage broadcast
\* Explicit tuple annotation required: since all 4 grouped variables now share
\* the same unified message-record type (Task A3), Apalache's Snowcat cannot
\* tell a 4-tuple `<<a,a,a,a>>` from `Seq(a)` by inference alone here. This is
\* a nullary operator (no parameters), so the tag is the tuple value type
\* itself -- not a function signature.
\* @type: <<Int -> Set({ type: Str, src: Str, round: Int, proposal: { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool, healthy: Bool }, valid_round: Int, id: { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool, healthy: Bool } }), Int -> Set({ type: Str, src: Str, round: Int, proposal: { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool, healthy: Bool }, valid_round: Int, id: { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool, healthy: Bool } }), Int -> Set({ type: Str, src: Str, round: Int, proposal: { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool, healthy: Bool }, valid_round: Int, id: { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool, healthy: Bool } }), Int -> Set({ type: Str, src: Str, round: Int, proposal: { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool, healthy: Bool }, valid_round: Int, id: { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool, healthy: Bool } })>>;
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
    \* @type: Int;
    suspicious_safe_blocks,  \* Consecutive healthy blocks counted during SUSPICIOUS, gating
                              \* its exit to ANCHORED -- leaks (not hard-resets) on a
                              \* non-healthy block, mirroring safe_blocks' own leak semantics
    \* @type: Int;
    unhealthy_streak,        \* Consecutive warning-only (non-critical) blocks while
                              \* ANCHORED or RECOVERING -- gates the downward transition
                              \* instead of firing on a single noisy reading (E5's flapping finding)
    \* @type: Int;
    failed_recovery_attempts, \* Consecutive RECOVERING -> SOVEREIGN regressions since the
                               \* last successful RECOVERING -> ANCHORED -- exponentially
                               \* backs off RECOVERING's down-hysteresis grace period, so a
                               \* repeated precisely-timed flapping attack (not just a single
                               \* well-timed block) gets progressively harder, not equally
                               \* cheap every cycle.
    \* @type: Bool;
    reanchoring_proof_valid  \* Boolean: ZK re-anchoring proof confirmed on-chain

\* Top-level FSM tuple consumed by EngramTendermint actions
fsmVars == <<state, safe_blocks, suspicious_duration, suspicious_safe_blocks, unhealthy_streak, failed_recovery_attempts, reanchoring_proof_valid>>


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
