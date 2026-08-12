# LIVE E5 spot-check: HysteresisWait=2, env=noisy_da

repeated docker stop/start celestia-bridge cycling, proven mechanism from live_lifecycle_test.py. Duration: 300s, polling interval: 3s, WAN-realism baseline: chaos-wan-latency.

**Granularity caveat:** metrics below are computed from fixed-interval polling, not per-block state -- see this script's module doc.

| Node | Samples | Total Transitions | Flapping Count | Anchored Uptime |
|---|---:|---:|---:|---:|
| engram-node01 | 85 | 14 | 13 | 15.29% |
| engram-node02 | 85 | 14 | 13 | 15.29% |
| engram-node03 | 85 | 14 | 13 | 15.29% |
| engram-node04 | 85 | 14 | 13 | 15.29% |
