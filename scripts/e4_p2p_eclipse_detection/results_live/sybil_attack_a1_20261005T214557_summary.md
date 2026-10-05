# LIVE Sybil/slot-exhaustion attack, leg 'a1'

10 attackers, all on engram-net (same subnet as the 4 real validators)

Total duration: 192s. MaxPeersPerSubnet=8.

## Real observed subnet-peer counts (target: engram-node01, subnet 172.28.0.0/24)

- Baseline (pre-attack): 0
- Peak during attack: 8 (peak total peers across all subnets: 14)
- After teardown (recovery): 0

## Verdict

- Ingress filter held the target subnet at or below MaxPeersPerSubnet during the attack: **True**
- FSM state never left ANCHORED/SUSPICIOUS during the attack (no false SOVEREIGN degradation from a defended attack): **False**
- Safety held (all 4 validators' AppHash never diverged at the same height): **True**
- Divergence events: 0
- Liveness held (block rate during attack vs. baseline, no collapse toward 0): **True** (baseline 0.792 blocks/s, attack 0.791 blocks/s, recovery 0.824 blocks/s)
- engram-node01's own nil-prevote ratio (fraction of committed heights where its precommit was nil, not just 'no divergence'): baseline=0.0, attack=0.0, recovery=0.0

## Full timeline

| t (s) | phase | fsm_state | height | target_subnet_peers | total_peers |
|---:|---|---|---:|---:|---:|
| 0.0 | baseline | ANCHORED | 1498 | 0 | 6 |
| 5.0 | baseline | ANCHORED | 1502 | 0 | 6 |
| 10.1 | baseline | ANCHORED | 1506 | 0 | 6 |
| 15.1 | baseline | ANCHORED | 1510 | 0 | 6 |
| 20.2 | baseline | ANCHORED | 1514 | 0 | 6 |
| 25.3 | baseline | ANCHORED | 1518 | 0 | 6 |
| 35.1 | attack | ANCHORED | 1525 | 2 | 8 |
| 40.9 | attack | ANCHORED | 1529 | 8 | 14 |
| 46.8 | attack | SUSPICIOUS | 1533 | 8 | 14 |
| 52.7 | attack | ANCHORED | 1538 | 8 | 14 |
| 57.8 | attack | ANCHORED | 1542 | 8 | 14 |
| 62.9 | attack | ANCHORED | 1546 | 8 | 14 |
| 67.9 | attack | ANCHORED | 1550 | 8 | 14 |
| 73.0 | attack | ANCHORED | 1554 | 8 | 14 |
| 78.1 | attack | ANCHORED | 1558 | 8 | 14 |
| 83.2 | attack | ANCHORED | 1562 | 8 | 14 |
| 88.2 | attack | ANCHORED | 1567 | 8 | 14 |
| 93.3 | attack | ANCHORED | 1571 | 8 | 14 |
| 98.4 | attack | ANCHORED | 1575 | 8 | 14 |
| 103.4 | attack | SUSPICIOUS | 1579 | 8 | 14 |
| 108.4 | attack | SUSPICIOUS | 1583 | 8 | 14 |
| 113.5 | attack | ANCHORED | 1588 | 8 | 14 |
| 118.5 | attack | ANCHORED | 1592 | 8 | 14 |
| 123.6 | attack | ANCHORED | 1596 | 8 | 14 |
| 130.1 | recovery | ANCHORED | 1601 | 0 | 6 |
| 135.1 | recovery | ANCHORED | 1605 | 0 | 6 |
| 140.9 | recovery | ANCHORED | 1610 | 0 | 6 |
| 146.7 | recovery | ANCHORED | 1614 | 0 | 6 |
| 151.7 | recovery | ANCHORED | 1619 | 0 | 6 |
| 156.8 | recovery | ANCHORED | 1623 | 0 | 6 |
| 161.8 | recovery | SUSPICIOUS | 1627 | 0 | 6 |
| 166.9 | recovery | SUSPICIOUS | 1631 | 0 | 6 |
| 172.0 | recovery | ANCHORED | 1635 | 0 | 6 |
| 177.0 | recovery | ANCHORED | 1640 | 0 | 6 |
| 182.0 | recovery | ANCHORED | 1644 | 0 | 6 |
| 187.1 | recovery | ANCHORED | 1648 | 0 | 6 |
