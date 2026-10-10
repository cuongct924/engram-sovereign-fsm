# LIVE Sybil/slot-exhaustion attack, leg 'a1'

10 attackers, all on engram-net (same subnet as the 4 real validators)

Total duration: 182s. MaxPeersPerSubnet=8.

## Real observed subnet-peer counts (target: engram-node01, subnet 172.28.0.0/24)

- Baseline (pre-attack): 0
- Peak during attack: 0 (peak total peers across all subnets: 6)
- After teardown (recovery): 0

## Verdict

- Ingress filter held the target subnet at or below MaxPeersPerSubnet during the attack: **True**
- FSM state never left ANCHORED/SUSPICIOUS during the attack (no false SOVEREIGN degradation from a defended attack): **True**
- Safety held (all 4 validators' AppHash never diverged at the same height): **True**
- Divergence events: 0
- Liveness held (block rate during attack vs. baseline, no collapse toward 0): **True** (baseline 0.830 blocks/s, attack 0.801 blocks/s, recovery 0.826 blocks/s)
- engram-node01's own nil-prevote ratio (fraction of committed heights where its precommit was nil, not just 'no divergence'): baseline=0.0, attack=0.0, recovery=0.0

## Full timeline

| t (s) | phase | fsm_state | height | target_subnet_peers | total_peers |
|---:|---|---|---:|---:|---:|
| 0.0 | baseline | ANCHORED | 379 | 0 | 6 |
| 5.0 | baseline | SUSPICIOUS | 383 | 0 | 6 |
| 10.2 | baseline | SUSPICIOUS | 387 | 0 | 6 |
| 15.2 | baseline | ANCHORED | 392 | 0 | 6 |
| 20.3 | baseline | ANCHORED | 396 | 0 | 6 |
| 25.3 | baseline | ANCHORED | 400 | 0 | 6 |
| 30.4 | attack | ANCHORED | 404 | 0 | 6 |
| 35.4 | attack | ANCHORED | 408 | 0 | 6 |
| 40.5 | attack | ANCHORED | 412 | 0 | 6 |
| 45.6 | attack | ANCHORED | 416 | 0 | 6 |
| 50.7 | attack | ANCHORED | 420 | 0 | 6 |
| 55.7 | attack | ANCHORED | 424 | 0 | 6 |
| 60.8 | attack | ANCHORED | 428 | 0 | 6 |
| 65.9 | attack | SUSPICIOUS | 433 | 0 | 6 |
| 70.9 | attack | SUSPICIOUS | 437 | 0 | 6 |
| 76.0 | attack | ANCHORED | 441 | 0 | 6 |
| 81.0 | attack | ANCHORED | 445 | 0 | 6 |
| 86.1 | attack | ANCHORED | 449 | 0 | 6 |
| 91.2 | attack | ANCHORED | 453 | 0 | 6 |
| 96.2 | attack | ANCHORED | 457 | 0 | 6 |
| 101.3 | attack | ANCHORED | 461 | 0 | 6 |
| 106.4 | attack | ANCHORED | 465 | 0 | 6 |
| 111.5 | attack | ANCHORED | 469 | 0 | 6 |
| 116.5 | attack | ANCHORED | 473 | 0 | 6 |
| 121.6 | recovery | ANCHORED | 477 | 0 | 6 |
| 126.7 | recovery | SUSPICIOUS | 481 | 0 | 6 |
| 131.7 | recovery | SUSPICIOUS | 486 | 0 | 6 |
| 136.8 | recovery | ANCHORED | 490 | 0 | 6 |
| 141.9 | recovery | ANCHORED | 494 | 0 | 6 |
| 146.9 | recovery | ANCHORED | 498 | 0 | 6 |
| 152.0 | recovery | ANCHORED | 502 | 0 | 6 |
| 157.0 | recovery | ANCHORED | 506 | 0 | 6 |
| 162.1 | recovery | ANCHORED | 511 | 0 | 6 |
| 167.2 | recovery | ANCHORED | 515 | 0 | 6 |
| 172.2 | recovery | ANCHORED | 519 | 0 | 6 |
| 177.3 | recovery | ANCHORED | 523 | 0 | 6 |
