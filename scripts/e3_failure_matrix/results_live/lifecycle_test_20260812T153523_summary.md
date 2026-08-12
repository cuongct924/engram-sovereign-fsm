# LIVE full-lifecycle fault-injection test (real docker celestia-bridge stop/start)

Total duration: 344s. Final phase reached ANCHORED: True

## Real transitions observed

| t (s) | Phase | Node | From | To |
|---:|---|---|---|---|
| 12 | P2_da_outage | engram-node01 | ANCHORED | SUSPICIOUS |
| 12 | P2_da_outage | engram-node02 | ANCHORED | SUSPICIOUS |
| 12 | P2_da_outage | engram-node03 | ANCHORED | SUSPICIOUS |
| 12 | P2_da_outage | engram-node04 | ANCHORED | SUSPICIOUS |
| 24 | P3_quick_recovery | engram-node01 | SUSPICIOUS | ANCHORED |
| 24 | P3_quick_recovery | engram-node02 | SUSPICIOUS | ANCHORED |
| 24 | P3_quick_recovery | engram-node03 | SUSPICIOUS | ANCHORED |
| 24 | P3_quick_recovery | engram-node04 | SUSPICIOUS | ANCHORED |
| 27 | P4_sustained_outage_enter | engram-node01 | ANCHORED | SUSPICIOUS |
| 27 | P4_sustained_outage_enter | engram-node02 | ANCHORED | SUSPICIOUS |
| 27 | P4_sustained_outage_enter | engram-node03 | ANCHORED | SUSPICIOUS |
| 27 | P4_sustained_outage_enter | engram-node04 | ANCHORED | SUSPICIOUS |
| 58 | P4_sustained_outage_escalate | engram-node01 | SUSPICIOUS | SOVEREIGN |
| 58 | P4_sustained_outage_escalate | engram-node02 | SUSPICIOUS | SOVEREIGN |
| 58 | P4_sustained_outage_escalate | engram-node03 | SUSPICIOUS | SOVEREIGN |
| 58 | P4_sustained_outage_escalate | engram-node04 | SUSPICIOUS | SOVEREIGN |
| 74 | P5_recovery_start | engram-node01 | SOVEREIGN | RECOVERING |
| 74 | P5_recovery_start | engram-node02 | SOVEREIGN | RECOVERING |
| 74 | P5_recovery_start | engram-node03 | SOVEREIGN | RECOVERING |
| 74 | P5_recovery_start | engram-node04 | SOVEREIGN | RECOVERING |
| 80 | P6_regression | engram-node01 | RECOVERING | SOVEREIGN |
| 80 | P6_regression | engram-node02 | RECOVERING | SOVEREIGN |
| 80 | P6_regression | engram-node03 | RECOVERING | SOVEREIGN |
| 80 | P6_regression | engram-node04 | RECOVERING | SOVEREIGN |
| 98 | P7_final_recovery | engram-node01 | SOVEREIGN | RECOVERING |
| 98 | P7_final_recovery | engram-node02 | SOVEREIGN | RECOVERING |
| 98 | P7_final_recovery | engram-node03 | SOVEREIGN | RECOVERING |
| 98 | P7_final_recovery | engram-node04 | SOVEREIGN | RECOVERING |
| 222 | P7_final_recovery | engram-node01 | RECOVERING | SOVEREIGN |
| 222 | P7_final_recovery | engram-node02 | RECOVERING | SOVEREIGN |
| 222 | P7_final_recovery | engram-node03 | RECOVERING | SOVEREIGN |
| 222 | P7_final_recovery | engram-node04 | RECOVERING | SOVEREIGN |
| 224 | P7_final_recovery | engram-node01 | SOVEREIGN | RECOVERING |
| 224 | P7_final_recovery | engram-node02 | SOVEREIGN | RECOVERING |
| 224 | P7_final_recovery | engram-node03 | SOVEREIGN | RECOVERING |
| 224 | P7_final_recovery | engram-node04 | SOVEREIGN | RECOVERING |
| 338 | P7_final_recovery | engram-node01 | RECOVERING | SOVEREIGN |
| 338 | P7_final_recovery | engram-node02 | RECOVERING | SOVEREIGN |
| 338 | P7_final_recovery | engram-node03 | RECOVERING | SOVEREIGN |
| 338 | P7_final_recovery | engram-node04 | RECOVERING | SOVEREIGN |
| 340 | P7_final_recovery | engram-node01 | SOVEREIGN | RECOVERING |
| 340 | P7_final_recovery | engram-node02 | SOVEREIGN | RECOVERING |
| 340 | P7_final_recovery | engram-node03 | SOVEREIGN | RECOVERING |
| 340 | P7_final_recovery | engram-node04 | SOVEREIGN | RECOVERING |
| 342 | P7_final_recovery | engram-node02 | RECOVERING | ANCHORED |
| 342 | P7_final_recovery | engram-node04 | RECOVERING | ANCHORED |
| 344 | P7_final_recovery | engram-node01 | RECOVERING | ANCHORED |
| 344 | P7_final_recovery | engram-node03 | RECOVERING | ANCHORED |
