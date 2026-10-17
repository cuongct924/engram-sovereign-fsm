# E2 live throughput/latency (block-interval proxy)

Mean/p50/p95 seconds between consecutive height increments, averaged across all validators per scenario (p95 is the max across nodes). Fixed-interval RPC polling, not per-round consensus timing -- a throughput proxy, not a true latency percentile.

| Scenario | Mean (s) | p50 (s) | p95 (s) |
|---|---:|---:|---:|
| s1 | 1.39 | 1.52 | 1.53 |
| s2 | 1.36 | 1.03 | 2.06 |
| s3 | 1.37 | 1.03 | 2.06 |
| s4 | 1.65 | 1.53 | 3.14 |
| s5 | 1.30 | 1.52 | 1.55 |
| s6 | 1.31 | 1.52 | 1.54 |
| s7 | 1.40 | 1.03 | 2.07 |
