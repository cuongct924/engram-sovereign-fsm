# LIVE Censorship test (E8 A7 adversarial half)

Target tx: 161 bytes (hex, see raw CSV/logs for full value).

Total duration: 179s.

## Verdict

- **engram-node04 proposer turns during the censoring window: [529, 533, 537, 541, 545, 549, 553, 557, 565, 569, 573, 577, 581, 585, 589, 593, 597]** (empty means the attack was never actually exercised -- treat safety/liveness below as vacuous if so)
- **Censorship actually observed (a engram-node04 block omitted the target tx): True**
- Safety held (honest AppHash never diverged at the same height): **True**
- Divergence events: 0
- Liveness held (block rate during censoring vs. baseline, no collapse toward 0): **True** (baseline 0.825 blocks/s, censoring 0.728 blocks/s, recovery 0.735 blocks/s)
- Reject/round-skip log signals per honest node during the censoring window: {'engram-node01': 0, 'engram-node02': 0, 'engram-node03': 0}

