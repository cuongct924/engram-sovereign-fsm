# LIVE E9 combined-failure trace (BTC + DA + P2P churn, overlapping)

Total duration: 445s. Final phase reached ANCHORED: True. WAN-realism baseline: none.

## Fault-injection / healing event markers

| t (s) | Event |
|---:|---|
| 20 | chaos-btc-delay started (BTC congestion pressure) |
| 100 | all 7 celestia bridges stopped (DA outage layered on BTC pressure) |
| 170 | chaos-loss churn burst starting (3rd simultaneous fault class) |
| 279 | P2P churn burst complete |
| 323 | chaos-btc-delay cleaned up |
| 326 | all 7 celestia bridges restarted |

## Real transitions observed

| t (s) | Phase | Node | From | To |
|---:|---|---|---|---|
| 126 | P3_btc_plus_da | engram-node02 | ANCHORED | SUSPICIOUS |
| 126 | P3_btc_plus_da | engram-node04 | ANCHORED | SUSPICIOUS |
| 126 | P3_btc_plus_da | engram-node05 | ANCHORED | SUSPICIOUS |
| 126 | P3_btc_plus_da | engram-node06 | ANCHORED | SUSPICIOUS |
| 140 | P3_btc_plus_da | engram-node03 | ANCHORED | SUSPICIOUS |
| 140 | P3_btc_plus_da | engram-node07 | ANCHORED | SUSPICIOUS |
| 151 | P3_btc_plus_da | engram-node01 | ANCHORED | SUSPICIOUS |
| 265 | P4_triple_fault_churn_cycle_3_on | engram-node06 | SUSPICIOUS | SOVEREIGN |
| 265 | P4_triple_fault_churn_cycle_3_on | engram-node07 | SUSPICIOUS | SOVEREIGN |
| 277 | P4_triple_fault_churn_cycle_3_off | engram-node01 | SUSPICIOUS | SOVEREIGN |
| 277 | P4_triple_fault_churn_cycle_3_off | engram-node02 | SUSPICIOUS | SOVEREIGN |
| 277 | P4_triple_fault_churn_cycle_3_off | engram-node03 | SUSPICIOUS | SOVEREIGN |
| 277 | P4_triple_fault_churn_cycle_3_off | engram-node05 | SUSPICIOUS | SOVEREIGN |
| 292 | P5_peak_triple_fault | engram-node04 | SUSPICIOUS | SOVEREIGN |
| 341 | P6_healing | engram-node01 | SOVEREIGN | RECOVERING |
| 341 | P6_healing | engram-node02 | SOVEREIGN | RECOVERING |
| 341 | P6_healing | engram-node03 | SOVEREIGN | RECOVERING |
| 341 | P6_healing | engram-node04 | SOVEREIGN | RECOVERING |
| 341 | P6_healing | engram-node05 | SOVEREIGN | RECOVERING |
| 341 | P6_healing | engram-node06 | SOVEREIGN | RECOVERING |
| 341 | P6_healing | engram-node07 | SOVEREIGN | RECOVERING |
| 349 | P6_healing | engram-node06 | RECOVERING | SOVEREIGN |
| 349 | P6_healing | engram-node07 | RECOVERING | SOVEREIGN |
| 357 | P6_healing | engram-node06 | SOVEREIGN | RECOVERING |
| 357 | P6_healing | engram-node07 | SOVEREIGN | RECOVERING |
| 412 | P7_recovery | engram-node03 | RECOVERING | SOVEREIGN |
| 412 | P7_recovery | engram-node04 | RECOVERING | SOVEREIGN |
| 412 | P7_recovery | engram-node05 | RECOVERING | SOVEREIGN |
| 420 | P7_recovery | engram-node03 | SOVEREIGN | RECOVERING |
| 420 | P7_recovery | engram-node04 | SOVEREIGN | RECOVERING |
| 420 | P7_recovery | engram-node05 | SOVEREIGN | RECOVERING |
| 435 | P7_recovery | engram-node04 | RECOVERING | ANCHORED |
| 435 | P7_recovery | engram-node05 | RECOVERING | ANCHORED |
| 435 | P7_recovery | engram-node06 | RECOVERING | ANCHORED |
| 435 | P7_recovery | engram-node07 | RECOVERING | ANCHORED |
| 445 | P7_recovery | engram-node01 | RECOVERING | ANCHORED |
| 445 | P7_recovery | engram-node02 | RECOVERING | ANCHORED |
| 445 | P7_recovery | engram-node03 | RECOVERING | ANCHORED |
