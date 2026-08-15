# LIVE E5 absorb-edge spot-check: edge=down, value=6, env=noisy_da

Tier 1 fields (time_outside_anchored/demotion_count or exit_count/max_suspicious_duration) backfilled from already-collected samples in `scripts/e5_hysteresis_flapping/results_live/live_spot_check_down6_20260815T135335.csv` -- no new Docker run.

**Granularity caveat:** metrics below are computed from fixed-interval polling, not per-block state -- see this script's module doc.

| Node | Samples | Total Transitions | Flapping Count | Anchored Uptime | Time Outside Anchored | Demotion Count |
|---|---:|---:|---:|---:|---:|---:|
| engram-node01 | 96 | 13 | 12 | 36.46% | 61 | 7 |
| engram-node02 | 96 | 13 | 12 | 36.46% | 61 | 7 |
| engram-node03 | 96 | 13 | 12 | 36.46% | 61 | 7 |
| engram-node04 | 96 | 13 | 12 | 36.46% | 61 | 7 |
