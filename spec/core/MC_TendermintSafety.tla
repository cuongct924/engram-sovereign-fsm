---------------- MODULE MC_TendermintSafety ----------------
EXTENDS EngramTendermint, TLC

CONSTANTS
    \* @type: Str;
    p1,
    \* @type: Str;
    p2,
    \* @type: Str;
    p3,
    \* @type: Str;
    p4,
    \* @type: Str;
    v1,
    \* @type: Str;
    v3

\* @type: Int -> Str;
MC_Proposer == [r \in 0..MAX_ROUND |-> IF r % 2 = 0 THEN p1 ELSE p2]

\* Symmetry reduction: p1/p2/p3 (HonestNodes) are interchangeable -- nothing
\* outside MC_Proposer references any of them by identity, and TLC
\* re-evaluates MC_Proposer consistently under any permutation of these
\* model values (p3 never proposing in this schedule doesn't break the
\* symmetry, it just means p3's "role" is always the non-proposer one under
\* every permutation). p4 (Byzantine) stays out of the group.
SymmetryPerms == Permutations({p1, p2, p3})

StateConstraint ==
    /\ real_time <= MAX_TIMESTAMP
    /\ \A p \in HonestNodes: round[p] <= MAX_ROUND

Termination ==
    /\  \/ \A p \in HonestNodes : step[p] = "DECIDED"
        \/ real_time >= MAX_TIMESTAMP
        \/ \E p \in HonestNodes : round[p] >= MAX_ROUND
    /\ UNCHANGED <<tendermintCoreVars, temporalVars, invariantVars, fsmVars>>
    /\ UNCHANGED <<msgs_propose, msgs_prevote, msgs_precommit, msgs_timeout, evidence, received_timely_proposal, inspected_proposal, censorshipVars>>
    /\ action' = "Termination"

\* TendermintInit + FSMInit don't cover certificateVars (quorum_certs/
\* timeout_certs) -- those are server-layer state, only initialized by
\* EngramServer's ServerInit. Since this driver tests EngramTendermint in
\* isolation (no EngramServer), mock them closed/empty here so TLC's Init
\* produces a legal state for every declared VARIABLE.
MC_TendermintInit ==
    /\ TendermintInit
    /\ FSMInit
    /\ quorum_certs = {}
    /\ timeout_certs = {}

\* TendermintNext's own actions (e.g. AdvanceRealTime) don't mention
\* fsmVars/networkSensorVars/certificateVars at all -- in the composed spec
\* that's fine because EngramServer's ServerAdvanceRealTime explicitly
\* freezes them for every Tendermint-only step. Standalone here, freeze them
\* the same way. Apalache note: the extra UNCHANGED must be distributed into
\* each disjunct individually (matching TendermintNext's own internal
\* structure one level down) rather than wrapped around the whole
\* `TendermintNext` operator -- Apalache's assignment analysis checks each
\* top-level disjunct of the final Next for a syntactic assignment to every
\* variable, and does not distribute a wrapping conjunct into an inlined
\* operator's own inner disjuncts (TLC has no such requirement, since it
\* evaluates the formula directly instead of statically analyzing it).
\* Note: AdvanceRealTime itself already carries UNCHANGED <<certificateVars,
\* censorshipVars>> (see EngramTendermint.tla), so only fsmVars/
\* networkSensorVars need adding for that branch -- repeating certificateVars
\* here would be a redundant (Apalache: "spurious") second assignment.
MC_TendermintNext ==
    \/ (AdvanceRealTime /\ UNCHANGED <<fsmVars, networkSensorVars>>)
    \/ /\ SynchronizedLocalClocks
       /\ \E p \in HonestNodes : MessageProcessing(p)
       /\ UNCHANGED <<fsmVars, networkSensorVars, certificateVars>>
    \/ (ByzantineDataWithholding /\ UNCHANGED <<fsmVars, networkSensorVars, certificateVars>>)
    \/ (SubmitToCelestiaDA /\ UNCHANGED <<fsmVars, networkSensorVars, certificateVars>>)
    \* Termination itself already carries UNCHANGED <<..., fsmVars>> -- only
    \* networkSensorVars/certificateVars need adding here.
    \/ (Termination /\ UNCHANGED <<networkSensorVars, certificateVars>>)
=============================================================================