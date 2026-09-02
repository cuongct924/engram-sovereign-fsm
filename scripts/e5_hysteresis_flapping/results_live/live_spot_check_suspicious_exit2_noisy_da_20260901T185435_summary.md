# LIVE E5 absorb-edge spot-check: edge=suspicious_exit, value=2, env=noisy_da

down (5b): ~80% healthy baseline, ~20% warning blips. suspicious_exit (5c): ~80% warning baseline, ~20% healthy blips -- inverted from down, matching suspicious_exit_sweep_test.go's actual model.. Duration: 300s, polling interval: 3s, WAN-realism baseline: chaos-wan-latency.

**Granularity caveat:** metrics below are computed from fixed-interval polling, not per-block state -- see this script's module doc.

| Node | Samples | Total Transitions | Flapping Count | Anchored Uptime | Exit Count | Max Suspicious Duration |
|---|---:|---:|---:|---:|---:|---:|
| engram-node01 | 45 | 3 | 1 | 4.44% | 1 | 11 |
| engram-node02 | 45 | 3 | 1 | 4.44% | 1 | 11 |
| engram-node03 | 45 | 3 | 1 | 4.44% | 1 | 11 |
| engram-node04 | 45 | 3 | 1 | 4.44% | 1 | 11 |
| engram-node05 | 45 | 3 | 1 | 4.44% | 1 | 11 |
| engram-node06 | 45 | 3 | 1 | 4.44% | 1 | 11 |
| engram-node07 | 45 | 3 | 1 | 4.44% | 1 | 11 |
