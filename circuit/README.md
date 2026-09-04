# Engram Re-anchoring: ZK Proof System & Research Benchmarks

Development, results, and design decisions for Engram re-anchoring's ZK proof system.

## Overview
The ZK system proves a blockchain header chain is valid *and really committed*, to recover system
state. Two architectures were evaluated:
1. Dynamic Padding (production choice).
2. Recursive Aggregation (research spike — rejected).

## Formal Definition: Proof of Recovery
- `SOVEREIGN`-mode blocks carry only local-PoS security, not Bitcoin-anchored finality.
- Returning to `ANCHORED` needs a Proof of Recovery, not a Proof of Execution: it proves the
  sovereign interval kept continuity and FSM recovery policy, AND that the interval's two boundary
  blocks were really committed by `>=2/3` of the genesis validator set — without re-verifying any
  transaction, keeping constraints low for $O(1)$ verification and constant proof size.
- Built with Noir/UltraHonk (`circuit/reanchoring/src/main.nr`), Poseidon2 t=4 as the chaining hash
  (`hash_header`), standard knowledge soundness.
- Public input $x = (rt_{last}, rt_{new}, n, h_{first}, H_{first}, H_{last}, V)$: checkpoint roots,
  count, the interval's first header height, both boundary blocks' real CometBFT hashes, and the
  genesis validator-set hash — 7 fields, 224 bytes total (`SubmitRecoveryProof` in
  `x/sovereignty/keeper/msg_server.go` parses and cross-checks all of them against on-chain state).
- Witness $w = (H_{k+1}, \dots, H_{k+n})$ — lightweight headers, not transaction bodies.
- $\Phi_{Recovery}(x, w) = 1$ iff every header $i \in [n]$ satisfies:

| Condition | Constraint | Guarantee |
|---|---|---|
| (a) Continuity | $H_{k+i+1}$.`prev_hash` $= \mathrm{hash\_header}(H_{k+i})$ | No injected or reordered blocks |
| (b) Policy | $H_{k+i}$.`fsm_state` $\in \{\texttt{SOVEREIGN},\texttt{RECOVERING}\}$ | No illegal jump to `ANCHORED` |
| (c) Circuit breaker | $H_{k+i}$.`withdrawal_locked` $= \mathrm{true}$ | No withdrawal during reduced security |
| (d) Root binding | $H_{k+1}$.`prev_hash` $=rt_{last}$, $\mathrm{hash\_header}(H_{k+n}) = rt_{new}$ | Endpoints bound to the on-chain accumulator, not a free witness value |
| (e) Boundary signatures | `>=2/3` voting power's real ed25519 precommits over each boundary block, against a validator set matching a real Merkle root | Interval's two endpoints are real committed CometBFT blocks, not a fabricated chain |

Notes:
- `state_root` inside each header is CometBFT's own per-block `AppHash`; `hash_header`'s output
  (the accumulator bound in (d)) is a distinct value from any header's own `state_root` field.
- **Why boundary-only signatures, not per-header:** verifying every header's own signatures would
  multiply condition (e)'s cost (14 ed25519 verifications: 2 blocks × 7 validators) by up to
  $N_{max}=256\times$. Condition (d)'s accumulator already makes the *interior* unforgeable — a
  prover can't fabricate any header without changing `rt_new`, and only real `CommitFSMTransition`
  execution (`x/sovereignty/preblock.go`) can ever advance the on-chain accumulator to a value
  `SubmitRecoveryProof` accepts. Condition (e) then ties just the two *endpoints* to real signed
  blocks — the whole interval is bound to real history at the cost of 2 blocks' signatures, not
  $N_{max}$'s worth.
- $n$ (`count`) is capped at a compile-time ceiling $N_{max} = 256$; longer intervals chain a
  rolling sequence of proofs, each starting where the last left off, `count` checked against the
  real interval length via `LastAnchoredHeight`.
- **Chaining hash:** Poseidon2 replaced an earlier SHA-256 chaining hash once an
  independently-verified Go implementation existed (`gnark-crypto` v0.21.0+, cross-checked
  byte-for-byte across 6 vectors). `circuit/reanchoring_witness/src/main.nr` keeps its own copy of
  `hash_header` that must stay in sync (`hash_header_matches_main_circuit_fixture` test) — a past
  sync miss broke real on-chain proofs and is the one trap worth remembering here.

## 1. Production Design: Dynamic Padding (N_MAX=256)
- Single circuit (`circuit/reanchoring/src/main.nr`), fixed capacity of 256 headers.
- A runtime witness `count` (1..=256) plus padding logic picks how many of the 256 slots are real
  vs. padding — no recompiling per N.
- Cost is constant regardless of the real interval length.

### Performance Results
Measured on nargo 1.0.0-beta.22 / bb 5.0.0-nightly.20260522, current circuit (Poseidon2 +
condition (e) boundary signatures included) — constant regardless of `count`, since padding always
processes all 256 slots and boundary verification is per-proof, not per-header:

| count | Execute | Prove Time | Verify Time | Proof Size | Public Inputs |
| :--- | :--- | :--- | :--- | :--- | :--- |
| 1 | 60.9s | 38.3s | 25ms | 14,656 B | 224 B |
| 4 | 60.3s | 38.3s | 24ms | 14,656 B | 224 B |
| 16 | 60.3s | 41.4s | 23ms | 14,656 B | 224 B |
| 64 | 60.5s | 40.5s | 25ms | 14,656 B | 224 B |
| 256 (N_MAX) | 60.6s | 40.4s | 24ms | 14,656 B | 224 B |

