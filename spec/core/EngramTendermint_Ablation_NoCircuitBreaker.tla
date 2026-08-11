-------------------- MODULE EngramTendermint_Ablation_NoCircuitBreaker ---------------------------
EXTENDS Integers, FiniteSets, EngramVars, EngramFSM

(***************************************************************************)
(* TODO [FUTURE WORK - APPENDIX]: PIPELINED TENDERMINT (PHASE MERGING)     *)
(* Goal: Block time < 2s (Optimistic Pre-Confirmations) via LiDO Appx D.   *)
(*                                                                         *)
(* Implementation Steps:                                                   *)
(* 1. REMOVE PRECOMMIT: Delete `msgsPrecommit` and related actions.        *)
(* 2. OVERLOAD PREVOTE: A `PREVOTE` referencing `r-1` acts as its commit.  *)
(* 3. DELEGATE COMMIT: Leader[r] delegates block commit to Proposer[r+1].  *)
(* 4. UPDATE LIVENESS: Committing now needs 2 consecutive honest leaders.  *)
(***************************************************************************)


(* ======================== PROTOCOL PARAMETERS ============================= *)
\* General consensus parameters
CONSTANTS
    \* @type: Set(Str);
    HonestNodes,           \* Set of honest (non-byzantine) nodes (processes)
    \* @type: Set(Str);
    ByzantineNodes,         \* Set of Byzantine nodes (may be empty)
    \* @type: Int;
    N,              \* Total number of nodes: |HonestNodes| + |ByzantineNodes|
    \* @type: Int;
    T,              \* Upper bound on the number of Byzantine processes
    \* @type: Int;
    MAX_ROUND,      \* Maximum round number (bounds state space for TLC)
    \* @type: Int -> Str;
    Proposer        \* Proposer schedule: maps each round to a process


\* Timing parameters
CONSTANTS
    \* @type: Int;
    MAX_TIMESTAMP,  \* Maximum clock value (set to large number or \infty)
    \* @type: Int;
    MIN_TIMESTAMP,  \* Minimum clock value (starting offset)
    \* @type: Int;
    DELAY,          \* Maximum message delivery delay
    \* @type: Int;
    PRECISION,      \* Maximum skew between any two correct local clocks
    \* @type: Int;
    TIMEOUT_DURATION \* Propose-step timeout duration


\* External chain height bounds
CONSTANTS
    \* @type: Int;
    MAX_BTC_HEIGHT,     \* Bitcoin block height upper bound for TLC
    \* @type: Int;
    MAX_ENGRAM_HEIGHT,  \* Engram block height upper bound for TLC
    \* @type: Int;
    MAX_IGNORE_ROUNDS   \* Censorship threshold: rounds a tx can be ignored

ASSUME(N = Cardinality(HonestNodes \union ByzantineNodes))

\* The total variable is used for WF_vars and Spec definitions.
tendermintVars ==
    <<tendermintCoreVars, temporalVars, bookkeepingVars,
      invariantVars, censorshipVars>>

(* ======================== BASIC DEFINITIONS ============================= *)
\* @type: Set(Str);
AllProcs == HonestNodes \union ByzantineNodes      \* the set of all processes

\* @type: Set(Int);
Rounds == 0..MAX_ROUND               \* the set of potential rounds
\* @type: Int;
NilRound == -1   \* a special value to denote a nil round, outside of Rounds
\* @type: Set(Int);
RoundsOrNil == Rounds \union {NilRound}

\* @type: Set(Int);
Timestamps == 0..MAX_TIMESTAMP       \* the set of clock ticks
\* @type: Int;
NilTimestamp == -1 \* a special value to denote a nil timestamp, outside of Ticks
\* @type: Set(Int);
TimestampsOrNil == Timestamps \union {NilTimestamp}

\* @type: Set(Str);
Values == {"TX_NORMAL", "TX_WITHDRAWAL"}
\* @type: Set(Str);
ValidValues == Values
\* @type: Str;
NilValue == "NIL_TX"
\* @type: Set(Str);
ValuesOrNil == Values \union {NilValue}


(* ======================== ENGRAM TYPE VARS ============================= *)
\* @type: Set(Str);
FSMStates == {"ANCHORED", "SUSPICIOUS", "SOVEREIGN", "RECOVERING"}
\* @type: Str;
NilFSMState == "NONE"
\* @type: Set(Str);
FSMStatesOrNil == FSMStates \union {NilFSMState}


\* @type: Set(Int);
BTCHeights == 0..MAX_BTC_HEIGHT
\* @type: Set(<<Str, Int>>);
ValidHashes  == { <<"BTC_BLOCK", h>> : h \in BTCHeights }
\* @type: Set(<<Str, Int>>);
ForgedHashes == { <<"BTC_FORK", h>>  : h \in BTCHeights }
\* @type: Set(<<Str, Int>>);
AllHashes    == {<<"NIL", -1>>} \cup ValidHashes \cup ForgedHashes
\* @type: Int;
NilBTCHeight == -1
\* @type: Set({ checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> });
BTCReceipts == [
    checkpoint_block_height : BTCHeights,       \* Height of the Bitcoin block containing the OP_RETURN tx
    checkpoint_block_hash   : AllHashes         \* Hash of the Bitcoin block containing the Engram Checkpoint (OP_RETURN tx)
]
\* @type: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> };
NilBTCReceipt == [
    checkpoint_block_height     |-> NilBTCHeight,
    checkpoint_block_hash       |-> <<"NIL", -1>>
]
\* @type: Set({ checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> });
BTCReceiptsOrNil == BTCReceipts \union {NilBTCReceipt}


\* @type: Set(Int);
DAHeights == 0..MAX_ENGRAM_HEIGHT
\* @type: Int;
NilDAHeights == -1
\* @type: Set({ published_block_height: Int, attestation: Bool });
DAReceipts == [
    published_block_height: DAHeights,       \* Height of published block N-k
    attestation: BOOLEAN                     \* Verification from Blobstream
]
\* @type: { published_block_height: Int, attestation: Bool };
NilDAReceipt == [
    published_block_height      |-> -1,
    attestation                 |-> FALSE
]
\* @type: Set({ published_block_height: Int, attestation: Bool });
DAReceiptsOrNil == DAReceipts \union {NilDAReceipt}

