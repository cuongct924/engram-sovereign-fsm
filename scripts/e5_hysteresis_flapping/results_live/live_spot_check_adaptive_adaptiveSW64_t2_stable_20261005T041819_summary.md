# LIVE E5 absorb-edge spot-check: edge=down, value=2, env=stable

down (5b): no injected noise, 100% healthy control. suspicious_exit (5c): sustained warning, 0% healthy blips -- the opposite control (system must exhaust MaxSuspiciousTime).. Duration: 300s, polling interval: 3s, WAN-realism baseline: chaos-wan-latency.

**Granularity caveat:** metrics below are computed from fixed-interval polling, not per-block state -- see this script's module doc.

| Node | Samples | Total Transitions | Flapping Count | Anchored Uptime | Time Outside Anchored | Demotion Count |
|---|---:|---:|---:|---:|---:|---:|
| engram-node01 | 84 | 4 | 3 | 92.86% | 6 | 2 |
| engram-node02 | 84 | 4 | 3 | 92.86% | 6 | 2 |
| engram-node03 | 84 | 4 | 3 | 92.86% | 6 | 2 |
| engram-node04 | 84 | 4 | 3 | 92.86% | 6 | 2 |
| engram-node05 | 84 | 4 | 3 | 92.86% | 6 | 2 |
| engram-node06 | 84 | 4 | 3 | 92.86% | 6 | 2 |
| engram-node07 | 84 | 4 | 3 | 92.86% | 6 | 2 |
