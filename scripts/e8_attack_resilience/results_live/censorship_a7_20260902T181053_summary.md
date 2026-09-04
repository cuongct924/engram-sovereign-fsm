# LIVE Censorship test (E8 A7 adversarial half)

Target tx: 161 bytes (hex, see raw CSV/logs for full value).

Total duration: 812s.

## Verdict

- **engram-node04 proposer turns during the censoring window: [3450, 3457, 3464, 3471, 3478, 3492, 3499, 3506, 3513]** (empty means the attack was never actually exercised -- treat safety/liveness below as vacuous if so)
- **Censorship actually observed (a engram-node04 block omitted the target tx): True**
- Safety held (honest AppHash never diverged at the same height): **True**
- Divergence events: 0
- Liveness held (block rate during censoring vs. baseline, no collapse toward 0): **True** (baseline 0.739 blocks/s, censoring 0.759 blocks/s, recovery 0.693 blocks/s)
- Reject/round-skip log signals per honest node during the censoring window: {'engram-node01': 0, 'engram-node02': 0, 'engram-node03': 0, 'engram-node05': 0, 'engram-node06': 0, 'engram-node07': 0}