(* ======================== PROPOSAL & DECISION STRUCTURE ============================= *)
\* @type: Set({ value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool });
Proposals == [
    value: ValuesOrNil,
    timestamp: TimestampsOrNil,
    round: RoundsOrNil,
    fsm_state: FSMStatesOrNil,
    da_receipt: DAReceiptsOrNil,
    btc_receipt: BTCReceiptsOrNil,
    zk_proof_ref: BOOLEAN
]
\* @type: { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool };
NilProposal == [
    value           |-> NilValue,
    timestamp       |-> NilTimestamp,
    round           |-> NilRound,
    fsm_state       |-> NilFSMState,
    da_receipt      |-> NilDAReceipt,
    btc_receipt     |-> NilBTCReceipt,
    zk_proof_ref    |-> FALSE
]
\* @type: Set({ value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool });
ProposalsOrNil == Proposals \union {NilProposal}

\* @type: Set({ prop: { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool }, round: Int });
Decisions == [
    prop: Proposals,
    round: Rounds  \* The round where the decision is made
]
\* @type: { prop: { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool }, round: Int };
NilDecision == [
    prop  |-> NilProposal,
    round |-> NilRound
]
\* @type: Set({ prop: { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool }, round: Int });
DecisionsOrNil == Decisions \union {NilDecision}

(* ======================== QUORUM THRESHOLDS ============================== *)
\* @type: Int;
THRESHOLD1 == T + 1       \* f+1: at least one honest node
\* @type: Int;
THRESHOLD2 == 2 * T + 1   \* 2f+1: quorum (requires N > 3T)

(* ======================== BASIC MATH HELPERS ============================= *)
\* a value hash is modeled as identity
\* @type: (t) => t;
Id(v) == v

\* @type: (Int, Int) => Int;
Min2(a,b) == IF a <= b THEN a ELSE b
\* @type: (Set(Int)) => Int;
\* Min(S) == FoldSet( Min2, MAX_TIMESTAMP, S )
Min(S) == CHOOSE x \in S : \A y \in S : x <= y

\* @type: (Int, Int) => Int;
Max2(a,b) == IF a >= b THEN a ELSE b
\* @type: (Set(Int)) => Int;
\* Max(S) == FoldSet( Max2, NilTimestamp, S )
Max(S) == CHOOSE x \in S : \A y \in S : y <= x

\* @type: (Set(a)) => Int;
\* Card(S) ==
\*   LET
\*     PlusOne(i, m) == i + 1   (would need its own type tag if uncommented)
\*   IN FoldSet( PlusOne, 0, S )
Card(S) == Cardinality(S)


(********************* TIME UTILITIES ******************************)
\* Checks that t has not exceeded the model's timestamp bound.
\* Set MAX_TIMESTAMP to a large value to model an unbounded clock.
\* @type: (Int) => Bool;
ValidTime(t) == t < MAX_TIMESTAMP


\* Clock-synchrony predicate: a message is timely if it arrives within the
\* [messageTime - Precision, messageTime + Precision + Delay] window.
\* @type: (Int, Int) => Bool;
IsTimely(processTime, messageTime) ==
    /\ processTime >= messageTime - PRECISION
    /\ processTime <= messageTime + PRECISION + DELAY


\* TRUE if all pairs of correct clocks are within Precision of each other.
\* @type: Bool;
SynchronizedLocalClocks ==
    \A p \in HonestNodes : \A q \in HonestNodes :
        p /= q =>
            \/ /\ local_clock[p] >= local_clock[q]
               /\ local_clock[p] - local_clock[q] <= PRECISION
            \/ /\ local_clock[p] <  local_clock[q]
               /\ local_clock[q] - local_clock[p] <= PRECISION

(********************* DYNAMIC TOLERANCE CALCULATION *********************)
\* The tolerance expands dynamically based on the Consensus Round.
\* It only applies to exogenous physical metrics (DA Blobstream & Bitcoin SPV).
DATolerance(r) ==
    CASE r <= 1 -> 0
      [] r = 2  -> 2
      [] r >= 3 -> 4
      [] OTHER  -> 0

BTCTolerance(r) ==
    CASE r <= 2 -> 0
      [] r >= 3 -> 1
      [] OTHER  -> 0



(* ======================== PROPOSAL HELPERS ================================ *)
\* TRUE if the proposal value is a cross-chain withdrawal transaction
\* @type: (Str) => Bool;
ContainsWithdrawal(propVal) == propVal = "TX_WITHDRAWAL"

\* Black-box verification: O(1) time complexity simulation for ZK-Proofs
\* @type: (Bool, { published_block_height: Int, attestation: Bool }) => Bool;
VerifyZkProof(zk_proof, da_receipt) ==
    /\ zk_proof = TRUE                                  \* Leader claims proof exists
    /\ da_receipt.attestation = TRUE                            \* DA layer confirms data is available
    /\ da_receipt.published_block_height > h_engram_verified    \* Check if the proof corresponds to the recovery target


\* Abstracts a cryptographic hash as an identity mapping that returns
\* the Canonical Hash for a given block height.
\* @type: (Int) => <<Str, Int>>;
ExpectedBlockHash(height) == <<"BTC_BLOCK", height>>

\* Simulates an SPV Light Client validating a BTC Receipt.
\* If an eclipsed Proposer submits a forged branch (e.g., <<"BTC_FORK", height>>), the SPV client will reject it.
\* @type: ({ checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }) => Bool;
VerifySPVProof(receipt) ==
    /\ receipt.checkpoint_block_height <= h_btc_current
    /\ receipt.checkpoint_block_height >= h_btc_anchored
    \* The receipt's hash must stricty match the Canonical Chain's hash
    /\ receipt.checkpoint_block_hash = ExpectedBlockHash(receipt.checkpoint_block_height)


