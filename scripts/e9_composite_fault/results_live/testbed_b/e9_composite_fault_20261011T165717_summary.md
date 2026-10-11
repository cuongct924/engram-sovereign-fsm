# LIVE E9 combined-failure trace (BTC + DA + P2P churn, overlapping)

Total duration: 447s. Final phase reached ANCHORED: True. WAN-realism baseline: none.

## Fault-injection / healing event markers

| t (s) | Event |
|---:|---|
| 21 | chaos-btc-delay started (BTC congestion pressure) |
| 99 | all 7 celestia bridges stopped (DA outage layered on BTC pressure) |
| 160 | chaos-loss churn burst starting (3rd simultaneous fault class) |
| 277 | P2P churn burst complete |
| 325 | chaos-btc-delay cleaned up |
| 330 | all 7 celestia bridges restarted |

## Real transitions observed

| t (s) | Phase | Node | From | To |
|---:|---|---|---|---|
| 29 | P2_btc_pressure_building | engram-node04 | SUSPICIOUS | ANCHORED |
| 29 | P2_btc_pressure_building | engram-node05 | SUSPICIOUS | ANCHORED |
| 29 | P2_btc_pressure_building | engram-node07 | SUSPICIOUS | ANCHORED |
| 42 | P2_btc_pressure_building | engram-node01 | SUSPICIOUS | ANCHORED |
| 42 | P2_btc_pressure_building | engram-node02 | SUSPICIOUS | ANCHORED |
| 42 | P2_btc_pressure_building | engram-node03 | SUSPICIOUS | ANCHORED |
| 42 | P2_btc_pressure_building | engram-node06 | SUSPICIOUS | ANCHORED |
| 118 | P3_btc_plus_da | engram-node06 | ANCHORED | SUSPICIOUS |
| 118 | P3_btc_plus_da | engram-node07 | ANCHORED | SUSPICIOUS |
| 132 | P3_btc_plus_da | engram-node02 | ANCHORED | SUSPICIOUS |
| 132 | P3_btc_plus_da | engram-node03 | ANCHORED | SUSPICIOUS |
| 132 | P3_btc_plus_da | engram-node04 | ANCHORED | SUSPICIOUS |
| 145 | P3_btc_plus_da | engram-node01 | ANCHORED | SUSPICIOUS |
| 145 | P3_btc_plus_da | engram-node05 | ANCHORED | SUSPICIOUS |
| 275 | P4_triple_fault_churn_cycle_3_off | engram-node06 | SUSPICIOUS | SOVEREIGN |
| 275 | P4_triple_fault_churn_cycle_3_off | engram-node07 | SUSPICIOUS | SOVEREIGN |
| 288 | P5_peak_triple_fault | engram-node01 | SUSPICIOUS | SOVEREIGN |
| 288 | P5_peak_triple_fault | engram-node02 | SUSPICIOUS | SOVEREIGN |
| 288 | P5_peak_triple_fault | engram-node03 | SUSPICIOUS | SOVEREIGN |
| 288 | P5_peak_triple_fault | engram-node04 | SUSPICIOUS | SOVEREIGN |
| 288 | P5_peak_triple_fault | engram-node05 | SUSPICIOUS | SOVEREIGN |
| 344 | P6_healing | engram-node01 | SOVEREIGN | RECOVERING |
| 344 | P6_healing | engram-node02 | SOVEREIGN | RECOVERING |
| 344 | P6_healing | engram-node03 | SOVEREIGN | RECOVERING |
| 344 | P6_healing | engram-node04 | SOVEREIGN | RECOVERING |
| 344 | P6_healing | engram-node05 | SOVEREIGN | RECOVERING |
| 344 | P6_healing | engram-node06 | SOVEREIGN | RECOVERING |
| 344 | P6_healing | engram-node07 | SOVEREIGN | RECOVERING |
| 361 | P6_healing | engram-node06 | RECOVERING | SOVEREIGN |
| 361 | P6_healing | engram-node07 | RECOVERING | SOVEREIGN |
| 369 | P7_recovery | engram-node06 | SOVEREIGN | RECOVERING |
| 369 | P7_recovery | engram-node07 | SOVEREIGN | RECOVERING |
| 424 | P7_recovery | engram-node04 | RECOVERING | SOVEREIGN |
| 424 | P7_recovery | engram-node05 | RECOVERING | SOVEREIGN |
| 424 | P7_recovery | engram-node06 | RECOVERING | SOVEREIGN |
| 431 | P7_recovery | engram-node04 | SOVEREIGN | RECOVERING |
| 431 | P7_recovery | engram-node05 | SOVEREIGN | RECOVERING |
| 431 | P7_recovery | engram-node06 | SOVEREIGN | RECOVERING |
| 439 | P7_recovery | engram-node03 | RECOVERING | ANCHORED |
| 439 | P7_recovery | engram-node04 | RECOVERING | ANCHORED |
| 439 | P7_recovery | engram-node05 | RECOVERING | ANCHORED |
| 439 | P7_recovery | engram-node06 | RECOVERING | ANCHORED |
| 439 | P7_recovery | engram-node07 | RECOVERING | ANCHORED |
| 447 | P7_recovery | engram-node01 | RECOVERING | ANCHORED |
| 447 | P7_recovery | engram-node02 | RECOVERING | ANCHORED |