Circuit stats: 1,097,274 ACIR opcodes, 3,234,226 circuit size — same for every `count`; VK
byte-identical across all measured values.

**SHA-256 vs. Poseidon2** (same circuit, condition (e) included):

| Chaining hash | ACIR opcodes | Circuit size | Prove Time | Verify Time | Proof Size |
| :--- | ---: | ---: | ---: | ---: | ---: |
| SHA-256 (superseded) | — | 6,609,992 | 130.40s | 22ms | 14,656 B |
| Poseidon2 (current) | 1,097,274 | 3,234,226 | ~38-41s | ~23-25ms | 14,656 B |

- Poseidon2 roughly halves circuit size and cuts prove time to about a third.
- Smaller win than pre-condition(e), since the 14 ed25519 boundary verifications don't shrink with
  the hash choice and now dominate the circuit.

**Padding overhead alone** (measured under an earlier, pre-condition(e) circuit, isolating just the
padding-logic cost from the hash/signature choices above):

| Metric | Fixed-N | Dynamic Padding | Overhead |
| :--- | :--- | :--- | :--- |
| ACIR opcodes | 3,071 | 6,143 | ~2.0x |
| Circuit size | 39,298 | 47,613 | ~1.2x |
| Prove Time | 0.684s | 1.059s | ~1.5x |
| Verify Time | 22ms | 22.8ms | ~flat |
| Proof size | 14,656 B | 14,656 B | same |

### Rationale
- **Constant cost:** predictable performance for validators.
- **Full flexibility:** proves any 1–256 header interval immediately.
- **Liveness:** closes the E2 S7 race (`docs/EXPERIMENT.md:105-107`) and the trailing-remainder
  problem (a segment shorter than a fixed N could never be proven at all).

## 2. Research Spike: Recursive Aggregation (rejected)
Investigated aggregating multiple small proofs recursively:
- Leaf circuit: byte-for-byte copy of the deployed N=4 `reanchoring` circuit, proven with
  `bb prove --verifier_target noir-recursive`.
- Aggregator: verifies M leaf proofs via `verify_honk_proof`, leaf VK pinned as a compile-time
  constant, chaining `rt_last`/`rt_new` across leaves.

| M (leaves) | Headers | ACIR opcodes | Circuit size | Prove time | Verify time |
| ---: | ---: | ---: | ---: | ---: | ---: |
| 1 | 4 | 58 | 703,582 | ~191s¹ | 24ms |
| 2 | 8 | 85 | 1,469,553 | 74s | 60ms |
| 4 | 16 | 89 | 3,001,406 | 175s | 20ms |

¹ Likely inflated by one-time CRS (structured reference string) growth mid-run; doesn't change the
conclusion below.

Direct hashing vs. recursive aggregation, same header count:

| Headers | Direct (Prove / Gates) | Recursive (Prove / Gates) | Speed Gap |
| :--- | :--- | :--- | :--- |
| 8 | 0.113s / 1,106 | 74.0s / 1,469,553 | ~650x |
| 16 | 0.146s / 2,338 | 175.0s / 3,001,406 | ~1,200x |

**Why rejected:**
- `verify_honk_proof`'s in-circuit cost (pairing/commitment checks for a whole inner proof) dwarfs
  the ~12-opcode cost of hashing one header — recursion pays that cost per leaf.
- Recursion suits proofs that are themselves expensive to re-verify (rollups), not cheap items like
  header hashes.
- Matching N_MAX=256 needs ~M=64 leaves; circuit size roughly doubles per doubling of M —
  infeasible on validator hardware. Touched no production code (`circuit/reanchoring/`,
  `x/sovereignty/keeper/zk_assets/vk` unmodified); §1's dynamic padding is the shipped fix for the
  same gap.

## 3. Deterministic Verifier
- `VerifyZKProof` (`x/sovereignty/keeper/reanchor.go`) runs entirely inside `engramd`: no external
  `bb` binary, no PATH/checksum dependency.
- `verifyZKProofWasm` (`zk_verifier.go`) loads an embedded `wasm32-wasi` UltraHonk verifier
  (`zk_assets/barretenberg.wasm.gz`) through [`wazero`](https://wazero.io), a pure-Go WASM runtime —
  the same bytecode, interpreted by the same runtime, produces the identical result on any
  validator's OS/CPU.
- The `.wasm` is built by
  [engram-barretenberg-wasm](https://github.com/cuongct924/engram-barretenberg-wasm), pinned to the
  exact `aztec-packages` commit this repo's `bb` CLI (`BB_VERSION`) was built from — UltraHonk's
  proof/VK binary layout isn't stable across commits, so the prover and this embedded verifier must
  always share one toolchain build.
- `TestVerifyZKProof_RealProof` cross-validates the embedded verifier against a real proof
  generated by that same pinned `bb` release, accepting the genuine proof and rejecting a tampered
  copy.

## 4. Verification and Integration
- **Noir:** 10 test scenarios (count=1/4/130/256 satisfy the circuit; count=0, count>N_MAX, wrong
  `fsm_state`, a padding-cannot-extend-real-coverage attack, an rt_new-forgery attempt, and an
  insufficient-boundary-quorum attempt all correctly `should_fail`), plus a hash cross-check
  against `circuit/reanchoring_witness/`'s fixture.
- **Go:** `TestHashHeader_MatchesCircuitFixture` cross-checks the Go accumulator against the
  circuit's own fixture, byte-for-byte; `BenchmarkVerifyZKProof` runs against the regenerated
  embedded VK.
- **Full repo:** `go build ./... && go vet ./... && go test ./...` clean.
- **End-to-end:** validated through experiment E2 S7 (recovery flow).