(********************* CORE PROPOSAL VALIDITY (SEMANTIC FIREWALL) ******************************)
\* The core validity predicate for proposals
\* @type: ({ value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool }) => Bool;
IsValidProposal(prop) ==
    LET 
        da_tol  == DATolerance(prop.round)
        btc_tol == BTCTolerance(prop.round)
    IN
        /\ prop.value \in ValidValues
        /\ prop.timestamp \in MIN_TIMESTAMP..MAX_TIMESTAMP
        /\ prop.fsm_state = CalculateNextFSMState   \* Cross-check
        
        \* DA Pipeline Check: Data must be available and within the allowed gap
        /\ (prop.fsm_state \in {"ANCHORED", "RECOVERING"} \/ IsDAHealthy) => 
            /\ prop.da_receipt.attestation = TRUE
            /\ prop.da_receipt.published_block_height <= h_engram_current
            /\ prop.da_receipt.published_block_height >= (h_engram_current - DA_THRESHOLD - da_tol)

        \* Settlement Monotonicity & BTC Light Client Hash Check
        /\ prop.btc_receipt.checkpoint_block_height >= (h_btc_current - btc_tol)
        /\ VerifySPVProof(prop.btc_receipt)

        \* ABLATION (Remove Circuit Breaker): the withdrawal-lock conjunct
        \* that normally lives here has been deleted for this experiment --
        \* see core/EngramTendermint.tla's IsValidProposal for the real,
        \* checked-in version. Sanity_NeverAttemptWithdrawalLeakage (an
        \* invariant designed to hold in the real spec) is expected to FAIL
        \* here, proving TLC can actually reach a withdrawal-during-SOVEREIGN/
        \* RECOVERING state once this line is gone.

        \* RE-ANCHORING Logic: Mandatory ZK-Proof when hysteresis wait is met
        \* If not met, strict enforcement that no fake ZK-proof is attached.
        /\  IF prop.fsm_state = "RECOVERING" /\ safe_blocks = HYSTERESIS_WAIT 
            THEN VerifyZkProof(prop.zk_proof_ref, prop.da_receipt)
            ELSE prop.zk_proof_ref = FALSE


\* Censorship sensor: TRUE iff process p should reject proposal for being censored
\* @type: (Str, { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool }) => Bool;
IsCensoring(p, prop) ==
    \E tx \in forced_tx_queue :
        /\ tx_ignored_rounds[p][tx] >= MAX_IGNORE_ROUNDS
        /\ prop.value /= tx


