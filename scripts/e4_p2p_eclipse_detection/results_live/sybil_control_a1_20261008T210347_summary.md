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
- Liveness held (block rate during attack vs. baseline, no collapse toward 0): **True** (baseline 0.832 blocks/s, attack 0.800 blocks/s, recovery 0.789 blocks/s)
- engram-node01's own nil-prevote ratio (fraction of committed heights where its precommit was nil, not just 'no divergence'): baseline=0.0, attack=0.0, recovery=0.0

## Full timeline

| t (s) | phase | fsm_state | height | target_subnet_peers | total_peers |
|---:|---|---|---:|---:|---:|
| 0.0 | baseline | ANCHORED | 377 | 0 | 6 |
| 5.1 | baseline | ANCHORED | 381 | 0 | 6 |
| 10.1 | baseline | ANCHORED | 385 | 0 | 6 |
| 15.2 | baseline | ANCHORED | 389 | 0 | 6 |
| 20.2 | baseline | SUSPICIOUS | 394 | 0 | 6 |
| 25.3 | baseline | SUSPICIOUS | 398 | 0 | 6 |
| 30.3 | attack | ANCHORED | 402 | 0 | 6 |
| 35.4 | attack | ANCHORED | 406 | 0 | 6 |
| 40.6 | attack | ANCHORED | 410 | 0 | 6 |
| 45.7 | attack | ANCHORED | 414 | 0 | 6 |
| 50.7 | attack | ANCHORED | 419 | 0 | 6 |
| 55.8 | attack | ANCHORED | 423 | 0 | 6 |
| 60.9 | attack | ANCHORED | 427 | 0 | 6 |
| 66.0 | attack | ANCHORED | 431 | 0 | 6 |
| 71.1 | attack | ANCHORED | 435 | 0 | 6 |
| 76.2 | attack | SUSPICIOUS | 439 | 0 | 6 |
| 81.2 | attack | SUSPICIOUS | 443 | 0 | 6 |
| 86.3 | attack | ANCHORED | 447 | 0 | 6 |
| 91.3 | attack | ANCHORED | 451 | 0 | 6 |
| 96.4 | attack | ANCHORED | 455 | 0 | 6 |
| 101.4 | attack | ANCHORED | 458 | 0 | 6 |
| 106.5 | attack | ANCHORED | 462 | 0 | 6 |
| 111.5 | attack | ANCHORED | 467 | 0 | 6 |
| 116.6 | attack | ANCHORED | 470 | 0 | 6 |
| 121.6 | recovery | ANCHORED | 475 | 0 | 6 |
| 126.7 | recovery | ANCHORED | 479 | 0 | 6 |
| 131.7 | recovery | ANCHORED | 483 | 0 | 6 |
| 136.8 | recovery | SUSPICIOUS | 487 | 0 | 6 |
| 141.8 | recovery | SUSPICIOUS | 491 | 0 | 6 |
| 146.9 | recovery | ANCHORED | 495 | 0 | 6 |
| 152.0 | recovery | ANCHORED | 499 | 0 | 6 |
| 157.1 | recovery | ANCHORED | 503 | 0 | 6 |
| 162.2 | recovery | ANCHORED | 507 | 0 | 6 |
| 167.2 | recovery | ANCHORED | 511 | 0 | 6 |
| 172.3 | recovery | ANCHORED | 515 | 0 | 6 |
| 177.4 | recovery | ANCHORED | 519 | 0 | 6 |
