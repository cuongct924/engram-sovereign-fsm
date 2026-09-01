# Engram Re-anchoring: ZK Proof System & Research Benchmarks

Development, results, and design decisions for Engram re-anchoring's ZK proof system.

## Overview
The ZK system proves a blockchain header chain is valid, to recover system state. Two architectures were evaluated:
1. Dynamic Padding (production choice).
2. Recursive Aggregation (research spike — rejected).

## Scope: policy-compliance proof, not a proof of CometBFT history

$\Phi_{Recovery}$ proves the witness header chain is *internally* well-formed (continuity,
FSM/withdrawal-lock policy) and terminates at the two on-chain-recorded endpoints. It does **not**
verify CometBFT commit signatures, a validator-set hash, or any other cryptographic tie to real
consensus messages — nothing in the circuit itself distinguishes a real block history from a
self-consistent chain of numbers a prover invented. Call it what it is: a proof that *if* the
witness corresponds to real chain history, that history complied with the circuit-breaker policy —
not a proof that it does correspond to real history, and not a proof of transaction execution.

That correspondence is established outside the circuit, by construction: `rt_new` must equal
`hash_header` applied to the claimed final header (§ Formal Definition, condition (d)), and the
only value that can ever equal that on-chain is the accumulator `x/sovereignty/preblock.go`'s
`CommitFSMTransition` computes step-by-step from real, consensus-committed blocks (see
`HeaderHistory.Accumulator` in that file). A prover can't shortcut the chain by fabricating
intermediate headers, because doing so changes the accumulator's terminal hash, and
`SubmitRecoveryProof` (`x/sovereignty/keeper/msg_server.go`) only accepts an `rt_new` matching a
value the chain itself produced. Full in-circuit binding to CometBFT commit signatures and the
validator-set hash remains future work — see § Appendix: In-Circuit Commit-Signature Verification.

## Formal Definition: Proof of Recovery
- `SOVEREIGN`-mode blocks carry only local-PoS security, not Bitcoin-anchored finality.
- Returning to `ANCHORED` needs the proof described above, without re-verifying any transaction — keeping constraints low for $O(1)$ verification and constant proof size.
- Built with Noir/UltraHonk (`circuit/reanchoring/src/main.nr`), SHA-256 as the chaining hash (§ Why SHA-256, Not Poseidon2), standard knowledge soundness.
- Public input $x = (rt_{last}, rt_{new}, n)$; witness $w = (H_{k+1}, \dots, H_{k+n})$ — lightweight headers, not transaction bodies.
- $\Phi_{Recovery}(x, w) = 1$ iff every header $i \in [n]$ satisfies:

| Condition | Constraint | Guarantee |
|---|---|---|
| (a) Continuity | $H_{k+i+1}$.`prev_hash` $= \mathrm{hash\_header}(H_{k+i})$ | No injected or reordered blocks |
| (b) Policy | $H_{k+i}$.`fsm_state` $\in \{\texttt{SOVEREIGN},\texttt{RECOVERING}\}$ | No illegal jump to `ANCHORED` |
| (c) Circuit breaker | $H_{k+i}$.`withdrawal_locked` $= \mathrm{true}$ | No withdrawal during reduced security |
| (d) Root binding | $H_{k+1}$.`prev_hash` $=rt_{last}$, $\mathrm{hash\_header}(H_{k+n}) = rt_{new}$ | Both endpoints bound to the on-chain accumulator, not a free witness value |

Condition (d)'s `rt_new` side binds to the hash *output*, not a free field inside the last header
— see § Scope above for why that distinction is what makes the endpoints unforgeable. `state_root`
inside each header is CometBFT's own per-block `AppHash`, reduced into the scalar field
(`x/sovereignty/types.ReduceToField`); `hash_header`'s output (the accumulator) is a different
value from any individual header's `state_root` field.

$n$ (`count` in the circuit) is bounded by a compile-time ceiling $N_{max} = 256$ — see §1 for the padding design and its cost trade-off. Longer intervals chain a rolling sequence of proofs, each starting where the last left off (`SubmitRecoveryProof`, `x/sovereignty/keeper/msg_server.go`, which also enforces `count` equals the real number of headers between the two checkpoints via `LastAnchoredHeight`).

