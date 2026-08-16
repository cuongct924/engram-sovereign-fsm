# LIVE Timeout-flooding test

docs/EXPERIMENT.md's E8 "Timeout flooding by Byzantine nodes" row -- node04 stays alive and honest everywhere else, but actively floods genuinely signed Timeout messages every 50ms (engram-consensus-core's timeoutFloodRoutine), instead of the crash-only (`chaos-crash`) approximation used previously.

Total duration: 133s.

## Verdict

- Safety held (3 honest nodes' AppHash never diverged): **True**
- Divergence events: 0
- Cadence held (flood-phase height rate >= 50% of baseline): **True**
  - baseline: 0.728 blocks/s
  - flood:    0.737 blocks/s
  - recovery: 0.627 blocks/s


## Per-peer rate limiter (reactor.go's PeerState.allowTimeoutMessage)

Real "rate limit exceeded" drop count on each honest node, counted from a live `docker logs -f` tail spanning the flood phase (not an inference):

- engram-node01: 0 drops
- engram-node02: 0 drops
- engram-node03: 0 drops

## Resource usage (docker stats, CPU%/memory)

| Node | Phase | Avg CPU% | Max CPU% | Avg Mem (MiB) | Max Mem (MiB) |
|---|---|---:|---:|---:|---:|
| engram-node01 | baseline | 2.60 | 4.53 | 83.8 | 84.0 |
| engram-node02 | baseline | 2.76 | 3.38 | 79.7 | 79.8 |
| engram-node03 | baseline | 2.72 | 3.85 | 81.8 | 82.0 |
| engram-node04 | baseline | 2.63 | 3.68 | 80.0 | 80.9 |
| engram-node01 | flood | 3.48 | 4.40 | 84.0 | 86.1 |
| engram-node02 | flood | 3.44 | 5.45 | 80.9 | 82.1 |
| engram-node03 | flood | 3.26 | 4.54 | 81.9 | 83.8 |
| engram-node04 | flood | 5.07 | 7.78 | 73.1 | 76.7 |
| engram-node01 | recovery | 3.43 | 7.99 | 87.5 | 87.9 |
| engram-node02 | recovery | 4.09 | 10.88 | 82.1 | 82.2 |
| engram-node03 | recovery | 2.20 | 2.95 | 83.8 | 83.9 |
| engram-node04 | recovery | 2.98 | 6.12 | 53.0 | 53.9 |
