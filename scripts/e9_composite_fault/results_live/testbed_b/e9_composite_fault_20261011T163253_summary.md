# LIVE E9 combined-failure trace (BTC + DA + P2P churn, overlapping)

Total duration: 417s. Final phase reached ANCHORED: True. WAN-realism baseline: none.

## Fault-injection / healing event markers

| t (s) | Event |
|---:|---|
| 21 | chaos-btc-delay started (BTC congestion pressure) |
| 106 | all 7 celestia bridges stopped (DA outage layered on BTC pressure) |
| 170 | chaos-loss churn burst starting (3rd simultaneous fault class) |
| 281 | P2P churn burst complete |
| 319 | chaos-btc-delay cleaned up |
| 323 | all 7 celestia bridges restarted |

## Real transitions observed

| t (s) | Phase | Node | From | To |
|---:|---|---|---|---|
| 14 | P1_baseline | engram-node01 | ANCHORED | SUSPICIOUS |
| 14 | P1_baseline | engram-node02 | ANCHORED | SUSPICIOUS |
| 14 | P1_baseline | engram-node03 | ANCHORED | SUSPICIOUS |
| 43 | P2_btc_pressure_building | engram-node01 | SUSPICIOUS | ANCHORED |
| 43 | P2_btc_pressure_building | engram-node02 | SUSPICIOUS | ANCHORED |
| 43 | P2_btc_pressure_building | engram-node03 | SUSPICIOUS | ANCHORED |
| 43 | P2_btc_pressure_building | engram-node04 | SUSPICIOUS | ANCHORED |
| 43 | P2_btc_pressure_building | engram-node05 | SUSPICIOUS | ANCHORED |
| 43 | P2_btc_pressure_building | engram-node06 | SUSPICIOUS | ANCHORED |
| 43 | P2_btc_pressure_building | engram-node07 | SUSPICIOUS | ANCHORED |
| 77 | P2_btc_pressure_building | engram-node06 | ANCHORED | SUSPICIOUS |
| 77 | P2_btc_pressure_building | engram-node07 | ANCHORED | SUSPICIOUS |
| 90 | P2_btc_pressure_building | engram-node01 | ANCHORED | SUSPICIOUS |
| 90 | P2_btc_pressure_building | engram-node02 | ANCHORED | SUSPICIOUS |
| 90 | P2_btc_pressure_building | engram-node03 | ANCHORED | SUSPICIOUS |
| 90 | P2_btc_pressure_building | engram-node04 | ANCHORED | SUSPICIOUS |
| 90 | P2_btc_pressure_building | engram-node05 | ANCHORED | SUSPICIOUS |
| 232 | P4_triple_fault_churn_cycle_2_on | engram-node07 | SUSPICIOUS | SOVEREIGN |
| 242 | P4_triple_fault_churn_cycle_2_off | engram-node02 | SUSPICIOUS | SOVEREIGN |
| 242 | P4_triple_fault_churn_cycle_2_off | engram-node03 | SUSPICIOUS | SOVEREIGN |
| 242 | P4_triple_fault_churn_cycle_2_off | engram-node04 | SUSPICIOUS | SOVEREIGN |
| 242 | P4_triple_fault_churn_cycle_2_off | engram-node05 | SUSPICIOUS | SOVEREIGN |
| 242 | P4_triple_fault_churn_cycle_2_off | engram-node06 | SUSPICIOUS | SOVEREIGN |
| 254 | P4_triple_fault_churn_cycle_3_on | engram-node01 | SUSPICIOUS | SOVEREIGN |
| 336 | P6_healing | engram-node04 | SOVEREIGN | RECOVERING |
| 336 | P6_healing | engram-node05 | SOVEREIGN | RECOVERING |
| 336 | P6_healing | engram-node06 | SOVEREIGN | RECOVERING |
| 336 | P6_healing | engram-node07 | SOVEREIGN | RECOVERING |
| 345 | P6_healing | engram-node01 | SOVEREIGN | RECOVERING |
| 345 | P6_healing | engram-node02 | SOVEREIGN | RECOVERING |
| 345 | P6_healing | engram-node03 | SOVEREIGN | RECOVERING |
| 363 | P7_recovery | engram-node07 | RECOVERING | SOVEREIGN |
| 371 | P7_recovery | engram-node07 | SOVEREIGN | RECOVERING |
| 417 | P7_recovery | engram-node01 | RECOVERING | ANCHORED |
| 417 | P7_recovery | engram-node02 | RECOVERING | ANCHORED |
| 417 | P7_recovery | engram-node03 | RECOVERING | ANCHORED |
| 417 | P7_recovery | engram-node04 | RECOVERING | ANCHORED |
| 417 | P7_recovery | engram-node05 | RECOVERING | ANCHORED |
| 417 | P7_recovery | engram-node06 | RECOVERING | ANCHORED |
| 417 | P7_recovery | engram-node07 | RECOVERING | ANCHORED |
