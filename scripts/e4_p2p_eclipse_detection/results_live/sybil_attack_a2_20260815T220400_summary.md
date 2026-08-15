# LIVE Sybil/slot-exhaustion attack, leg 'a2'

12 attackers across 4 distinct subnets (attacker-subnet-a/b/c/d, 3 each)

Total duration: 215s. MaxPeersPerSubnet=8.

## Real observed subnet-peer counts (target: engram-node01, subnet 172.28.0.0/24)

- Baseline (pre-attack): 0
- Peak during attack: 8 (peak total peers across all subnets: 11)
- After teardown (recovery): 0

## Verdict

- Ingress filter held the target subnet at or below MaxPeersPerSubnet during the attack: **True**
- FSM state never left ANCHORED/SUSPICIOUS during the attack (no false SOVEREIGN degradation from a defended attack): **True**
- Safety held (all 4 validators' AppHash never diverged at the same height): **True**
- Divergence events: 0
- Liveness held (block rate during attack vs. baseline, no collapse toward 0): **True** (baseline 0.612 blocks/s, attack 0.536 blocks/s, recovery 0.623 blocks/s)

## Resource usage during attack (docker stats)

| Container | Avg CPU% | Max CPU% | Avg Mem (MiB) | Max Mem (MiB) |
|---|---:|---:|---:|---:|
| engram-node01 | 8.76 | 19.31 | 142.7 | 146.9 |
| engram-node02 | 9.03 | 35.87 | 128.5 | 128.9 |
| engram-node03 | 8.85 | 30.41 | 130.8 | 131.0 |
| engram-node04 | 8.47 | 23.00 | 130.2 | 130.8 |
| attacker-a2-a1 | 8.95 | 24.87 | 34.2 | 45.4 |
| attacker-a2-a2 | 8.36 | 36.93 | 19.0 | 19.1 |
| attacker-a2-a3 | 10.26 | 30.37 | 32.4 | 44.1 |
| attacker-a2-b1 | 10.94 | 29.51 | 30.4 | 42.2 |
| attacker-a2-b2 | 9.47 | 25.86 | 33.5 | 44.7 |
| attacker-a2-b3 | 5.45 | 13.98 | 24.5 | 24.5 |
| attacker-a2-c1 | 9.14 | 20.37 | 63.7 | 74.0 |
| attacker-a2-c2 | 9.97 | 25.44 | 39.0 | 50.4 |
| attacker-a2-c3 | 9.03 | 25.10 | 29.4 | 36.7 |
| attacker-a2-d1 | 9.85 | 32.91 | 27.3 | 36.1 |
| attacker-a2-d2 | 11.47 | 31.03 | 31.4 | 42.7 |
| attacker-a2-d3 | 8.19 | 23.02 | 32.4 | 43.1 |

## Full timeline

| t (s) | phase | fsm_state | height | target_subnet_peers | total_peers |
|---:|---|---|---:|---:|---:|
| 0.0 | baseline | ANCHORED | 3590 | 0 | 3 |
| 6.9 | baseline | ANCHORED | 3594 | 0 | 3 |
| 14.0 | baseline | ANCHORED | 3599 | 0 | 3 |
| 21.1 | baseline | ANCHORED | 3603 | 0 | 3 |
| 27.8 | baseline | ANCHORED | 3607 | 0 | 3 |
| 51.4 | attack | ANCHORED | 3618 | 0 | 3 |
| 60.2 | attack | ANCHORED | 3619 | 8 | 11 |
| 68.1 | attack | ANCHORED | 3623 | 8 | 11 |
| 75.3 | attack | ANCHORED | 3628 | 8 | 11 |
| 82.1 | attack | ANCHORED | 3632 | 8 | 11 |
| 89.4 | attack | ANCHORED | 3637 | 8 | 11 |
| 96.8 | attack | ANCHORED | 3641 | 8 | 11 |
| 104.0 | attack | ANCHORED | 3645 | 8 | 11 |
| 113.1 | attack | ANCHORED | 3649 | 8 | 11 |
| 120.4 | attack | ANCHORED | 3653 | 8 | 11 |
| 127.5 | attack | ANCHORED | 3658 | 8 | 11 |
| 134.7 | attack | ANCHORED | 3662 | 8 | 11 |
| 140.9 | attack | ANCHORED | 3666 | 8 | 11 |
| 151.6 | recovery | ANCHORED | 3672 | 0 | 3 |
| 158.9 | recovery | ANCHORED | 3677 | 0 | 3 |
| 166.1 | recovery | ANCHORED | 3681 | 0 | 3 |
| 172.7 | recovery | ANCHORED | 3686 | 0 | 3 |
| 179.9 | recovery | ANCHORED | 3690 | 0 | 3 |
| 187.1 | recovery | ANCHORED | 3694 | 0 | 3 |
| 194.2 | recovery | ANCHORED | 3698 | 0 | 3 |
| 200.6 | recovery | ANCHORED | 3702 | 0 | 3 |
| 207.7 | recovery | ANCHORED | 3707 | 0 | 3 |
