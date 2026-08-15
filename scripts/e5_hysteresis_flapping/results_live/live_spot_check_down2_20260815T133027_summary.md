# LIVE E5 absorb-edge spot-check: edge=down, value=2, env=noisy_da

Tier 1 fields (time_outside_anchored/demotion_count or exit_count/max_suspicious_duration) backfilled from already-collected samples in `scripts/e5_hysteresis_flapping/results_live/live_spot_check_down2_20260815T133027.csv` -- no new Docker run.

**Granularity caveat:** metrics below are computed from fixed-interval polling, not per-block state -- see this script's module doc.

| Node | Samples | Total Transitions | Flapping Count | Anchored Uptime | Time Outside Anchored | Demotion Count |
|---|---:|---:|---:|---:|---:|---:|
| engram-node01 | 71 | 7 | 4 | 4.23% | 68 | 2 |
| engram-node02 | 71 | 7 | 4 | 4.23% | 68 | 2 |
| engram-node03 | 71 | 7 | 4 | 4.23% | 68 | 2 |
| engram-node04 | 71 | 8 | 4 | 5.63% | 67 | 2 |
