# LIVE Timeout-flooding test

docs/EXPERIMENT.md's E8 "Timeout flooding by Byzantine nodes" row -- node04 stays alive and honest everywhere else, but actively floods genuinely signed Timeout messages every 50ms (engram-consensus-core's timeoutFloodRoutine), instead of the crash-only (`chaos-crash`) approximation used previously.

Total duration: 128s.

## Verdict

- Safety held (3 honest nodes' AppHash never diverged): **True**
- Divergence events: 0
- Cadence held (flood-phase height rate >= 50% of baseline): **True**
  - baseline: 0.597 blocks/s
  - flood:    0.576 blocks/s
  - recovery: 0.509 blocks/s


## Per-peer rate limiter (reactor.go's PeerState.allowTimeoutMessage)

Real "rate limit exceeded" drop count on each honest node, counted from a live `docker logs -f` tail spanning the flood phase (not an inference):

- engram-node01: 0 drops
- engram-node02: 0 drops
- engram-node03: 0 drops

## Resource usage (docker stats, CPU%/memory)

| Node | Phase | Avg CPU% | Max CPU% | Avg Mem (MiB) | Max Mem (MiB) |
|---|---|---:|---:|---:|---:|
| engram-node01 | baseline | 2.11 | 3.28 | 88.0 | 88.4 |
| engram-node02 | baseline | 2.47 | 3.70 | 83.6 | 84.4 |
| engram-node03 | baseline | 2.41 | 4.66 | 83.9 | 84.0 |
| engram-node04 | baseline | 2.68 | 4.13 | 54.3 | 54.4 |
| engram-node01 | flood | 3.32 | 6.26 | 88.0 | 88.2 |
| engram-node02 | flood | 3.80 | 7.20 | 84.2 | 84.5 |
| engram-node03 | flood | 4.00 | 8.04 | 84.8 | 86.2 |
| engram-node04 | flood | 5.10 | 8.05 | 53.8 | 54.4 |
| engram-node01 | recovery | 2.02 | 3.41 | 90.0 | 90.2 |
| engram-node02 | recovery | 1.85 | 2.82 | 84.3 | 84.5 |
| engram-node03 | recovery | 1.90 | 3.48 | 86.1 | 86.2 |
| engram-node04 | recovery | 4.51 | 14.01 | 64.6 | 72.0 |
