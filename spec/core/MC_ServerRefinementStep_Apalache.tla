---------------- MODULE MC_ServerRefinementStep_Apalache ----------------
(*
 * Apalache driver for the INDUCTIVE refinement step of the server layer (C1, teacher feedback item 7).
 *
 * Unlike TLC's RefinementSafety, which explores reachable behaviours, this asks about ONE step from ANY state that
 * satisfies a candidate inductive invariant Inv:
 *
 *   (B1)  every initial state satisfies Inv                                   -- --init=MC_Init   --inv=Inv        --length=0
 *   (B2)  every initial state maps to an initial state of the abstract object  -- --init=MC_Init   --inv=AbsInitOK  --length=0
 *   (S1)  Inv is preserved by every concrete step                              -- --init=IndInit   --next=MC_Next --inv=Inv      --length=1
 *   (S2)  every concrete step from an Inv state is an abstract step or maps to a stuttering step
 *                                                                              -- --init=IndInit   --next=BadStep  --inv=StepOK   --length=1
 *
 * (S2) is stated as a search for a violation: BadStep is a concrete step that is neither an abstract Next step nor
 * leaves every abstract variable unchanged, and it sets the ghost flag step_ok to FALSE. StepOK is violated after one
 * BadStep, so a counterexample to (S2) is exactly a step that does not refine; NoError means none exists from any Inv state.
 *
 * The abstract variables are the refinement mapping of EngramServerRefinement.tla (tree, fsm_state, round, local_times,
 * rem_time, h_btc_current, h_btc_anchored), taken through INSTANCE ... WITH there. The module EXTENDS the generated
 * EngramServerRefinement_Apalache (make_apalache_refinement.py), where the recursive SumStakeOp is replaced.
 *
 * Inv starts as the invariants TLC already checks (reachable-state safety). It is NOT expected to be inductive: the
 * counterexamples to (S1) and (S2) are what strengthen it (docs/FORMAL_PLAN_AIR.md section 3C). A claim is justified only
 * when B1, B2, S1 and S2 all return NoError for one Inv, for the constants of ApalacheCInit below.
 *)
EXTENDS EngramServerRefinement_Apalache

CONSTANTS
    \* @type: Str;
    n1,
    \* @type: Str;
    n2,
    \* @type: Str;
    n3,
    \* @type: Str;
    n4

VARIABLE
    \* @type: Bool;
    step_ok

\* @type: Int -> Str;
MC_Proposer == [r \in 0..2 |-> IF r % 2 = 0 THEN n1 ELSE n2]

(* ======================== INIT / NEXT ==================================== *)
MC_Init == ServerInit /\ step_ok = TRUE
MC_Next == ServerNext /\ UNCHANGED step_ok

(* ======================== CANDIDATE INDUCTIVE INVARIANT ================== *)
\* @type: Bool;
Inv ==
    /\ FSMTypeOK
    /\ CoreTendermintInvariant
    /\ HybridTendermintInvariant
    /\ CircuitBreakerSafety

\* @type: Bool;
IndInit == Inv /\ step_ok = TRUE

(* ======================== ABSTRACT SIDE ================================== *)
\* the initial state of the abstract object, read through the mapping
\* @type: Bool;
AbsInitOK == AbstractConsensus!Init

\* the abstract step the concrete step must correspond to, or stuttering of every abstract variable
\* @type: Bool;
AbsStepOrStutter ==
    \/ AbstractConsensus!Next
    \/ AbstractConsensus!vars' = AbstractConsensus!vars

\* a concrete step that does not refine, recorded in the ghost flag
\* @type: Bool;
BadStep ==
    /\ ServerNext
    /\ ~AbsStepOrStutter
    /\ step_ok' = FALSE

\* @type: Bool;
StepOK == step_ok

(* ======================== CONSTANTS FOR APALACHE ========================= *)
\* Same bounds as MC_ServerSafety_Apalache.tla (kept small: TendermintTypeOK inside CoreTendermintInvariant makes Apalache
\* enumerate the proposal space), plus K_DEEP_FINALITY of the refinement mapping.
ApalacheCInit ==
    /\ n1 = "n1" /\ n2 = "n2" /\ n3 = "n3" /\ n4 = "n4"
    /\ Nodes = {"n1", "n2", "n3", "n4"}
    /\ Method = {"TX_NORMAL", "TX_WITHDRAWAL"}
    /\ ByzantineNodes = {"n1"}
    /\ HonestNodes = {"n2", "n3", "n4"}
    /\ Proposer = MC_Proposer
    /\ MAX_ROUND = 2
    /\ MAX_BTC_HEIGHT = 1
    /\ MAX_ENGRAM_HEIGHT = 1
    /\ MAX_TIMESTAMP = 2
    /\ MAX_IGNORE_ROUNDS = 1
    /\ MAX_CENSORSHIP_ROUNDS = 3
    /\ RESET_TIME = 2
    /\ SUSPICIOUS_THRESHOLD = 1
    /\ SOVEREIGN_THRESHOLD = 2
    /\ DA_THRESHOLD = 1
    /\ BTC_THRESHOLD = 0
    /\ RECOVERY_HYSTERESIS_WAIT = 1
    /\ SUSPICIOUS_HYSTERESIS_WAIT = 1
    /\ MAX_SUSPICIOUS_TIME = 1
    /\ DOWN_HYSTERESIS_THRESHOLD = 1
    /\ MAX_DOWN_HYSTERESIS_THRESHOLD = 8
    /\ STABLE_WINDOW = 2
    /\ MIN_PEERS = 2
    /\ MIN_SUBNET_DIVERSITY = 2
    /\ MIN_ANCHOR_PEERS = 1
    /\ MAX_CHURN_RATE = 1
    /\ MIN_AVG_TENURE = 1
    /\ MAX_PEER_LATENCY = 1
    /\ N = 4
    /\ T = 1
    /\ MIN_TIMESTAMP = 0
    /\ PRECISION = 1
    /\ DELAY = 1
    /\ TIMEOUT_DURATION = 2
    /\ K_DEEP_FINALITY = 2

=============================================================================
