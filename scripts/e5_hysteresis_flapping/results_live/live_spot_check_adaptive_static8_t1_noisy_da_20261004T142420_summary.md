# LIVE E5 absorb-edge spot-check: edge=down, value=8, env=noisy_da

down (5b): ~80% healthy baseline, ~20% warning blips. suspicious_exit (5c): ~80% warning baseline, ~20% healthy blips -- inverted from down, matching suspicious_exit_sweep_test.go's actual model.. Duration: 300s, polling interval: 3s, WAN-realism baseline: chaos-wan-latency.

**Granularity caveat:** metrics below are computed from fixed-interval polling, not per-block state -- see this script's module doc.

| Node | Samples | Total Transitions | Flapping Count | Anchored Uptime | Time Outside Anchored | Demotion Count |
|---|---:|---:|---:|---:|---:|---:|
| engram-node01 | 73 | 8 | 5 | 2.74% | 71 | 1 |
| engram-node02 | 73 | 8 | 5 | 2.74% | 71 | 1 |
| engram-node03 | 73 | 8 | 5 | 2.74% | 71 | 1 |
| engram-node04 | 73 | 8 | 5 | 2.74% | 71 | 1 |
| engram-node05 | 73 | 8 | 5 | 2.74% | 71 | 1 |
| engram-node06 | 73 | 8 | 5 | 2.74% | 71 | 1 |
| engram-node07 | 73 | 8 | 5 | 2.74% | 71 | 1 |
