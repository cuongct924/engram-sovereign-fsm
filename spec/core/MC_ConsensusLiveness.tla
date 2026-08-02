---------------- MODULE MC_ConsensusLiveness ----------------
EXTENDS EngramConsensus, TLC

CONSTANTS n1, n2, n3, tx1

MC_Nodes == {n1, n2, n3}
MC_Method == {tx1}
MC_Stake == [n \in MC_Nodes |-> 10]

MC_Next == 
    IF rem_time = 0 
    THEN TimeoutStartNext \/ EarlyStartNext 
    ELSE Next

MC_Spec == Init /\ [][MC_Next]_vars

StateSpaceLimit == round <= 3

\* LiDO LIVENESS-TO-SAFETY reduction
PacemakerProgress == [][rem_time = 0 => round' > round]_vars
=================================================================