# LIVE Timeout-flooding test

docs/EXPERIMENT.md's E8 "Timeout flooding by Byzantine nodes" row -- node04 stays alive and honest everywhere else, but actively floods genuinely signed Timeout messages every 50ms (engram-consensus-core's timeoutFloodRoutine), instead of the crash-only (`chaos-crash`) approximation used previously.

Total duration: 132s.

## Verdict

- Safety held (3 honest nodes' AppHash never diverged): **True**
- Divergence events: 0
- Cadence held (flood-phase height rate >= 50% of baseline): **True**
  - baseline: 0.808 blocks/s
  - flood:    0.608 blocks/s
  - recovery: 0.586 blocks/s


## Per-peer rate limiter (reactor.go's PeerState.allowTimeoutMessage)

Real "rate limit exceeded" drop count on each honest node, counted from `docker logs --since` the flood phase started (not an inference):

- engram-node01: 37 drops
- engram-node02: 38 drops
- engram-node03: 38 drops
