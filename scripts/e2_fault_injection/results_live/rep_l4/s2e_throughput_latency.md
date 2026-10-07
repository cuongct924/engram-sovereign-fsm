# E2 live throughput/latency (block-interval proxy)

Mean/p50/p95 seconds between consecutive height increments, averaged across all validators per scenario (p95 is the max across nodes). Fixed-interval RPC polling, not per-round consensus timing -- a throughput proxy, not a true latency percentile.

| Scenario | Mean (s) | p50 (s) | p95 (s) |
|---|---:|---:|---:|
| s1 | 1.42 | 1.54 | 1.63 |
| s2 | 1.41 | 1.04 | 2.12 |
| s3 | 1.45 | 1.05 | 2.17 |
| s4 | 1.50 | 1.09 | 2.19 |
| s5 | 1.36 | 1.52 | 1.56 |
| s6 | 1.30 | 1.52 | 1.54 |
| s7 | 1.54 | 1.14 | 2.32 |
