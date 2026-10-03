# E2 live throughput/latency (block-interval proxy)

Mean/p50/p95 seconds between consecutive height increments, averaged across all validators per scenario (p95 is the max across nodes). Fixed-interval RPC polling, not per-round consensus timing -- a throughput proxy, not a true latency percentile.

| Scenario | Mean (s) | p50 (s) | p95 (s) |
|---|---:|---:|---:|
| s1 | 1.27 | 1.52 | 1.53 |
| s2 | 1.46 | 1.28 | 2.48 |
| s3 | 1.73 | 2.04 | 2.09 |
| s4 | 2.19 | 2.22 | 4.80 |
| s5 | 1.33 | 1.12 | 2.05 |
| s6 | 1.32 | 1.52 | 1.54 |
| s7 | 1.85 | 2.05 | 2.36 |
