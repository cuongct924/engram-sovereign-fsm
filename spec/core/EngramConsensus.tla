--------------------------- MODULE EngramConsensus ---------------------------
EXTENDS Naturals, FiniteSets

(*
 * Apalache note: cache-record types below are written as fully expanded
 * structural types ({ type: Str, cert_round: Int, ... }) rather than a named
 * alias, even though every use is within this one file. A named alias here
 * ("CACHE") still tripped Snowcat ("Cannot apply c to the argument..."),
 * so this file follows the same inline convention as the cross-module
 * driver files instead of debugging that further.
 *)

(********************* INTERFACE & CONSTANTS ************************)
\* Apalache note: record shapes are inlined (no @typeAlias) rather than
\* named -- this Apalache release does not unify a named alias against a
\* freshly-built literal of the same shape once the literal is constructed
\* in a different module than the one declaring the alias, and every driver
\* lives outside this file (see core/MC_FSMSafety_Apalache.tla for the
\* original repro of this).
CONSTANTS
    \* @type: Set(Str);
    Nodes,
    \* @type: Set(Str);
    Method,
    \* @type: Str -> Int;
    Stake,
    \* @type: Int;
    RESET_TIME,
    \* @type: Int;
    TOTAL_STAKE,
    \* @type: Int;
    MAX_BTC_HEIGHT,
    \* @type: Int;
    K_DEEP_FINALITY

(********************* CONSENSUS LAYER VARIABLES ***********)
VARIABLES
    \* @type: Set({ type: Str, cert_round: Int, caller: Str, method: Str, voters: Set(Str), btc_anchored: Int });
    tree,                 \* Buffer tree (AdoB)
    \* @type: Str -> Int;
    local_times,          \* Logical time of each node
    \* @type: Int;
    round,                \* Current consensus round
    \* @type: Int;
    rem_time,             \* Countdown timer
    \* @type: Str;
    fsm_state,            \* FSM state
    \* @type: Int;
    h_btc_current,        \* Current Bitcoin height
    \* @type: Int;
    h_btc_anchored        \* Engram checkpoint height


vars == <<tree, local_times, round, rem_time, 
            fsm_state, h_btc_current, h_btc_anchored>>

(********************* QUORUM OPTIMIZATION (MEMOIZATION) ****************)
RECURSIVE SumStakeOp(_)
\* @type: (Set(Str)) => Int;
SumStakeOp(Q) == IF Q = {} THEN 0 ELSE LET n == CHOOSE x \in Q : TRUE IN Stake[n] + SumStakeOp(Q \ {n})

SumStake[Q \in SUBSET Nodes] == SumStakeOp(Q)

\* Precompute the set of valid Quorums once
ValidQuorums == {q \in SUBSET Nodes : SumStake[q] * 3 > TOTAL_STAKE * 2}

IsSQuorum(Q) == Q \in ValidQuorums

(********************* INITIALIZATION ***********************************) 
Init == 
    /\ tree = {} 
    /\ local_times = [n \in Nodes |-> 0] 
    /\ round = 1        
    /\ rem_time = RESET_TIME
    /\ fsm_state = "ANCHORED"
    /\ h_btc_current = K_DEEP_FINALITY
    /\ h_btc_anchored = K_DEEP_FINALITY


(********************* ABSTRACT PACEMAKER (LiDO) ******************) 
Elapse == 
    \* Block the countdown timer if the Leader has started Pull (E) or Invoke (M)
    /\ ~\E c \in tree : c.cert_round = round /\ c.type \in {"E", "M"} 
    /\ rem_time > 0 
    /\ rem_time' = rem_time - 1 
    /\ UNCHANGED <<tree, round, local_times>>
    /\ UNCHANGED <<fsm_state, h_btc_current, h_btc_anchored>>

TimeoutStartNext == 
    /\ rem_time = 0 
    /\ round' = round + 1 
    /\ rem_time' = RESET_TIME 
    /\ UNCHANGED <<tree, local_times>>
    /\ UNCHANGED <<fsm_state, h_btc_current, h_btc_anchored>>

EarlyStartNext ==
    /\ \E c \in tree : c.type = "C" /\ c.cert_round = round
    /\ round' = round + 1
    /\ rem_time' = RESET_TIME
    /\ UNCHANGED <<tree, local_times>>
    /\ UNCHANGED <<fsm_state, h_btc_current, h_btc_anchored>>

\* Round advances once f+1 honest nodes give up on it (a T-cache exists),
\* independent of whether the leader (honest or Byzantine) ever completes
\* Push. Without this, Elapse's block on a pending Pull/Invoke has no
\* escape valve other than Push -- meaning WF_vars(Push(n)) for EVERY node
\* (Byzantine included) becomes a load-bearing liveness assumption, which
\* is incompatible with a leader that Pulls and then stays silent forever
\* (see LIVENESS_DEADLOCK_FINDING.md). This operator gives the abstract
\* model a formal counterpart to the concrete f+1 fast-forward mechanism
\* (UponfPlusOneTimeoutsAny / ServerUponTimeoutCert -> T_QC).
TimeoutSkipNext ==
    /\ \E c \in tree : c.type = "T" /\ c.cert_round = round
    /\ round' = round + 1
    /\ rem_time' = RESET_TIME
    /\ UNCHANGED <<tree, local_times>>
    /\ UNCHANGED <<fsm_state, h_btc_current, h_btc_anchored>>


