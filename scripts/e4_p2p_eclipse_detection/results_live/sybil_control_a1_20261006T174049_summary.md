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
- Liveness held (block rate during attack vs. baseline, no collapse toward 0): **True** (baseline 0.754 blocks/s, attack 0.804 blocks/s, recovery 0.774 blocks/s)
- engram-node01's own nil-prevote ratio (fraction of committed heights where its precommit was nil, not just 'no divergence'): baseline=0.0, attack=0.0, recovery=0.0

## Full timeline

| t (s) | phase | fsm_state | height | target_subnet_peers | total_peers |
|---:|---|---|---:|---:|---:|
| 0.0 | baseline | ANCHORED | 431 | 0 | 6 |
| 5.0 | baseline | ANCHORED | 435 | 0 | 6 |
| 10.1 | baseline | ANCHORED | 439 | 0 | 6 |
| 15.1 | baseline | ANCHORED | 442 | 0 | 6 |
| 20.2 | baseline | ANCHORED | 446 | 0 | 6 |
| 25.2 | baseline | ANCHORED | 450 | 0 | 6 |
| 30.3 | attack | ANCHORED | 454 | 0 | 6 |
| 35.3 | attack | SUSPICIOUS | 458 | 0 | 6 |
| 40.4 | attack | SUSPICIOUS | 463 | 0 | 6 |
| 45.4 | attack | ANCHORED | 467 | 0 | 6 |
| 50.4 | attack | ANCHORED | 471 | 0 | 6 |
| 55.5 | attack | ANCHORED | 475 | 0 | 6 |
| 60.5 | attack | ANCHORED | 479 | 0 | 6 |
| 65.6 | attack | ANCHORED | 483 | 0 | 6 |
| 70.6 | attack | ANCHORED | 487 | 0 | 6 |
| 75.7 | attack | ANCHORED | 491 | 0 | 6 |
| 80.7 | attack | ANCHORED | 495 | 0 | 6 |
| 85.8 | attack | ANCHORED | 499 | 0 | 6 |
| 90.8 | attack | ANCHORED | 503 | 0 | 6 |
| 95.9 | attack | SUSPICIOUS | 507 | 0 | 6 |
| 101.0 | attack | SUSPICIOUS | 511 | 0 | 6 |
| 106.0 | attack | ANCHORED | 515 | 0 | 6 |
| 111.0 | attack | ANCHORED | 519 | 0 | 6 |
| 116.1 | attack | ANCHORED | 523 | 0 | 6 |
| 121.1 | recovery | ANCHORED | 527 | 0 | 6 |
| 126.2 | recovery | ANCHORED | 531 | 0 | 6 |
| 131.2 | recovery | ANCHORED | 535 | 0 | 6 |
| 136.3 | recovery | ANCHORED | 539 | 0 | 6 |
| 141.4 | recovery | ANCHORED | 543 | 0 | 6 |
| 146.4 | recovery | ANCHORED | 547 | 0 | 6 |
| 151.5 | recovery | ANCHORED | 551 | 0 | 6 |
| 156.5 | recovery | SUSPICIOUS | 555 | 0 | 6 |
| 161.5 | recovery | SUSPICIOUS | 559 | 0 | 6 |
| 166.6 | recovery | ANCHORED | 563 | 0 | 6 |
| 171.6 | recovery | ANCHORED | 566 | 0 | 6 |
| 176.7 | recovery | ANCHORED | 570 | 0 | 6 |
