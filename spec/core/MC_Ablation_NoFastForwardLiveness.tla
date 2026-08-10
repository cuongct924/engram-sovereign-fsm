---------------- MODULE MC_Ablation_NoFastForwardLiveness ----------------
(*
 * TLC ablation driver: "Remove f+1 timeout fast-forward".
 *
 * EXTENDS EngramServer_Ablation_NoFastForward, a copy of EngramServer.tla
 * sitting on top of EngramTendermint_Ablation_NoFastForward (neutered
 * UponfPlusOneTimeoutsAny(p) == FALSE -- lagging honest nodes can no longer
 * fast-forward on f+1 observed timeouts from a higher round, only on their
 * own full local timeout).
 *
 * This is a liveness-only ablation (Apalache has no temporal+fairness
 * support, so this one is TLC-only, unlike the other ablations).
 *
 * MC_ServerFairness and EngramServer_Ablation_NoFastForward.tla itself
 * (2026-08-09): both re-synced against EngramServer.tla's Hướng A+B
 * bootstrap-deadlock fix (see LIVENESS_DEADLOCK_FINDING.md) -- the ablated
 * copy had silently drifted stale (missing ServerHonestTimeout/
 * ServerHonestRoundSkip entirely, along with the ServerByzantinePull/
 * ServerByzantineDataWithholding already-closed-round guards), and this
 * driver's own fairness formula never required the two new actions to
 * fire even once the base module gained them. Without this sync, a run
 * against this driver cannot distinguish "liveness fails because f+1 was
 * removed" from "liveness fails because of the unrelated, already-fixed
 * Byzantine-silent-leader bootstrap deadlock" -- both would stutter at the
 * same state (ServerByzantinePull, then nothing else ever enabled).
 *)
EXTENDS EngramServer_Ablation_NoFastForward, TLC, Sequences

CONSTANTS n1, n2, n3, n4

MC_Nodes  == {n1, n2, n3, n4}
MC_Method == {"TX_NORMAL", "TX_WITHDRAWAL"}
MC_Byzantine == {n1}
MC_Honest   == MC_Nodes \ MC_Byzantine

SymmetryPerms == Permutations(MC_Honest)

MC_NodeSeq  == <<n1, n2, n3, n4>>
MC_Proposer == [r \in 0..5 |-> MC_NodeSeq[(r % 4) + 1]]

MC_ServerInit == ServerInit
MC_ServerNext == ServerNext

MC_ServerFairness ==
    /\ WF_serverVars(ServerAdvanceRealTime)
    /\ \A p \in MC_Honest : WF_serverVars(ServerMessageProcessing(p))
    /\ WF_serverVars(ServerHonestTimeout)
    /\ \A p \in MC_Honest : WF_serverVars(ServerHonestRoundSkip(p))

MC_ServerSpec ==
    MC_ServerInit /\ [][MC_ServerNext]_serverVars /\ MC_ServerFairness

StateSpaceLimit ==
    /\ \A n \in MC_Honest : round[n] <= MAX_ROUND
    /\ real_time <= MAX_TIMESTAMP
    /\ h_btc_current <= MAX_BTC_HEIGHT
    /\ h_engram_current <= MAX_ENGRAM_HEIGHT
    /\ h_engram_verified <= h_engram_current
    /\ h_btc_submitted <= h_btc_current
    /\ h_btc_anchored <= h_btc_submitted
    /\ Cardinality(active_peers) \in {2, 3}
    /\ Cardinality(anchor_peers) <= 3
    /\ Cardinality(blacklisted_peers) <= 2
    /\ peer_churn_rate \in {0, MAX_CHURN_RATE}
    /\ avg_peer_tenure \in {MIN_AVG_TENURE, MIN_AVG_TENURE + 2}
    /\ peer_latency \in {0, MAX_PEER_LATENCY}
    /\ is_btc_spv_failed \in BOOLEAN
    /\ is_das_failed \in BOOLEAN
    /\ is_attestation_failed \in BOOLEAN
    /\ state \in {"ANCHORED", "SUSPICIOUS", "SOVEREIGN", "RECOVERING"}

=============================================================================
