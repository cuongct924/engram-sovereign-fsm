# LIVE E5 absorb-edge spot-check: edge=down, value=6, env=noisy_da

down (5b): ~80% healthy baseline, ~20% warning blips. suspicious_exit (5c): ~80% warning baseline, ~20% healthy blips -- inverted from down, matching suspicious_exit_sweep_test.go's actual model.. Duration: 300s, polling interval: 3s, WAN-realism baseline: chaos-wan-latency.

**Granularity caveat:** metrics below are computed from fixed-interval polling, not per-block state -- see this script's module doc.

| Node | Samples | Total Transitions | Flapping Count | Anchored Uptime | Time Outside Anchored | Demotion Count |
|---|---:|---:|---:|---:|---:|---:|
| engram-node01 | 69 | 17 | 4 | 24.64% | 52 | 6 |
| engram-node02 | 69 | 17 | 4 | 24.64% | 52 | 6 |
| engram-node03 | 69 | 17 | 4 | 24.64% | 52 | 6 |
| engram-node04 | 69 | 16 | 4 | 26.09% | 51 | 6 |
| engram-node05 | 69 | 16 | 4 | 24.64% | 52 | 6 |
| engram-node06 | 69 | 16 | 4 | 24.64% | 52 | 6 |
| engram-node07 | 69 | 16 | 4 | 24.64% | 52 | 6 |
