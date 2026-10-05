# LIVE Sybil/slot-exhaustion attack, leg 'a2'

12 attackers across 4 distinct subnets (attacker-subnet-a/b/c/d, 3 each)

Total duration: 208s. MaxPeersPerSubnet=8.

## Real observed subnet-peer counts (target: engram-node01, subnet 172.28.0.0/24)

- Baseline (pre-attack): 0
- Peak during attack: 8 (peak total peers across all subnets: 14)
- After teardown (recovery): 0

## Verdict

- Ingress filter held the target subnet at or below MaxPeersPerSubnet during the attack: **True**
- FSM state never left ANCHORED/SUSPICIOUS during the attack (no false SOVEREIGN degradation from a defended attack): **False**
- Safety held (all 4 validators' AppHash never diverged at the same height): **True**
- Divergence events: 0
- Liveness held (block rate during attack vs. baseline, no collapse toward 0): **True** (baseline 0.830 blocks/s, attack 0.770 blocks/s, recovery 0.749 blocks/s)
- engram-node01's own nil-prevote ratio (fraction of committed heights where its precommit was nil, not just 'no divergence'): baseline=0.0, attack=0.0, recovery=0.0

## Full timeline

| t (s) | phase | fsm_state | height | target_subnet_peers | total_peers |
|---:|---|---|---:|---:|---:|
| 0.0 | baseline | ANCHORED | 539 | 0 | 6 |
| 5.0 | baseline | ANCHORED | 543 | 0 | 6 |
| 10.1 | baseline | ANCHORED | 547 | 0 | 6 |
| 15.2 | baseline | ANCHORED | 552 | 0 | 6 |
| 20.2 | baseline | ANCHORED | 556 | 0 | 6 |
| 25.3 | baseline | ANCHORED | 560 | 0 | 6 |
| 48.2 | attack | SUSPICIOUS | 576 | 5 | 11 |
| 54.3 | attack | SUSPICIOUS | 580 | 8 | 14 |
| 60.6 | attack | ANCHORED | 585 | 8 | 14 |
| 65.7 | attack | ANCHORED | 589 | 8 | 14 |
| 70.8 | attack | ANCHORED | 593 | 8 | 14 |
| 75.9 | attack | ANCHORED | 597 | 8 | 14 |
| 80.9 | attack | ANCHORED | 601 | 8 | 14 |
| 85.9 | attack | ANCHORED | 605 | 8 | 14 |
| 91.0 | attack | ANCHORED | 609 | 8 | 14 |
| 96.0 | attack | ANCHORED | 613 | 8 | 14 |
| 101.1 | attack | ANCHORED | 617 | 8 | 14 |
| 106.1 | attack | SUSPICIOUS | 621 | 8 | 14 |
| 111.2 | attack | SUSPICIOUS | 625 | 8 | 14 |
| 116.3 | attack | ANCHORED | 629 | 8 | 14 |
| 121.3 | attack | ANCHORED | 633 | 8 | 14 |
| 126.4 | attack | ANCHORED | 637 | 8 | 14 |
| 131.5 | attack | ANCHORED | 641 | 8 | 14 |
| 136.5 | attack | ANCHORED | 645 | 8 | 14 |
| 143.8 | recovery | ANCHORED | 651 | 0 | 6 |
| 148.9 | recovery | ANCHORED | 654 | 0 | 6 |
| 155.2 | recovery | ANCHORED | 658 | 0 | 6 |
| 161.2 | recovery | ANCHORED | 663 | 0 | 6 |
| 167.2 | recovery | SUSPICIOUS | 667 | 0 | 6 |
| 172.3 | recovery | SUSPICIOUS | 671 | 0 | 6 |
| 177.3 | recovery | ANCHORED | 675 | 0 | 6 |
| 182.4 | recovery | ANCHORED | 679 | 0 | 6 |
| 187.4 | recovery | ANCHORED | 683 | 0 | 6 |
| 192.5 | recovery | ANCHORED | 687 | 0 | 6 |
| 197.6 | recovery | ANCHORED | 691 | 0 | 6 |
| 202.6 | recovery | ANCHORED | 695 | 0 | 6 |
