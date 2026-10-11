# LIVE E9 combined-failure trace (BTC + DA + P2P churn, overlapping)

Total duration: 562s. Final phase reached ANCHORED: True. WAN-realism baseline: chaos-wan-latency.

## Fault-injection / healing event markers

| t (s) | Event |
|---:|---|
| 20 | chaos-btc-delay started (BTC congestion pressure) |
| 91 | all 7 celestia bridges stopped (DA outage layered on BTC pressure) |
| 157 | chaos-loss churn burst starting (3rd simultaneous fault class) |
| 264 | P2P churn burst complete |
| 300 | chaos-btc-delay cleaned up |
| 303 | all 7 celestia bridges restarted |

## Real transitions observed

| t (s) | Phase | Node | From | To |
|---:|---|---|---|---|
| 16 | P1_baseline | engram-node01 | ANCHORED | SUSPICIOUS |
| 16 | P1_baseline | engram-node02 | ANCHORED | SUSPICIOUS |
| 16 | P1_baseline | engram-node03 | ANCHORED | SUSPICIOUS |
| 16 | P1_baseline | engram-node04 | ANCHORED | SUSPICIOUS |
| 16 | P1_baseline | engram-node05 | ANCHORED | SUSPICIOUS |
| 16 | P1_baseline | engram-node06 | ANCHORED | SUSPICIOUS |
| 16 | P1_baseline | engram-node07 | ANCHORED | SUSPICIOUS |
| 33 | P2_btc_pressure_building | engram-node07 | SUSPICIOUS | ANCHORED |
| 39 | P2_btc_pressure_building | engram-node01 | SUSPICIOUS | ANCHORED |
| 39 | P2_btc_pressure_building | engram-node02 | SUSPICIOUS | ANCHORED |
| 39 | P2_btc_pressure_building | engram-node03 | SUSPICIOUS | ANCHORED |
| 39 | P2_btc_pressure_building | engram-node04 | SUSPICIOUS | ANCHORED |
| 39 | P2_btc_pressure_building | engram-node05 | SUSPICIOUS | ANCHORED |
| 39 | P2_btc_pressure_building | engram-node06 | SUSPICIOUS | ANCHORED |
| 52 | P2_btc_pressure_building | engram-node05 | ANCHORED | SOVEREIGN |
| 52 | P2_btc_pressure_building | engram-node06 | ANCHORED | SOVEREIGN |
| 52 | P2_btc_pressure_building | engram-node07 | ANCHORED | SOVEREIGN |
| 55 | P2_btc_pressure_building | engram-node01 | ANCHORED | SOVEREIGN |
| 55 | P2_btc_pressure_building | engram-node02 | ANCHORED | SOVEREIGN |
| 55 | P2_btc_pressure_building | engram-node03 | ANCHORED | SOVEREIGN |
| 55 | P2_btc_pressure_building | engram-node04 | ANCHORED | SOVEREIGN |
| 315 | P6_healing | engram-node01 | SOVEREIGN | RECOVERING |
| 315 | P6_healing | engram-node02 | SOVEREIGN | RECOVERING |
| 315 | P6_healing | engram-node03 | SOVEREIGN | RECOVERING |
| 315 | P6_healing | engram-node04 | SOVEREIGN | RECOVERING |
| 315 | P6_healing | engram-node05 | SOVEREIGN | RECOVERING |
| 315 | P6_healing | engram-node06 | SOVEREIGN | RECOVERING |
| 315 | P6_healing | engram-node07 | SOVEREIGN | RECOVERING |
| 556 | P7_recovery | engram-node01 | RECOVERING | ANCHORED |
| 556 | P7_recovery | engram-node02 | RECOVERING | ANCHORED |
| 556 | P7_recovery | engram-node03 | RECOVERING | ANCHORED |
| 556 | P7_recovery | engram-node04 | RECOVERING | ANCHORED |
| 556 | P7_recovery | engram-node05 | RECOVERING | ANCHORED |
| 556 | P7_recovery | engram-node06 | RECOVERING | ANCHORED |
| 556 | P7_recovery | engram-node07 | RECOVERING | ANCHORED |
