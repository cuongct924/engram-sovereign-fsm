# E2 live throughput/latency (block-interval proxy)

Mean/p50/p95 seconds between consecutive height increments, averaged across all validators per scenario (p95 is the max across nodes). Fixed-interval RPC polling, not per-round consensus timing -- a throughput proxy, not a true latency percentile.

| Scenario | Mean (s) | p50 (s) | p95 (s) |
|---|---:|---:|---:|
| s1 | 1.27 | 1.53 | 1.54 |
| s2 | 1.35 | 1.03 | 2.08 |
| s3 | 1.35 | 1.04 | 2.08 |
| s4 | 1.37 | 1.04 | 2.09 |
| s5 | 1.35 | 1.04 | 2.09 |
| s6 | 1.32 | 1.52 | 1.55 |
| s7 | 1.38 | 1.03 | 2.09 |
