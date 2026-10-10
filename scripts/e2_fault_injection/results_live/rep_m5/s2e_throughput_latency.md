# E2 live throughput/latency (block-interval proxy)

Mean/p50/p95 seconds between consecutive height increments, averaged across all validators per scenario (p95 is the max across nodes). Fixed-interval RPC polling, not per-round consensus timing -- a throughput proxy, not a true latency percentile.

| Scenario | Mean (s) | p50 (s) | p95 (s) |
|---|---:|---:|---:|
| s1 | 1.27 | 1.53 | 1.54 |
| s2 | 1.36 | 1.04 | 2.07 |
| s3 | 1.38 | 1.04 | 2.09 |
| s4 | 1.45 | 1.04 | 2.09 |
| s5 | 1.28 | 1.52 | 1.56 |
| s6 | 1.36 | 1.52 | 1.53 |
| s7 | 1.39 | 1.04 | 2.09 |
