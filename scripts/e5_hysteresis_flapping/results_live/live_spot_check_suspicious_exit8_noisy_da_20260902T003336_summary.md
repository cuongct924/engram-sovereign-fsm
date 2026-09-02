# LIVE E5 absorb-edge spot-check: edge=suspicious_exit, value=8, env=noisy_da

down (5b): ~80% healthy baseline, ~20% warning blips. suspicious_exit (5c): ~80% warning baseline, ~20% healthy blips -- inverted from down, matching suspicious_exit_sweep_test.go's actual model.. Duration: 300s, polling interval: 3s, WAN-realism baseline: chaos-wan-latency.

**Granularity caveat:** metrics below are computed from fixed-interval polling, not per-block state -- see this script's module doc.

| Node | Samples | Total Transitions | Flapping Count | Anchored Uptime | Exit Count | Max Suspicious Duration |
|---|---:|---:|---:|---:|---:|---:|
| engram-node01 | 64 | 4 | 2 | 0.00% | 0 | 23 |
| engram-node02 | 64 | 4 | 2 | 0.00% | 0 | 23 |
| engram-node03 | 64 | 4 | 2 | 0.00% | 0 | 23 |
| engram-node04 | 64 | 4 | 2 | 0.00% | 0 | 23 |
| engram-node05 | 64 | 4 | 2 | 0.00% | 0 | 23 |
| engram-node06 | 64 | 4 | 2 | 0.00% | 0 | 23 |
| engram-node07 | 64 | 4 | 2 | 0.00% | 0 | 23 |
