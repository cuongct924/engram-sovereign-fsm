# LIVE Sybil/slot-exhaustion attack, leg 'a1'

10 attackers, all on engram-net (same subnet as the 4 real validators)

Total duration: 122s. MaxPeersPerSubnet=8.

## Real observed subnet-peer counts (target: engram-node01, subnet 172.28.0.0/24)

- Baseline (pre-attack): 0
- Peak during attack: 8 (peak total peers across all subnets: 11)
- After teardown (recovery): 0

## Verdict

- Ingress filter held the target subnet at or below MaxPeersPerSubnet during the attack: **True**
- FSM state never left ANCHORED/SUSPICIOUS during the attack (no false SOVEREIGN degradation from a defended attack): **True**
- Safety held (all 4 validators' AppHash never diverged at the same height): **True**
- Divergence events: 0
- Liveness held (block rate during attack vs. baseline, no collapse toward 0): **True** (baseline 0.826 blocks/s, attack 0.831 blocks/s, recovery 0.843 blocks/s)
- engram-node01's own nil-prevote ratio (fraction of committed heights where its precommit was nil, not just 'no divergence'): baseline=0.0, attack=0.0, recovery=0.0

## Full timeline

| t (s) | phase | fsm_state | height | target_subnet_peers | total_peers |
|---:|---|---|---:|---:|---:|
| 0.0 | baseline | SOVEREIGN | 194 | 0 | 3 |
| 3.0 | baseline | SOVEREIGN | 196 | 0 | 3 |
| 6.1 | baseline | SOVEREIGN | 199 | 0 | 3 |
| 9.1 | baseline | SOVEREIGN | 201 | 0 | 3 |
| 12.1 | baseline | SOVEREIGN | 204 | 0 | 3 |
| 28.9 | attack | SOVEREIGN | 217 | 0 | 3 |
| 32.0 | attack | SOVEREIGN | 220 | 8 | 11 |
| 35.1 | attack | SOVEREIGN | 222 | 8 | 11 |
| 38.1 | attack | SOVEREIGN | 225 | 8 | 11 |
| 41.2 | attack | SOVEREIGN | 227 | 8 | 11 |
| 44.2 | attack | SOVEREIGN | 230 | 8 | 11 |
| 47.2 | attack | SOVEREIGN | 233 | 8 | 11 |
| 50.3 | attack | SOVEREIGN | 235 | 8 | 11 |
| 53.3 | attack | SOVEREIGN | 238 | 8 | 11 |
| 56.3 | attack | SOVEREIGN | 240 | 8 | 11 |
| 59.4 | attack | SOVEREIGN | 243 | 8 | 11 |
| 62.4 | attack | SOVEREIGN | 245 | 8 | 11 |
| 65.4 | attack | SOVEREIGN | 248 | 8 | 11 |
| 68.4 | attack | SOVEREIGN | 250 | 8 | 11 |
| 71.5 | attack | SOVEREIGN | 252 | 8 | 11 |
| 74.6 | attack | SOVEREIGN | 255 | 8 | 11 |
| 77.6 | attack | SOVEREIGN | 258 | 8 | 11 |
| 80.6 | attack | SOVEREIGN | 260 | 8 | 11 |
| 83.7 | attack | SOVEREIGN | 263 | 8 | 11 |
| 86.7 | attack | SOVEREIGN | 265 | 8 | 11 |
| 91.5 | recovery | SOVEREIGN | 269 | 0 | 3 |
| 94.6 | recovery | SOVEREIGN | 272 | 0 | 3 |
| 97.6 | recovery | SOVEREIGN | 274 | 0 | 3 |
| 100.6 | recovery | SOVEREIGN | 277 | 0 | 3 |
| 103.7 | recovery | SOVEREIGN | 279 | 0 | 3 |
| 106.7 | recovery | SOVEREIGN | 282 | 0 | 3 |
| 109.7 | recovery | SOVEREIGN | 284 | 0 | 3 |
| 112.7 | recovery | SOVEREIGN | 287 | 0 | 3 |
| 115.8 | recovery | SOVEREIGN | 289 | 0 | 3 |
| 118.8 | recovery | SOVEREIGN | 292 | 0 | 3 |
