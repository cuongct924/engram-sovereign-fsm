---------------- MODULE MC_Ablation_NoFastForwardLiveness_Straggler ----------------
(*
 * Directly targets the "lagging honest node" scenario f+1 fast-forward exists for
 * (spec/README.md:718's own description: "guarantees that lagging nodes fast-forward
 * once f+1 honest nodes have timed out"), confirmed via OnRoundCatchup/
 * UponfPlusOneTimeoutsAny's guards (EngramTendermint.tla): 2 honest nodes
 * independently local-timeout and broadcast TIMEOUT for round+1 while a 3rd honest
 * node stays behind in the old round. UponQuorumOfPrevotesAny/PrecommitsAny
 * (2f+1-of-anything, already in the base spec, no f+1 involved) can't rescue the
 * straggler: the two "ahead" nodes have stopped voting in the old round, so its own
 * quorum count there can never complete. Only UponfPlusOneTimeoutsAny (ablated
 * here) or OnRoundCatchup (needs SUBSTANTIVE round activity -- a real propose/
 * prevote/precommit, not just a timeout broadcast) could pull it forward. This is a
 * DIFFERENT scenario from same-round vote-splitting, which the standard
 * 2f+1-of-anything mechanism already handles fine regardless of f+1 -- see
 * MC_Ablation_NoFastForwardLiveness_Targeted2.tla's now-superseded framing.
 *
 * Reorders MC_Proposer so round 0's proposer is honest (n2), not Byzantine (n1) --
 * avoids the whole Byzantine-round0 bootstrap machinery (ServerByzantinePull/
 * ServerHonestTimeout/ServerHonestRoundSkip, EngramServer.tla) for the one round
 * pair (0->1) this scenario needs, since none of their guards
 * (Proposer[r] \in ByzantineNodes) are satisfiable for r=0 or r=1 under this
 * schedule -- removes that entire branch of the search, not just deprioritizes it.
 *)
EXTENDS EngramServer_Ablation_NoFastForward, TLC, Sequences

CONSTANTS n1, n2, n3, n4

MC_Nodes  == {n1, n2, n3, n4}
MC_Method == {"TX_NORMAL", "TX_WITHDRAWAL"}
MC_Byzantine == {n1}
MC_Honest   == MC_Nodes \ MC_Byzantine

MC_NodeSeq  == <<n2, n3, n4, n1>>
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

\* Same rationale as MC_Ablation_NoFastForwardLiveness_Targeted2.tla: prunes the
\* dominant sensor-churn branching factor (UpdateSensors' BTC/DA/P2P combinator),
\* not required by MC_ServerFairness and irrelevant to round-advance mechanics.
NoiseFree ==
    action' /= "UpdateEnvironment"

=============================================================================
