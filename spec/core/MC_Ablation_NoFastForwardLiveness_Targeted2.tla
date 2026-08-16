---------- MODULE MC_Ablation_NoFastForwardLiveness_Targeted2 ----------
(*
 * Further-targeted variant on top of MC_Ablation_NoFastForwardLiveness_Targeted.cfg's
 * reasoning: adds an ACTION_CONSTRAINT pruning two Server-level actions that
 * MC_ServerFairness never requires fairness on (see MC_Ablation_NoFastForwardLiveness.tla),
 * so excluding them cannot invalidate the liveness proof's own assumptions, and that
 * are not necessary to reach the target scenario -- an honest-proposer round (r=1 or
 * r=2 under MC_Proposer's round-robin schedule) failing to reach a 2f+1 precommit
 * quorum, which ServerHonestTimeout/ServerHonestRoundSkip cannot rescue since both
 * require Proposer[r] \in ByzantineNodes:
 *
 * - ServerUpdateEnvironment (EngramServer.tla:283): fires UpdateSensors, an
 *   8-way-plus nondeterministic BTC/DA/P2P sensor combinator every step -- the
 *   dominant branching factor in the 80M+/89M+ state runs so far. FSM/sensor state
 *   doesn't gate Tendermint round-advance mechanics (StateSpaceLimit already pins
 *   peer/churn/tenure/latency to small fixed sets; fsm_state stays ANCHORED from
 *   FSMInit unless this action fires).
 * - ServerByzantineDataWithholding (EngramServer.tla:396): requires an
 *   already-existing E_QC for round r (same guard as ServerByzantinePull, which is
 *   kept enabled -- it's the only source of round 0's E_QC, since Proposer[0]=n1 is
 *   Byzantine under MC_Proposer). Only produces an M_QC; skipping past a stalled
 *   round only needs a T_QC (ServerHonestTimeout -> ServerHonestRoundSkip), not an
 *   M_QC, so excluding this cannot block reaching round 1/2.
 *
 * ServerByzantinePull is deliberately NOT excluded: it's the only source of round
 * 0's E_QC, which ServerHonestTimeout itself requires to exist before it can fire
 * -- excluding it would strand every honest node in round 0 forever, a different
 * (uninteresting) stutter unrelated to the f+1 question this ablation targets.
 *)
EXTENDS MC_Ablation_NoFastForwardLiveness

NoiseFree ==
    action' \notin {"UpdateEnvironment", "ByzantineDataWithholding"}

=============================================================================
