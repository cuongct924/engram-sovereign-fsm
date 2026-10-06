# Engram Re-anchoring: ZK Proof System & Research Benchmarks

Development, results, and design decisions for Engram re-anchoring's ZK proof system.

## Overview
The ZK system proves a blockchain header chain is valid *and really committed*, to recover system
state. Three architectures were evaluated:
1. Dynamic Padding (production leaf circuit, up to `N_MAX=256` headers per proof).
2. Chunk Aggregation (opt-in, up to `K*N_MAX=1024` headers per proof, `circuit/reanchoring_aggregator`).
3. Per-header recursive aggregation (research spike, rejected).

## Formal Definition: Proof of Recovery
- `SOVEREIGN`-mode blocks carry only local-PoS security, not Bitcoin-anchored finality.
- Returning to `ANCHORED` needs a Proof of Recovery, not a Proof of Execution: it proves the
  sovereign interval kept continuity and recorded only `SOVEREIGN` or `RECOVERING` states with the lock
  flag set (it does not check transitions against the transition function), AND that the interval's two boundary
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
| (b) Mode-label consistency | $H_{k+i}$.`fsm_state` $\in \{\texttt{SOVEREIGN},\texttt{RECOVERING}\}$ | Every recorded state in the interval is one of the two; transitions are not checked against the transition function |
| (c) Lock-flag consistency | $H_{k+i}$.`withdrawal_locked` $= \mathrm{true}$ | Every recorded lock flag is set; the flag is the chain's own record, so this does not show that no withdrawal executed |
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
  real interval length via `LastAnchoredHeight` (or fold up to 4 chunks into one proof, §2).
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

## 2. Chunk Aggregation (opt-in)
- `circuit/reanchoring_aggregator` verifies up to `K=4` leaf proofs of `circuit/reanchoring`
  (`verify_honk_proof`, leaf VK and key hash compiled in as constants from
  `src/leaf_vk.nr`, generated by `gen_leaf_vk.py`) and emits one proof with the **same 7 public
  inputs** as a leaf, so `SubmitRecoveryProof`'s 224-byte layout is unchanged.
- Chunk `i` must start at chunk `i-1`'s `rt_new` and height `first_height + count`, under the same
  `validator_set_hash`; the outputs are the first chunk's `rt_last`/`first_height`/`first_block_hash`,
  the last real chunk's `rt_new`/`last_block_hash`, and the summed `count`.
- Slots `i >= n_real` are padding: `verify_honk_proof` is unconditional, so a padding slot repeats a
  valid leaf proof and every constraint on it is gated by `i < n_real`.
- Leaf proofs use `bb prove -t noir-recursive`; the aggregate uses the default target (what the
  embedded verifier expects). A single leaf (`count <= 256`) keeps the default-target proof.
- `x/sovereignty/keeper/reanchor.go` picks the VK by the claimed `count`: `<= 256` verifies against
  `zk_assets/vk`, larger against `zk_assets/vk_aggregate`. Regenerate `vk_aggregate` (and
  `src/leaf_vk.nr`) whenever `circuit/reanchoring` changes.
- Prover: `scripts/reanchoring_prover/prove_aggregate_and_submit.sh`, used by `prove_and_submit.sh`
  when `ENGRAM_AGGREGATE=1` and the tracked interval exceeds `N_MAX`.

### Measured cost
`scripts/e6_zk_reanchoring_benchmark/benchmark_aggregate.sh` (nargo 1.0.0-beta.22, bb
5.0.0-nightly.20260522, 10-core / 32 GiB macOS workstation; synthetic chain with real ed25519
precommits; aggregate prove/verify over 5 trials, leaf prove once per chunk). Results:
`results/table6e_aggregate.csv`. §1's timings come from a different host, so compare absolute
times only within one table.

| L (headers) | Chunks | Chaining: prove | Chaining: proofs | Aggregate: total prove | Aggregate: proof |
|---:|---:|---:|---:|---:|---:|
| 256 | 1 | 31.5 s | 14,656 B | 57.2 s | 14,656 B |
| 512 | 2 | 60.5 s | 29,312 B | 89.9 s | 14,656 B |
| 1024 | 4 | 120.1 s | 58,624 B | 146.7 s | 14,656 B |

| Circuit | Circuit size | Prove | Peak RSS |
|---|---:|---:|---:|
| Leaf (256 headers) | 3,234,226 | 30.0-31.5 s | 5.7-5.9 GB |
| Aggregator (K=4) | 3,002,299 | 25.7-29.5 s | 5.2 GB |

Validator-side verification through the embedded WASM verifier
(`BenchmarkVerifyRecoveryProofWasm`): leaf 19.8 ms, aggregate 20.2 ms per proof, so a chain of `n`
leaf proofs costs `n` x 19.8 ms and one aggregate costs 20.2 ms for any `L <= 1024`.

- Verification cost and submitted bytes are constant in `L` up to `K*N_MAX`; prove cost is the leaf
  proofs plus one ~26-29 s aggregation (+22% at L=1024, +49% at L=512).
- No benefit for `L <= 256`: a single leaf proof is submitted, and the chain rejects an aggregate
  claiming `count <= 256` (it is verified against the leaf VK).
- Every chunk boundary needs a witnessable commit: `build-boundary-witness` fails when the commit
  at a boundary height has `round != 0` (`build_vote_sign_bytes` supports round 0 only), which now
  also applies to the interior boundaries of a multi-chunk interval.
- Aggregate soundness is tested by `circuit/reanchoring_aggregator/negative_tests.sh` (12 witnesses:
  reversed or repeated chunks, wrong count/`rt_new`/first height/validator set, a height gap, a
  different validator set in chunk 2, padding extension, `n_real` out of range).
- `SubmitRecoveryProof` accepts a real 512-header aggregate against fixture chain state
  (`TestSubmitRecoveryProof_AcceptsRealAggregate`).
- Not measured: a live Docker testnet run, parallel proving of chunks, intervals beyond `K*N_MAX`
  (those chain aggregate proofs).

### Per-header recursion (rejected)
A spike aggregated leaves of the N=4 circuit (`circuit/reanchoring_recursion_spike*`):

| M (leaves) | Headers | Circuit size | Prove time |
| ---: | ---: | ---: | ---: |
| 2 | 8 | 1,469,553 | 74 s |
| 4 | 16 | 3,001,406 | 175 s |

Against direct hashing at the same header count it was ~650x (8 headers) to ~1,200x (16 headers)
slower, because `verify_honk_proof` costs ~700k gates per leaf versus ~12 opcodes to hash one header.
Recursion pays off only when each leaf is itself expensive, which is why aggregation is applied to
256-header chunks (the leaf circuit is 3.2M gates, so verifying it in-circuit adds ~22%).

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
- **Aggregator:** `circuit/reanchoring_aggregator/negative_tests.sh` (12 witnesses) and the Go
  tests in `x/sovereignty/keeper/zk_aggregate_test.go` (VK selection by `count`, real 2-chunk
  aggregate through the embedded verifier and `SubmitRecoveryProof`).
- **Full repo:** `go build ./... && go vet ./... && go test ./...` clean.
- **End-to-end:** validated through experiment E2 S7 (recovery flow).
