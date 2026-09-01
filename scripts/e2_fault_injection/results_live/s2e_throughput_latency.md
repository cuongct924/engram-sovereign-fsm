# E2 live throughput/latency (block-interval proxy)

Mean/p50/p95 seconds between consecutive height increments, averaged across all validators per scenario (p95 is the max across nodes). Fixed-interval RPC polling, not per-round consensus timing -- a throughput proxy, not a true latency percentile.

| Scenario | Mean (s) | p50 (s) | p95 (s) |
|---|---:|---:|---:|
| s1 | 1.34 | 1.52 | 1.53 |
| s2 | 1.39 | 1.52 | 2.05 |
| s3 | 1.42 | 1.19 | 3.05 |
| s4 | 1.89 | 1.81 | 4.27 |
| s5 | 1.44 | 1.52 | 3.04 |
| s6 | 1.30 | 1.52 | 1.54 |
| s7 | 1.93 | 2.00 | 3.10 |
