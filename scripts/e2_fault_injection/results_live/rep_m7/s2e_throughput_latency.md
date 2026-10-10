# E2 live throughput/latency (block-interval proxy)

Mean/p50/p95 seconds between consecutive height increments, averaged across all validators per scenario (p95 is the max across nodes). Fixed-interval RPC polling, not per-round consensus timing -- a throughput proxy, not a true latency percentile.

| Scenario | Mean (s) | p50 (s) | p95 (s) |
|---|---:|---:|---:|
| s1 | 1.39 | 1.52 | 1.53 |
| s2 | 1.34 | 1.02 | 2.05 |
| s3 | 1.34 | 1.02 | 2.05 |
| s4 | 1.48 | 1.02 | 2.48 |
| s5 | 1.28 | 1.32 | 2.54 |
| s6 | 1.35 | 1.52 | 1.53 |
| s7 | 1.37 | 1.02 | 2.06 |
