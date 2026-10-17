# E2 live throughput/latency (block-interval proxy)

Mean/p50/p95 seconds between consecutive height increments, averaged across all validators per scenario (p95 is the max across nodes). Fixed-interval RPC polling, not per-round consensus timing -- a throughput proxy, not a true latency percentile.

| Scenario | Mean (s) | p50 (s) | p95 (s) |
|---|---:|---:|---:|
| s1 | 1.27 | 1.52 | 1.52 |
| s2 | 1.34 | 1.03 | 2.05 |
| s3 | 1.35 | 1.03 | 2.06 |
| s4 | 1.65 | 1.55 | 2.68 |
| s5 | 1.39 | 1.33 | 2.50 |
| s6 | 1.31 | 1.41 | 1.46 |
| s7 | 1.27 | 1.10 | 1.69 |
