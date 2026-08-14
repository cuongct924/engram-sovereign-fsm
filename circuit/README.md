# Engram Re-anchoring: ZK Proof System & Research Benchmarks

This document summarizes the development, performance results, and architectural decisions for the Zero-Knowledge (ZK) proof system used in Engram re-anchoring.

## Overview
The ZK system proves the validity of a blockchain header chain to recover the system state. We evaluated two primary architectures:
1. Dynamic Padding (Production choice).
2. Recursive Aggregation (Research spike - Rejected).

## 1. Production Design: Dynamic Padding (N_MAX=256)
The system uses a single circuit with a fixed maximum capacity of 256 headers. The actual number of headers to prove is provided at runtime using a witness count and padding logic.

### Performance Results
Measured on nargo 1.0.0-beta.22 and bb 5.0.0-nightly.20260522. Costs are constant regardless of the actual header count.

| Header Count | Prove Time | Verify Time | Proof Size | Public Inputs |
| :--- | :--- | :--- | :--- | :--- |
| 1 | 1.054s | 23.5ms | 14,656 B | 96 B |
| 4 | 1.065s | 22.2ms | 14,656 B | 96 B |
| 130 | 0.993s | 21.9ms | 14,656 B | 96 B |
| 256 (N_MAX) | 1.059s | 22.8ms | 14,656 B | 96 B |

**Circuit Stats:** 6,143 ACIR opcodes and 47,613 circuit size.

### Comparison to Old Design (Fixed-N=256)
The new design costs measurable overhead to gain runtime flexibility:

| Metric | Old (Fixed-N) | New (Dynamic) | Overhead |
| :--- | :--- | :--- | :--- |
| ACIR opcodes | 3,071 | 6,143 | ~2.0x |
| Circuit size | 39,298 | 47,613 | ~1.2x |
| Prove Time | 0.684s | 1.059s | ~1.5x |
| Verify Time | 22ms | 22.8ms | ~flat |
| Proof size | 14,656 B | 14,656 B | same |

### Rationale
This design replaces compile-time scaling. It provides:
- Constant Costs: Predictable performance for validators.
- Full Flexibility: Proves any interval from 1 to 256 headers immediately.
- Liveness: Solves the "trailing remainder" problem where intervals smaller than a fixed size could never be proven.

## 2. Research Spike: Recursive Aggregation
We investigated recursive aggregation to combine multiple small proofs. The results showed a significant performance penalty for this specific use case.

### Comparative Performance (Direct vs. Recursive)
The table below compares proving the same number of headers using direct hashing (Production) versus recursive aggregation (Spike):

| Headers | Direct Hashing (Prove / Gates) | Recursive Aggregate (Prove / Gates) | Speed Gap |
| :--- | :--- | :--- | :--- |
| **8** | 0.113s / 1,106 | 74.0s / 1,469,553 | **~650x** |
| **16** | 0.146s / 2,338 | 175.0s / 3,001,406 | **~1,200x** |

**Result:** Aggregation is ~1,200x slower and ~1,300x larger at the 16-header scale.

### Technical Insight
* **Structural Failure:** The ~700k-gate overhead of in-circuit verification (pairings/commitments) vastly outweighs the 12-opcode cost of header hashing.
* **Inefficient Trade-off:** Recursion is ideal for aggregating complex proofs (e.g., Rollups), but inferior to direct inclusion for inexpensive tasks like hashing.

## 3. Verification and Integration
The **ZK Proof System** is verified across multiple levels:
- Noir: Passed 8 test scenarios covering edge cases and attack vectors.
- Go Benchmarks: `BenchmarkVerifyZKProof` confirmed at ~19.2ms per proof.
- End-to-End: Successfully validated through experiment E2 S7 (recovery flow).

Last updated: August 13, 2026.