(* ======================== RECORD CONSTRUCTORS ============================ *)
\* @type: (Str, Int, Int, Str, { published_block_height: Int, attestation: Bool }, { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, Bool) => { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool };
Proposal(v, t, r, fsm_s, da_receipt, btc_receipt, has_proof) ==
    [
        value        |-> v,
        timestamp    |-> t,
        round        |-> r,
        fsm_state    |-> fsm_s,
        da_receipt   |-> da_receipt,
        btc_receipt  |-> btc_receipt,
        zk_proof_ref |-> has_proof
    ]

\* @type: ({ value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool }, Int) => { prop: { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool }, round: Int };
Decision(prop, r) ==
    [
        prop  |-> prop,
        round |-> r
    ]


(* ======================== BYZANTINE MESSAGE SETS ========================= *)
\* Pre-populate message buffers with byzantine nodes' default messages so they
\* can immediately contribute to quorums (modelling BFT adversary capability).
\* Only T × MAX_ROUND messages total — negligible state space cost.

\* Apalache note: every message record below carries all 6 fields
\* (type/src/round/proposal/valid_round/id), with Nil sentinels on fields a
\* given type doesn't use -- keeps msgs_propose/msgs_prevote/msgs_precommit/
\* msgs_timeout structurally identical so Apalache's Snowcat can unify sets
\* built from different message kinds without needing a Variant type.
\* Behaviorally inert: no comparison reads a field outside its own kind.
\* @type: (Int) => Set({ type: Str, src: Str, round: Int, proposal: { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool }, valid_round: Int, id: { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool } });
FaultyTimeouts(r) ==
    { [type |-> "TIMEOUT",    src |-> f, round |-> r,
       proposal |-> NilProposal, valid_round |-> NilRound, id |-> NilProposal] : f \in ByzantineNodes }

\* @type: (Int) => Set({ type: Str, src: Str, round: Int, proposal: { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool }, valid_round: Int, id: { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool } });
FaultyPrevotes(r) ==
    { [type |-> "PREVOTE",   src |-> f, round |-> r, id |-> Id(NilProposal),
       proposal |-> NilProposal, valid_round |-> NilRound] : f \in ByzantineNodes }

\* @type: (Int) => Set({ type: Str, src: Str, round: Int, proposal: { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool }, valid_round: Int, id: { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool } });
FaultyPrecommits(r) ==
    { [type |-> "PRECOMMIT", src |-> f, round |-> r, id |-> Id(NilProposal),
       proposal |-> NilProposal, valid_round |-> NilRound] : f \in ByzantineNodes }

(* ======================== INITIALIZATION ================================= *)
\* Helper: set of all structurally valid proposal messages for round r
\* Unused elsewhere in this module (dead code) -- annotated with its own
\* literal 5-field shape rather than the unified 6-field message shape used
\* by BroadcastProposal, since nothing ever mixes this Set with the real
\* msgs_propose sets.
\* @type: (Int) => Set({ type: Str, src: Str, round: Int, proposal: { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool }, valid_round: Int });
RoundProposals(r) ==
    [
        type        : {"PROPOSAL"},
        src         : AllProcs,
        round       : {r},
        proposal    : Proposals,
        valid_round : RoundsOrNil
    ]

\* Sanity check: message function contains only messages for their own round
\* @type: (Int -> Set({ type: Str, src: Str, round: Int, proposal: { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool }, valid_round: Int, id: { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool } })) => Bool;
BenignRoundsInMessages(msgfun) ==
  \* the message function never contains a message for a wrong round
  \A r \in Rounds:
    \A m \in msgfun[r]:
      r = m.round

\* Initial state — some Byzantine messages may already be present
TendermintInit ==
    /\ round              = [p \in HonestNodes |-> 0]
    /\ local_clock       \in [HonestNodes -> MIN_TIMESTAMP..(MIN_TIMESTAMP + PRECISION)]
    /\ local_rem_time     = [p \in HonestNodes |-> TIMEOUT_DURATION]
    /\ real_time          = 0
    /\ step               = [p \in HonestNodes |-> "PROPOSE"]
    /\ decision           = [p \in HonestNodes |-> NilDecision]
    /\ locked_value        = [p \in HonestNodes |-> NilValue]
    /\ locked_round        = [p \in HonestNodes |-> NilRound]
    /\ valid_value         = [p \in HonestNodes |-> NilProposal]
    /\ valid_round         = [p \in HonestNodes |-> NilRound]
    /\ msgs_propose        = [r \in Rounds |-> {}]
    /\ msgs_prevote        = [r \in Rounds |-> FaultyPrevotes(r)]
    /\ msgs_precommit      = [r \in Rounds |-> FaultyPrecommits(r)]
    /\ msgs_timeout        = [r \in Rounds |-> FaultyTimeouts(r)]
    /\ received_timely_proposal = [p \in HonestNodes |-> {}]
    /\ inspected_proposal      = [r \in Rounds, p \in HonestNodes |-> NilTimestamp]
    /\ BenignRoundsInMessages(msgs_propose)
    /\ BenignRoundsInMessages(msgs_prevote)
    /\ BenignRoundsInMessages(msgs_precommit)
    /\ evidence           = {}
    /\ action             = "Init"
    /\ begin_round         = [r \in Rounds |->
                                IF r = 0
                                THEN Min({local_clock[p] : p \in HonestNodes})
                                ELSE MAX_TIMESTAMP]
    /\ end_consensus       = [p \in HonestNodes |-> NilTimestamp]
    /\ last_begin_round    = [r \in Rounds |->
                                IF r = 0
                                THEN Max({local_clock[p] : p \in HonestNodes})
                                ELSE NilTimestamp]
    /\ proposal_time         = [r \in Rounds |-> NilTimestamp]
    /\ proposal_received_time = [r \in Rounds |-> NilTimestamp]
    /\ forced_tx_queue      = {"TX_NORMAL"}
    /\ tx_ignored_rounds    = [p \in HonestNodes |-> [tx \in ValidValues |-> 0]]


(* ======================== MESSAGE BROADCAST HELPERS ====================== *)
\* @type: (Str, Int, { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool }, Int) => Bool;
BroadcastProposal(pSrc, pRound, pProposal, pValidRound) ==
    LET
        \* @type: { type: Str, src: Str, round: Int, proposal: { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool }, valid_round: Int, id: { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool } };
        new_msg == [
            type        |-> "PROPOSAL",
            src         |-> pSrc,
            round       |-> pRound,
            proposal    |-> pProposal,
            valid_round |-> pValidRound,
            id          |-> NilProposal
        ]
    IN
    msgs_propose' = [msgs_propose EXCEPT ![pRound] = msgs_propose[pRound] \union {new_msg}]

\* @type: (Str, Int, { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool }) => Bool;
BroadcastPrevote(pSrc, pRound, pId) ==
    LET
        \* @type: { type: Str, src: Str, round: Int, proposal: { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool }, valid_round: Int, id: { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool } };
        new_msg == [
            type        |-> "PREVOTE",
            src         |-> pSrc,
            round       |-> pRound,
            id          |-> pId,
            proposal    |-> NilProposal,
            valid_round |-> NilRound
        ]
    IN
    msgs_prevote' = [msgs_prevote EXCEPT ![pRound] = msgs_prevote[pRound] \union {new_msg}]

\* @type: (Str, Int, { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool }) => Bool;
BroadcastPrecommit(pSrc, pRound, pId) ==
    LET
        \* @type: { type: Str, src: Str, round: Int, proposal: { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool }, valid_round: Int, id: { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool } };
        new_msg == [
            type        |-> "PRECOMMIT",
            src         |-> pSrc,
            round       |-> pRound,
            id          |-> pId,
            proposal    |-> NilProposal,
            valid_round |-> NilRound
        ]
    IN
    msgs_precommit' = [msgs_precommit EXCEPT ![pRound] = msgs_precommit[pRound] \union {new_msg}]

\* @type: (Str, Int) => Bool;
BroadcastTimeout(pSrc, pRound) ==
    LET
        \* @type: { type: Str, src: Str, round: Int, proposal: { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool }, valid_round: Int, id: { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool } };
        new_msg == [
            type        |-> "TIMEOUT",
            src         |-> pSrc,
            round       |-> pRound,
            proposal    |-> NilProposal,
            valid_round |-> NilRound,
            id          |-> NilProposal
        ]
    IN
    msgs_timeout' = [msgs_timeout EXCEPT ![pRound] = msgs_timeout[pRound] \union {new_msg}]


(* ======================== ROUND MANAGEMENT ================================ *)
\* Increment ignored-round counters for all pending forced transactions
UpdateIgnoredRounds(p) ==
    tx_ignored_rounds' = [p_idx \in HonestNodes |->
        [tx \in ValidValues |->
            IF p_idx = p THEN
                IF \E m \in msgs_propose[round[p]] : m.proposal.value = tx
                THEN 0
                ELSE MinVal(tx_ignored_rounds[p][tx] + 1, MAX_IGNORE_ROUNDS + 1)
            ELSE tx_ignored_rounds[p_idx][tx]
        ]
    ]

\* Move process p to round r, resetting its step and timeout
\* @type: (Str, Int) => Bool;
StartRound(p, r) ==
    /\ r \in Rounds
    /\ step[p] /= "DECIDED" \* a decided process does not participate in consensus
    /\ round' = [round EXCEPT ![p] = r]
    /\ step' = [step EXCEPT ![p] = "PROPOSE"]
    \* We only need to update (last)beginRound[r] once a process enters round `r`
    /\ begin_round' = [begin_round EXCEPT ![r] = Min2(@, local_clock[p])]
    /\ last_begin_round' = [last_begin_round EXCEPT ![r] = Max2(@, local_clock[p])]

    /\ local_rem_time' = [local_rem_time EXCEPT ![p] = TIMEOUT_DURATION]
    /\ UpdateIgnoredRounds(p)

