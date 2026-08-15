---------------- MODULE MC_TendermintSafety_Apalache ----------------
(*
 * MC_TendermintSafety_Apalache — Apalache bounded-safety driver for the
 * Tendermint consensus engine (EngramTendermint.tla), standalone (no
 * EngramServer hooks, matching MC_TendermintSafety.tla's own scope).
 *
 * Reuses MC_TendermintSafety's Init/Next via
 * `check --cinit=ApalacheCInit --init=MC_TendermintInit --next=MC_TendermintNext`.
 * Does not replace MC_TendermintSafety.tla/.cfg — a complementary entry point.
 *
 * IMPORTANT: p1..p4/v1/v3 here are MC_TendermintSafety.tla's own TLC model
 * values -- Apalache has no equivalent, so CInit binds them to concrete
 * strings directly (mirrors MC_ConsensusSafety_Apalache.tla's approach).
 *)
EXTENDS MC_TendermintSafety

ApalacheCInit ==
    /\ p1 = "p1" /\ p2 = "p2" /\ p3 = "p3" /\ p4 = "p4"
    /\ v1 = "v1" /\ v3 = "v3"
    /\ HonestNodes = {"p1", "p2", "p3"}
    /\ ByzantineNodes = {"p4"}
    /\ N = 4
    /\ T = 1
    \* Bounds tightened from MC_TendermintSafety.cfg's TLC values (10/10/2) --
    \* Apalache force-enumerates the full Cartesian product for any `x \in
    \* Proposals`-style membership test (TendermintTypeOK checks
    \* valid_value[p] \in ProposalsOrNil), and at MAX_BTC_HEIGHT=MAX_ENGRAM_
    \* HEIGHT=10 that product is exactly 2,804,160 elements -- past
    \* Apalache's "Too many elements to enumerate" rewriter ceiling. Smaller
    \* here for a first pilot pass; the real coverage numbers still come
    \* from TLC (MC_TendermintSafety.cfg), which doesn't need to materialize
    \* Proposals as an explicit set.
    /\ MAX_ROUND = 2
    /\ MIN_TIMESTAMP = 0
    /\ MAX_TIMESTAMP = 2
    /\ DELAY = 1
    /\ PRECISION = 0
    /\ TIMEOUT_DURATION = 1
    /\ MAX_BTC_HEIGHT = 1
    /\ MAX_ENGRAM_HEIGHT = 1
    /\ MAX_IGNORE_ROUNDS = 2
    /\ MAX_CENSORSHIP_ROUNDS = 3
    /\ SUSPICIOUS_THRESHOLD = 2
    /\ SOVEREIGN_THRESHOLD = 4
    /\ DA_THRESHOLD = 2
    /\ HYSTERESIS_WAIT = 2
    /\ SUSPICIOUS_HYSTERESIS_WAIT = 2
    /\ MAX_SUSPICIOUS_TIME = 1
    /\ DOWN_HYSTERESIS_THRESHOLD = 1
    /\ MAX_DOWN_HYSTERESIS_THRESHOLD = 8
    /\ MIN_PEERS = 3
    /\ MIN_SUBNET_DIVERSITY = 2
    /\ MIN_ANCHOR_PEERS = 1
    /\ MAX_CHURN_RATE = 10
    /\ MIN_AVG_TENURE = 100
    /\ MAX_PEER_LATENCY = 50
    \* Literal 2 instead of MAX_ROUND: Apalache's `0..X` range needs a constant
    \* integer literal at this point, not a same-CInit CONSTANT reference
    \* (known issue: https://apalache-mc.org/docs/apalache/known-issues.html).
    /\ Proposer = [r \in 0..2 |-> IF r % 2 = 0 THEN "p1" ELSE "p2"]

=============================================================================
