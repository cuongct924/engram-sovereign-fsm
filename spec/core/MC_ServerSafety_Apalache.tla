---------------- MODULE MC_ServerSafety_Apalache ----------------
(*
 * MC_ServerSafety_Apalache — Apalache bounded-safety driver for the
 * Server layer, EXTENDS EngramServer directly (NOT EngramServerRefinement /
 * MC_ServerRefinementSafety.tla).
 *
 * This is deliberate, not an oversight: MC_ServerRefinementSafety.tla
 * carries `ASSUME QuorumOverlap` unconditionally at module scope (needed
 * for TLC's refinement check), and QuorumOverlap depends on
 * AbstractConsensus!ValidQuorums -> SumStake -> the RECURSIVE SumStakeOp
 * operator in EngramConsensus.tla. Apalache's `check` pipeline processes
 * every top-level ASSUME regardless of which --init/--next/--inv are
 * requested (confirmed empirically: `apalache-mc typecheck` on
 * EngramServerRefinement.tla only warns about SumStakeOp via its
 * "labelling operator as recursive" workaround, but `apalache-mc check`
 * hits a hard "Apalache does not support recursive operators" error at
 * ConfigurationPass the moment QuorumOverlap is anywhere in the module --
 * even when checking only CoreTendermintInvariant/HybridTendermintInvariant,
 * which never reference it). EngramServer.tla itself never INSTANCEs
 * EngramConsensus (only EngramServerRefinement.tla does), so extending it
 * directly sidesteps the RECURSIVE operator entirely -- this also matches
 * the intended scope: this driver checks Server-layer safety only, not the
 * refinement obligation (RefinementSafety stays TLC-only regardless).
 *
 * Small network/schedule config below is a minimal reimplementation of
 * MC_ServerRefinementSafety.tla's MC_Nodes/MC_Method/MC_Byzantine/MC_Honest/
 * MC_Proposer (duplicated rather than shared, specifically to avoid pulling
 * in EngramServerRefinement.tla's ASSUME QuorumOverlap transitively).
 *)
EXTENDS EngramServer

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

ApalacheCInit ==
    /\ n1 = "n1" /\ n2 = "n2" /\ n3 = "n3" /\ n4 = "n4"
    /\ Nodes = {"n1", "n2", "n3", "n4"}
    /\ Method = {"TX_NORMAL", "TX_WITHDRAWAL"}
    /\ ByzantineNodes = {"n1"}
    /\ HonestNodes = {"n2", "n3", "n4"}
    /\ Proposer = MC_Proposer
    \* Bounds shrunk from MC_ServerRefinementSafety.cfg's TLC values (3/3/20)
    \* -- `valid_value[p] \in ProposalsOrNil` (TendermintTypeOK, inside
    \* CoreTendermintInvariant) forces Apalache to enumerate the full
    \* Proposals Cartesian product; TLC's own values would be ~880K
    \* elements, past what's practical for a first pilot pass.
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
