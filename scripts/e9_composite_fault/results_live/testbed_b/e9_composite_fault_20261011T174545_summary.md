# LIVE E9 combined-failure trace (BTC + DA + P2P churn, overlapping)

Total duration: 440s. Final phase reached ANCHORED: True. WAN-realism baseline: none.

## Fault-injection / healing event markers

| t (s) | Event |
|---:|---|
| 21 | chaos-btc-delay started (BTC congestion pressure) |
| 99 | all 7 celestia bridges stopped (DA outage layered on BTC pressure) |
| 162 | chaos-loss churn burst starting (3rd simultaneous fault class) |
| 278 | P2P churn burst complete |
| 321 | chaos-btc-delay cleaned up |
| 325 | all 7 celestia bridges restarted |

## Real transitions observed

| t (s) | Phase | Node | From | To |
|---:|---|---|---|---|
| 122 | P3_btc_plus_da | engram-node03 | ANCHORED | SUSPICIOUS |
| 122 | P3_btc_plus_da | engram-node05 | ANCHORED | SUSPICIOUS |
| 122 | P3_btc_plus_da | engram-node06 | ANCHORED | SUSPICIOUS |
| 122 | P3_btc_plus_da | engram-node07 | ANCHORED | SUSPICIOUS |
| 134 | P3_btc_plus_da | engram-node01 | ANCHORED | SUSPICIOUS |
| 134 | P3_btc_plus_da | engram-node02 | ANCHORED | SUSPICIOUS |
| 134 | P3_btc_plus_da | engram-node04 | ANCHORED | SUSPICIOUS |
| 276 | P4_triple_fault_churn_cycle_3_off | engram-node03 | SUSPICIOUS | SOVEREIGN |
| 276 | P4_triple_fault_churn_cycle_3_off | engram-node05 | SUSPICIOUS | SOVEREIGN |
| 276 | P4_triple_fault_churn_cycle_3_off | engram-node06 | SUSPICIOUS | SOVEREIGN |
| 276 | P4_triple_fault_churn_cycle_3_off | engram-node07 | SUSPICIOUS | SOVEREIGN |
| 287 | P5_peak_triple_fault | engram-node02 | SUSPICIOUS | SOVEREIGN |
| 287 | P5_peak_triple_fault | engram-node04 | SUSPICIOUS | SOVEREIGN |
| 299 | P5_peak_triple_fault | engram-node01 | SUSPICIOUS | SOVEREIGN |
| 346 | P6_healing | engram-node01 | SOVEREIGN | RECOVERING |
| 346 | P6_healing | engram-node02 | SOVEREIGN | RECOVERING |
| 346 | P6_healing | engram-node03 | SOVEREIGN | RECOVERING |
| 346 | P6_healing | engram-node04 | SOVEREIGN | RECOVERING |
| 346 | P6_healing | engram-node05 | SOVEREIGN | RECOVERING |
| 346 | P6_healing | engram-node06 | SOVEREIGN | RECOVERING |
| 354 | P6_healing | engram-node07 | SOVEREIGN | RECOVERING |
| 409 | P7_recovery | engram-node04 | RECOVERING | SOVEREIGN |
| 409 | P7_recovery | engram-node05 | RECOVERING | SOVEREIGN |
| 417 | P7_recovery | engram-node04 | SOVEREIGN | RECOVERING |
| 417 | P7_recovery | engram-node05 | SOVEREIGN | RECOVERING |
| 440 | P7_recovery | engram-node01 | RECOVERING | ANCHORED |
| 440 | P7_recovery | engram-node02 | RECOVERING | ANCHORED |
| 440 | P7_recovery | engram-node03 | RECOVERING | ANCHORED |
| 440 | P7_recovery | engram-node04 | RECOVERING | ANCHORED |
| 440 | P7_recovery | engram-node05 | RECOVERING | ANCHORED |
| 440 | P7_recovery | engram-node06 | RECOVERING | ANCHORED |
| 440 | P7_recovery | engram-node07 | RECOVERING | ANCHORED |
