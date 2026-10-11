# LIVE E9 combined-failure trace (BTC + DA + P2P churn, overlapping)

Total duration: 450s. Final phase reached ANCHORED: True. WAN-realism baseline: none.

## Fault-injection / healing event markers

| t (s) | Event |
|---:|---|
| 23 | chaos-btc-delay started (BTC congestion pressure) |
| 105 | all 7 celestia bridges stopped (DA outage layered on BTC pressure) |
| 173 | chaos-loss churn burst starting (3rd simultaneous fault class) |
| 293 | P2P churn burst complete |
| 334 | chaos-btc-delay cleaned up |
| 337 | all 7 celestia bridges restarted |

## Real transitions observed

| t (s) | Phase | Node | From | To |
|---:|---|---|---|---|
| 129 | P3_btc_plus_da | engram-node03 | ANCHORED | SUSPICIOUS |
| 129 | P3_btc_plus_da | engram-node05 | ANCHORED | SUSPICIOUS |
| 129 | P3_btc_plus_da | engram-node06 | ANCHORED | SUSPICIOUS |
| 129 | P3_btc_plus_da | engram-node07 | ANCHORED | SUSPICIOUS |
| 141 | P3_btc_plus_da | engram-node01 | ANCHORED | SUSPICIOUS |
| 141 | P3_btc_plus_da | engram-node02 | ANCHORED | SUSPICIOUS |
| 141 | P3_btc_plus_da | engram-node04 | ANCHORED | SUSPICIOUS |
| 291 | P4_triple_fault_churn_cycle_3_off | engram-node02 | SUSPICIOUS | SOVEREIGN |
| 291 | P4_triple_fault_churn_cycle_3_off | engram-node03 | SUSPICIOUS | SOVEREIGN |
| 291 | P4_triple_fault_churn_cycle_3_off | engram-node06 | SUSPICIOUS | SOVEREIGN |
| 291 | P4_triple_fault_churn_cycle_3_off | engram-node07 | SUSPICIOUS | SOVEREIGN |
| 303 | P5_peak_triple_fault | engram-node01 | SUSPICIOUS | SOVEREIGN |
| 303 | P5_peak_triple_fault | engram-node05 | SUSPICIOUS | SOVEREIGN |
| 344 | P6_healing | engram-node04 | SUSPICIOUS | SOVEREIGN |
| 361 | P6_healing | engram-node01 | SOVEREIGN | RECOVERING |
| 361 | P6_healing | engram-node02 | SOVEREIGN | RECOVERING |
| 361 | P6_healing | engram-node03 | SOVEREIGN | RECOVERING |
| 361 | P6_healing | engram-node04 | SOVEREIGN | RECOVERING |
| 361 | P6_healing | engram-node05 | SOVEREIGN | RECOVERING |
| 361 | P6_healing | engram-node06 | SOVEREIGN | RECOVERING |
| 361 | P6_healing | engram-node07 | SOVEREIGN | RECOVERING |
| 418 | P7_recovery | engram-node01 | RECOVERING | SOVEREIGN |
| 418 | P7_recovery | engram-node02 | RECOVERING | SOVEREIGN |
| 418 | P7_recovery | engram-node03 | RECOVERING | SOVEREIGN |
| 426 | P7_recovery | engram-node01 | SOVEREIGN | RECOVERING |
| 426 | P7_recovery | engram-node02 | SOVEREIGN | RECOVERING |
| 426 | P7_recovery | engram-node03 | SOVEREIGN | RECOVERING |
| 450 | P7_recovery | engram-node01 | RECOVERING | ANCHORED |
| 450 | P7_recovery | engram-node02 | RECOVERING | ANCHORED |
| 450 | P7_recovery | engram-node03 | RECOVERING | ANCHORED |
| 450 | P7_recovery | engram-node04 | RECOVERING | ANCHORED |
| 450 | P7_recovery | engram-node05 | RECOVERING | ANCHORED |
| 450 | P7_recovery | engram-node06 | RECOVERING | ANCHORED |
| 450 | P7_recovery | engram-node07 | RECOVERING | ANCHORED |
