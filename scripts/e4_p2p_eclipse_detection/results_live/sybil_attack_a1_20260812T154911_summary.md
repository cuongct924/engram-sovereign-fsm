# LIVE Sybil/slot-exhaustion attack, leg 'a1'

10 attackers, all on engram-net (same subnet as the 4 real validators)

Total duration: 185s. MaxPeersPerSubnet=8.

## Real observed subnet-peer counts (target: engram-node01, subnet 172.28.0.0/24)

- Baseline (pre-attack): 0
- Peak during attack: 8 (peak total peers across all subnets: 11)
- After teardown (recovery): 0

## Verdict

- Ingress filter held the target subnet at or below MaxPeersPerSubnet during the attack: **True**
- FSM state never left ANCHORED/SUSPICIOUS during the attack (no false SOVEREIGN degradation from a defended attack): **True**

## Full timeline

| t (s) | phase | fsm_state | height | target_subnet_peers | total_peers |
|---:|---|---|---:|---:|---:|
| 0.0 | baseline | ANCHORED | 4851 | 0 | 3 |
| 5.0 | baseline | ANCHORED | 4854 | 0 | 3 |
| 10.0 | baseline | ANCHORED | 4858 | 0 | 3 |
| 15.0 | baseline | ANCHORED | 4861 | 0 | 3 |
| 20.1 | baseline | ANCHORED | 4865 | 0 | 3 |
| 25.1 | baseline | ANCHORED | 4868 | 0 | 3 |
| 32.3 | attack | ANCHORED | 4872 | 0 | 3 |
| 37.4 | attack | ANCHORED | 4875 | 8 | 11 |
| 42.4 | attack | ANCHORED | 4878 | 8 | 11 |
| 47.4 | attack | ANCHORED | 4881 | 8 | 11 |
| 52.4 | attack | ANCHORED | 4885 | 8 | 11 |
| 57.4 | attack | ANCHORED | 4889 | 8 | 11 |
| 62.4 | attack | ANCHORED | 4891 | 8 | 11 |
| 67.4 | attack | ANCHORED | 4894 | 8 | 11 |
| 72.5 | attack | ANCHORED | 4897 | 8 | 11 |
| 77.5 | attack | ANCHORED | 4901 | 8 | 11 |
| 82.5 | attack | ANCHORED | 4904 | 8 | 11 |
| 87.5 | attack | ANCHORED | 4907 | 8 | 11 |
| 92.5 | attack | ANCHORED | 4910 | 8 | 11 |
| 97.5 | attack | ANCHORED | 4913 | 8 | 11 |
| 102.6 | attack | ANCHORED | 4917 | 8 | 11 |
| 107.6 | attack | ANCHORED | 4920 | 8 | 11 |
| 112.6 | attack | ANCHORED | 4923 | 8 | 11 |
| 117.6 | attack | ANCHORED | 4926 | 8 | 11 |
| 124.4 | recovery | ANCHORED | 4931 | 0 | 3 |
| 129.5 | recovery | ANCHORED | 4933 | 0 | 3 |
| 134.5 | recovery | ANCHORED | 4937 | 0 | 3 |
| 139.5 | recovery | ANCHORED | 4940 | 0 | 3 |
| 144.5 | recovery | ANCHORED | 4943 | 0 | 3 |
| 149.6 | recovery | ANCHORED | 4946 | 0 | 3 |
| 154.6 | recovery | ANCHORED | 4949 | 0 | 3 |
| 159.6 | recovery | ANCHORED | 4953 | 0 | 3 |
| 164.6 | recovery | ANCHORED | 4956 | 0 | 3 |
| 169.6 | recovery | ANCHORED | 4959 | 0 | 3 |
| 174.6 | recovery | ANCHORED | 4962 | 0 | 3 |
| 179.6 | recovery | ANCHORED | 4965 | 0 | 3 |