(********************* FORK-CHOICE & CAN_ELECT ********************)
\* Find the latest CCache block that node s supports.
\* @type: (Set({ type: Str, cert_round: Int, caller: Str, method: Str, voters: Set(Str), btc_anchored: Int }), Str) => { type: Str, cert_round: Int, caller: Str, method: Str, voters: Set(Str), btc_anchored: Int };
Active(tr, s) ==
    LET s_votes == {c \in tr : c.type = "C" /\ s \in c.voters}
    IN IF s_votes = {} THEN [type |-> "C", cert_round |-> 0, caller |-> s, method |-> "None", voters |-> {}, btc_anchored |-> 0]
       ELSE CHOOSE c \in s_votes : \A c2 \in s_votes : c.cert_round >= c2.cert_round

\* K-Deep Finality Rule
\* @type: ({ type: Str, cert_round: Int, caller: Str, method: Str, voters: Set(Str), btc_anchored: Int }, Int) => Bool;
IsKDeep(c, k) ==
    /\ c.btc_anchored <= h_btc_anchored     \* The anchor point of this block has not been lost due to Reorg.
    /\ h_btc_current - c.btc_anchored >= k  \* Bitcoin's on-chain depth has reached a safe level.


\* Simplify: The branch with the largest total stake is based on the number of voters.
\* @type: ({ type: Str, cert_round: Int, caller: Str, method: Str, voters: Set(Str), btc_anchored: Int }) => Bool;
IsMaxStakeBranch(c) ==
    \/ c.cert_round = 0    \* Exceptions are always valid for Genesis blocks
    \/ SumStake[c.voters] >= TOTAL_STAKE \div 2


\* @type: (Set({ type: Str, cert_round: Int, caller: Str, method: Str, voters: Set(Str), btc_anchored: Int }), { type: Str, cert_round: Int, caller: Str, method: Str, voters: Set(Str), btc_anchored: Int }, Set(Str), Str) => Bool;
CanElect(tr, c, Q, state_fsm) ==
    /\ c.type = "C" 
    /\ \A s \in Q : c.cert_round >= Active(tr, s).cert_round
    /\ CASE state_fsm = "ANCHORED"   -> IsKDeep(c, K_DEEP_FINALITY)
         [] state_fsm = "SUSPICIOUS" -> IsKDeep(c, K_DEEP_FINALITY)
         [] state_fsm = "SOVEREIGN"  -> IsMaxStakeBranch(c)
         [] OTHER                    -> TRUE   


(********************* ADOB CORE OPERATIONS ***********************)
Pull(n) == 
    LET Q == CHOOSE q \in SUBSET Nodes : IsSQuorum(q)
        \* method |-> "None" here purely for record-shape uniformity with
        \* E/M/T caches (nothing reads .method off a "C" cache) -- Apalache's
        \* type checker needs every element of the `tree` set to share one
        \* record shape, unlike TLC which tolerates heterogeneous records.
        VirtualRoot == [type |-> "C", cert_round |-> 0, caller |-> CHOOSE nn \in Nodes : TRUE, method |-> "None", voters |-> {}, btc_anchored |-> 0]
        ValidCaches == {c \in tree : c.type = "C"} \cup {VirtualRoot}
    IN \E Cmax \in ValidCaches:
       /\ CanElect(tree, Cmax, Q, fsm_state)
       /\ round > local_times[n]
       /\ local_times' = [s \in Nodes |-> IF s \in Q THEN round ELSE local_times[s]]
       \* Boundary-value reduction: chosen_anchor only feeds IsKDeep, which is
       \* monotonic in c.btc_anchored (smaller = safer, both for the
       \* not-lost-to-reorg check and the depth check) -- so the two extremes
       \* {h_btc_anchored, h_btc_current} dominate every value in between for
       \* safety-checking purposes. Was the full inclusive range
       \* h_btc_anchored..h_btc_current, a major state-space driver repeated
       \* across Pull/Invoke/Push/Timeout; narrowed after L1's standalone
       \* Safety check exhausted disk at 115M+ distinct states without this.
       /\ \E chosen_anchor \in {h_btc_anchored, h_btc_current} :
           LET new_E_cache == [ 
               type             |-> "E",
               cert_round       |-> round,
               caller           |-> n,
               method           |-> "None",
               voters           |-> Q,
               btc_anchored     |-> chosen_anchor 
           ]
           IN tree' = tree \cup {new_E_cache}
       /\ UNCHANGED <<round, rem_time>>
       /\ UNCHANGED <<fsm_state, h_btc_current, h_btc_anchored>>


