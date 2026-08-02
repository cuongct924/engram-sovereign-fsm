---------------- MODULE MC_TendermintSafety ----------------
EXTENDS EngramTendermint, TLC

CONSTANTS p1, p2, p3, p4, v1, v3

MC_Proposer == [r \in 0..MAX_ROUND |-> IF r % 2 = 0 THEN p1 ELSE p2]

StateConstraint == 
    /\ real_time <= MAX_TIMESTAMP
    /\ \A p \in Corr: round[p] <= MAX_ROUND

Termination == 
    /\  \/ \A p \in Corr : step[p] = "DECIDED"
        \/ real_time >= MAX_TIMESTAMP
        \/ \E p \in Corr : round[p] >= MAX_ROUND
    /\ UNCHANGED <<coreVars, temporalVars, invariantVars, fsmVars>>
    /\ UNCHANGED <<msgs_propose, msgs_prevote, msgs_precommit, msgs_timeout, evidence, received_timely_proposal, inspected_proposal, censorVars>>
    /\ UNCHANGED <<qcs, tcs>>
    /\ action' = "Termination"

MC_TendermintInit == 
    /\ TendermintInit 
    /\ FSMInit
    /\ qcs = {}
    /\ tcs = {}

MC_TendermintNext == 
    \/ (TendermintNext /\ UNCHANGED <<qcs, tcs>>)
    \/ Termination
=============================================================================