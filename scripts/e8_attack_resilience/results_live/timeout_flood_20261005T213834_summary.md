# LIVE Timeout-flooding test

docs/EXPERIMENT.md's E8 "Timeout flooding by Byzantine nodes" row -- node04 stays alive and honest everywhere else, but actively floods genuinely signed Timeout messages every 50ms (engram-consensus-core's timeoutFloodRoutine), instead of the crash-only (`chaos-crash`) approximation used previously.

Total duration: 143s.

## Verdict

- Safety held (3 honest nodes' AppHash never diverged): **True**
- Divergence events: 0
- Cadence held (flood-phase height rate >= 50% of baseline): **True**
  - baseline: 0.754 blocks/s
  - flood:    0.785 blocks/s
  - recovery: 0.714 blocks/s


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
| engram-node01 | baseline | 3.71 | 4.92 | 383.2 | 383.4 |
| engram-node02 | baseline | 3.45 | 4.82 | 372.3 | 372.4 |
| engram-node03 | baseline | 3.36 | 5.18 | 413.4 | 413.7 |
| engram-node04 | baseline | 3.81 | 5.51 | 87.9 | 87.9 |
| engram-node05 | baseline | 2.91 | 4.28 | 406.1 | 406.2 |
| engram-node06 | baseline | 3.12 | 4.48 | 469.8 | 469.8 |
| engram-node07 | baseline | 4.03 | 4.87 | 419.9 | 420.2 |
| engram-node01 | flood | 5.32 | 15.81 | 383.4 | 383.5 |
| engram-node02 | flood | 4.30 | 6.68 | 376.4 | 377.1 |
| engram-node03 | flood | 4.29 | 7.99 | 413.5 | 413.5 |
| engram-node04 | flood | 6.57 | 8.89 | 66.9 | 71.8 |
| engram-node05 | flood | 4.46 | 9.17 | 407.5 | 408.3 |
| engram-node06 | flood | 4.78 | 8.92 | 469.8 | 469.8 |
| engram-node07 | flood | 4.12 | 6.99 | 419.9 | 419.9 |
| engram-node01 | recovery | 4.11 | 4.85 | 383.5 | 383.5 |
| engram-node02 | recovery | 4.16 | 5.15 | 377.3 | 377.4 |
| engram-node03 | recovery | 4.02 | 5.19 | 413.5 | 413.5 |
| engram-node04 | recovery | 3.35 | 4.18 | 61.9 | 77.5 |
| engram-node05 | recovery | 4.03 | 4.89 | 408.1 | 408.1 |
| engram-node06 | recovery | 6.18 | 17.07 | 469.8 | 470.1 |
| engram-node07 | recovery | 4.00 | 4.69 | 420.0 | 420.4 |
