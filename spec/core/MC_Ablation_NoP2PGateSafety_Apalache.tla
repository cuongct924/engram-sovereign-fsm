---------------- MODULE MC_Ablation_NoP2PGateSafety_Apalache ----------------
(*
 * Apalache ablation driver: "Remove P2P Health Gate".
 *
 * EXTENDS EngramServer_Ablation_NoP2PGate, built on EngramFSM_Ablation_
 * NoP2PGate (dropped the IsP2PQualityHealthy conjunct from IsHealthyCondition)
 * and a pass-through EngramTendermint_Ablation_NoP2PGate (needed only
 * because IsValidProposal cross-checks against CalculateNextFSMState).
 *
 * Sanity_NeverRecoverWithBadP2P is a NEW check defined specifically for
 * this ablation (no equivalent exists in the real spec's checked-in
 * Sanity_* list) -- it is expected to FAIL, proving an eclipsed node (P2P
 * unhealthy) can still reach RECOVERING once P2P is no longer part of
 * IsHealthyCondition, as long as BTC/DA still look fine.
 *)
EXTENDS EngramServer_Ablation_NoP2PGate

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

\* EXPECT VIOLATION: proves an eclipsed node (bad P2P) can still enter
\* RECOVERING once the P2P health gate is ablated from IsHealthyCondition.
Sanity_NeverRecoverWithBadP2P ==
    ~(state = "RECOVERING" /\ ~IsP2PQualityHealthy)

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
