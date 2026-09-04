# LIVE E7 throughput/latency A/B under real load

Real MsgSubmitForcedTxRequest load, 20.0 tx/s target, 60s x 5 repeats per mode. `extended` is measured against the real 7-validator engram-validator-cluster (already running, the same deployment E2/E4/E5/E6's data comes from); `vanilla` is a local single `engramd --vanilla` process this script owns -- a deliberate simplification (a second full 7-node vanilla cluster is a separate infra undertaking), so treat `vanilla` as a lower bound on overhead-free throughput, not a strict cluster-vs-cluster comparison. `blocks_per_sec` and `mean/p50/p95 interval` measure different things (module docstring) -- `steady_blocks_per_sec` (=1/mean interval) and `gap_fraction` reconcile them explicitly below.

## Aggregate across repeats (mean ± 95% CI)

| Mode | n | Blocks/s | Steady blocks/s (1/mean interval) | Gap fraction | Tx-accepted/s | p95 interval (s) |
|---|---:|---:|---:|---:|---:|---:|
| extended | 5 | 0.743 ± 0.008 | 0.993 ± 0.023 | 0.251 ± 0.016 | 19.99 ± 0.00 | 1.021 ± 0.006 |
| vanilla | 5 | 0.953 ± 0.007 | 1.000 ± 0.000 | 0.046 ± 0.006 | 20.00 ± 0.00 | 1.017 ± 0.001 |

## Per-repeat detail

| Mode | Repeat | Sent | Accepted | Blocks/s | Steady blocks/s | Gap fraction | Tx-accepted/s | Mean interval (s) | p50 (s) | p95 (s) |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| extended | 1 | 1200 | 1200 | 0.733 | 1.002 | 0.269 | 20.00 | 0.998 | 0.999 | 1.016 |
| extended | 2 | 1200 | 1200 | 0.750 | 0.971 | 0.228 | 20.00 | 1.030 | 1.002 | 1.028 |
| extended | 3 | 1200 | 1200 | 0.750 | 1.019 | 0.264 | 19.99 | 0.981 | 1.001 | 1.030 |
| extended | 4 | 1200 | 1200 | 0.750 | 1.014 | 0.260 | 19.99 | 0.987 | 1.000 | 1.014 |
| extended | 5 | 1200 | 1200 | 0.733 | 0.959 | 0.235 | 20.00 | 1.043 | 1.001 | 1.018 |
| vanilla | 1 | 1200 | 1200 | 0.966 | 1.000 | 0.033 | 20.00 | 1.000 | 0.999 | 1.015 |
| vanilla | 2 | 1200 | 1200 | 0.950 | 0.999 | 0.049 | 20.00 | 1.001 | 1.003 | 1.017 |
| vanilla | 3 | 1200 | 1200 | 0.950 | 1.000 | 0.050 | 20.00 | 1.000 | 1.000 | 1.019 |
| vanilla | 4 | 1200 | 1200 | 0.950 | 1.000 | 0.050 | 20.00 | 1.000 | 0.999 | 1.016 |
| vanilla | 5 | 1200 | 1200 | 0.950 | 0.999 | 0.050 | 20.00 | 1.001 | 1.001 | 1.016 |

## Effect size (Cohen's d, extended vs. vanilla, pooled stdev)

Only meaningful between two repeat groups of matching n -- the one place E7 has a real paired
comparison; not computed for any other metric.

| Rate (tx/s) | Blocks/s, d | Gap fraction, d |
|---:|---:|---:|
| 20.0 | -25.3 | 14.6 |

Both far exceed conventional "large" (|d|=0.8) -- consistent with the tight per-repeat CI already
shown above (extended's blocks/s range 0.733-0.750 across all 5 repeats never overlaps vanilla's
0.950-0.966), so the gap is a stable, repeatable effect, not sampling noise.
