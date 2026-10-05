# LIVE Timeout-flooding test

docs/EXPERIMENT.md's E8 "Timeout flooding by Byzantine nodes" row -- node04 stays alive and honest everywhere else, but actively floods genuinely signed Timeout messages every 50ms (engram-consensus-core's timeoutFloodRoutine), instead of the crash-only (`chaos-crash`) approximation used previously.

Total duration: 137s.

## Verdict

- Safety held (3 honest nodes' AppHash never diverged): **True**
- Divergence events: 0
- Cadence held (flood-phase height rate >= 50% of baseline): **True**
  - baseline: 0.816 blocks/s
  - flood:    0.783 blocks/s
  - recovery: 0.801 blocks/s


## Per-peer rate limiter (reactor.go's PeerState.allowTimeoutMessage)

Real "rate limit exceeded" drop count on each honest node, counted from a live `docker logs -f` tail spanning the flood phase (not an inference):

- engram-node01: 0 drops
- engram-node02: 0 drops
- engram-node03: 0 drops
- engram-node05: 0 drops
- engram-node06: 0 drops
- engram-node07: 0 drops

## Resource usage (docker stats, CPU%/memory)

| Node | Phase | Avg CPU% | Max CPU% | Avg Mem (MiB) | Max Mem (MiB) |
|---|---|---:|---:|---:|---:|
| engram-node01 | baseline | 3.80 | 4.99 | 508.3 | 508.3 |
| engram-node02 | baseline | 3.92 | 4.52 | 511.0 | 511.1 |
| engram-node03 | baseline | 4.19 | 4.98 | 503.2 | 513.3 |
| engram-node04 | baseline | 4.20 | 4.74 | 394.8 | 394.9 |
| engram-node05 | baseline | 3.26 | 4.36 | 489.0 | 489.1 |
| engram-node06 | baseline | 3.54 | 4.24 | 559.5 | 559.6 |
| engram-node07 | baseline | 3.79 | 4.39 | 516.6 | 516.7 |
| engram-node01 | flood | 4.47 | 5.74 | 508.6 | 509.1 |
| engram-node02 | flood | 4.65 | 6.17 | 524.5 | 546.5 |
| engram-node03 | flood | 4.36 | 5.66 | 498.1 | 498.1 |
| engram-node04 | flood | 6.25 | 7.79 | 68.9 | 74.3 |
| engram-node05 | flood | 4.33 | 5.44 | 484.2 | 484.5 |
| engram-node06 | flood | 4.38 | 5.63 | 561.2 | 563.5 |
| engram-node07 | flood | 6.04 | 26.48 | 525.5 | 540.6 |
| engram-node01 | recovery | 4.04 | 5.24 | 512.9 | 512.9 |
| engram-node02 | recovery | 4.27 | 5.47 | 546.3 | 546.5 |
| engram-node03 | recovery | 4.17 | 5.42 | 498.1 | 498.1 |
| engram-node04 | recovery | 7.44 | 20.51 | 75.1 | 82.8 |
| engram-node05 | recovery | 4.05 | 6.56 | 484.6 | 484.7 |
| engram-node06 | recovery | 4.62 | 5.49 | 563.7 | 563.9 |
| engram-node07 | recovery | 5.12 | 10.62 | 540.6 | 540.7 |
