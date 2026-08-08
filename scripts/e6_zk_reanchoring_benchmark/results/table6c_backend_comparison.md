**Table 6C -- Backend Comparison (measured, N=256 sovereign blocks):**

Noir+Honk measures circuit/reanchoring/src/main.nr (Pedersen header-chain continuity, real nargo+bb pipeline, table6b_scaling.csv). Plonky3 measures the same dominant cost driver -- N chained Poseidon2 permutations -- via Plonky3's own first-party benchmark example (prove_prime_field_31, pinned commit a31a1443a114c58735850daa5b5fc5c43c138d9d), NOT a hand-rolled reimplementation of main.nr's exact header struct; see benchmark_plonky3.sh's header comment for why. The two circuits are therefore not bit-identical, but both isolate the same cost driver (one hash invocation per header) that dominates constraint count on the Noir side (table6a_6b.md's regression). Trusted setup / PQ secure are well-established properties of the underlying commitment scheme (KZG pairing-based for UltraHonk, FRI hash-based for Plonky3), not a per-run measurement -- documented here as qualitative facts, not fabricated numbers. Recursion support is left as a qualitative note, not scored: Barretenberg ships documented recursive-UltraHonk-verification support (used elsewhere in this repo's own toolchain notes); this specific Plonky3 checkout (0.6.0-era, no dedicated recursion crate/example found in its own README/CHANGELOG at the pinned commit) does not ship a ready-made recursive-verifier example to measure against, so no claim is made about its recursion maturity relative to Barretenberg's.

| Metric | Noir + Honk | Plonky3 (Poseidon2/FRI) |
| --- | ---: | ---: |
| Proof size | 14,656 B | 1,278,939 B |
| Verify time | 23.0 ms | 32.2 ms |
| Prove time | 4.136 s | 0.044 s |
| Trusted setup | Yes (KZG/Aztec Ignition SRS) | No (FRI, transparent) |
| PQ secure | No (elliptic-curve pairings) | Yes (hash-based FRI) |
| Recursion support | Documented (Barretenberg recursive-UltraHonk) | Not evaluated at this checkout (see note above) |

**Full N-sweep (both backends, real measurements):**

| N | Noir prove (s) | Noir verify (ms) | Noir proof (B) | Plonky3 prove (s) | Plonky3 verify (ms) | Plonky3 proof (B) |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 8 | 0.412 | 21.0 | 14,656 | 0.033 | 16.5 | 1,202,590 |
| 16 | 0.491 | 20.0 | 14,656 | 0.032 | 16.7 | 1,258,957 |
| 32 | 0.675 | 20.0 | 14,656 | 0.011 | 17.7 | 1,264,381 |
| 64 | 1.107 | 21.0 | 14,656 | 0.030 | 25.2 | 1,271,491 |
| 128 | 2.216 | 26.0 | 14,656 | 0.023 | 25.3 | 1,273,344 |
| 256 | 4.136 | 23.0 | 14,656 | 0.044 | 32.2 | 1,278,939 |