Invoke(n, m) == 
    /\ m \in Method
    /\ \E c \in tree : 
       /\ c.type = "E"
       /\ c.caller = n
       /\ c.cert_round = round
       /\ \E chosen_anchor \in {h_btc_anchored, h_btc_current} :
           LET new_M_cache == [ 
               type             |-> "M",
               cert_round       |-> round,
               caller           |-> n,
               method           |-> m,
               voters           |-> {n},
               btc_anchored     |-> chosen_anchor 
           ]
           IN tree' = tree \cup {new_M_cache}
    /\ UNCHANGED <<local_times, round, rem_time>>
    /\ UNCHANGED <<fsm_state, h_btc_current, h_btc_anchored>>


Push(n) ==
    /\ \E c \in tree :
       /\ c.type = "M"
       /\ c.caller = n
       /\ c.cert_round = round
       /\ \E chosen_anchor \in {h_btc_anchored, h_btc_current} :
           LET new_C_cache == [
               type             |-> "C",
               cert_round       |-> round,
               caller           |-> n,
               method           |-> "None",
               voters           |-> {n},
               btc_anchored     |-> chosen_anchor
           ]
           IN tree' = tree \cup {new_C_cache}
    /\ UNCHANGED <<local_times, round, rem_time>>
    /\ UNCHANGED <<fsm_state, h_btc_current, h_btc_anchored>>


\* Timeout(n): a stake-quorum of processes gives up on the current round
\* (mirrors Server_UponTimeoutCert -> T_QC, per the four server hooks in
\* EngramServer.tla / EngramServerRefinement.tla's MappedTCaches). Structured
\* like Push: adds exactly one new cache to tree, round/rem_time untouched —
\* a T_QC does not itself force the concrete round to advance (that still
\* happens via a precommit quorum), so neither does its abstract counterpart.
\* Added after RefinementSafety caught a real gap: MappedTCaches already
\* mapped T_QCs into tree as "T"-typed caches, but no Next action produced
\* one, so any concrete Server_UponTimeoutCert step had no matching abstract
\* action and vacuously violated [][Next]_vars.
Timeout(n) ==
    LET Q == CHOOSE q \in SUBSET Nodes : IsSQuorum(q) IN
    /\ ~\E c \in tree : c.type = "T" /\ c.cert_round = round
    /\ \E chosen_anchor \in {h_btc_anchored, h_btc_current} :
        LET new_T_cache == [
            type             |-> "T",
            cert_round       |-> round,
            caller           |-> n,
            method           |-> "None",
            voters           |-> Q,
            btc_anchored     |-> chosen_anchor
        ]
        IN tree' = tree \cup {new_T_cache}
    /\ UNCHANGED <<local_times, round, rem_time>>
    /\ UNCHANGED <<fsm_state, h_btc_current, h_btc_anchored>>

(********************* OPTIMIZED ENVIRONMENT ******************)
UpdateEnv == 
    /\ h_btc_current' \in {h_btc_current, h_btc_current + 1}
    /\ h_btc_anchored' \in h_btc_anchored..h_btc_current'   \* Allow h_btc_anchored to catch up comfortably
    /\ fsm_state' = fsm_state  
    /\ UNCHANGED <<tree, local_times, round, rem_time>>


FSMStateChange ==
    /\ fsm_state' \in {"ANCHORED", "SOVEREIGN"}
    /\ fsm_state' /= fsm_state
    /\ UNCHANGED <<tree, local_times, round, rem_time>>
    /\ UNCHANGED <<h_btc_current, h_btc_anchored>>


\* Bitcoin Reorg simulation loses OP_RETURN transaction
BitcoinReorg == 
    /\ h_btc_anchored > 0  
    /\ h_btc_current' \in {h_btc_current, h_btc_current + 1} 
    /\ \E lost_anchor \in 0..(h_btc_anchored - 1): h_btc_anchored' = lost_anchor 
    /\ fsm_state' = "SOVEREIGN"
    /\ UNCHANGED <<tree, local_times, round, rem_time>>

(********************* NEXT STATE  ****************)
Next ==
    \/ Elapse
    \/ TimeoutStartNext
    \/ EarlyStartNext
    \/ TimeoutSkipNext
    \/ \E n \in Nodes : Pull(n)
    \/ \E n \in Nodes, m \in Method : Invoke(n, m)
    \/ \E n \in Nodes : Push(n)
    \/ \E n \in Nodes : Timeout(n)
    \/ UpdateEnv
    \/ FSMStateChange
    \/ BitcoinReorg

(********************* SAFETY & FAIRNESS (LIVENESS) ************************)
Safety == Init /\ [][Next]_vars


Liveness ==
    /\ WF_vars(TimeoutStartNext)
    /\ WF_vars(EarlyStartNext)
    /\ WF_vars(TimeoutSkipNext)
    /\ WF_vars(UpdateEnv)
    /\ \A n \in Nodes :
        /\ WF_vars(Pull(n))
        /\ WF_vars(Push(n))
    /\ \A n \in Nodes, m \in Method : WF_vars(Invoke(n, m))


Spec == Safety /\ Liveness
=====================================================================