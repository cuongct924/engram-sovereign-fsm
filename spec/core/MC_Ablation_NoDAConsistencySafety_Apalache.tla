---------------- MODULE MC_Ablation_NoDAConsistencySafety_Apalache ----------------
(*
 * Apalache ablation driver: "Remove DA receipt consistency".
 *
 * Replaces the README's previous "813 states via random simulation" claim
 * for this ablation -- that number was never produced by an exhaustive or
 * reproducible run and is being redone here the same way as the other 4
 * ablations (checked-in modified spec copy + driver, not an ad-hoc edit).
 *
 * EXTENDS EngramServer_Ablation_NoDAConsistency, built on
 * EngramTendermint_Ablation_NoDAConsistency (dropped the
 * `prop.da_receipt.attestation = TRUE` conjunct from IsValidProposal's DA
 * Pipeline Check).
 *
 * Expected to find a violation of Sanity_NeverProposeWithheldData, proving
 * a Byzantine leader can withhold DA data and still have its proposal
 * accepted once the attestation check is ablated.
 *)
EXTENDS EngramServer_Ablation_NoDAConsistency

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

\* EXPECT VIOLATION: proves a Byzantine leader can withhold DA data
\* (attestation = FALSE) and still have the proposal broadcast/accepted
\* once the DA consistency check is ablated.
Sanity_NeverProposeWithheldData ==
    \A r \in Rounds : \A m \in msgs_propose[r] :
        m.proposal.da_receipt.attestation = TRUE

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
