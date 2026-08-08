# LIVE full-lifecycle fault-injection test (real docker celestia-bridge stop/start)

Total duration: 280s. Final phase reached ANCHORED: True

## Real transitions observed

| t (s) | Phase | Node | From | To |
|---:|---|---|---|---|
| 261 | P5_recovery_start | engram-node01 | ANCHORED | SOVEREIGN |
| 261 | P5_recovery_start | engram-node02 | ANCHORED | SOVEREIGN |
| 261 | P5_recovery_start | engram-node03 | ANCHORED | SOVEREIGN |
| 261 | P5_recovery_start | engram-node04 | ANCHORED | SOVEREIGN |
| 263 | P5_recovery_start | engram-node01 | SOVEREIGN | RECOVERING |
| 263 | P5_recovery_start | engram-node02 | SOVEREIGN | RECOVERING |
| 263 | P5_recovery_start | engram-node03 | SOVEREIGN | RECOVERING |
| 263 | P5_recovery_start | engram-node04 | SOVEREIGN | RECOVERING |
| 266 | P6_regression | engram-node01 | RECOVERING | SOVEREIGN |
| 266 | P6_regression | engram-node02 | RECOVERING | SOVEREIGN |
| 266 | P6_regression | engram-node03 | RECOVERING | SOVEREIGN |
| 266 | P6_regression | engram-node04 | RECOVERING | SOVEREIGN |
| 276 | P7_final_recovery | engram-node01 | SOVEREIGN | RECOVERING |
| 276 | P7_final_recovery | engram-node02 | SOVEREIGN | RECOVERING |
| 276 | P7_final_recovery | engram-node03 | SOVEREIGN | RECOVERING |
| 276 | P7_final_recovery | engram-node04 | SOVEREIGN | RECOVERING |
| 280 | P7_final_recovery | engram-node01 | RECOVERING | ANCHORED |
| 280 | P7_final_recovery | engram-node02 | RECOVERING | ANCHORED |
| 280 | P7_final_recovery | engram-node03 | RECOVERING | ANCHORED |
| 280 | P7_final_recovery | engram-node04 | RECOVERING | ANCHORED |