(* ======================== PROTOCOL ACTIONS ================================ *)

\* -- InsertProposal: called by EngramServer to inject a pre-built proposal --
\* @type: (Str, { value: Str, timestamp: Int, round: Int, fsm_state: Str, da_receipt: { published_block_height: Int, attestation: Bool }, btc_receipt: { checkpoint_block_height: Int, checkpoint_block_hash: <<Str, Int>> }, zk_proof_ref: Bool }) => Bool;
InsertProposal(p, prop) ==
    LET r == round[p] IN
    /\ p = Proposer[r]
    /\ step[p] = "PROPOSE"
    /\ \A m \in msgs_propose[r] : m.src /= p
    /\ BroadcastProposal(p, r, prop, valid_round[p])
    /\ IsValidProposal(prop)
    /\ proposal_time' = [proposal_time EXCEPT ![r] = real_time]
    /\ UNCHANGED <<tendermintCoreVars, temporalVars, propAuditVars, censorshipVars>>
    /\ UNCHANGED <<msgs_prevote, msgs_precommit, msgs_timeout, evidence>>
    /\ UNCHANGED <<begin_round, end_consensus, last_begin_round, proposal_received_time>>
    /\ action' = "InsertProposal"


\* -- ReceiveProposal: time-bounded proposal buffer (IsTimely filter) --
\* @type: (Str) => Bool;
ReceiveProposal(p) ==
    LET r == round[p] IN
    \E msg \in msgs_propose[r] :
        /\ msg.type       = "PROPOSAL"
        /\ msg.src        = Proposer[r]
        /\ msg.valid_round = NilRound
        /\ inspected_proposal[r, p] = NilTimestamp
        /\ msg \notin received_timely_proposal[p]
        /\ inspected_proposal' = [inspected_proposal EXCEPT ![r, p] = local_clock[p]]
        /\ LET is_timely == IsTimely(local_clock[p], msg.proposal.timestamp) IN
               \/ /\ is_timely
                  /\ received_timely_proposal' =
                         [received_timely_proposal EXCEPT ![p] = @ \union {msg}]
                  /\ IF proposal_received_time[r] = NilTimestamp
                     THEN proposal_received_time' = [proposal_received_time EXCEPT ![r] = real_time]
                     ELSE UNCHANGED <<proposal_received_time>>
               \/ /\ ~is_timely
                  /\ UNCHANGED <<received_timely_proposal, proposal_received_time>>
        
        /\ UNCHANGED <<tendermintCoreVars, temporalVars, msgsBroadcastVars>>
        /\ UNCHANGED <<censorshipVars>>
        /\ UNCHANGED <<evidence, begin_round, end_consensus, last_begin_round, proposal_time>>
        /\ action' = "ReceiveProposal"

\* -- UponProposalInPropose: gatekeeper — evaluates proposal validity and votes --
\* If censorship detected: broadcast timeout and skip to next round.
\* Otherwise: PREVOTE for the proposal (or Nil if invalid).
\* @type: (Str) => Bool;
UponProposalInPropose(p) ==
    LET r == round[p] IN
    \E msg \in received_timely_proposal[p] :
        /\ msg.type       = "PROPOSAL"
        /\ msg.round      = r
        /\ msg.src        = Proposer[r]
        /\ msg.valid_round = NilRound
        /\ step[p]        = "PROPOSE"
        /\ evidence' = {msg} \union evidence
        /\ LET
               prop == msg.proposal
           IN
           IF IsCensoring(p, prop)
           THEN
               \* Censorship branch: reject and force round advance
               /\ BroadcastTimeout(p, r)
               /\ StartRound(p, r + 1)
               /\ UNCHANGED <<locked_value, locked_round, valid_value, valid_round, decision>>
               /\ UNCHANGED <<msgs_propose, msgs_prevote, msgs_precommit,
                              received_timely_proposal, inspected_proposal>>
               /\ UNCHANGED <<local_clock, real_time>>
               /\ UNCHANGED <<end_consensus, proposal_time, proposal_received_time>>
               /\ UNCHANGED <<forced_tx_queue>>
           ELSE
               \* Normal branch: vote for proposal or Nil
               /\ LET vote_target ==
                      IF IsValidProposal(prop)
                         /\ (locked_round[p] = NilRound \/ locked_value[p] = prop.value)
                      THEN prop
                      ELSE NilProposal
                  IN BroadcastPrevote(p, r, vote_target)
               /\ step' = [step EXCEPT ![p] = "PREVOTE"]
               /\ UNCHANGED <<round, decision, locked_value, locked_round,
                              valid_value, valid_round>>
               /\ UNCHANGED <<msgs_propose, msgs_precommit, msgs_timeout,
                              received_timely_proposal, inspected_proposal>>
               /\ UNCHANGED <<temporalVars, invariantVars, censorshipVars>>
        /\ action' = "UponProposalInPropose"


\* -- UponProposalInProposeAndPrevote: handles re-proposed locked values --
\* Triggered when proposal.validRound >= 0 (network locked in a prior round).
\* @type: (Str) => Bool;
UponProposalInProposeAndPrevote(p) ==
    LET r == round[p] IN
    \E msg \in msgs_propose[r] :
        /\ msg.type       = "PROPOSAL"
        /\ msg.src        = Proposer[r]
        /\ msg.valid_round >= 0 /\ msg.valid_round < r
        /\ step[p]        = "PROPOSE"
        /\ LET
               prop == msg.proposal
               vr   == msg.valid_round
               pv   == { m \in msgs_prevote[vr] : m.id = Id(prop) }
           IN
           /\ Cardinality(pv) >= THRESHOLD2
           /\ evidence' = pv \union {msg} \union evidence
           /\ LET mid ==
                  IF IsValidProposal(prop)
                     /\ (locked_round[p] <= vr \/ locked_value[p] = prop.value)
                  THEN Id(prop)
                  ELSE NilProposal
              IN BroadcastPrevote(p, r, mid)
        /\ step' = [step EXCEPT ![p] = "PREVOTE"]
        /\ UNCHANGED <<temporalVars, invariantVars, censorshipVars>>
        /\ UNCHANGED <<round, decision, locked_value, locked_round, valid_value, valid_round>>
        /\ UNCHANGED <<msgs_propose, msgs_precommit, msgs_timeout,
                       received_timely_proposal, inspected_proposal>>
        /\ action' = "UponProposalInProposeAndPrevote"


