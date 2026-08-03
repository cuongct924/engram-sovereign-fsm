---------------- MODULE MC_Ablation_NoHysteresisSafety_Apalache ----------------
(*
 * Apalache ablation driver: "Remove Hysteresis".
 *
 * EXTENDS EngramServer_Ablation_NoHysteresis, which sits on top of
 * EngramFSM_Ablation_NoHysteresis (dropped the `safe_blocks =
 * HYSTERESIS_WAIT` precondition from CalculateNextFSMState's RECOVERING ->
 * ANCHORED branch) and EngramTendermint_Ablation_NoHysteresis (a pass-
 * through copy, needed only because IsValidProposal cross-checks against
 * CalculateNextFSMState and TLA+ has no operator-override mechanism).
 *
 * Expected to find a violation of Sanity_NeverFlapInRecovering, proving the
 * hysteresis counter is load-bearing against state oscillation.
 *)
EXTENDS EngramServer_Ablation_NoHysteresis

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
MC_Proposer == [r \in 0..2 |-> IF r % 2 = 0 THEN n2 ELSE n1]

MC_ServerInit == ServerInit
MC_ServerNext == ServerNext

\* EXPECT VIOLATION: proves the system flaps in/out of RECOVERING without
\* stabilizing once the hysteresis wait is ablated.
Sanity_NeverFlapInRecovering ==
    ~(state = "RECOVERING" /\ safe_blocks > 0 /\ ~IsHealthyCondition)

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
