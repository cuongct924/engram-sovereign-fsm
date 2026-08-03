---------------- MODULE MC_Ablation_NoCircuitBreakerSafety ----------------
(*
 * MC_Ablation_NoCircuitBreakerSafety — Ablation driver: "Remove Circuit
 * Breaker".
 *
 * Extends EngramServer_Ablation_NoCircuitBreaker, a modified copy of
 * EngramServer.tla built on top of EngramTendermint_Ablation_
 * NoCircuitBreaker.tla, which deletes the withdrawal-lock conjunct from
 * IsValidProposal (the "Economic Circuit Breaker" that normally halts
 * cross-chain withdrawals while fsm_state \in {SOVEREIGN, RECOVERING}).
 *
 * Checks Sanity_NeverAttemptWithdrawalLeakage -- this invariant is designed
 * to HOLD in the real (unablated) spec; here we expect TLC to find a
 * counterexample, proving the circuit breaker is load-bearing.
 *
 * Reuses the same node/schedule/parameter setup as
 * MC_ServerRefinementSafety.cfg (N=4, T=1, 1 Byzantine node) -- this
 * driver does not go through EngramServerRefinement, since the ablation
 * only needs to demonstrate the withdrawal-leakage state, not the
 * refinement obligation.
 *)
EXTENDS EngramServer_Ablation_NoCircuitBreaker, TLC, Sequences

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

\* EXPECT FAIL: Proves the attacker attempts a withdrawal while in
\* SOVEREIGN or RECOVERING state, once the circuit breaker is ablated.
Sanity_NeverAttemptWithdrawalLeakage ==
    \A r \in Rounds : \A m \in msgs_propose[r] :
        (m.proposal.fsm_state \in {"SOVEREIGN", "RECOVERING"}) => (m.proposal.value /= "TX_WITHDRAWAL")

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
