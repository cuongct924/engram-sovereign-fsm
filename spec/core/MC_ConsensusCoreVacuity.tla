------------------------ MODULE MC_ConsensusCoreVacuity ------------------------
(*
 * Reachability (vacuity) checks for the consensus-core refinement run
 * (MC_ConsensusCoreRefinementSafety). Each NotReached_* invariant is the NEGATION of a behaviour the run must be able to
 * exercise, so TLC is EXPECTED to violate it. If it does not within the bound, a "no violation" result of the safety run
 * says nothing about that behaviour and the configuration is too small to count.
 *
 * The environment is frozen in this part of the decomposition (sensors and the FSM state stay at genesis), so FSM
 * transitions are NOT a behaviour of this run; they belong to the environment part (MC_EnvironmentRefinementSafety).
 *)
EXTENDS MC_ConsensusCoreRefinementSafety

\* some honest node left round 0 (the timeout / round-skip path is exercised)
NotReached_RoundChange == \A p \in HonestNodes : round[p] < 1

\* a Byzantine proposer put a proposal into a round it leads
NotReached_ByzantineProposal ==
    \A r \in Rounds : (Proposer[r] \in ByzantineNodes) => msgs_propose[r] = {}

\* some honest node decided a block
NotReached_Decision == \A p \in HonestNodes : decision[p] = NilDecision

=============================================================================
