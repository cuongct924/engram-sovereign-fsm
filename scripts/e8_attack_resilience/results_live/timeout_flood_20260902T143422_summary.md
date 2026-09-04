# LIVE Timeout-flooding test

docs/EXPERIMENT.md's E8 "Timeout flooding by Byzantine nodes" row -- node04 stays alive and honest everywhere else, but actively floods genuinely signed Timeout messages every 50ms (engram-consensus-core's timeoutFloodRoutine), instead of the crash-only (`chaos-crash`) approximation used previously.

Total duration: 136s.

## Verdict

- Safety held (3 honest nodes' AppHash never diverged): **True**
- Divergence events: 0
- Cadence held (flood-phase height rate >= 50% of baseline): **True**
  - baseline: 0.770 blocks/s
  - flood:    0.706 blocks/s
  - recovery: 0.737 blocks/s


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
| engram-node01 | baseline | 6.20 | 9.40 | 455.9 | 456.5 |
| engram-node02 | baseline | 6.32 | 9.72 | 382.8 | 388.1 |
| engram-node03 | baseline | 6.08 | 11.20 | 443.4 | 444.3 |
| engram-node04 | baseline | 5.72 | 7.24 | 78.4 | 79.4 |
| engram-node05 | baseline | 5.40 | 7.85 | 502.6 | 503.2 |
| engram-node06 | baseline | 6.99 | 8.62 | 405.0 | 432.1 |
| engram-node07 | baseline | 8.94 | 29.41 | 417.6 | 418.5 |
| engram-node01 | flood | 9.11 | 15.69 | 450.1 | 450.9 |
| engram-node02 | flood | 8.11 | 12.27 | 380.5 | 380.8 |
| engram-node03 | flood | 7.96 | 16.25 | 445.2 | 447.0 |
| engram-node04 | flood | 9.71 | 21.59 | 69.1 | 77.0 |
| engram-node05 | flood | 7.76 | 16.49 | 501.5 | 501.7 |
| engram-node06 | flood | 8.10 | 14.12 | 395.2 | 396.0 |
| engram-node07 | flood | 7.56 | 13.93 | 415.7 | 416.7 |
| engram-node01 | recovery | 11.10 | 16.85 | 450.0 | 450.0 |
| engram-node02 | recovery | 10.94 | 19.62 | 381.2 | 382.2 |
| engram-node03 | recovery | 9.48 | 14.02 | 447.0 | 447.1 |
| engram-node04 | recovery | 7.76 | 11.85 | 57.3 | 64.2 |
| engram-node05 | recovery | 8.91 | 11.57 | 500.4 | 501.5 |
| engram-node06 | recovery | 17.48 | 51.74 | 396.2 | 396.4 |
| engram-node07 | recovery | 9.92 | 14.60 | 414.9 | 414.9 |