\* -- UponQuorumOfPrevotesAny: 2f+1 PREVOTEs for anything -> advance to PRECOMMIT --
\* @type: (Str) => Bool;
UponQuorumOfPrevotesAny(p) ==
    /\ step[p] = "PREVOTE"
    /\ LET all_prevotes == msgs_prevote[round[p]]
           voters == { m.src : m \in all_prevotes } 
       IN
       /\ Cardinality(voters) >= THRESHOLD2
       /\ evidence' = all_prevotes \union evidence
       /\ BroadcastPrecommit(p, round[p], NilProposal)
       /\ step' = [step EXCEPT ![p] = "PRECOMMIT"]
       /\ UNCHANGED <<temporalVars, invariantVars, censorshipVars>>
       /\ UNCHANGED <<round, decision, locked_value, locked_round, valid_value, valid_round>>
       /\ UNCHANGED <<msgs_propose, msgs_prevote, msgs_timeout, received_timely_proposal, inspected_proposal>>
       /\ action' = "UponQuorumOfPrevotesAny"


\* -- UponProposalInPrevoteOrCommitAndPrevote: 2f+1 PREVOTEs for a specific value -> LOCK --
\* @type: (Str) => Bool;
UponProposalInPrevoteOrCommitAndPrevote(p) ==
    LET r == round[p] IN
    \E msg \in msgs_propose[r] :
        /\ msg.type = "PROPOSAL"
        /\ msg.src  = Proposer[r]
        /\ step[p] \in {"PREVOTE", "PRECOMMIT"}
        /\ LET
               prop == msg.proposal
               pv   == { m \in msgs_prevote[r] : m.id = Id(prop) }
           IN
           /\ Cardinality(pv) >= THRESHOLD2
           /\ evidence' = pv \union {msg} \union evidence
           /\ IF step[p] = "PREVOTE"
              THEN
                  /\ locked_value'  = [locked_value  EXCEPT ![p] = prop.value]
                  /\ locked_round'  = [locked_round  EXCEPT ![p] = r]
                  /\ BroadcastPrecommit(p, r, Id(prop))
                  /\ step'         = [step EXCEPT ![p] = "PRECOMMIT"]
                  /\ UNCHANGED <<valid_value, valid_round>>
              ELSE
                  /\ valid_value'   = [valid_value  EXCEPT ![p] = prop]
                  /\ valid_round'   = [valid_round  EXCEPT ![p] = r]
                  /\ UNCHANGED <<locked_value, locked_round, msgs_precommit, step>>
        /\ UNCHANGED <<temporalVars, invariantVars, censorshipVars>>
        /\ UNCHANGED <<round, decision>>
        /\ UNCHANGED <<msgs_propose, msgs_prevote, msgs_timeout,
                       received_timely_proposal, inspected_proposal>>
        /\ action' = "UponProposalInPrevoteOrCommitAndPrevote"


\* -- UponQuorumOfPrecommitsAny: 2f+1 PRECOMMITs without decision -> next round --
\* @type: (Str) => Bool;
UponQuorumOfPrecommitsAny(p) ==
    /\ LET all_precommits == msgs_precommit[round[p]]
           unique_committers == { m.src : m \in all_precommits } 
       IN
       /\ Cardinality(unique_committers) >= THRESHOLD2
       /\ evidence' = all_precommits \union evidence
       /\ round[p] + 1 \in Rounds
       /\ StartRound(p, round[p] + 1)
       /\ UNCHANGED <<msgsBroadcastVars, propAuditVars>>
       /\ UNCHANGED <<local_clock, real_time>>
       /\ UNCHANGED <<end_consensus, proposal_time, proposal_received_time>>
       /\ UNCHANGED <<decision, locked_value, locked_round, valid_value, valid_round>>
       /\ UNCHANGED <<forced_tx_queue>>
       /\ action' = "UponQuorumOfPrecommitsAny"
                        


\* -- UponProposalInPrecommitNoDecision: commit function — 2f+1 PRECOMMITs for a value --
\* @type: (Str) => Bool;
UponProposalInPrecommitNoDecision(p) ==
    LET r == round[p] IN
    \E msg \in msgs_propose[r] :
        /\ msg.type  = "PROPOSAL"
        /\ msg.src   = Proposer[r]
        /\ decision[p] = NilDecision
        /\ inspected_proposal[r, p] /= NilTimestamp
        /\ LET
               prop == msg.proposal
               pv   == { m \in msgs_precommit[r] : m.id = Id(prop) }
           IN
           /\ Cardinality(pv) >= THRESHOLD2
           /\ evidence' = pv \union {msg} \union evidence
           /\ decision' = [decision EXCEPT ![p] = Decision(prop, r)]
        /\ end_consensus' = [end_consensus EXCEPT ![p] = local_clock[p]]
        /\ step'         = [step EXCEPT ![p] = "DECIDED"]
        /\ UNCHANGED <<temporalVars, msgsBroadcastVars, propAuditVars, censorshipVars>>
        /\ UNCHANGED <<round, locked_value, locked_round, valid_value, valid_round>>
        /\ UNCHANGED <<begin_round, last_begin_round, proposal_time, proposal_received_time>>
        /\ action' = "UponProposalInPrecommitNoDecision"


\* -- OnTimeoutPropose: node is not leader, proposal timed out -> PREVOTE Nil --
\* @type: (Str) => Bool;
OnTimeoutPropose(p) ==
    /\ step[p] = "PROPOSE"
    /\ p /= Proposer[round[p]]
    /\ local_rem_time[p] = 0
    /\ BroadcastPrevote(p, round[p], NilProposal)
    /\ step' = [step EXCEPT ![p] = "PREVOTE"]
    /\ UNCHANGED <<temporalVars, invariantVars, censorshipVars>>
    /\ UNCHANGED <<round, decision, locked_value, locked_round, valid_value, valid_round>>
    /\ UNCHANGED <<msgs_propose, msgs_precommit, msgs_timeout,
                   evidence, received_timely_proposal, inspected_proposal>>
    /\ action' = "OnTimeoutPropose"


