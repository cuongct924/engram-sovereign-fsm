# LIVE E9 combined-failure trace (BTC + DA + P2P churn, overlapping)

Total duration: 319s. Final phase reached ANCHORED: True. WAN-realism baseline: chaos-wan-latency.

## Fault-injection / healing event markers

| t (s) | Event |
|---:|---|
| 20 | chaos-btc-delay started (BTC congestion pressure) |
| 82 | celestia-bridge stopped (DA outage layered on BTC pressure) |
| 150 | chaos-loss churn burst starting (3rd simultaneous fault class) |
| 251 | P2P churn burst complete |
| 284 | chaos-btc-delay cleaned up |
| 285 | celestia-bridge restarted |

## Real transitions observed

| t (s) | Phase | Node | From | To |
|---:|---|---|---|---|
| 152 | P4_triple_fault_churn_cycle_1_on | engram-node01 | ANCHORED | SOVEREIGN |
| 152 | P4_triple_fault_churn_cycle_1_on | engram-node02 | ANCHORED | SOVEREIGN |
| 152 | P4_triple_fault_churn_cycle_1_on | engram-node03 | ANCHORED | SOVEREIGN |
| 152 | P4_triple_fault_churn_cycle_1_on | engram-node04 | ANCHORED | SOVEREIGN |
| 308 | P6_healing | engram-node01 | SOVEREIGN | RECOVERING |
| 308 | P6_healing | engram-node02 | SOVEREIGN | RECOVERING |
| 308 | P6_healing | engram-node03 | SOVEREIGN | RECOVERING |
| 308 | P6_healing | engram-node04 | SOVEREIGN | RECOVERING |
| 315 | P6_healing | engram-node01 | RECOVERING | ANCHORED |
| 315 | P6_healing | engram-node02 | RECOVERING | ANCHORED |
| 315 | P6_healing | engram-node03 | RECOVERING | ANCHORED |
| 315 | P6_healing | engram-node04 | RECOVERING | ANCHORED |
