# LIVE Sybil/slot-exhaustion attack, leg 'a1'

10 attackers, all on engram-net (same subnet as the 4 real validators)

Total duration: 159s. MaxPeersPerSubnet=8.

## Real observed subnet-peer counts (target: engram-node01, subnet 172.28.0.0/24)

- Baseline (pre-attack): 0
- Peak during attack: 8 (peak total peers across all subnets: 11)
- After teardown (recovery): 0

## Verdict

- Ingress filter held the target subnet at or below MaxPeersPerSubnet during the attack: **True**
- FSM state never left ANCHORED/SUSPICIOUS during the attack (no false SOVEREIGN degradation from a defended attack): **False**

## Full timeline

| t (s) | phase | fsm_state | height | target_subnet_peers | total_peers |
|---:|---|---|---:|---:|---:|
| 0.0 | baseline | RECOVERING | 212 | 0 | 3 |
| 5.0 | baseline | RECOVERING | 212 | 0 | 3 |
| 10.1 | baseline | RECOVERING | 212 | 0 | 3 |
| 15.1 | baseline | RECOVERING | 212 | 0 | 3 |
| 22.9 | attack | RECOVERING | 215 | 0 | 3 |
| 28.0 | attack | RECOVERING | 216 | 8 | 11 |
| 33.0 | attack | RECOVERING | 216 | 8 | 11 |
| 38.1 | attack | RECOVERING | 216 | 8 | 11 |
| 43.1 | attack | RECOVERING | 216 | 8 | 11 |
| 48.1 | attack | RECOVERING | 216 | 8 | 11 |
| 53.1 | attack | RECOVERING | 216 | 8 | 11 |
| 58.1 | attack | RECOVERING | 216 | 8 | 11 |
| 63.1 | attack | RECOVERING | 216 | 8 | 11 |
| 68.1 | attack | RECOVERING | 216 | 8 | 11 |
| 73.1 | attack | RECOVERING | 216 | 8 | 11 |
| 78.2 | attack | RECOVERING | 216 | 8 | 11 |
| 83.2 | attack | RECOVERING | 218 | 8 | 11 |
| 88.2 | attack | RECOVERING | 220 | 8 | 11 |
| 93.2 | attack | RECOVERING | 220 | 8 | 11 |
| 98.2 | attack | RECOVERING | 220 | 8 | 11 |
| 103.3 | attack | RECOVERING | 220 | 8 | 11 |
| 108.3 | attack | RECOVERING | 220 | 8 | 11 |
| 119.2 | recovery |  | -1 | 0 | 0 |
| 124.2 | recovery |  | -1 | 0 | 0 |
| 129.2 | recovery |  | -1 | 0 | 0 |
| 134.2 | recovery |  | -1 | 0 | 0 |
| 139.2 | recovery |  | -1 | 0 | 0 |
| 144.2 | recovery |  | -1 | 0 | 0 |
| 149.2 | recovery |  | -1 | 0 | 0 |
| 154.2 | recovery |  | -1 | 0 | 0 |
