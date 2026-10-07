# LIVE Sybil/slot-exhaustion attack, leg 'a1'

10 attackers, all on engram-net (same subnet as the 4 real validators)

Total duration: 182s. MaxPeersPerSubnet=8.

## Real observed subnet-peer counts (target: engram-node01, subnet 172.28.0.0/24)

- Baseline (pre-attack): 0
- Peak during attack: 0 (peak total peers across all subnets: 6)
- After teardown (recovery): 0

## Verdict

- Ingress filter held the target subnet at or below MaxPeersPerSubnet during the attack: **True**
- FSM state never left ANCHORED/SUSPICIOUS during the attack (no false SOVEREIGN degradation from a defended attack): **False**
- Safety held (all 4 validators' AppHash never diverged at the same height): **True**
- Divergence events: 0
- Liveness held (block rate during attack vs. baseline, no collapse toward 0): **True** (baseline 0.790 blocks/s, attack 0.781 blocks/s, recovery 0.793 blocks/s)
- engram-node01's own nil-prevote ratio (fraction of committed heights where its precommit was nil, not just 'no divergence'): baseline=0.0, attack=0.0, recovery=0.0

## Full timeline

| t (s) | phase | fsm_state | height | target_subnet_peers | total_peers |
|---:|---|---|---:|---:|---:|
| 0.0 | baseline | ANCHORED | 430 | 0 | 6 |
| 5.1 | baseline | ANCHORED | 435 | 0 | 6 |
| 10.2 | baseline | ANCHORED | 439 | 0 | 6 |
| 15.2 | baseline | ANCHORED | 443 | 0 | 6 |
| 20.3 | baseline | ANCHORED | 447 | 0 | 6 |
| 25.3 | baseline | ANCHORED | 451 | 0 | 6 |
| 30.4 | attack | ANCHORED | 455 | 0 | 6 |
| 35.4 | attack | SUSPICIOUS | 459 | 0 | 6 |
| 40.4 | attack | SUSPICIOUS | 463 | 0 | 6 |
| 45.5 | attack | ANCHORED | 467 | 0 | 6 |
| 50.5 | attack | ANCHORED | 471 | 0 | 6 |
| 55.6 | attack | ANCHORED | 475 | 0 | 6 |
| 60.6 | attack | ANCHORED | 479 | 0 | 6 |
| 65.6 | attack | ANCHORED | 483 | 0 | 6 |
| 70.7 | attack | ANCHORED | 487 | 0 | 6 |
| 75.7 | attack | ANCHORED | 491 | 0 | 6 |
| 80.8 | attack | ANCHORED | 494 | 0 | 6 |
| 85.9 | attack | ANCHORED | 498 | 0 | 6 |
| 91.0 | attack | ANCHORED | 502 | 0 | 6 |
| 96.0 | attack | SUSPICIOUS | 506 | 0 | 6 |
| 101.0 | attack | SUSPICIOUS | 510 | 0 | 6 |
| 106.1 | attack | ANCHORED | 514 | 0 | 6 |
| 111.1 | attack | ANCHORED | 518 | 0 | 6 |
| 116.2 | attack | ANCHORED | 522 | 0 | 6 |
| 121.2 | recovery | ANCHORED | 526 | 0 | 6 |
| 126.3 | recovery | ANCHORED | 530 | 0 | 6 |
| 131.3 | recovery | ANCHORED | 534 | 0 | 6 |
| 136.3 | recovery | ANCHORED | 538 | 0 | 6 |
| 141.4 | recovery | ANCHORED | 542 | 0 | 6 |
| 146.4 | recovery | ANCHORED | 546 | 0 | 6 |
| 151.4 | recovery | ANCHORED | 550 | 0 | 6 |
| 156.5 | recovery | SUSPICIOUS | 553 | 0 | 6 |
| 161.5 | recovery | SUSPICIOUS | 558 | 0 | 6 |
| 166.6 | recovery | ANCHORED | 561 | 0 | 6 |
| 171.7 | recovery | ANCHORED | 566 | 0 | 6 |
| 176.7 | recovery | ANCHORED | 570 | 0 | 6 |
