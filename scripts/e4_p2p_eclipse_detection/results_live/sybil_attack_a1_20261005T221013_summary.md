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
- Liveness held (block rate during attack vs. baseline, no collapse toward 0): **True** (baseline 0.832 blocks/s, attack 0.782 blocks/s, recovery 0.784 blocks/s)
- engram-node01's own nil-prevote ratio (fraction of committed heights where its precommit was nil, not just 'no divergence'): baseline=0.0, attack=0.0, recovery=0.0

## Full timeline

| t (s) | phase | fsm_state | height | target_subnet_peers | total_peers |
|---:|---|---|---:|---:|---:|
| 0.0 | baseline | ANCHORED | 2627 | 0 | 6 |
| 5.0 | baseline | ANCHORED | 2631 | 0 | 6 |
| 10.1 | baseline | ANCHORED | 2635 | 0 | 6 |
| 15.2 | baseline | ANCHORED | 2639 | 0 | 6 |
| 20.2 | baseline | ANCHORED | 2643 | 0 | 6 |
| 25.3 | baseline | ANCHORED | 2647 | 0 | 6 |
| 34.9 | attack | SUSPICIOUS | 2654 | 1 | 7 |
| 41.0 | attack | ANCHORED | 2659 | 8 | 14 |
| 46.6 | attack | ANCHORED | 2662 | 8 | 14 |
| 52.4 | attack | ANCHORED | 2666 | 8 | 14 |
| 57.6 | attack | ANCHORED | 2671 | 8 | 14 |
| 62.6 | attack | ANCHORED | 2675 | 8 | 14 |
| 67.7 | attack | ANCHORED | 2679 | 8 | 14 |
| 72.7 | attack | ANCHORED | 2683 | 8 | 14 |
| 77.8 | attack | ANCHORED | 2687 | 8 | 14 |
| 82.9 | attack | ANCHORED | 2692 | 8 | 14 |
| 87.9 | attack | SUSPICIOUS | 2696 | 8 | 14 |
| 92.9 | attack | SUSPICIOUS | 2699 | 8 | 14 |
| 98.0 | attack | ANCHORED | 2704 | 8 | 14 |
| 103.0 | attack | ANCHORED | 2708 | 8 | 14 |
| 108.0 | attack | ANCHORED | 2712 | 8 | 14 |
| 113.1 | attack | ANCHORED | 2716 | 8 | 14 |
| 118.1 | attack | ANCHORED | 2720 | 8 | 14 |
| 123.2 | attack | ANCHORED | 2724 | 8 | 14 |
| 129.6 | recovery | ANCHORED | 2729 | 0 | 6 |
| 134.7 | recovery | ANCHORED | 2733 | 0 | 6 |
| 140.4 | recovery | ANCHORED | 2736 | 0 | 6 |
| 146.2 | recovery | ANCHORED | 2740 | 0 | 6 |
| 151.8 | recovery | SUSPICIOUS | 2744 | 0 | 6 |
| 156.8 | recovery | SUSPICIOUS | 2749 | 0 | 6 |
| 161.9 | recovery | ANCHORED | 2753 | 0 | 6 |
| 166.9 | recovery | ANCHORED | 2757 | 0 | 6 |
| 171.9 | recovery | ANCHORED | 2761 | 0 | 6 |
| 177.0 | recovery | ANCHORED | 2766 | 0 | 6 |
| 182.0 | recovery | ANCHORED | 2770 | 0 | 6 |
| 187.1 | recovery | ANCHORED | 2774 | 0 | 6 |
