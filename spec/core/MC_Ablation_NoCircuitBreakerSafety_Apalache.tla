---------------- MODULE MC_Ablation_NoCircuitBreakerSafety_Apalache ----------------
(*
 * Apalache ablation driver: "Remove Circuit Breaker".
 *
 * EXTENDS EngramServer_Ablation_NoCircuitBreaker directly (not through
 * EngramServerRefinement) -- same reasoning as MC_ServerSafety_Apalache.tla:
 * avoids the RECURSIVE SumStakeOp trap, and the ablation only needs to
 * demonstrate the withdrawal-leakage state, not the refinement obligation.
 *
 * Unlike the normal Server-layer pilot (which expects NoError), this driver
 * is EXPECTED to find a violation of Sanity_NeverAttemptWithdrawalLeakage --
 * that is the entire point of the ablation. Apalache's bounded SMT search
 * does not need to exhaustively widen at every depth the way TLC's BFS
 * does, so it can find a specific short counterexample much faster when
 * one exists at a shallow-to-moderate depth.
 *)
EXTENDS EngramServer_Ablation_NoCircuitBreaker

CONSTANTS
    \* @type: Str;
    n1,
    \* @type: Str;
    n2,
    \* @type: Str;
    n3,
    \* @type: Str;
    n4

\* @type: Int -> Str;
MC_Proposer == [r \in 0..2 |-> IF r % 2 = 0 THEN n1 ELSE n2]

MC_ServerInit == ServerInit
MC_ServerNext == ServerNext

\* EXPECT VIOLATION: proves the attacker can attempt a withdrawal while in
\* SOVEREIGN or RECOVERING state once the circuit breaker is ablated.
Sanity_NeverAttemptWithdrawalLeakage ==
    \A r \in Rounds : \A m \in msgs_propose[r] :
        (m.proposal.fsm_state \in {"SOVEREIGN", "RECOVERING"}) => (m.proposal.value /= "TX_WITHDRAWAL")

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
    /\ RESET_TIME = 2
    /\ SUSPICIOUS_THRESHOLD = 1
    /\ SOVEREIGN_THRESHOLD = 2
    /\ DA_THRESHOLD = 1
    /\ HYSTERESIS_WAIT = 1
    /\ MAX_SUSPICIOUS_TIME = 1
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

=============================================================================
