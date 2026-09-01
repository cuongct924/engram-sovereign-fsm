# LIVE E9 combined-failure trace (BTC + DA + P2P churn, overlapping)

Total duration: 352s. Final phase reached ANCHORED: True. WAN-realism baseline: chaos-wan-latency.

## Fault-injection / healing event markers

| t (s) | Event |
|---:|---|
| 24 | chaos-btc-delay started (BTC congestion pressure) |
| 97 | all 7 celestia bridges stopped (DA outage layered on BTC pressure) |
| 163 | chaos-loss churn burst starting (3rd simultaneous fault class) |
| 274 | P2P churn burst complete |
| 310 | chaos-btc-delay cleaned up |
| 314 | all 7 celestia bridges restarted |

## Real transitions observed

| t (s) | Phase | Node | From | To |
|---:|---|---|---|---|
| 12 | P1_baseline | engram-node01 | ANCHORED | SUSPICIOUS |
| 12 | P1_baseline | engram-node02 | ANCHORED | SUSPICIOUS |
| 16 | P1_baseline | engram-node01 | SUSPICIOUS | ANCHORED |
| 16 | P1_baseline | engram-node02 | SUSPICIOUS | ANCHORED |
| 48 | P2_btc_pressure_building | engram-node01 | ANCHORED | SOVEREIGN |
| 48 | P2_btc_pressure_building | engram-node02 | ANCHORED | SOVEREIGN |
| 48 | P2_btc_pressure_building | engram-node04 | ANCHORED | SOVEREIGN |
| 48 | P2_btc_pressure_building | engram-node05 | ANCHORED | SOVEREIGN |
| 48 | P2_btc_pressure_building | engram-node06 | ANCHORED | SOVEREIGN |
| 48 | P2_btc_pressure_building | engram-node07 | ANCHORED | SOVEREIGN |
| 51 | P2_btc_pressure_building | engram-node03 | ANCHORED | SOVEREIGN |
| 326 | P6_healing | engram-node03 | SOVEREIGN | RECOVERING |
| 326 | P6_healing | engram-node04 | SOVEREIGN | RECOVERING |
| 326 | P6_healing | engram-node05 | SOVEREIGN | RECOVERING |
| 326 | P6_healing | engram-node06 | SOVEREIGN | RECOVERING |
| 326 | P6_healing | engram-node07 | SOVEREIGN | RECOVERING |
| 329 | P6_healing | engram-node01 | SOVEREIGN | RECOVERING |
| 329 | P6_healing | engram-node02 | SOVEREIGN | RECOVERING |
| 333 | P6_healing | engram-node01 | RECOVERING | ANCHORED |
| 333 | P6_healing | engram-node02 | RECOVERING | ANCHORED |
| 333 | P6_healing | engram-node03 | RECOVERING | ANCHORED |
| 333 | P6_healing | engram-node04 | RECOVERING | ANCHORED |
| 333 | P6_healing | engram-node05 | RECOVERING | ANCHORED |
| 333 | P6_healing | engram-node06 | RECOVERING | ANCHORED |
| 333 | P6_healing | engram-node07 | RECOVERING | ANCHORED |
| 340 | P6_healing | engram-node01 | ANCHORED | SUSPICIOUS |
| 340 | P6_healing | engram-node02 | ANCHORED | SUSPICIOUS |
| 340 | P6_healing | engram-node03 | ANCHORED | SUSPICIOUS |
| 340 | P6_healing | engram-node04 | ANCHORED | SUSPICIOUS |
| 340 | P6_healing | engram-node05 | ANCHORED | SUSPICIOUS |
| 340 | P6_healing | engram-node06 | ANCHORED | SUSPICIOUS |
| 340 | P6_healing | engram-node07 | ANCHORED | SUSPICIOUS |
| 343 | P6_healing | engram-node01 | SUSPICIOUS | ANCHORED |
| 343 | P6_healing | engram-node02 | SUSPICIOUS | ANCHORED |
| 343 | P6_healing | engram-node03 | SUSPICIOUS | ANCHORED |
| 343 | P6_healing | engram-node04 | SUSPICIOUS | ANCHORED |
| 343 | P6_healing | engram-node05 | SUSPICIOUS | ANCHORED |
| 343 | P6_healing | engram-node06 | SUSPICIOUS | ANCHORED |
| 343 | P6_healing | engram-node07 | SUSPICIOUS | ANCHORED |
