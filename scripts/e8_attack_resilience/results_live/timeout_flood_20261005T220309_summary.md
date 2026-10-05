# LIVE Timeout-flooding test

docs/EXPERIMENT.md's E8 "Timeout flooding by Byzantine nodes" row -- node04 stays alive and honest everywhere else, but actively floods genuinely signed Timeout messages every 50ms (engram-consensus-core's timeoutFloodRoutine), instead of the crash-only (`chaos-crash`) approximation used previously.

Total duration: 130s.

## Verdict

- Safety held (3 honest nodes' AppHash never diverged): **True**
- Divergence events: 0
- Cadence held (flood-phase height rate >= 50% of baseline): **True**
  - baseline: 0.791 blocks/s
  - flood:    0.791 blocks/s
  - recovery: 0.786 blocks/s


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
| engram-node01 | baseline | 4.62 | 5.30 | 424.9 | 425.1 |
| engram-node02 | baseline | 4.17 | 4.68 | 394.3 | 394.4 |
| engram-node03 | baseline | 4.38 | 5.28 | 447.9 | 448.0 |
| engram-node04 | baseline | 4.39 | 5.53 | 82.5 | 82.8 |
| engram-node05 | baseline | 4.16 | 4.80 | 416.1 | 418.2 |
| engram-node06 | baseline | 4.53 | 4.85 | 498.9 | 499.3 |
| engram-node07 | baseline | 6.08 | 15.05 | 438.2 | 438.2 |
| engram-node01 | flood | 5.44 | 18.17 | 424.9 | 425.1 |
| engram-node02 | flood | 4.23 | 5.34 | 396.4 | 396.4 |
| engram-node03 | flood | 4.19 | 5.53 | 448.0 | 448.3 |
| engram-node04 | flood | 5.62 | 6.52 | 66.7 | 73.0 |
| engram-node05 | flood | 4.36 | 5.94 | 416.1 | 416.2 |
| engram-node06 | flood | 4.23 | 5.53 | 498.9 | 498.9 |
| engram-node07 | flood | 4.52 | 6.01 | 438.3 | 438.5 |
| engram-node01 | recovery | 4.17 | 5.95 | 424.8 | 424.8 |
| engram-node02 | recovery | 4.03 | 5.47 | 396.4 | 396.4 |
| engram-node03 | recovery | 3.96 | 6.16 | 448.6 | 449.3 |
| engram-node04 | recovery | 5.31 | 16.25 | 71.6 | 78.8 |
| engram-node05 | recovery | 4.33 | 5.70 | 416.2 | 416.3 |
| engram-node06 | recovery | 4.02 | 6.19 | 499.0 | 499.4 |
| engram-node07 | recovery | 4.31 | 5.61 | 440.4 | 440.4 |
