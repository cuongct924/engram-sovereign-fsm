# LIVE Timeout-flooding test

docs/EXPERIMENT.md's E8 "Timeout flooding by Byzantine nodes" row -- node04 stays alive and honest everywhere else, but actively floods genuinely signed Timeout messages every 2ms (engram-consensus-core's timeoutFloodRoutine), instead of the crash-only (`chaos-crash`) approximation used previously.

Total duration: 141s.

## Verdict

- Safety held (3 honest nodes' AppHash never diverged): **True**
- Divergence events: 0
- Cadence held (flood-phase height rate >= 50% of baseline): **True**
  - baseline: 0.825 blocks/s
  - flood:    0.789 blocks/s
  - recovery: 0.849 blocks/s


## Per-peer rate limiter (reactor.go's PeerState.allowTimeoutMessage)

Real "rate limit exceeded" drop count on each honest node, counted from `docker logs --since` the flood phase started (not an inference):

- engram-node01: 28071 drops
- engram-node02: 29238 drops
- engram-node03: 30376 drops

## Resource usage (docker stats, CPU%/memory)

| Node | Phase | Avg CPU% | Max CPU% | Avg Mem (MiB) | Max Mem (MiB) |
|---|---|---:|---:|---:|---:|
| engram-node01 | baseline | 2.42 | 4.55 | 78.7 | 79.7 |
| engram-node02 | baseline | 3.26 | 5.56 | 87.4 | 89.7 |
| engram-node03 | baseline | 2.71 | 3.76 | 79.4 | 82.2 |
| engram-node04 | baseline | 2.65 | 4.82 | 72.6 | 72.9 |
| engram-node01 | flood | 7.85 | 12.72 | 82.7 | 84.1 |
| engram-node02 | flood | 8.97 | 12.61 | 91.5 | 92.2 |
| engram-node03 | flood | 9.51 | 22.40 | 83.7 | 84.5 |
| engram-node04 | flood | 23.26 | 28.47 | 63.6 | 68.7 |
| engram-node01 | recovery | 3.08 | 4.66 | 84.1 | 84.1 |
| engram-node02 | recovery | 3.72 | 5.32 | 92.1 | 92.3 |
| engram-node03 | recovery | 3.05 | 4.37 | 84.1 | 84.3 |
| engram-node04 | recovery | 4.28 | 9.75 | 67.3 | 69.8 |