\* -- OnQuorumOfNilPrevotes: 2f+1 nil PREVOTEs -> PRECOMMIT Nil --
\* @type: (Str) => Bool;
OnQuorumOfNilPrevotes(p) ==
    /\ step[p] = "PREVOTE"
    /\ LET pv == { m \in msgs_prevote[round[p]] : m.id = Id(NilProposal) } IN
           /\ Cardinality(pv) >= THRESHOLD2
           /\ evidence' = pv \union evidence
           /\ BroadcastPrecommit(p, round[p], Id(NilProposal))
           /\ step' = [step EXCEPT ![p] = "PRECOMMIT"]
           /\ UNCHANGED <<temporalVars, invariantVars, censorshipVars>>
           /\ UNCHANGED <<round, decision, locked_value, locked_round, valid_value, valid_round>>
           /\ UNCHANGED <<msgs_propose, msgs_prevote, msgs_timeout,
                          received_timely_proposal, inspected_proposal>>
           /\ action' = "OnQuorumOfNilPrevotes"


\* -- OnRoundCatchup: fast-forward — f+1 messages from higher round observed --
\* @type: (Str) => Bool;
OnRoundCatchup(p) ==
    \E r \in {rr \in Rounds : rr > round[p]} :
        LET 
            round_msgs == msgs_propose[r] \union msgs_prevote[r] \union msgs_precommit[r]
            faster    == { m.src : m \in round_msgs }
        IN  
            /\ Cardinality(faster) >= THRESHOLD1
            /\ evidence' = round_msgs \union evidence
            /\ StartRound(p, r)
            /\ UNCHANGED <<local_clock, real_time>>
            /\ UNCHANGED <<end_consensus, proposal_time, proposal_received_time>>
            /\ UNCHANGED <<decision, locked_value, locked_round, valid_value, valid_round>>
            /\ UNCHANGED <<msgsBroadcastVars, propAuditVars>>
            /\ UNCHANGED <<forced_tx_queue>>
            /\ action' = "OnRoundCatchup"


\* -- UponfPlusOneTimeoutsAny: f+1 timeout messages from higher round -> advance --
\* @type: (Str) => Bool;
UponfPlusOneTimeoutsAny(p) ==
    \E r \in {rr \in Rounds : rr > round[p]} :
        LET 
            timers == { m.src : m \in msgs_timeout[r] }
        IN 
            /\ Cardinality(timers) >= THRESHOLD1
            /\ evidence' = msgs_timeout[r] \union evidence
            /\ StartRound(p, r)
            /\ UNCHANGED <<msgsBroadcastVars, propAuditVars>>
            /\ UNCHANGED <<local_clock, real_time>>
            /\ UNCHANGED <<end_consensus, proposal_time, proposal_received_time>>
            /\ UNCHANGED <<decision, locked_value, locked_round, valid_value, valid_round>>
            /\ UNCHANGED <<forced_tx_queue>>
            /\ action' = "UponfPlusOneTimeoutsAny"

\* -- OnLocalTimerExpire: local countdown reached zero -> broadcast timeout --
\* @type: (Str) => Bool;
OnLocalTimerExpire(p) ==
    /\ local_rem_time[p] = 0
    /\ BroadcastTimeout(p, round[p])
    /\ UNCHANGED <<tendermintCoreVars, temporalVars, invariantVars, censorshipVars>>
    /\ UNCHANGED <<msgs_propose, msgs_prevote, msgs_precommit, evidence,
                   received_timely_proposal, inspected_proposal>>
    /\ action' = "OnLocalTimerExpire"


(* ======================== CLOCK ADVANCE ==================================== *)
\* Advance the global real_time and update all local clocks and timers accordingly
AdvanceRealTime ==
    /\ real_time < MAX_TIMESTAMP
    /\ real_time' = real_time + 1
    /\ local_clock' = [p \in HonestNodes |-> local_clock[p] + 1]
    /\ local_rem_time' = [p \in HonestNodes |->
            IF local_rem_time[p] > 0 
                /\ ~\E m \in received_timely_proposal[p] : m.round = round[p]
            THEN local_rem_time[p] - 1
            ELSE local_rem_time[p]]
    /\ UNCHANGED <<tendermintCoreVars, invariantVars>>
    /\ UNCHANGED <<msgsBroadcastVars, propAuditVars>>
    /\ UNCHANGED <<certificateVars, censorshipVars>>
    /\ UNCHANGED <<evidence>>
    /\ action' = "AdvanceRealTime"


(* ======================== MESSAGE DISPATCH ================================= *)
\* Aggregate all per-process message-processing actions
\* process timely messages
\* @type: (Str) => Bool;
MessageProcessing(p) ==
    \* start round
    \* \/ InsertProposal(p)
    \* reception step
    \/ ReceiveProposal(p)
    \* processing step
    \/ UponProposalInPropose(p)
    \/ UponProposalInProposeAndPrevote(p)
    \/ UponQuorumOfPrevotesAny(p)
    \/ UponProposalInPrevoteOrCommitAndPrevote(p)
    \/ UponQuorumOfPrecommitsAny(p)
    \/ UponProposalInPrecommitNoDecision(p)
    \* the actions below are not essential for safety, but added for completeness
    \/ OnTimeoutPropose(p)
    \/ OnQuorumOfNilPrevotes(p)
    \/ OnRoundCatchup(p)
    \/ UponfPlusOneTimeoutsAny(p)
    \/ OnLocalTimerExpire(p)


