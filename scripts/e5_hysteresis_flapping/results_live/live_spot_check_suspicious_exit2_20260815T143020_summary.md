# LIVE E5 absorb-edge spot-check: edge=suspicious_exit, value=2, env=noisy_da

Tier 1 fields (time_outside_anchored/demotion_count or exit_count/max_suspicious_duration) backfilled from already-collected samples in `scripts/e5_hysteresis_flapping/results_live/live_spot_check_suspicious_exit2_20260815T143020.csv` -- no new Docker run.

**Granularity caveat:** metrics below are computed from fixed-interval polling, not per-block state -- see this script's module doc.

| Node | Samples | Total Transitions | Flapping Count | Anchored Uptime | Exit Count | Max Suspicious Duration |
|---|---:|---:|---:|---:|---:|---:|
| engram-node01 | 96 | 14 | 13 | 36.46% | 7 | 22 |
| engram-node02 | 96 | 14 | 13 | 36.46% | 7 | 22 |
| engram-node03 | 96 | 14 | 13 | 37.50% | 7 | 22 |
| engram-node04 | 96 | 14 | 13 | 37.50% | 7 | 22 |
