---------------- MODULE MC_ConsensusLiveness ----------------
EXTENDS EngramConsensus, TLC

CONSTANTS n1, n2, n3, tx1

MC_Nodes == {n1, n2, n3}
MC_Method == {tx1}
MC_Stake == [n \in MC_Nodes |-> 10]

\* Symmetry reduction: unlike the Server layer, the abstract LiDO layer has
\* no Byzantine/Honest split at all -- every node in MC_Nodes is fully
\* interchangeable (same Stake, no identity-specific logic anywhere in
\* EngramConsensus.tla). Sound to permute all of them.
SymmetryPerms == Permutations(MC_Nodes)

MC_Next == 
    IF rem_time = 0 
    THEN TimeoutStartNext \/ EarlyStartNext 
    ELSE Next

MC_Spec == Init /\ [][MC_Next]_vars

StateSpaceLimit == round <= 3

\* LiDO LIVENESS-TO-SAFETY reduction
PacemakerProgress == [][rem_time = 0 => round' > round]_vars
=================================================================