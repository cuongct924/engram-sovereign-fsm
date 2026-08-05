**Table 6A -- Circuit Composition (measured, real nargo/bb):**

| Component | Cost model | Fit |
| --- | --- | --- |
| Fixed overhead (root binding + Honk setup) | -32.0 ACIR opcodes | R^2=1.0000 |
| Marginal cost per header (continuity + FSM legality + withdrawal lock) | 42.00 ACIR opcodes/header | linear fit across N=4..256 |

| N (headers) | ACIR opcodes | Honk circuit size (padded) |
| ---: | ---: | ---: |
| 4 | 136 | 28680 |
| 8 | 304 | 28680 |
| 16 | 640 | 28680 |
| 32 | 1312 | 35977 |
| 64 | 2656 | 70217 |
| 128 | 5344 | 138697 |
| 256 | 10720 | 275657 |

**Table 6B -- Scaling Benchmark (measured, real nargo/bb, UltraHonk backend):**

| Sovereign Blocks (N) | ACIR Opcodes | Compile (s) | Prove (s) | Verify (ms) | Proof Size | Blocks/s |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 4 | 136 | 0.217 | 0.380 | 22.0 | 14656 B | 10.5 |
| 8 | 304 | 0.219 | 0.412 | 21.0 | 14656 B | 19.4 |
| 16 | 640 | 0.224 | 0.491 | 20.0 | 14656 B | 32.6 |
| 32 | 1312 | 0.239 | 0.675 | 20.0 | 14656 B | 47.4 |
| 64 | 2656 | 0.272 | 1.107 | 21.0 | 14656 B | 57.8 |
| 128 | 5344 | 0.335 | 2.216 | 26.0 | 14656 B | 57.8 |
| 256 | 10720 | 0.476 | 4.136 | 23.0 | 14656 B | 61.9 |
