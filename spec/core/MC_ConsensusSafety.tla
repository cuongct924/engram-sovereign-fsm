---------------- MODULE MC_ConsensusSafety ----------------
EXTENDS EngramConsensus, TLC

CONSTANTS
    \* @type: Str;
    n1,
    \* @type: Str;
    n2,
    \* @type: Str;
    n3,
    \* @type: Str;
    n4,
    \* @type: Str;
    tx1,
    \* @type: Str;
    tx2

\* n4 stays declared (harmless unused model value) but is excluded from
\* MC_Nodes below -- see the StateSpaceLimit note.
MC_Nodes == {n1, n2, n3}
MC_Method == {tx1, tx2}
MC_Stake == [n \in Nodes |-> 10]

\* Tightened twice after round<=3/tree<=5 (4 nodes) generated 650M+ states /
\* 115M+ distinct states and exhausted disk, and round<=2/tree<=4 (4 nodes)
\* was still growing at ~70MB/min of DiskStateQueue with no sign of slowing
\* after 9 minutes. SUBSET Nodes (the quorum powerset in ValidQuorums) is the
\* main combinatorial driver, so this pass also drops to 3 nodes (2^3=8
\* subsets instead of 2^4=16) on top of a much shallower round/tree bound.
\* This is a deliberate small-scope smoke-test pass, not a compromise on
\* rigor -- it's the first data point in a bound-sensitivity sequence
\* (widen round/tree/node-count from here and confirm no new violations
\* appear), and/or move this check to Apalache's bounded symbolic search
\* (Task A6/A8), which doesn't enumerate the full reachable set to disk the
\* way TLC's BFS does.
StateSpaceLimit ==
    /\ round <= 1
    /\ Cardinality(tree) <= 2

SymmetryPerms == Permutations(Nodes)
===========================================================