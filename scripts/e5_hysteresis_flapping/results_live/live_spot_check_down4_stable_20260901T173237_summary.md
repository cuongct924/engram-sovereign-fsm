# LIVE E5 absorb-edge spot-check: edge=down, value=4, env=stable

down (5b): no injected noise, 100% healthy control. suspicious_exit (5c): sustained warning, 0% healthy blips -- the opposite control (system must exhaust MaxSuspiciousTime).. Duration: 300s, polling interval: 3s, WAN-realism baseline: chaos-wan-latency.

**Granularity caveat:** metrics below are computed from fixed-interval polling, not per-block state -- see this script's module doc.

| Node | Samples | Total Transitions | Flapping Count | Anchored Uptime | Time Outside Anchored | Demotion Count |
|---|---:|---:|---:|---:|---:|---:|
| engram-node01 | 89 | 6 | 5 | 96.63% | 3 | 3 |
| engram-node02 | 89 | 6 | 5 | 96.63% | 3 | 3 |
| engram-node03 | 89 | 6 | 5 | 96.63% | 3 | 3 |
| engram-node04 | 89 | 6 | 5 | 96.63% | 3 | 3 |
| engram-node05 | 89 | 10 | 9 | 94.38% | 5 | 5 |
| engram-node06 | 89 | 8 | 7 | 95.51% | 4 | 4 |
| engram-node07 | 89 | 10 | 9 | 94.38% | 5 | 5 |
