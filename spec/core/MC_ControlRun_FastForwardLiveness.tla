---------------- MODULE MC_ControlRun_FastForwardLiveness ----------------
(*
 * TLC control-run driver for the "Remove f+1 timeout fast-forward" ablation
 * (MC_Ablation_NoFastForwardLiveness.tla).
 *
 * Byte-for-byte identical to that ablation driver except for one line:
 * EXTENDS EngramServer (the REAL module, with UponfPlusOneTimeoutsAny
 * intact) instead of EngramServer_Ablation_NoFastForward. Same constants,
 * same MC_ServerFairness, same StateSpaceLimit, same property
 * (EventualDecisionUnderGSTLiveness) -- the only thing that changes is
 * whether f+1 fast-forward exists.
 *
 * Exists to close a real methodological gap flagged in README.md's
 * ablation-D writeup: the ablated run alone shows f+1's removal correlates
 * with a liveness violation, but doesn't rule out the same schedule/
 * fairness formula stalling even against the real spec (i.e. that the
 * driver itself, not the ablation, is the cause). This driver is the
 * control: if it passes at the same or greater depth with no violation
 * while the ablated driver stalls, the ablation is confirmed as the actual
 * cause. Mirrors the (previously uncommitted, prose-only) control run
 * already done for Ablation B ("Remove Hysteresis", README.md's Lemma
 * 9.3.2.B) -- this one is committed as a real, independently re-runnable
 * file instead.
 *
 * MC_ServerFairness (2026-08-09): includes WF_serverVars(ServerHonestTimeout)
 * and WF_serverVars(ServerHonestRoundSkip(p)) for each honest p, matching
 * MC_ServerRefinementLiveness.tla's post-Hướng-A+B fairness formula (see
 * LIVENESS_DEADLOCK_FINDING.md) -- an earlier version of this driver
 * (copied verbatim from the ablated driver before ITS OWN fairness
 * formula was corrected) omitted these, which meant this control run
 * stuttered on the exact same already-known, already-fixed
 * Byzantine-silent-leader bootstrap deadlock as the ablated run, rather
 * than actually testing whether f+1 removal causes a liveness problem
 * beyond that.
 *)
EXTENDS EngramServer, TLC, Sequences

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
