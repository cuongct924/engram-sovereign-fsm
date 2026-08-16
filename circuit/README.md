# Engram Re-anchoring: ZK Proof System & Research Benchmarks

Development, results, and design decisions for Engram re-anchoring's ZK proof system.

## Overview
The ZK system proves a blockchain header chain is valid, to recover system state. Two architectures were evaluated:
1. Dynamic Padding (production choice).
2. Recursive Aggregation (research spike — rejected).

## Formal Definition: Proof of Recovery
- `SOVEREIGN`-mode blocks carry only local-PoS security, not Bitcoin-anchored finality.
- Returning to `ANCHORED` needs a **Proof of Recovery**, not a Proof of Execution: it proves the sovereign interval kept continuity and FSM recovery policy, without re-verifying any transaction — keeping constraints low for $O(1)$ verification and constant proof size.
- Built with Noir/UltraHonk (`circuit/reanchoring/src/main.nr`), Poseidon2 as random oracle, standard knowledge soundness.
- Public input $x = (rt_{last}, rt_{new}, n)$; witness $w = (H_{k+1}, \dots, H_{k+n})$ — lightweight headers, not transaction bodies.
- $\Phi_{Recovery}(x, w) = 1$ iff every header $i \in [n]$ satisfies:

| Condition | Constraint | Guarantee |
|---|---|---|
| (a) Continuity | $H_{k+i+1}.\mathrm{prev\_hash} = \mathrm{Poseidon2}(H_{k+i})$ | No injected or reordered blocks |
| (b) Policy | $H_{k+i}.\mathrm{fsm\_state} \in \{\texttt{SOVEREIGN},\texttt{RECOVERING}\}$ | No illegal jump to `ANCHORED` |
| (c) Circuit breaker | $H_{k+i}.\mathrm{withdrawal\_locked} = \mathrm{true}$ | No withdrawal during reduced security |
| (d) Root binding | $H_{k+1}.\mathrm{old\_state\_root}=rt_{last}, H_{k+n}.\mathrm{new\_state\_root}=rt_{new}$ | New root bound to the validated chain |

`state_root` is CometBFT's own per-block `AppHash`; $\Phi_{Recovery}$ just needs some Merkle-rooted commitment to bind against.

$n$ (`count` in the circuit) is bounded by a compile-time ceiling $N_{max} = 256$ — see §1 for the padding design and its cost trade-off. Longer intervals chain a rolling sequence of proofs, each starting where the last left off (`SubmitRecoveryProof`, `x/sovereignty/keeper/msg_server.go`).

## 1. Production Design: Dynamic Padding (N_MAX=256)
- Single circuit (`circuit/reanchoring/src/main.nr`), fixed capacity of 256 headers.
- A runtime witness `count` (1..=256) plus padding logic picks how many of the 256 slots are real vs. padding — no recompiling per N.
- Replaces the old approach of sweeping `global N` at compile time: cost is now constant regardless of the real interval length.

### Performance Results
Measured on nargo 1.0.0-beta.22 and bb 5.0.0-nightly.20260522 — constant cost regardless of header count:

| Header Count | Prove Time | Verify Time | Proof Size | Public Inputs |
| :--- | :--- | :--- | :--- | :--- |
| 1 | 1.054s | 23.5ms | 14,656 B | 96 B |
| 4 | 1.065s | 22.2ms | 14,656 B | 96 B |
| 130 | 0.993s | 21.9ms | 14,656 B | 96 B |
| 256 (N_MAX) | 1.059s | 22.8ms | 14,656 B | 96 B |

**Circuit stats:**
- 6,143 ACIR opcodes, 47,613 circuit size — same for every `count` (same compiled circuit; only witness data differs).
- VK is byte-identical across all four `count` values (`diff`-confirmed).
- Public inputs: 96 bytes, `rt_last‖rt_new‖count` (widened from the old fixed-N design's 64 bytes; `SubmitRecoveryProof` in `x/sovereignty/keeper/msg_server.go` updated to match).

### Comparison to Old Design (Fixed-N=256)
Overhead paid for runtime flexibility:

| Metric | Old (Fixed-N) | New (Dynamic) | Overhead |
| :--- | :--- | :--- | :--- |
| ACIR opcodes | 3,071 | 6,143 | ~2.0x |
| Circuit size | 39,298 | 47,613 | ~1.2x |
| Prove Time | 0.684s | 1.059s | ~1.5x |
| Verify Time | 22ms | 22.8ms | ~flat |
| Proof size | 14,656 B | 14,656 B | same |

The overhead comes from padding-gate logic (`active` boolean + OR-gate per assertion) and the dynamic `headers[count-1]` index (a 256-way selector chain).

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
- **Noir:** 8 test scenarios in `circuit/reanchoring/` (count=1/4/130/256 satisfy the circuit; count=0, count>N_MAX, wrong `fsm_state`, and a padding-cannot-extend-real-coverage attack all correctly `should_fail`), plus a hash cross-check against the real circuit's fixture in `circuit/reanchoring_witness/`.
- **Go benchmarks:** `BenchmarkVerifyZKProof` ~19.2ms/proof, run against the regenerated embedded VK.
- **Full repo:** `go build ./... && go vet ./... && go test ./...` clean after the 96-byte public-input layout update.
- **End-to-end:** validated through experiment E2 S7 (recovery flow).

Last updated: August 16, 2026.
