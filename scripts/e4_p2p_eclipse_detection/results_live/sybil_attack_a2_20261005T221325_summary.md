# LIVE Sybil/slot-exhaustion attack, leg 'a2'

12 attackers across 4 distinct subnets (attacker-subnet-a/b/c/d, 3 each)

Total duration: 192s. MaxPeersPerSubnet=8.

## Real observed subnet-peer counts (target: engram-node01, subnet 172.28.0.0/24)

- Baseline (pre-attack): 0
- Peak during attack: 8 (peak total peers across all subnets: 14)
- After teardown (recovery): 0

## Verdict

- Ingress filter held the target subnet at or below MaxPeersPerSubnet during the attack: **True**
- FSM state never left ANCHORED/SUSPICIOUS during the attack (no false SOVEREIGN degradation from a defended attack): **True**
- Safety held (all 4 validators' AppHash never diverged at the same height): **True**
- Divergence events: 0
- Liveness held (block rate during attack vs. baseline, no collapse toward 0): **True** (baseline 0.832 blocks/s, attack 0.765 blocks/s, recovery 0.790 blocks/s)
- engram-node01's own nil-prevote ratio (fraction of committed heights where its precommit was nil, not just 'no divergence'): baseline=0.0, attack=0.0, recovery=0.0

## Full timeline

| t (s) | phase | fsm_state | height | target_subnet_peers | total_peers |
|---:|---|---|---:|---:|---:|
| 0.0 | baseline | ANCHORED | 2778 | 0 | 6 |
| 5.0 | baseline | ANCHORED | 2782 | 0 | 6 |
| 10.1 | baseline | ANCHORED | 2786 | 0 | 6 |
| 15.2 | baseline | ANCHORED | 2791 | 0 | 6 |
| 20.2 | baseline | SUSPICIOUS | 2795 | 0 | 6 |
| 25.3 | baseline | SUSPICIOUS | 2799 | 0 | 6 |
| 36.6 | attack | ANCHORED | 2807 | 3 | 9 |
| 42.8 | attack | ANCHORED | 2811 | 8 | 14 |
| 48.7 | attack | ANCHORED | 2815 | 8 | 14 |
| 54.8 | attack | ANCHORED | 2819 | 8 | 14 |
| 60.9 | attack | ANCHORED | 2824 | 8 | 14 |
| 65.9 | attack | ANCHORED | 2828 | 8 | 14 |
| 71.0 | attack | ANCHORED | 2832 | 8 | 14 |
| 76.1 | attack | ANCHORED | 2836 | 8 | 14 |
| 81.2 | attack | SUSPICIOUS | 2841 | 8 | 14 |
| 86.2 | attack | ANCHORED | 2845 | 8 | 14 |
| 91.3 | attack | ANCHORED | 2849 | 8 | 14 |
| 96.3 | attack | ANCHORED | 2853 | 8 | 14 |
| 101.4 | attack | ANCHORED | 2857 | 8 | 14 |
| 106.4 | attack | ANCHORED | 2861 | 8 | 14 |
| 111.5 | attack | ANCHORED | 2865 | 8 | 14 |
| 116.5 | attack | ANCHORED | 2869 | 8 | 14 |
| 121.6 | attack | ANCHORED | 2873 | 8 | 14 |
| 128.9 | recovery | ANCHORED | 2879 | 0 | 6 |
| 134.0 | recovery | ANCHORED | 2883 | 0 | 6 |
| 139.9 | recovery | SUSPICIOUS | 2887 | 0 | 6 |
| 145.9 | recovery | SUSPICIOUS | 2892 | 0 | 6 |
| 151.8 | recovery | ANCHORED | 2897 | 0 | 6 |
| 156.8 | recovery | ANCHORED | 2901 | 0 | 6 |
| 161.9 | recovery | ANCHORED | 2905 | 0 | 6 |
| 166.9 | recovery | ANCHORED | 2909 | 0 | 6 |
| 172.0 | recovery | ANCHORED | 2913 | 0 | 6 |
| 177.0 | recovery | ANCHORED | 2917 | 0 | 6 |
| 182.1 | recovery | ANCHORED | 2921 | 0 | 6 |
| 187.2 | recovery | ANCHORED | 2925 | 0 | 6 |
