# LIVE E9 combined-failure trace (BTC + DA + P2P churn, overlapping)

Total duration: 568s. Final phase reached ANCHORED: True. WAN-realism baseline: chaos-wan-latency.

## Fault-injection / healing event markers

| t (s) | Event |
|---:|---|
| 22 | chaos-btc-delay started (BTC congestion pressure) |
| 97 | all 7 celestia bridges stopped (DA outage layered on BTC pressure) |
| 164 | chaos-loss churn burst starting (3rd simultaneous fault class) |
| 269 | P2P churn burst complete |
| 303 | chaos-btc-delay cleaned up |
| 306 | all 7 celestia bridges restarted |

## Real transitions observed

| t (s) | Phase | Node | From | To |
|---:|---|---|---|---|
| 39 | P2_btc_pressure_building | engram-node02 | ANCHORED | SOVEREIGN |
| 39 | P2_btc_pressure_building | engram-node03 | ANCHORED | SOVEREIGN |
| 39 | P2_btc_pressure_building | engram-node04 | ANCHORED | SOVEREIGN |
| 39 | P2_btc_pressure_building | engram-node05 | ANCHORED | SOVEREIGN |
| 39 | P2_btc_pressure_building | engram-node06 | ANCHORED | SOVEREIGN |
| 39 | P2_btc_pressure_building | engram-node07 | ANCHORED | SOVEREIGN |
| 43 | P2_btc_pressure_building | engram-node01 | ANCHORED | SOVEREIGN |
| 325 | P6_healing | engram-node01 | SOVEREIGN | RECOVERING |
| 325 | P6_healing | engram-node02 | SOVEREIGN | RECOVERING |
| 325 | P6_healing | engram-node03 | SOVEREIGN | RECOVERING |
| 325 | P6_healing | engram-node04 | SOVEREIGN | RECOVERING |
| 325 | P6_healing | engram-node05 | SOVEREIGN | RECOVERING |
| 325 | P6_healing | engram-node06 | SOVEREIGN | RECOVERING |
| 325 | P6_healing | engram-node07 | SOVEREIGN | RECOVERING |
| 379 | P7_recovery | engram-node01 | RECOVERING | SOVEREIGN |
| 379 | P7_recovery | engram-node02 | RECOVERING | SOVEREIGN |
| 381 | P7_recovery | engram-node01 | SOVEREIGN | RECOVERING |
| 381 | P7_recovery | engram-node02 | SOVEREIGN | RECOVERING |
| 562 | P7_recovery | engram-node01 | RECOVERING | ANCHORED |
| 562 | P7_recovery | engram-node02 | RECOVERING | ANCHORED |
| 562 | P7_recovery | engram-node03 | RECOVERING | ANCHORED |
| 562 | P7_recovery | engram-node04 | RECOVERING | ANCHORED |
| 562 | P7_recovery | engram-node05 | RECOVERING | ANCHORED |
| 562 | P7_recovery | engram-node06 | RECOVERING | ANCHORED |
| 562 | P7_recovery | engram-node07 | RECOVERING | ANCHORED |
