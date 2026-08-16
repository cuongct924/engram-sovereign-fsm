# LIVE Timeout-flooding test

docs/EXPERIMENT.md's E8 "Timeout flooding by Byzantine nodes" row -- node04 stays alive and honest everywhere else, but actively floods genuinely signed Timeout messages every 50ms (engram-consensus-core's timeoutFloodRoutine), instead of the crash-only (`chaos-crash`) approximation used previously.

Total duration: 182s.

## Verdict

- Safety held (3 honest nodes' AppHash never diverged): **True**
- Divergence events: 0
- Cadence held (flood-phase height rate >= 50% of baseline): **True**
  - baseline: 0.815 blocks/s
  - flood:    0.569 blocks/s
  - recovery: 0.509 blocks/s


## Per-peer rate limiter (reactor.go's PeerState.allowTimeoutMessage)

Real "rate limit exceeded" drop count on each honest node, counted from `docker logs --since` the flood phase started (not an inference):

- engram-node01: -1 drops
- engram-node02: -1 drops
- engram-node03: -1 drops

## Resource usage (docker stats, CPU%/memory)

| Node | Phase | Avg CPU% | Max CPU% | Avg Mem (MiB) | Max Mem (MiB) |
|---|---|---:|---:|---:|---:|
| engram-node01 | baseline | 3.60 | 6.49 | 378.1 | 378.3 |
| engram-node02 | baseline | 3.47 | 6.37 | 369.1 | 369.3 |
| engram-node03 | baseline | 4.77 | 14.43 | 364.5 | 364.9 |
| engram-node04 | baseline | 4.31 | 11.26 | 368.4 | 368.4 |
| engram-node01 | flood | 5.54 | 27.93 | 378.2 | 378.4 |
| engram-node02 | flood | 4.13 | 7.98 | 369.1 | 369.4 |
| engram-node03 | flood | 4.24 | 7.55 | 364.4 | 364.7 |
| engram-node04 | flood | 4.41 | 6.10 | 52.9 | 63.0 |
| engram-node01 | recovery | 3.17 | 5.14 | 378.5 | 379.3 |
| engram-node02 | recovery | 2.59 | 4.03 | 369.3 | 369.4 |
| engram-node03 | recovery | 3.32 | 4.40 | 364.7 | 364.8 |
| engram-node04 | recovery | 3.94 | 7.43 | 55.7 | 62.1 |
