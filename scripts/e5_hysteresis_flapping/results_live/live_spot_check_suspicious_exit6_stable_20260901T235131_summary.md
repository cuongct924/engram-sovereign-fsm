# LIVE E5 absorb-edge spot-check: edge=suspicious_exit, value=6, env=stable

down (5b): no injected noise, 100% healthy control. suspicious_exit (5c): sustained warning, 0% healthy blips -- the opposite control (system must exhaust MaxSuspiciousTime).. Duration: 300s, polling interval: 3s, WAN-realism baseline: chaos-wan-latency.

**Granularity caveat:** metrics below are computed from fixed-interval polling, not per-block state -- see this script's module doc.

| Node | Samples | Total Transitions | Flapping Count | Anchored Uptime | Exit Count | Max Suspicious Duration |
|---|---:|---:|---:|---:|---:|---:|
| engram-node01 | 99 | 1 | 0 | 0.00% | 0 | 24 |
| engram-node02 | 99 | 1 | 0 | 0.00% | 0 | 24 |
| engram-node03 | 99 | 1 | 0 | 0.00% | 0 | 24 |
| engram-node04 | 99 | 1 | 0 | 0.00% | 0 | 24 |
| engram-node05 | 99 | 1 | 0 | 0.00% | 0 | 24 |
| engram-node06 | 99 | 1 | 0 | 0.00% | 0 | 24 |
| engram-node07 | 99 | 1 | 0 | 0.00% | 0 | 24 |
