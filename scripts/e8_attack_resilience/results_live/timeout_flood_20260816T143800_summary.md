# LIVE Timeout-flooding test

docs/EXPERIMENT.md's E8 "Timeout flooding by Byzantine nodes" row -- node04 stays alive and honest everywhere else, but actively floods genuinely signed Timeout messages every 50ms (engram-consensus-core's timeoutFloodRoutine), instead of the crash-only (`chaos-crash`) approximation used previously.

Total duration: 133s.

## Verdict

- Safety held (3 honest nodes' AppHash never diverged): **True**
- Divergence events: 0
- Cadence held (flood-phase height rate >= 50% of baseline): **True**
  - baseline: 0.626 blocks/s
  - flood:    0.800 blocks/s
  - recovery: 0.703 blocks/s


## Per-peer rate limiter (reactor.go's PeerState.allowTimeoutMessage)

Real "rate limit exceeded" drop count on each honest node, counted from a live `docker logs -f` tail spanning the flood phase (not an inference):

- engram-node01: 0 drops
- engram-node02: 0 drops
- engram-node03: 0 drops

## Resource usage (docker stats, CPU%/memory)

| Node | Phase | Avg CPU% | Max CPU% | Avg Mem (MiB) | Max Mem (MiB) |
|---|---|---:|---:|---:|---:|
| engram-node01 | baseline | 4.07 | 8.60 | 380.9 | 380.9 |
| engram-node02 | baseline | 3.42 | 4.20 | 370.1 | 370.2 |
| engram-node03 | baseline | 4.57 | 9.24 | 365.8 | 365.9 |
| engram-node04 | baseline | 5.16 | 12.31 | 71.7 | 71.8 |
| engram-node01 | flood | 4.07 | 7.92 | 381.1 | 381.5 |
| engram-node02 | flood | 4.28 | 5.91 | 370.1 | 370.4 |
| engram-node03 | flood | 4.13 | 6.95 | 366.3 | 370.6 |
| engram-node04 | flood | 10.72 | 50.33 | 80.4 | 86.7 |
| engram-node01 | recovery | 5.83 | 20.50 | 381.2 | 381.3 |
| engram-node02 | recovery | 4.51 | 10.76 | 371.5 | 371.8 |
| engram-node03 | recovery | 5.57 | 18.46 | 370.6 | 371.2 |
| engram-node04 | recovery | 5.91 | 14.70 | 55.9 | 56.1 |
