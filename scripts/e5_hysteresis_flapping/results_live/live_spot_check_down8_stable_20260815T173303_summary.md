# LIVE E5 absorb-edge spot-check: edge=down, value=8, env=stable

down (5b): no injected noise, 100% healthy control. suspicious_exit (5c): sustained warning, 0% healthy blips -- the opposite control (system must exhaust MaxSuspiciousTime).. Duration: 300s, polling interval: 3s, WAN-realism baseline: chaos-wan-latency.

**Granularity caveat:** metrics below are computed from fixed-interval polling, not per-block state -- see this script's module doc.

| Node | Samples | Total Transitions | Flapping Count | Anchored Uptime | Time Outside Anchored | Demotion Count |
|---|---:|---:|---:|---:|---:|---:|
| engram-node01 | 99 | 0 | 0 | 100.00% | 0 | 0 |
| engram-node02 | 99 | 0 | 0 | 100.00% | 0 | 0 |
| engram-node03 | 99 | 0 | 0 | 100.00% | 0 | 0 |
| engram-node04 | 99 | 0 | 0 | 100.00% | 0 | 0 |
