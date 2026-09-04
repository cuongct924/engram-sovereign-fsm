**Table 6A -- Circuit Composition (measured, real nargo/bb):**

| Component | Cost model | Fit |
| --- | --- | --- |
| Fixed overhead (root binding + Honk setup) | -1.0 ACIR opcodes | R^2=1.0000 |
| Marginal cost per header (continuity + FSM legality + withdrawal lock) | 12.00 ACIR opcodes/header | linear fit across N=4..256 |

| N (headers) | ACIR opcodes | Honk circuit size (padded) |
| ---: | ---: | ---: |
| 4 | 47 | 490 |
| 8 | 95 | 1106 |
| 16 | 191 | 2338 |
| 32 | 383 | 4802 |
| 64 | 767 | 9730 |
| 128 | 1535 | 19586 |
| 256 | 3071 | 39298 |

**Table 6B -- Scaling Benchmark (measured, real nargo/bb, UltraHonk backend):**

| Sovereign Blocks (N) | ACIR Opcodes | Compile (s) | Prove (s) | Verify (ms) | Proof Size | Blocks/s |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 4 | 47 | 0.206 | 0.130 | 33.0 | 14656 B | 30.8 |
| 8 | 95 | 0.210 | 0.113 | 22.0 | 14656 B | 70.8 |
| 16 | 191 | 0.213 | 0.146 | 22.0 | 14656 B | 109.6 |
| 32 | 383 | 0.215 | 0.187 | 23.0 | 14656 B | 171.1 |
| 64 | 767 | 0.228 | 0.274 | 22.0 | 14656 B | 233.6 |
| 128 | 1535 | 0.242 | 0.410 | 22.0 | 14656 B | 312.2 |
| 256 | 3071 | 0.268 | 0.684 | 22.0 | 14656 B | 374.3 |
