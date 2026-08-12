# LIVE Timeout-flooding test

docs/EXPERIMENT.md's E8 "Timeout flooding by Byzantine nodes" row -- node04 stays alive and honest everywhere else, but actively floods genuinely signed Timeout messages every 2ms (engram-consensus-core's timeoutFloodRoutine), instead of the crash-only (`chaos-crash`) approximation used previously.

Total duration: 136s.

## Verdict

- Safety held (3 honest nodes' AppHash never diverged): **True**
- Divergence events: 0
- Cadence held (flood-phase height rate >= 50% of baseline): **True**
  - baseline: 0.586 blocks/s
  - flood:    0.813 blocks/s
  - recovery: 0.744 blocks/s


## Per-peer rate limiter (reactor.go's PeerState.allowTimeoutMessage)

Real "rate limit exceeded" drop count on each honest node, counted from `docker logs --since` the flood phase started (not an inference):

- engram-node01: 26563 drops
- engram-node02: 27675 drops
- engram-node03: 28777 drops

## Resource usage (docker stats, CPU%/memory)

| Node | Phase | Avg CPU% | Max CPU% | Avg Mem (MiB) | Max Mem (MiB) |
|---|---|---:|---:|---:|---:|
| engram-node01 | baseline | 3.13 | 4.76 | 84.7 | 84.7 |
| engram-node02 | baseline | 2.79 | 3.91 | 86.6 | 86.6 |
| engram-node03 | baseline | 2.71 | 4.37 | 86.1 | 86.1 |
| engram-node04 | baseline | 2.22 | 3.10 | 57.1 | 57.1 |
| engram-node01 | flood | 6.86 | 9.41 | 86.2 | 87.4 |
| engram-node02 | flood | 6.40 | 8.44 | 88.0 | 89.0 |
| engram-node03 | flood | 6.84 | 9.28 | 87.8 | 88.6 |
| engram-node04 | flood | 19.01 | 25.60 | 64.7 | 68.5 |
| engram-node01 | recovery | 2.07 | 3.65 | 87.5 | 87.7 |
| engram-node02 | recovery | 1.90 | 2.62 | 88.8 | 88.8 |
| engram-node03 | recovery | 1.72 | 3.03 | 88.7 | 88.7 |
| engram-node04 | recovery | 3.38 | 11.02 | 45.3 | 59.2 |
