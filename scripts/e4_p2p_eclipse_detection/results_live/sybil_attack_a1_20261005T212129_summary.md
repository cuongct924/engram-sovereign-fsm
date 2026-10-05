# LIVE Sybil/slot-exhaustion attack, leg 'a1'

10 attackers, all on engram-net (same subnet as the 4 real validators)

Total duration: 202s. MaxPeersPerSubnet=8.

## Real observed subnet-peer counts (target: engram-node01, subnet 172.28.0.0/24)

- Baseline (pre-attack): 0
- Peak during attack: 8 (peak total peers across all subnets: 14)
- After teardown (recovery): 0

## Verdict

- Ingress filter held the target subnet at or below MaxPeersPerSubnet during the attack: **True**
- FSM state never left ANCHORED/SUSPICIOUS during the attack (no false SOVEREIGN degradation from a defended attack): **True**
- Safety held (all 4 validators' AppHash never diverged at the same height): **True**
- Divergence events: 0
- Liveness held (block rate during attack vs. baseline, no collapse toward 0): **True** (baseline 0.794 blocks/s, attack 0.800 blocks/s, recovery 0.788 blocks/s)
- engram-node01's own nil-prevote ratio (fraction of committed heights where its precommit was nil, not just 'no divergence'): baseline=0.0, attack=0.0, recovery=0.0

## Full timeline

| t (s) | phase | fsm_state | height | target_subnet_peers | total_peers |
|---:|---|---|---:|---:|---:|
| 0.0 | baseline | ANCHORED | 370 | 0 | 6 |
| 5.0 | baseline | ANCHORED | 374 | 0 | 6 |
| 10.1 | baseline | ANCHORED | 379 | 0 | 6 |
| 15.1 | baseline | ANCHORED | 383 | 0 | 6 |
| 20.2 | baseline | SUSPICIOUS | 387 | 0 | 6 |
| 25.2 | baseline | SUSPICIOUS | 390 | 0 | 6 |
| 45.1 | attack | ANCHORED | 405 | 0 | 6 |
| 50.9 | attack | ANCHORED | 409 | 8 | 14 |
| 56.7 | attack | ANCHORED | 414 | 8 | 14 |
| 61.9 | attack | ANCHORED | 418 | 8 | 14 |
| 66.9 | attack | ANCHORED | 422 | 8 | 14 |
| 71.9 | attack | ANCHORED | 426 | 8 | 14 |
| 77.0 | attack | SUSPICIOUS | 430 | 8 | 14 |
| 82.0 | attack | SUSPICIOUS | 435 | 8 | 14 |
| 87.1 | attack | ANCHORED | 439 | 8 | 14 |
| 92.2 | attack | ANCHORED | 443 | 8 | 14 |
| 97.2 | attack | ANCHORED | 447 | 8 | 14 |
| 102.3 | attack | ANCHORED | 451 | 8 | 14 |
| 107.4 | attack | ANCHORED | 455 | 8 | 14 |
| 112.4 | attack | ANCHORED | 460 | 8 | 14 |
| 117.5 | attack | ANCHORED | 464 | 8 | 14 |
| 122.5 | attack | ANCHORED | 468 | 8 | 14 |
| 127.6 | attack | ANCHORED | 472 | 8 | 14 |
| 132.6 | attack | ANCHORED | 476 | 8 | 14 |
| 139.6 | recovery | SUSPICIOUS | 482 | 0 | 6 |
| 144.6 | recovery | SUSPICIOUS | 486 | 0 | 6 |
| 150.4 | recovery | ANCHORED | 490 | 0 | 6 |
| 156.1 | recovery | ANCHORED | 494 | 0 | 6 |
| 161.2 | recovery | ANCHORED | 498 | 0 | 6 |
| 166.3 | recovery | ANCHORED | 502 | 0 | 6 |
| 171.3 | recovery | ANCHORED | 506 | 0 | 6 |
| 176.4 | recovery | ANCHORED | 510 | 0 | 6 |
| 181.5 | recovery | ANCHORED | 514 | 0 | 6 |
| 186.5 | recovery | ANCHORED | 519 | 0 | 6 |
| 191.6 | recovery | ANCHORED | 523 | 0 | 6 |
| 196.6 | recovery | SUSPICIOUS | 527 | 0 | 6 |
