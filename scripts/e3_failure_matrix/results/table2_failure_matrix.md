**Table 2 -- Failure Matrix (measured, real tests/e2e data, DefaultParams thresholds):**

| Scenario | BTC | DA | P2P | Observed FSM state | Withdrawals | Block production |
| --- | --- | --- | --- | --- | --- | --- |
| S1 Normal | healthy | healthy | healthy | ANCHORED | enabled | continuous (50 blocks, 0 halts) |
| S2 BTC congestion (settled) | healthy | healthy | healthy | ANCHORED | enabled | continuous (12 blocks, 0 halts) |
| S3 DA unavailable (settled) | healthy | healthy | healthy | ANCHORED | enabled | continuous (8 blocks, 0 halts) |
| S4 P2P eclipse partial (settled) | healthy | healthy | unhealthy | SOVEREIGN | locked | continuous (4 blocks, 0 halts) |
| S5 Anchor isolation (settled) | healthy | healthy | unhealthy | SOVEREIGN | locked | continuous (2 blocks, 0 halts) |
| S6 Combined BTC+DA failure (settled) | critical | failed | healthy | SOVEREIGN | locked | continuous (10 blocks, 0 halts) |
| S7 Recovery (settled) | healthy | healthy | healthy | ANCHORED | enabled | continuous (4 blocks, 0 halts) |
