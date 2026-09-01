# LIVE Sybil/slot-exhaustion attack, leg 'a2'

12 attackers across 4 distinct subnets (attacker-subnet-a/b/c/d, 3 each)

Total duration: 124s. MaxPeersPerSubnet=8.

## Real observed subnet-peer counts (target: engram-node01, subnet 172.28.0.0/24)

- Baseline (pre-attack): 0
- Peak during attack: 8 (peak total peers across all subnets: 11)
- After teardown (recovery): 0

## Verdict

- Ingress filter held the target subnet at or below MaxPeersPerSubnet during the attack: **True**
- FSM state never left ANCHORED/SUSPICIOUS during the attack (no false SOVEREIGN degradation from a defended attack): **True**
- Safety held (all 4 validators' AppHash never diverged at the same height): **True**
- Divergence events: 0
- Liveness held (block rate during attack vs. baseline, no collapse toward 0): **True** (baseline 0.812 blocks/s, attack 0.794 blocks/s, recovery 0.806 blocks/s)
- engram-node01's own nil-prevote ratio (fraction of committed heights where its precommit was nil, not just 'no divergence'): baseline=0.0, attack=0.0, recovery=0.0

## Full timeline

| t (s) | phase | fsm_state | height | target_subnet_peers | total_peers |
|---:|---|---|---:|---:|---:|
| 0.0 | baseline | SOVEREIGN | 315 | 0 | 3 |
| 3.0 | baseline | SOVEREIGN | 317 | 0 | 3 |
| 6.1 | baseline | SOVEREIGN | 320 | 0 | 3 |
| 9.1 | baseline | SOVEREIGN | 322 | 0 | 3 |
| 12.3 | baseline | SOVEREIGN | 325 | 0 | 3 |
| 30.3 | attack | SOVEREIGN | 339 | 0 | 3 |
| 33.3 | attack | SOVEREIGN | 342 | 8 | 11 |
| 36.4 | attack | SOVEREIGN | 344 | 8 | 11 |
| 39.4 | attack | SOVEREIGN | 347 | 8 | 11 |
| 42.5 | attack | SOVEREIGN | 349 | 8 | 11 |
| 45.6 | attack | SOVEREIGN | 352 | 8 | 11 |
| 48.6 | attack | SOVEREIGN | 354 | 8 | 11 |
| 51.6 | attack | SOVEREIGN | 357 | 8 | 11 |
| 54.7 | attack | SOVEREIGN | 359 | 8 | 11 |
| 57.7 | attack | SOVEREIGN | 362 | 8 | 11 |
| 60.7 | attack | SOVEREIGN | 364 | 8 | 11 |
| 63.9 | attack | SOVEREIGN | 366 | 8 | 11 |
| 66.9 | attack | SOVEREIGN | 369 | 8 | 11 |
| 70.0 | attack | SOVEREIGN | 371 | 8 | 11 |
| 73.0 | attack | SOVEREIGN | 373 | 8 | 11 |
| 76.1 | attack | SOVEREIGN | 376 | 8 | 11 |
| 79.1 | attack | SOVEREIGN | 378 | 8 | 11 |
| 82.1 | attack | SOVEREIGN | 380 | 8 | 11 |
| 85.2 | attack | SOVEREIGN | 383 | 8 | 11 |
| 88.2 | attack | SOVEREIGN | 385 | 8 | 11 |
| 93.8 | recovery | SOVEREIGN | 390 | 0 | 3 |
| 96.8 | recovery | SOVEREIGN | 392 | 0 | 3 |
| 99.9 | recovery | SOVEREIGN | 395 | 0 | 3 |
| 102.9 | recovery | SOVEREIGN | 397 | 0 | 3 |
| 105.9 | recovery | SOVEREIGN | 400 | 0 | 3 |
| 108.9 | recovery | SOVEREIGN | 402 | 0 | 3 |
| 112.0 | recovery | SOVEREIGN | 405 | 0 | 3 |
| 115.0 | recovery | SOVEREIGN | 407 | 0 | 3 |
| 118.0 | recovery | SOVEREIGN | 410 | 0 | 3 |
| 121.0 | recovery | SOVEREIGN | 412 | 0 | 3 |
