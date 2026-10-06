------------------------ MODULE MC_ServerRefinementVacuity ------------------------
(*
 * Reachability (vacuity) checks for the monolithic refinement run (MC_ServerRefinementSafety at a smaller bound).
 * Each NotReached_* invariant is the NEGATION of a behaviour the run must be able to exercise, so TLC is EXPECTED to
 * violate it. If it does not within the bound, a "no violation" result of the safety run at that bound says nothing about
 * that behaviour and the bound is too small to count.
 *
 * Unlike MC_ConsensusCoreVacuity, every action is enabled here (environment and consensus together), so FSM transitions
 * are a behaviour of this run and are required: the bound counts only if a block is decided after the FSM has left ANCHORED.
 *)
EXTENDS MC_ServerRefinementSafety

\* some honest node left round 0 (the timeout / round-skip path is exercised)
NotReached_RoundChange == \A p \in HonestNodes : round[p] < 1

\* a Byzantine proposer put a proposal into a round it leads
NotReached_ByzantineProposal ==
    \A r \in Rounds : (Proposer[r] \in ByzantineNodes) => msgs_propose[r] = {}

\* some honest node decided a block
NotReached_Decision == \A p \in HonestNodes : decision[p] = NilDecision

\* the FSM left ANCHORED (SUSPICIOUS, SOVEREIGN or RECOVERING)
NotReached_FSMTransition == state = "ANCHORED"

\* some honest node decided a block proposed in a state other than ANCHORED: an FSM transition and a decision both happened
NotReached_DecisionAfterFSMTransition ==
    \A p \in HonestNodes : decision[p] = NilDecision \/ decision[p].prop.fsm_state = "ANCHORED"

=============================================================================
