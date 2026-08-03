---------------- MODULE MC_FSMSafety_Apalache ----------------
(*
 * MC_FSMSafety_Apalache — Apalache bounded-safety driver for the FSM layer.
 *
 * Reuses MC_FSMSafety's MC_FSMInit/MC_FSMNext (the same Init/Next TLC checks)
 * as the pair Apalache checks symbolically via `check --cinit=ApalacheCInit
 * --init=MC_FSMInit --next=MC_FSMNext`. This file does not replace
 * MC_FSMSafety.tla/.cfg — it is a complementary Apalache entry point.
 *
 * IMPORTANT: ApalacheCInit mirrors MC_FSMSafety.cfg's CONSTANTS block by
 * hand. If that .cfg changes, update this operator to match.
 *
 * mc/ is a single flat directory (no per-layer subfolders) precisely so
 * this resolves with no extra flags: Apalache (like TLC) resolves EXTENDS
 * relative to the directory of the file being checked, and Apalache in
 * particular has no search-path flag to point at modules living elsewhere
 * (its launcher runs `java -jar`, which ignores CLASSPATH entirely — TLC
 * can work around this with `-cp`, Apalache cannot). `MC_FSMSafety` sits
 * right next to this file, and core/*.tla is reached via the six symlinks
 * alongside this file (EngramVars.tla, EngramFSM.tla, ... -> ../core/*.tla)
 * — one symlink set, shared by every layer's TLC and Apalache driver, not
 * re-symlinked per layer.
 *)
EXTENDS MC_FSMSafety

ApalacheCInit ==
    /\ SUSPICIOUS_THRESHOLD = 2
    /\ SOVEREIGN_THRESHOLD  = 4
    /\ DA_THRESHOLD         = 2
    /\ HYSTERESIS_WAIT      = 2
    /\ MAX_SUSPICIOUS_TIME  = 2
    /\ MIN_PEERS            = 3
    /\ MIN_SUBNET_DIVERSITY = 2
    /\ MIN_ANCHOR_PEERS     = 1
    /\ MAX_CHURN_RATE       = 10
    /\ MIN_AVG_TENURE       = 100
    /\ MAX_PEER_LATENCY     = 50

=============================================================================
