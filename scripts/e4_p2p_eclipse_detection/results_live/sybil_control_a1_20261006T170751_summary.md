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
- Liveness held (block rate during attack vs. baseline, no collapse toward 0): **True** (baseline 0.791 blocks/s, attack 0.802 blocks/s, recovery 0.792 blocks/s)
- engram-node01's own nil-prevote ratio (fraction of committed heights where its precommit was nil, not just 'no divergence'): baseline=0.0, attack=0.0, recovery=0.0

## Full timeline

| t (s) | phase | fsm_state | height | target_subnet_peers | total_peers |
|---:|---|---|---:|---:|---:|
| 0.0 | baseline | ANCHORED | 433 | 0 | 6 |
| 5.0 | baseline | ANCHORED | 436 | 0 | 6 |
| 10.1 | baseline | ANCHORED | 440 | 0 | 6 |
| 15.2 | baseline | ANCHORED | 445 | 0 | 6 |
| 20.2 | baseline | ANCHORED | 449 | 0 | 6 |
| 25.3 | baseline | ANCHORED | 453 | 0 | 6 |
| 30.3 | attack | ANCHORED | 456 | 0 | 6 |
| 35.4 | attack | SUSPICIOUS | 460 | 0 | 6 |
| 40.5 | attack | SUSPICIOUS | 464 | 0 | 6 |
| 45.5 | attack | ANCHORED | 468 | 0 | 6 |
| 50.5 | attack | ANCHORED | 472 | 0 | 6 |
| 55.6 | attack | ANCHORED | 476 | 0 | 6 |
| 60.6 | attack | ANCHORED | 480 | 0 | 6 |
| 65.7 | attack | ANCHORED | 484 | 0 | 6 |
| 70.7 | attack | ANCHORED | 488 | 0 | 6 |
| 75.8 | attack | ANCHORED | 492 | 0 | 6 |
| 80.8 | attack | ANCHORED | 496 | 0 | 6 |
| 85.9 | attack | ANCHORED | 500 | 0 | 6 |
| 90.9 | attack | ANCHORED | 504 | 0 | 6 |
| 96.0 | attack | SUSPICIOUS | 508 | 0 | 6 |
| 101.1 | attack | SUSPICIOUS | 512 | 0 | 6 |
| 106.1 | attack | ANCHORED | 516 | 0 | 6 |
| 111.2 | attack | ANCHORED | 520 | 0 | 6 |
| 116.3 | attack | ANCHORED | 525 | 0 | 6 |
| 121.4 | recovery | ANCHORED | 529 | 0 | 6 |
| 126.4 | recovery | ANCHORED | 533 | 0 | 6 |
| 131.4 | recovery | ANCHORED | 537 | 0 | 6 |
| 136.5 | recovery | ANCHORED | 541 | 0 | 6 |
| 141.5 | recovery | ANCHORED | 545 | 0 | 6 |
| 146.6 | recovery | ANCHORED | 549 | 0 | 6 |
| 151.6 | recovery | ANCHORED | 553 | 0 | 6 |
| 156.7 | recovery | SUSPICIOUS | 557 | 0 | 6 |
| 161.7 | recovery | SUSPICIOUS | 561 | 0 | 6 |
| 166.8 | recovery | ANCHORED | 565 | 0 | 6 |
| 171.8 | recovery | ANCHORED | 569 | 0 | 6 |
| 176.9 | recovery | ANCHORED | 573 | 0 | 6 |
