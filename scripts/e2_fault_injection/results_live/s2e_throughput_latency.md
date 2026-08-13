# E2 live throughput/latency (block-interval proxy)

Mean/p50/p95 seconds between consecutive height increments, averaged across all 4 validators per scenario (p95 is the max across nodes). Fixed-interval RPC polling, not per-round consensus timing -- a throughput proxy, not a true latency percentile.

| Scenario | Mean (s) | p50 (s) | p95 (s) |
|---|---:|---:|---:|
| s1 | 1.39 | 1.52 | 1.52 |
| s2 | 2.01 | 1.52 | 3.04 |
| s3 | 1.35 | 1.51 | 2.03 |
| s4 | 2.14 | 1.64 | 5.51 |
| s5 | 1.31 | 1.52 | 1.54 |
| s6 | n/a | n/a | n/a |
| s7 | n/a | n/a | n/a |
