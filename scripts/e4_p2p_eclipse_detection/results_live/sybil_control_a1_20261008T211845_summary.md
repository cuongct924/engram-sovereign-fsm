# LIVE Sybil/slot-exhaustion attack, leg 'a1'

10 attackers, all on engram-net (same subnet as the 4 real validators)

Total duration: 183s. MaxPeersPerSubnet=8.

## Real observed subnet-peer counts (target: engram-node01, subnet 172.28.0.0/24)

- Baseline (pre-attack): 0
- Peak during attack: 0 (peak total peers across all subnets: 6)
- After teardown (recovery): 0

## Verdict

- Ingress filter held the target subnet at or below MaxPeersPerSubnet during the attack: **True**
- FSM state never left ANCHORED/SUSPICIOUS during the attack (no false SOVEREIGN degradation from a defended attack): **True**
- Safety held (all 4 validators' AppHash never diverged at the same height): **True**
- Divergence events: 0
- Liveness held (block rate during attack vs. baseline, no collapse toward 0): **True** (baseline 0.788 blocks/s, attack 0.813 blocks/s, recovery 0.789 blocks/s)
- engram-node01's own nil-prevote ratio (fraction of committed heights where its precommit was nil, not just 'no divergence'): baseline=0.0, attack=0.0, recovery=0.0

## Full timeline

| t (s) | phase | fsm_state | height | target_subnet_peers | total_peers |
|---:|---|---|---:|---:|---:|
| 0.0 | baseline | ANCHORED | 382 | 0 | 6 |
| 5.1 | baseline | ANCHORED | 386 | 0 | 6 |
| 10.2 | baseline | SUSPICIOUS | 390 | 0 | 6 |
| 15.2 | baseline | SUSPICIOUS | 394 | 0 | 6 |
| 20.3 | baseline | ANCHORED | 398 | 0 | 6 |
| 25.4 | baseline | ANCHORED | 402 | 0 | 6 |
| 30.5 | attack | ANCHORED | 406 | 0 | 6 |
| 35.5 | attack | ANCHORED | 410 | 0 | 6 |
| 40.6 | attack | ANCHORED | 414 | 0 | 6 |
| 45.6 | attack | ANCHORED | 419 | 0 | 6 |
| 50.7 | attack | ANCHORED | 423 | 0 | 6 |
| 55.8 | attack | ANCHORED | 427 | 0 | 6 |
| 60.9 | attack | ANCHORED | 431 | 0 | 6 |
| 65.9 | attack | ANCHORED | 435 | 0 | 6 |
| 71.0 | attack | SUSPICIOUS | 439 | 0 | 6 |
| 76.1 | attack | SUSPICIOUS | 443 | 0 | 6 |
| 81.2 | attack | ANCHORED | 447 | 0 | 6 |
| 86.3 | attack | ANCHORED | 452 | 0 | 6 |
| 91.3 | attack | ANCHORED | 456 | 0 | 6 |
| 96.4 | attack | ANCHORED | 459 | 0 | 6 |
| 101.4 | attack | ANCHORED | 464 | 0 | 6 |
| 106.5 | attack | ANCHORED | 468 | 0 | 6 |
| 111.6 | attack | ANCHORED | 472 | 0 | 6 |
| 116.6 | attack | ANCHORED | 476 | 0 | 6 |
| 121.7 | recovery | ANCHORED | 480 | 0 | 6 |
| 126.8 | recovery | SUSPICIOUS | 484 | 0 | 6 |
| 131.8 | recovery | SUSPICIOUS | 488 | 0 | 6 |
| 136.9 | recovery | SUSPICIOUS | 491 | 0 | 6 |
| 141.9 | recovery | ANCHORED | 496 | 0 | 6 |
| 147.0 | recovery | ANCHORED | 500 | 0 | 6 |
| 152.1 | recovery | ANCHORED | 503 | 0 | 6 |
| 157.1 | recovery | ANCHORED | 508 | 0 | 6 |
| 162.2 | recovery | ANCHORED | 512 | 0 | 6 |
| 167.3 | recovery | ANCHORED | 516 | 0 | 6 |
| 172.4 | recovery | ANCHORED | 520 | 0 | 6 |
| 177.5 | recovery | ANCHORED | 524 | 0 | 6 |
