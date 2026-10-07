# E2 live throughput/latency (block-interval proxy)

Mean/p50/p95 seconds between consecutive height increments, averaged across all validators per scenario (p95 is the max across nodes). Fixed-interval RPC polling, not per-round consensus timing -- a throughput proxy, not a true latency percentile.

| Scenario | Mean (s) | p50 (s) | p95 (s) |
|---|---:|---:|---:|
| s1 | 1.27 | 1.52 | 1.53 |
| s2 | 1.42 | 1.04 | 2.08 |
| s3 | 1.45 | 1.04 | 2.13 |
| s4 | 1.45 | 1.05 | 2.14 |
| s5 | 1.33 | 1.52 | 1.55 |
| s6 | 1.33 | 1.52 | 1.57 |
| s7 | 1.50 | 1.14 | 2.25 |
