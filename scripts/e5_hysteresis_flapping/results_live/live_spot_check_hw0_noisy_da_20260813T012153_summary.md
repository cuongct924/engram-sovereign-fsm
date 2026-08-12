# LIVE E5 spot-check: HysteresisWait=0, env=noisy_da

repeated docker stop/start celestia-bridge cycling, proven mechanism from live_lifecycle_test.py. Duration: 300s, polling interval: 3s, WAN-realism baseline: chaos-wan-latency.

**Granularity caveat:** metrics below are computed from fixed-interval polling, not per-block state -- see this script's module doc.

| Node | Samples | Total Transitions | Flapping Count | Anchored Uptime |
|---|---:|---:|---:|---:|
| engram-node01 | 94 | 17 | 4 | 11.70% |
| engram-node02 | 94 | 17 | 4 | 11.70% |
| engram-node03 | 94 | 17 | 4 | 11.70% |
| engram-node04 | 94 | 17 | 4 | 11.70% |
