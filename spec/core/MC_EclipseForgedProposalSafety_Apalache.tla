------------- MODULE MC_EclipseForgedProposalSafety_Apalache -------------
(*
 * Apalache driver: "Eclipse Attack -- forged fsm_state rejection", closing
 * README.md's Lemma 7.5 open item (core/EngramServer_EclipseForgedProposal.tla
 * has the full design rationale for the new ByzantineForgedFSMState action).
 *
 * Unlike the 5 ablation drivers, this is a POSITIVE confirmation, not an
 * ablation -- EngramServer/EngramTendermint/EngramFSM are all unmodified.
 * Two separate checks against the SAME driver, same bound:
 *
 * 1. Sanity_ForgedProposalReachable (EXPECT VIOLATION): proves the attack
 *    actually gets attempted within this bound -- checks the `action`
 *    bookkeeping variable directly (already real, already used for exactly
 *    this purpose elsewhere in this spec) rather than re-deriving
 *    CalculateNextFSMState at check-time, which would have the same
 *    step-lag problem the Hysteresis/P2PGate ablation drivers found and
 *    worked around with a ghost variable -- action' is set the instant
 *    ByzantineForgedFSMState fires, no lag. The violation IS the trace:
 *    apalache-mc stops at the first state where action = "ByzantineForgedFSMState".
 * 2. Sanity_ForgedFSMStateRejectedUnderEclipse (EXPECT NO VIOLATION): a
 *    thin, deliberately-non-reinvented alias for FSMStateConsistency
 *    (already defined in EngramServer.tla) -- confirms that even though
 *    the attack IS attempted (per check 1, same bound), no honest node
 *    ever decides a proposal whose fsm_state disagrees with the real FSM
 *    state, i.e. the forged proposal is never actually accepted.
 *
 * Full-stack driver (not FSM-only, unlike the Hysteresis/P2PGate ablation
 * drivers) -- this scenario's property genuinely depends on
 * msgs_propose/decision/the real per-node prevote logic, which those two
 * narrower drivers deliberately skip. Mirrors
 * MC_Ablation_NoCircuitBreakerSafety_Apalache.tla's shape and constants
 * (same proven-fast full-stack configuration) since that's the other
 * full-stack Apalache driver in this suite.
 *)
EXTENDS EngramServer_EclipseForgedProposal

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
MC_ServerNext == ServerNextWithEclipseAttack

\* EXPECT VIOLATION: proves ByzantineForgedFSMState actually fires within
\* this bound (an eclipsed Byzantine proposer's forged-fsm_state proposal
\* really gets broadcast) -- the counterexample trace is the "attack
\* attempted" evidence.
Sanity_ForgedProposalReachable ==
    action /= "ByzantineForgedFSMState"

\* EXPECT NO VIOLATION: reuses FSMStateConsistency (EngramServer.tla)
\* verbatim -- confirms no honest node ever decides the forged proposal,
\* over the SAME bound where check 1 confirms the attack is attempted.
Sanity_ForgedFSMStateRejectedUnderEclipse ==
    FSMStateConsistency

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
    /\ HYSTERESIS_WAIT = 1
    /\ SUSPICIOUS_HYSTERESIS_WAIT = 1
    /\ MAX_SUSPICIOUS_TIME = 1
    /\ DOWN_HYSTERESIS_THRESHOLD = 1
    /\ MAX_DOWN_HYSTERESIS_THRESHOLD = 8
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
