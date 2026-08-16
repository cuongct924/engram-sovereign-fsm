# LIVE Timeout-flooding test

docs/EXPERIMENT.md's E8 "Timeout flooding by Byzantine nodes" row -- node04 stays alive and honest everywhere else, but actively floods genuinely signed Timeout messages every 50ms (engram-consensus-core's timeoutFloodRoutine), instead of the crash-only (`chaos-crash`) approximation used previously.

Total duration: 132s.

## Verdict

- Safety held (3 honest nodes' AppHash never diverged): **True**
- Divergence events: 0
- Cadence held (flood-phase height rate >= 50% of baseline): **True**
  - baseline: 0.743 blocks/s
  - flood:    0.587 blocks/s
  - recovery: 0.701 blocks/s


## Per-peer rate limiter (reactor.go's PeerState.allowTimeoutMessage)

Real "rate limit exceeded" drop count on each honest node, counted from a live `docker logs -f` tail spanning the flood phase (not an inference):

- engram-node01: 0 drops
- engram-node02: 0 drops
- engram-node03: 0 drops

## Resource usage (docker stats, CPU%/memory)

| Node | Phase | Avg CPU% | Max CPU% | Avg Mem (MiB) | Max Mem (MiB) |
|---|---|---:|---:|---:|---:|
| engram-node01 | baseline | 2.53 | 4.14 | 75.9 | 77.6 |
| engram-node02 | baseline | 2.51 | 3.28 | 69.5 | 69.5 |
| engram-node03 | baseline | 2.79 | 7.11 | 71.5 | 71.5 |
| engram-node04 | baseline | 2.12 | 2.76 | 69.4 | 69.4 |
| engram-node01 | flood | 3.53 | 4.57 | 77.6 | 77.7 |
| engram-node02 | flood | 3.88 | 5.93 | 72.7 | 73.6 |
| engram-node03 | flood | 4.00 | 7.22 | 73.1 | 75.7 |
| engram-node04 | flood | 5.36 | 8.33 | 60.6 | 64.1 |
| engram-node01 | recovery | 1.94 | 3.58 | 77.7 | 78.0 |
| engram-node02 | recovery | 2.80 | 4.74 | 73.7 | 73.9 |
| engram-node03 | recovery | 2.00 | 2.63 | 75.7 | 75.9 |
| engram-node04 | recovery | 2.12 | 5.23 | 51.0 | 54.1 |
