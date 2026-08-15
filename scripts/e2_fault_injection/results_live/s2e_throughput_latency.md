# E2 live throughput/latency (block-interval proxy)

Mean/p50/p95 seconds between consecutive height increments, averaged across all 4 validators per scenario (p95 is the max across nodes). Fixed-interval RPC polling, not per-round consensus timing -- a throughput proxy, not a true latency percentile.

| Scenario | Mean (s) | p50 (s) | p95 (s) |
|---|---:|---:|---:|
| s1 | 1.39 | 1.52 | 1.53 |
| s2 | 1.38 | 1.51 | 2.05 |
| s3 | 1.36 | 1.51 | 2.07 |
| s4 | 2.51 | 2.03 | 6.09 |
| s5 | 1.29 | 1.30 | 2.07 |
| s6 | 1.34 | 1.52 | 1.57 |
| s7 | 1.50 | 1.78 | 2.05 |