## Why SHA-256, Not Poseidon2

The circuit's chaining hash (`hash_header`) is SHA-256, not Poseidon2, despite Poseidon2 being far
cheaper to arithmetize. Reason: Poseidon2 at the state width this circuit needs (t=4 over BN254)
has no independently-verified Go implementation — `gnark-crypto`'s BN254 Poseidon2 package
explicitly supports only t=2,3, and the Horizen Labs reference implementation (the source most
Poseidon2 instantiations, including this circuit's original one, derive from) has a documented
history of bugs specifically in the t=4 external linear layer. The Go keeper
(`x/sovereignty/types.HashHeader`, called every block from `CommitFSMTransition`) must reproduce
the circuit's hash bit-for-bit; hand-porting an unverified t=4 Poseidon2 into Go for that purpose
was judged an unacceptable correctness risk for a consensus-critical value.

SHA-256 has no tunable parameters, so Go's `crypto/sha256` and Noir's `sha256::digest`
(`noir-lang/sha256`) are guaranteed to agree — cross-verified byte-for-byte
(`x/sovereignty/types.TestHashHeader_MatchesCircuitFixture`) before this circuit shipped with it.
The cost is real and large (§1's measurements): ACIR opcode count rose from 6,143 to 500,242 (~81x)
and prove time from ~1.05s to ~53.6s. Recovery proofs are submitted only during a rare
SOVEREIGN/RECOVERING interval, not on the consensus hot path, so this is judged an acceptable
trade for removing an unverified cryptographic primitive from a security-critical path. Revisiting
Poseidon2 (via a verified implementation, or a WASM-embedded call into Barretenberg's own
native Poseidon2 rather than a from-scratch Go port) is a legitimate future optimization once a
byte-for-byte-verified path exists.

## 1. Production Design: Dynamic Padding (N_MAX=256)
- Single circuit (`circuit/reanchoring/src/main.nr`), fixed capacity of 256 headers.
- A runtime witness `count` (1..=256) plus padding logic picks how many of the 256 slots are real vs. padding — no recompiling per N.
- Replaces the old approach of sweeping `global N` at compile time: cost is now constant regardless of the real interval length.

### Performance Results
Measured on nargo 1.0.0-beta.22 and bb 5.0.0-nightly.20260522, SHA-256 chaining hash (§ Why SHA-256, Not Poseidon2) via `scripts/e6_zk_reanchoring_benchmark/benchmark_prover.sh` — constant cost regardless of `count`:

| count | Execute (witness) | Prove Time | Verify Time | Proof Size | Public Inputs |
| :--- | :--- | :--- | :--- | :--- | :--- |
| 1 | 3.02s | 48.8s | 25ms | 14,656 B | 96 B |
| 4 | 3.03s | 48.1s | 25ms | 14,656 B | 96 B |
| 16 | 3.03s | 49.8s | 24ms | 14,656 B | 96 B |
| 64 | 3.13s | 48.0s | 23ms | 14,656 B | 96 B |
| 256 (N_MAX) | 3.23s | 49.4s | 25ms | 14,656 B | 96 B |

**Circuit stats:**
- 500,242 ACIR opcodes, 3,511,201 circuit size — same for every `count` (same compiled circuit; only witness data differs). Up from 6,143 opcodes / 47,613 circuit size under the prior Poseidon2 chaining hash (§ Why SHA-256, Not Poseidon2's trade-off).
- VK is byte-identical across all measured `count` values.
- Public inputs: 96 bytes, `rt_last‖rt_new‖count` (`SubmitRecoveryProof` in `x/sovereignty/keeper/msg_server.go` parses all three, including `count`, which it cross-checks against the real interval length via `LastAnchoredHeight`).

### Overhead of Dynamic Padding Itself (independent of the chaining hash)
Measured under the circuit's prior Poseidon2 chaining hash, isolating the padding-logic cost from
the hash-function trade-off (§ Why SHA-256, Not Poseidon2, measured separately above):

| Metric | Fixed-N | Dynamic Padding | Overhead |
| :--- | :--- | :--- | :--- |
| ACIR opcodes | 3,071 | 6,143 | ~2.0x |
| Circuit size | 39,298 | 47,613 | ~1.2x |
| Prove Time | 0.684s | 1.059s | ~1.5x |
| Verify Time | 22ms | 22.8ms | ~flat |
| Proof size | 14,656 B | 14,656 B | same |

The overhead comes from padding-gate logic (`active` boolean + OR-gate per assertion) and the dynamic `headers[count-1]` index (a 256-way selector chain) — unrelated to, and additive with, the SHA-256 switch's much larger cost.

### Rationale
- **Constant cost:** predictable performance for validators.
- **Full flexibility:** proves any 1–256 header interval immediately — no waiting to accumulate an exact or minimum count.
- **Liveness:** closes the E2 S7 race (an interval outgrowing a small fixed N's coverage, `docs/EXPERIMENT.md:105-107`) and the trailing-remainder problem (a segment shorter than a fixed N could never be proven at all).

## 2. Research Spike: Recursive Aggregation
Investigated aggregating multiple small proofs recursively:
- Leaf circuit (`circuit/reanchoring_recursion_spike/`) — a byte-for-byte copy of the deployed N=4 `reanchoring` circuit, proven with `bb prove --verifier_target noir-recursive`.
- Aggregator (`circuit/reanchoring_recursion_spike_aggregator/`) — verifies M leaf proofs via `verify_honk_proof`, leaf VK pinned as a compile-time `global` constant (never a witness), chaining `rt_last`/`rt_new` across leaves.
- `bb_proof_verification` pinned to `v5.0.0-nightly.20260522` (matching the locally installed `bb` build) — compiles and links cleanly.
- Aggregator exposes 2 public Fields (`rt_start`, `rt_end`) — the same 64-byte shape the pre-padding `SubmitRecoveryProof` hardcoded.
- Result: a significant performance penalty (see below).

### Comparative Performance (Direct vs. Recursive)

| M (leaves) | Headers covered | ACIR opcodes | Circuit size | Prove time | Verify time | Proof size |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 1 | 4  | 58 | 703,582   | ~191s¹ | 24ms | 14,656 B |
| 2 | 8  | 85 | 1,469,553 | 74s    | 60ms | 14,656 B |
| 4 | 16 | 89 | 3,001,406 | 175s   | 20ms | 14,656 B |

¹ Likely inflated by one-time CRS (structured reference string) growth — `~/.bb-crs/bn254_g1.dat` was extended mid-run, since the larger circuit needs more SRS points than the cache had been sized for; M=2/M=4 then reused an already-larger cache. Reported as-measured; doesn't change the conclusion below.

Direct hashing vs. recursive aggregation, same header count:

| Headers | Direct Hashing (Prove / Gates) | Recursive Aggregate (Prove / Gates) | Speed Gap |
| :--- | :--- | :--- | :--- |
| **8** | 0.113s / 1,106 | 74.0s / 1,469,553 | **~650x** |
| **16** | 0.146s / 2,338 | 175.0s / 3,001,406 | **~1,200x** |

**Result:** aggregation is ~1,200x slower and ~1,300x larger at the 16-header scale.

### Technical Insight
* **Structural failure:** `verify_honk_proof`'s in-circuit cost (pairing/commitment checks for a whole inner UltraHonk proof) dwarfs the ~12-opcode cost of hashing one header — recursion pays that cost per leaf.
* **Wrong tool for the job:** recursion suits proofs that are themselves expensive to re-verify (e.g. rollups), not cheap items like header hashes.
* **Infeasible at scale:** matching N_MAX=256 needs ~M=64 leaves; the circuit-size trend (~doubling per doubling of M) puts that in the hundreds-of-millions range — infeasible on validator hardware. Spike touched no Go code or production circuit (`circuit/reanchoring/`, `circuit/reanchoring_witness/`, `x/sovereignty/keeper/zk_assets/vk` unmodified); §1's dynamic padding is the shipped fix for the same E2 S7 gap.

## 3. Verification and Integration
- **Noir:** 9 test scenarios in `circuit/reanchoring/` (count=1/4/130/256 satisfy the circuit; count=0, count>N_MAX, wrong `fsm_state`, a padding-cannot-extend-real-coverage attack, and a fabricated-single-header rt_new-forgery attempt all correctly `should_fail`), plus a hash cross-check against the real circuit's fixture in `circuit/reanchoring_witness/`.
- **Go:** `x/sovereignty/types.TestHashHeader_MatchesCircuitFixture` cross-checks the Go accumulator implementation against the Noir circuit's own fixture, byte-for-byte.
- **Go benchmarks:** `BenchmarkVerifyZKProof` ~19.2ms/proof, run against the regenerated embedded VK.
- **Full repo:** `go build ./... && go vet ./... && go test ./...` clean.
- **End-to-end:** validated through experiment E2 S7 (recovery flow).

## Deterministic Verifier

`x/sovereignty/keeper/reanchor.go`'s `VerifyZKProof` runs entirely inside the `engramd` process: no
external `bb` binary, no PATH dependency, no per-validator checksum pinning. It calls
`verifyZKProofWasm` (`x/sovereignty/keeper/zk_verifier.go`), which loads an embedded
`wasm32-wasi` build of Barretenberg's UltraHonk verifier
(`zk_assets/barretenberg.wasm.gz`) through [`wazero`](https://wazero.io), a pure-Go WASM
runtime — the same wasm bytecode, interpreted by the same runtime, produces the identical result
on any validator's OS/CPU.

The `.wasm` is built by [engram-barretenberg-wasm](https://github.com/cuongct924/engram-barretenberg-wasm),
a minimal patched subset of upstream Barretenberg compiled directly to `wasm32-wasi` (not
`bb.js`/Emscripten — see that repo's README for why, and for the real bugs found wiring this up:
the WASI reactor `_initialize` pattern, prover/verifier toolchain version skew, and on-demand SRS
initialization). It is pinned to the exact `aztec-packages` commit the pinned `bb` CLI
(`BB_VERSION` in this repo's `Dockerfile`) was built from — Barretenberg's UltraHonk proof/VK
binary layout is not guaranteed stable across commits, so the prover (`bb prove`, used by
`scripts/reanchoring_prover/prove_and_submit.sh`) and this embedded verifier must always share one
toolchain build; moving `BB_VERSION` requires re-pinning and rebuilding the WASM asset to match.

`x/sovereignty/keeper/reanchor_test.go`'s `TestVerifyZKProof_RealProof` cross-validates the
embedded verifier against a real proof generated by that same pinned `bb` release, accepting the
genuine proof and rejecting a single-byte-tampered copy of it.

## Appendix: In-Circuit Commit-Signature Verification (Not Implemented)

The strongest possible binding of a recovery proof to real CometBFT history would verify each
witness header's originating block commit — its `BlockID`, the `>=2/3` validator signatures over
it, and the validator-set hash — inside the circuit itself, rather than relying on the accumulator
construction described in § Scope. This was evaluated and set aside: verifying one ed25519
signature costs on the order of thousands of Noir constraints, multiplied by every signing
validator (`>=2f+1`) for every header in the interval — for `N_MAX=256` headers and even a small
validator set, this pushes the circuit into a scale that has not been benchmarked here and is
plausibly infeasible to prove on validator-class hardware, in addition to being a substantial
circuit-engineering undertaking in its own right (canonical vote sign-bytes reconstruction,
batch signature verification, a validator-set Merkle/hash commitment scheme in-circuit).

The accumulator-based binding (§ Scope) achieves the same practical security property — a prover
cannot produce a valid `rt_new` without knowing the real header content for the entire interval,
because only real consensus (`CommitFSMTransition`) can ever advance the on-chain accumulator to a
value `SubmitRecoveryProof` will accept — without that added constraint cost. Revisiting full
in-circuit signature verification remains a legitimate research direction if a stronger,
execution-correctness-level guarantee is later required.

Last updated: August 31, 2026.
