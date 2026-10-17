# E2 live throughput/latency (block-interval proxy)

Mean/p50/p95 seconds between consecutive height increments, averaged across all validators per scenario (p95 is the max across nodes). Fixed-interval RPC polling, not per-round consensus timing -- a throughput proxy, not a true latency percentile.

| Scenario | Mean (s) | p50 (s) | p95 (s) |
|---|---:|---:|---:|
| s1 | 1.27 | 1.52 | 1.53 |
| s2 | 1.33 | 1.03 | 2.06 |
| s3 | 1.40 | 1.03 | 2.06 |
| s4 | 1.71 | 1.52 | 4.23 |
| s5 | 1.37 | 1.37 | 2.25 |
| s6 | 1.30 | 1.40 | 1.44 |
| s7 | 1.27 | 1.10 | 1.69 |
