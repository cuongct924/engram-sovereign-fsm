# LIVE E5 absorb-edge spot-check: edge=suspicious_exit, value=8, env=noisy_da

Tier 1 fields (time_outside_anchored/demotion_count or exit_count/max_suspicious_duration) backfilled from already-collected samples in `scripts/e5_hysteresis_flapping/results_live/live_spot_check_suspicious_exit8_20260815T150628.csv` -- no new Docker run.

**Granularity caveat:** metrics below are computed from fixed-interval polling, not per-block state -- see this script's module doc.

| Node | Samples | Total Transitions | Flapping Count | Anchored Uptime | Exit Count | Max Suspicious Duration |
|---|---:|---:|---:|---:|---:|---:|
| engram-node01 | 76 | 12 | 0 | 17.11% | 0 | 24 |
| engram-node02 | 76 | 12 | 0 | 17.11% | 0 | 24 |
| engram-node03 | 76 | 12 | 0 | 17.11% | 0 | 24 |
| engram-node04 | 76 | 12 | 0 | 17.11% | 0 | 24 |
