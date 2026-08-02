---------------- MODULE MC_ConsensusSafety ----------------
EXTENDS EngramConsensus, TLC

CONSTANTS n1, n2, n3, n4, tx1, tx2

MC_Nodes == {n1, n2, n3, n4}
MC_Method == {tx1, tx2}
MC_Stake == [n \in Nodes |-> 10]

StateSpaceLimit == 
    /\ round <= 3
    /\ Cardinality(tree) <= 5

SymmetryPerms == Permutations(Nodes)
===========================================================