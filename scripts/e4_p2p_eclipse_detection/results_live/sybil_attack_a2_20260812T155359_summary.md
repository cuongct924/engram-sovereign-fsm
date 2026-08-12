# LIVE Sybil/slot-exhaustion attack, leg 'a2'

12 attackers across 4 distinct subnets (attacker-subnet-a/b/c/d, 3 each)

Total duration: 186s. MaxPeersPerSubnet=8.

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
| 0.0 | baseline | ANCHORED | 5030 | 0 | 3 |
| 5.0 | baseline | ANCHORED | 5033 | 0 | 3 |
| 10.0 | baseline | ANCHORED | 5037 | 0 | 3 |
| 15.1 | baseline | ANCHORED | 5040 | 0 | 3 |
| 20.1 | baseline | ANCHORED | 5043 | 0 | 3 |
| 25.1 | baseline | ANCHORED | 5045 | 0 | 3 |
| 33.2 | attack | ANCHORED | 5050 | 1 | 4 |
| 38.2 | attack | ANCHORED | 5053 | 7 | 10 |
| 43.2 | attack | ANCHORED | 5057 | 8 | 11 |
| 48.2 | attack | ANCHORED | 5060 | 8 | 11 |
| 53.2 | attack | ANCHORED | 5062 | 8 | 11 |
| 58.3 | attack | ANCHORED | 5065 | 8 | 11 |
| 63.3 | attack | ANCHORED | 5069 | 8 | 11 |
| 68.3 | attack | ANCHORED | 5072 | 8 | 11 |
| 73.3 | attack | ANCHORED | 5075 | 8 | 11 |
| 78.3 | attack | ANCHORED | 5078 | 8 | 11 |
| 83.3 | attack | ANCHORED | 5081 | 8 | 11 |
| 88.3 | attack | ANCHORED | 5084 | 8 | 11 |
| 93.3 | attack | ANCHORED | 5087 | 8 | 11 |
| 98.3 | attack | ANCHORED | 5090 | 8 | 11 |
| 103.4 | attack | ANCHORED | 5093 | 8 | 11 |
| 108.4 | attack | ANCHORED | 5096 | 8 | 11 |
| 113.4 | attack | ANCHORED | 5099 | 8 | 11 |
| 118.4 | attack | ANCHORED | 5102 | 8 | 11 |
| 125.8 | recovery | ANCHORED | 5107 | 0 | 3 |
| 130.8 | recovery | ANCHORED | 5110 | 0 | 3 |
| 135.9 | recovery | ANCHORED | 5113 | 0 | 3 |
| 140.9 | recovery | ANCHORED | 5117 | 0 | 3 |
| 145.9 | recovery | ANCHORED | 5121 | 0 | 3 |
| 150.9 | recovery | ANCHORED | 5124 | 0 | 3 |
| 155.9 | recovery | ANCHORED | 5127 | 0 | 3 |
| 160.9 | recovery | ANCHORED | 5129 | 0 | 3 |
| 165.9 | recovery | ANCHORED | 5133 | 0 | 3 |
| 170.9 | recovery | ANCHORED | 5136 | 0 | 3 |
| 176.0 | recovery | ANCHORED | 5139 | 0 | 3 |
| 181.0 | recovery | ANCHORED | 5142 | 0 | 3 |
