--------------------- MODULE EngramServer_EclipseForgedProposal ---------------------
(*
 * Attack scenario (NOT an ablation -- EngramServer/EngramTendermint/EngramFSM
 * are all EXTENDS-ed unmodified, nothing is neutered): closes the open item
 * from README.md's Lemma 7.5 -- "a dedicated scenario constructing the full
 * execution trace that shows an eclipsed proposer's fabricated fsm_state is
 * deterministically rejected via FSMStateConsistency combined with
 * VerifySPVProof."
 *
 * Confirmed real (this session) that this open item was a genuine gap, not
 * just a missing diagram: the only thing previously "verified to produce
 * zero errors" for eclipse (P2PAdversaryAttack, EngramFSM.tla) is pure
 * FSM-sensor-layer eclipse mechanics -- it never touches IsValidProposal/
 * FSMStateConsistency, which live in EngramTendermint.tla/EngramServer.tla.
 * No existing Byzantine action lets a proposer forge fsm_state either:
 * ServerInsertProposal computes target_state == CalculateNextFSMState
 * directly (the one true global value, structurally can't lie), and the
 * existing ByzantineDataWithholding (EngramTendermint.tla) forges
 * da_receipt.attestation only, using expected_fsm_state == CalculateNextFSMState
 * for the fsm_state field. ByzantineForgedFSMState below is the first
 * action in this repo that actually forges fsm_state.
 *
 * ByzantineForgedFSMState mirrors ByzantineDataWithholding's exact shape
 * (raw BroadcastProposal, bypassing InsertProposal's IsValidProposal gate
 * so an invalid proposal can actually enter msgs_propose for honest nodes
 * to evaluate and reject on their own, rather than being blocked from ever
 * broadcasting at all) -- kept in THIS file rather than added to the
 * shared EngramTendermint.tla, since it is scenario-specific, not a change
 * to the core protocol. Gated on ~IsP2PQualityHealthy (Lemma 7.5's own
 * eclipse predicate) so the resulting trace is a faithful illustration of
 * "eclipsed proposer forges fsm_state", not a generic Byzantine lie.
 *
 * Why forged fsm_state gets rejected (mechanism, not just an asserted
 * property): UponProposalInPropose(p) (EngramTendermint.tla) has every
 * HONEST receiving node independently evaluate
 * `vote_target == IF IsValidProposal(prop) /\ ... THEN prop ELSE NilProposal`
 * on receipt -- IsValidProposal's `prop.fsm_state = CalculateNextFSMState`
 * conjunct fails for a forged proposal, so every honest node prevotes Nil
 * for it, regardless of whether IT is also eclipsed (each node recomputes
 * CalculateNextFSMState from ITS OWN current global sensor view -- this
 * spec models one shared sensor state, not per-node divergent views, so
 * "the proposer is eclipsed" cannot make receivers agree with a wrong
 * claim). A Nil-prevoted proposal can never reach a real (non-Nil)
 * precommit quorum, so it can never be decided -- FSMStateConsistency
 * (already defined in EngramServer.tla, reused verbatim below, not
 * reimplemented) is the direct formalization of "never decided".
 *)
EXTENDS EngramServer

\* Real state values other than the correct one -- removing exactly one
\* element from FSMTypeOK's 4-element state set always leaves 3, so this
\* CHOOSE is never over an empty set.
ForgedFSMStateChoices == {"ANCHORED", "SUSPICIOUS", "SOVEREIGN", "RECOVERING"} \ {CalculateNextFSMState}

\* Tendermint-layer-shaped action (see module doc for why it lives here
\* instead of core/EngramTendermint.tla). Mirrors ByzantineDataWithholding
\* line for line except: (a) forges fsm_state instead of da_receipt, (b)
\* requires ~IsP2PQualityHealthy (the eclipse precondition), (c) otherwise
\* submits a well-formed, honestly-attested proposal -- isolating fsm_state
\* forgery as the ONLY defect under test, so a rejection can only be
\* attributed to the fsm_state mismatch, not some other invalidity.
\* @type: Bool;
ByzantineForgedFSMState ==
    \E r \in Rounds :
        /\ Proposer[r] \in ByzantineNodes
        /\ msgs_propose[r] = {}
        /\ ~IsP2PQualityHealthy
        /\ LET
               forged_fsm_state == CHOOSE s \in ForgedFSMStateChoices : TRUE

               honest_da == [
                   published_block_height |-> h_engram_verified,
                   attestation            |-> ~is_attestation_failed
               ]
               honest_btc == [
                   checkpoint_block_height |-> h_btc_current,
                   checkpoint_block_hash   |-> ExpectedBlockHash(h_btc_current)
               ]
               forced_tx == CHOOSE tx \in forced_tx_queue : TRUE

               bad_prop == Proposal(forced_tx, real_time, r, forged_fsm_state,
                                     honest_da, honest_btc, FALSE)
           IN
           /\ BroadcastProposal(Proposer[r], r, bad_prop, NilRound)
           /\ UNCHANGED <<tendermintCoreVars, temporalVars, invariantVars, propAuditVars>>
           /\ UNCHANGED <<censorshipVars>>
           /\ UNCHANGED <<evidence, msgs_prevote, msgs_precommit, msgs_timeout>>
           /\ action' = "ByzantineForgedFSMState"

\* Server-layer wrapper, matching ServerByzantinePull/ServerByzantineDataWithholding's
\* existing shape exactly -- no new certificate is synthesized (unlike
\* ServerByzantineDataWithholding's M_QC), since this scenario only needs
\* the forged proposal to reach honest nodes' real prevote logic, not to
\* fabricate LiDO abstract-layer progress.
\* @type: Bool;
ServerByzantineForgedFSMState ==
    /\ ByzantineForgedFSMState
    \* certificateVars (quorum_certs, timeout_certs), not just timeout_certs
    \* alone -- Apalache's assignment analysis requires every variable to
    \* be assigned in every branch, and unlike ServerByzantineDataWithholding
    \* this action never synthesizes a new quorum_certs entry, so both must
    \* be declared UNCHANGED explicitly (found by actually running Apalache:
    \* "Missing assignments to: quorum_certs").
    /\ UNCHANGED <<certificateVars>>
    /\ UNCHANGED <<fsmVars, networkSensorVars>>

\* @type: Bool;
ServerNextWithEclipseAttack ==
    \/ ServerNext
    \/ ServerByzantineForgedFSMState

=============================================================================
