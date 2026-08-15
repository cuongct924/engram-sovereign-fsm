# LIVE Sybil/slot-exhaustion attack, leg 'a1'

10 attackers, all on engram-net (same subnet as the 4 real validators)

Total duration: 202s. MaxPeersPerSubnet=8.

## Real observed subnet-peer counts (target: engram-node01, subnet 172.28.0.0/24)

- Baseline (pre-attack): 0
- Peak during attack: 8 (peak total peers across all subnets: 11)
- After teardown (recovery): 0

## Verdict

- Ingress filter held the target subnet at or below MaxPeersPerSubnet during the attack: **True**
- FSM state never left ANCHORED/SUSPICIOUS during the attack (no false SOVEREIGN degradation from a defended attack): **True**
- Safety held (all 4 validators' AppHash never diverged at the same height): **True**
- Divergence events: 0
- Liveness held (block rate during attack vs. baseline, no collapse toward 0): **True** (baseline 0.654 blocks/s, attack 0.570 blocks/s, recovery 0.623 blocks/s)

## Resource usage during attack (docker stats)

| Container | Avg CPU% | Max CPU% | Avg Mem (MiB) | Max Mem (MiB) |
|---|---:|---:|---:|---:|
| engram-node01 | 8.31 | 24.76 | 137.8 | 146.7 |
| engram-node02 | 6.46 | 15.53 | 124.2 | 124.3 |
| engram-node03 | 8.11 | 32.84 | 126.9 | 127.2 |
| engram-node04 | 5.86 | 19.85 | 126.7 | 127.1 |
| attacker-a1-01 | 10.08 | 43.01 | 36.9 | 45.9 |
| attacker-a1-02 | 9.82 | 52.52 | 33.4 | 42.0 |
| attacker-a1-03 | 11.39 | 65.84 | 40.2 | 50.7 |
| attacker-a1-04 | 9.97 | 48.26 | 39.0 | 48.4 |
| attacker-a1-05 | 9.19 | 45.78 | 31.6 | 35.1 |
| attacker-a1-06 | 8.04 | 27.56 | 39.5 | 50.3 |
| attacker-a1-07 | 8.42 | 32.34 | 32.3 | 35.3 |
| attacker-a1-08 | 12.15 | 61.51 | 37.4 | 48.1 |
| attacker-a1-09 | 10.75 | 60.58 | 36.8 | 46.0 |
| attacker-a1-10 | 9.85 | 33.94 | 39.6 | 49.5 |

## Full timeline

| t (s) | phase | fsm_state | height | target_subnet_peers | total_peers |
|---:|---|---|---:|---:|---:|
| 0.0 | baseline | ANCHORED | 3458 | 0 | 3 |
| 6.1 | baseline | ANCHORED | 3462 | 0 | 3 |
| 13.3 | baseline | ANCHORED | 3466 | 0 | 3 |
| 20.4 | baseline | ANCHORED | 3471 | 0 | 3 |
| 27.5 | baseline | ANCHORED | 3476 | 0 | 3 |
| 41.2 | attack | ANCHORED | 3482 | 0 | 3 |
| 49.0 | attack | ANCHORED | 3485 | 5 | 8 |
| 56.3 | attack | ANCHORED | 3490 | 8 | 11 |
| 63.8 | attack | ANCHORED | 3494 | 8 | 11 |
| 72.2 | attack | ANCHORED | 3498 | 8 | 11 |
| 79.4 | attack | ANCHORED | 3502 | 8 | 11 |
| 86.9 | attack | ANCHORED | 3507 | 8 | 11 |
| 94.2 | attack | ANCHORED | 3510 | 8 | 11 |
| 101.2 | attack | ANCHORED | 3514 | 8 | 11 |
| 108.3 | attack | ANCHORED | 3519 | 8 | 11 |
| 115.5 | attack | ANCHORED | 3523 | 8 | 11 |
| 122.6 | attack | ANCHORED | 3528 | 8 | 11 |
| 129.0 | attack | ANCHORED | 3532 | 8 | 11 |
| 138.9 | recovery | ANCHORED | 3538 | 0 | 3 |
| 146.1 | recovery | ANCHORED | 3542 | 0 | 3 |
| 153.2 | recovery | ANCHORED | 3546 | 0 | 3 |
| 160.0 | recovery | ANCHORED | 3550 | 0 | 3 |
| 167.1 | recovery | ANCHORED | 3554 | 0 | 3 |
| 174.2 | recovery | ANCHORED | 3559 | 0 | 3 |
| 180.8 | recovery | ANCHORED | 3563 | 0 | 3 |
| 188.0 | recovery | ANCHORED | 3568 | 0 | 3 |
| 195.1 | recovery | ANCHORED | 3573 | 0 | 3 |
