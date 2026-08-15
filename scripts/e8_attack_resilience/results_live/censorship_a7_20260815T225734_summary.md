# LIVE Censorship test (E8 A7 adversarial half)

Target tx: 162 bytes (hex, see raw CSV/logs for full value).

Total duration: 175s.

## Verdict

- Safety held (honest AppHash never diverged at the same height): **True**
- Divergence events: 0
- Liveness held (block rate during censoring vs. baseline, no collapse toward 0): **True** (baseline 0.826 blocks/s, censoring 0.601 blocks/s, recovery 0.697 blocks/s)
- Reject/round-skip log signals per honest node during the censoring window: {'engram-node01': 0, 'engram-node02': 0, 'engram-node03': 0}

