# LIVE Sybil/slot-exhaustion attack, leg 'a2'

12 attackers across 4 distinct subnets (attacker-subnet-a/b/c/d, 3 each)

Total duration: 196s. MaxPeersPerSubnet=8.

## Real observed subnet-peer counts (target: engram-node01, subnet 172.28.0.0/24)

- Baseline (pre-attack): 0
- Peak during attack: 8 (peak total peers across all subnets: 14)
- After teardown (recovery): 0

## Verdict

- Ingress filter held the target subnet at or below MaxPeersPerSubnet during the attack: **True**
- FSM state never left ANCHORED/SUSPICIOUS during the attack (no false SOVEREIGN degradation from a defended attack): **False**
- Safety held (all 4 validators' AppHash never diverged at the same height): **True**
- Divergence events: 0
- Liveness held (block rate during attack vs. baseline, no collapse toward 0): **True** (baseline 0.831 blocks/s, attack 0.798 blocks/s, recovery 0.809 blocks/s)
- engram-node01's own nil-prevote ratio (fraction of committed heights where its precommit was nil, not just 'no divergence'): baseline=0.0, attack=0.0, recovery=0.0

## Full timeline

| t (s) | phase | fsm_state | height | target_subnet_peers | total_peers |
|---:|---|---|---:|---:|---:|
| 0.0 | baseline | ANCHORED | 1652 | 0 | 6 |
| 5.0 | baseline | ANCHORED | 1657 | 0 | 6 |
| 10.1 | baseline | ANCHORED | 1661 | 0 | 6 |
| 15.1 | baseline | ANCHORED | 1665 | 0 | 6 |
| 20.2 | baseline | ANCHORED | 1669 | 0 | 6 |
| 25.3 | baseline | ANCHORED | 1673 | 0 | 6 |
| 37.0 | attack | SUSPICIOUS | 1681 | 4 | 10 |
| 43.1 | attack | ANCHORED | 1686 | 8 | 14 |
| 49.0 | attack | ANCHORED | 1690 | 8 | 14 |
| 55.0 | attack | ANCHORED | 1695 | 8 | 14 |
| 60.2 | attack | ANCHORED | 1699 | 8 | 14 |
| 65.2 | attack | ANCHORED | 1703 | 8 | 14 |
| 70.3 | attack | ANCHORED | 1707 | 8 | 14 |
| 75.4 | attack | ANCHORED | 1711 | 8 | 14 |
| 80.4 | attack | ANCHORED | 1715 | 8 | 14 |
| 85.5 | attack | ANCHORED | 1720 | 8 | 14 |
| 90.6 | attack | SUSPICIOUS | 1724 | 8 | 14 |
| 95.6 | attack | SUSPICIOUS | 1728 | 8 | 14 |
| 100.7 | attack | ANCHORED | 1732 | 8 | 14 |
| 105.8 | attack | ANCHORED | 1736 | 8 | 14 |
| 110.8 | attack | ANCHORED | 1740 | 8 | 14 |
| 115.9 | attack | ANCHORED | 1745 | 8 | 14 |
| 120.9 | attack | ANCHORED | 1749 | 8 | 14 |
| 126.0 | attack | ANCHORED | 1753 | 8 | 14 |
| 133.1 | recovery | ANCHORED | 1759 | 0 | 6 |
| 138.2 | recovery | ANCHORED | 1763 | 0 | 6 |
| 144.1 | recovery | ANCHORED | 1767 | 0 | 6 |
| 150.0 | recovery | SUSPICIOUS | 1772 | 0 | 6 |
| 155.9 | recovery | SUSPICIOUS | 1777 | 0 | 6 |
| 161.0 | recovery | ANCHORED | 1781 | 0 | 6 |
| 166.0 | recovery | ANCHORED | 1785 | 0 | 6 |
| 171.1 | recovery | ANCHORED | 1789 | 0 | 6 |
| 176.1 | recovery | ANCHORED | 1794 | 0 | 6 |
| 181.2 | recovery | ANCHORED | 1798 | 0 | 6 |
| 186.2 | recovery | ANCHORED | 1802 | 0 | 6 |
| 191.3 | recovery | ANCHORED | 1806 | 0 | 6 |