(* ======================== ADVERSARY ACTIONS ================================ *)
\* Byzantine Data-Withholding Attack: byzantine leader broadcasts a structurally
\* valid proposal but sets attestation = FALSE (data is not actually available).
ByzantineDataWithholding ==
    \E r \in Rounds :
        /\ Proposer[r] \in ByzantineNodes
        /\ msgs_propose[r] = {}
        /\ LET
            expected_fsm_state == CalculateNextFSMState

            \* The attacker is hiding DA data.
            bad_da == [
                published_block_height  |-> h_engram_verified, 
                attestation             |-> FALSE
            ]

            perfect_btc == [
                checkpoint_block_height |-> h_btc_current, 
                checkpoint_block_hash   |-> ExpectedBlockHash(h_btc_current) 
            ]

            forced_tx == CHOOSE tx \in forced_tx_queue : TRUE
            
            bad_prop == Proposal(forced_tx, real_time, r, expected_fsm_state,
                                bad_da, perfect_btc, FALSE)

           IN
            /\ BroadcastProposal(Proposer[r], r, bad_prop, NilRound)
            /\ UNCHANGED <<tendermintCoreVars, temporalVars, invariantVars, propAuditVars>>
            /\ UNCHANGED <<censorshipVars>>
            /\ UNCHANGED <<evidence, msgs_prevote, msgs_precommit, msgs_timeout>>
            /\ action' = "ByzantineDataWithholding"



\* Censorship Resistance: injects a new transaction into the forced inclusion queue
SubmitToCelestiaDA ==
    \E tx \in ValidValues \ forced_tx_queue :
        /\ forced_tx_queue' = forced_tx_queue \union {tx}
        /\ UNCHANGED <<tendermintCoreVars, temporalVars, invariantVars>> 
        /\ UNCHANGED <<msgsBroadcastVars, propAuditVars>>
        /\ UNCHANGED <<evidence, tx_ignored_rounds>>
        /\ action' = "SubmitToCelestiaDA"


(* ======================== NEXT-STATE RELATION ============================= *)
(*
 * Note: the system may eventually deadlock (e.g., all processes decide).
 * This is intentional — the spec focuses on safety, not liveness.
 *)
TendermintNext ==
    \/ AdvanceRealTime
    \/ /\ SynchronizedLocalClocks
       /\ \E p \in HonestNodes : MessageProcessing(p)
    \/ ByzantineDataWithholding
    \/ SubmitToCelestiaDA


(* ======================== SAFETY INVARIANTS =============================== *)
\* TendermintTypeOK: type domain for all Tendermint-owned variables
TendermintTypeOK ==
    /\ \A p \in HonestNodes :
           /\ round[p]       \in Rounds
           /\ step[p]        \in {"PROPOSE", "PREVOTE", "PRECOMMIT", "DECIDED"}
           /\ decision[p]    \in DecisionsOrNil
           /\ locked_value[p] \in ValuesOrNil
           /\ locked_round[p] \in RoundsOrNil
           /\ valid_value[p]  \in ProposalsOrNil
           /\ valid_round[p]  \in RoundsOrNil
    /\ \A r \in Rounds :
           /\ \A m \in msgs_propose[r]   : m.round = r
           /\ \A m \in msgs_prevote[r]   : m.round = r
           /\ \A m \in msgs_precommit[r] : m.round = r

\* I1: All decided honest node (processes) agree on the same value
AgreementOnValue ==
    \A p, q \in HonestNodes :
        /\ decision[p] /= NilDecision
        /\ decision[q] /= NilDecision
        => decision[p].prop.value = decision[q].prop.value

\* I2: Decided timestamp falls within the consensus round interval
ConsensusTimeValid ==
    \A p \in HonestNodes :
        decision[p] /= NilDecision =>
            LET
                r == decision[p].prop.round
                t == decision[p].prop.timestamp
            IN
            /\ begin_round[r] - PRECISION - DELAY <= t
            /\ t <= end_consensus[p] + PRECISION

\* I3: If the proposer is honest (correst), timestamp >= round begin time
ConsensusSafeValidHonestNode ==
    \A p \in HonestNodes :
        decision[p] /= NilDecision =>
            LET
                pr == decision[p].prop.round
                t  == decision[p].prop.timestamp
            IN
            (Proposer[pr] \in HonestNodes) => begin_round[pr] <= t

\* I4: Only valid domain values can be decided (no garbage)
ExternalValidity ==
    \A p \in HonestNodes :
        decision[p] /= NilDecision => decision[p].prop.value \in ValidValues


(* ============================ ACCOUNTABILITY =============================== *)
\* Double-signing evidence: two distinct messages from the same process in the
\* same round — concretely, CometBFT's own DuplicateVoteEvidence detection.
DoubleSigningEvidence ==
    \E r \in Rounds, p \in AllProcs :
        \/ \E m1, m2 \in msgs_prevote[r] :
               /\ m1.src = p /\ m2.src = p /\ m1.id /= m2.id
        \/ \E m1, m2 \in msgs_precommit[r] :
               /\ m1.src = p /\ m2.src = p /\ m1.id /= m2.id
        \/ \E m1, m2 \in msgs_propose[r] :
               /\ m1.src = p /\ m2.src = p /\ m1.proposal /= m2.proposal

\* I6: Fork accountability — if agreement breaks, a double-signer exists
Accountability ==
    (~AgreementOnValue) => DoubleSigningEvidence


(* ======================== NO CROSS-MODE DOUBLE SPENDING ================= *)
\* A Sovereign decision (Soft-finality) must never overwrite or conflict with 
\* a Proposal already anchored to Bitcoin (Hard-finality).
NoCrossModeDoubleSpending ==
    \A p \in HonestNodes, q \in HonestNodes :
        /\ decision[p] /= NilDecision
        /\ decision[q] /= NilDecision
        /\ decision[p].prop.btc_receipt.checkpoint_block_height <= h_btc_anchored
        /\ decision[q].prop.fsm_state = "SOVEREIGN"
        => 
        \* Sovereign decisions must strictly succeed the anchored history
        decision[q].prop.round > decision[p].prop.round


(* ======================== MASTER INVARIANT ================================ *)
\* Combine all core invariants for convenient TLC checking
CoreTendermintInvariant ==
    /\ TendermintTypeOK
    /\ AgreementOnValue
    /\ ConsensusTimeValid
    /\ ConsensusSafeValidHonestNode
    /\ ExternalValidity
    /\ Accountability
    /\ NoCrossModeDoubleSpending


(* ======================== SPECIFICATION ================================== *)
\* Pure safety spec — liveness fairness is handled in EngramServer
TendermintSpec == TendermintInit /\ [][TendermintNext]_tendermintVars

=============================================================================