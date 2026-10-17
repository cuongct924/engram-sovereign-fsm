# LIVE E9 combined-failure trace (BTC + DA + P2P churn, overlapping)

Total duration: 374s. Final phase reached ANCHORED: True. WAN-realism baseline: none.

## Fault-injection / healing event markers

| t (s) | Event |
|---:|---|
| 20 | chaos-btc-delay started (BTC congestion pressure) |
| 94 | all 7 celestia bridges stopped (DA outage layered on BTC pressure) |
| 157 | chaos-loss churn burst starting (3rd simultaneous fault class) |
| 267 | P2P churn burst complete |
| 301 | chaos-btc-delay cleaned up |
| 305 | all 7 celestia bridges restarted |

## Real transitions observed

| t (s) | Phase | Node | From | To |
|---:|---|---|---|---|
| 40 | P2_btc_pressure_building | engram-node01 | ANCHORED | RECOVERING |
| 40 | P2_btc_pressure_building | engram-node02 | ANCHORED | RECOVERING |
| 40 | P2_btc_pressure_building | engram-node03 | ANCHORED | RECOVERING |
| 40 | P2_btc_pressure_building | engram-node04 | ANCHORED | SOVEREIGN |
| 40 | P2_btc_pressure_building | engram-node05 | ANCHORED | SOVEREIGN |
| 40 | P2_btc_pressure_building | engram-node06 | ANCHORED | SOVEREIGN |
| 40 | P2_btc_pressure_building | engram-node07 | ANCHORED | SOVEREIGN |
| 47 | P2_btc_pressure_building | engram-node04 | SOVEREIGN | RECOVERING |
| 47 | P2_btc_pressure_building | engram-node05 | SOVEREIGN | RECOVERING |
| 47 | P2_btc_pressure_building | engram-node06 | SOVEREIGN | RECOVERING |
| 47 | P2_btc_pressure_building | engram-node07 | SOVEREIGN | RECOVERING |
| 99 | P3_btc_plus_da | engram-node01 | RECOVERING | SOVEREIGN |
| 99 | P3_btc_plus_da | engram-node02 | RECOVERING | SOVEREIGN |
| 99 | P3_btc_plus_da | engram-node03 | RECOVERING | SOVEREIGN |
| 99 | P3_btc_plus_da | engram-node04 | RECOVERING | SOVEREIGN |
| 99 | P3_btc_plus_da | engram-node05 | RECOVERING | SOVEREIGN |
| 99 | P3_btc_plus_da | engram-node06 | RECOVERING | SOVEREIGN |
| 99 | P3_btc_plus_da | engram-node07 | RECOVERING | SOVEREIGN |
| 317 | P6_healing | engram-node04 | SOVEREIGN | RECOVERING |
| 317 | P6_healing | engram-node05 | SOVEREIGN | RECOVERING |
| 317 | P6_healing | engram-node06 | SOVEREIGN | RECOVERING |
| 317 | P6_healing | engram-node07 | SOVEREIGN | RECOVERING |
| 325 | P6_healing | engram-node01 | SOVEREIGN | RECOVERING |
| 325 | P6_healing | engram-node02 | SOVEREIGN | RECOVERING |
| 325 | P6_healing | engram-node03 | SOVEREIGN | RECOVERING |
| 374 | P7_recovery | engram-node01 | RECOVERING | ANCHORED |
| 374 | P7_recovery | engram-node02 | RECOVERING | ANCHORED |
| 374 | P7_recovery | engram-node03 | RECOVERING | ANCHORED |
| 374 | P7_recovery | engram-node04 | RECOVERING | ANCHORED |
| 374 | P7_recovery | engram-node05 | RECOVERING | ANCHORED |
| 374 | P7_recovery | engram-node06 | RECOVERING | ANCHORED |
| 374 | P7_recovery | engram-node07 | RECOVERING | ANCHORED |
