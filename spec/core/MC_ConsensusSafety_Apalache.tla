---------------- MODULE MC_ConsensusSafety_Apalache ----------------
(*
 * MC_ConsensusSafety_Apalache — Apalache bounded-safety driver for the
 * abstract LiDO consensus layer (EngramConsensus.tla), standalone (not
 * through the refinement INSTANCE).
 *
 * Reuses MC_ConsensusSafety's Init/Next (the same ones TLC checks) via
 * `check --cinit=ApalacheCInit --init=Init --next=Next`. Does not replace
 * MC_ConsensusSafety.tla/.cfg — a complementary Apalache entry point.
 *
 * IMPORTANT: Method/Nodes here are bound to string literals rather than the
 * TLC model values (n1,n2,n3/tx1,tx2) used in MC_ConsensusSafety.cfg —
 * Apalache has no equivalent of TLC model values, so CInit must supply
 * concrete typed values directly.
 *)
EXTENDS MC_ConsensusSafety

ApalacheCInit ==
    \* n1..n4/tx1/tx2 are MC_ConsensusSafety.tla's own TLC model-value
    \* CONSTANTS (used only inside its MC_Nodes/MC_Method, which this driver
    \* does not reference) -- assigned here only so every declared CONSTANT
    \* in the extended module has a value, as Apalache requires.
    /\ n1 = "n1" /\ n2 = "n2" /\ n3 = "n3" /\ n4 = "n4"
    /\ tx1 = "tx1" /\ tx2 = "tx2"
    /\ Nodes = {"n1", "n2", "n3"}
    /\ Method = {"tx1", "tx2"}
    /\ Stake = [n \in {"n1", "n2", "n3"} |-> 10]
    /\ RESET_TIME = 3
    /\ TOTAL_STAKE = 30
    /\ MAX_BTC_HEIGHT = 3
    /\ K_DEEP_FINALITY = 2

=============================================================================
