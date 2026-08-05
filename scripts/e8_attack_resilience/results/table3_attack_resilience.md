**Table 3 -- Attack-Resilience Test Suite (measured, real `go test` results):**

| Attack | Covering test(s) | Result | Expected (docs/EXPERIMENT.md) |
| --- | --- | --- | --- |
| Byzantine proposer sets fake fsm_state=ANCHORED under critical local sensors | TestProcessProposal_RejectsFSMStateMismatch | PASS | Honest validators prevote nil |
| DA attestation false but proposal contains block body/header | TestProcessProposal_RejectsMissingDAAttestation | PASS | Reject |
| BTC receipt rollback / forged checkpoint hash | TestProcessProposal_RejectsForgedBTCHash | PASS | Reject |
| Withdrawal tx during SOVEREIGN | TestProcessProposal_RejectsWithdrawalWhileSovereign | PASS | Blocked |
| Leader censorship of forced tx queue | TestProcessProposal_RejectsCensoringProposal, TestProcessProposal_AcceptsCensoredTxOnceIncluded | PASS | Rejected while omitted; accepted once included (M0d) |
| Premature/fake ZK re-anchoring proof claim (bonus, not in original E8 list) | TestProcessProposal_RejectsPrematureZKProofClaim | PASS | Reject |
| Timeout flooding by Byzantine nodes | -- | not covered | NOT COVERED -- needs live multi-node consensus (M0b) |
| Double-signing | -- | not covered | NOT COVERED -- needs CometBFT evidence module on a live node (M7) |
