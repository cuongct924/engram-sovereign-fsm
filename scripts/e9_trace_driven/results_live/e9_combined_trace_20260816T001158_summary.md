# LIVE E9 combined-failure trace (BTC + DA + P2P churn, overlapping)

Total duration: 343s. Final phase reached ANCHORED: True. WAN-realism baseline: chaos-wan-latency.

## Fault-injection / healing event markers

| t (s) | Event |
|---:|---|
| 21 | chaos-btc-delay started (BTC congestion pressure) |
| 87 | celestia-bridge stopped (DA outage layered on BTC pressure) |
| 159 | chaos-loss churn burst starting (3rd simultaneous fault class) |
| 271 | P2P churn burst complete |
| 305 | chaos-btc-delay cleaned up |
| 305 | celestia-bridge restarted |

## Real transitions observed

| t (s) | Phase | Node | From | To |
|---:|---|---|---|---|
| 110 | P3_btc_plus_da | engram-node02 | ANCHORED | SOVEREIGN |
| 110 | P3_btc_plus_da | engram-node03 | ANCHORED | SOVEREIGN |
| 110 | P3_btc_plus_da | engram-node04 | ANCHORED | SOVEREIGN |
| 113 | P3_btc_plus_da | engram-node01 | ANCHORED | SOVEREIGN |
| 318 | P6_healing | engram-node01 | SOVEREIGN | RECOVERING |
| 318 | P6_healing | engram-node02 | SOVEREIGN | RECOVERING |
| 318 | P6_healing | engram-node03 | SOVEREIGN | RECOVERING |
| 318 | P6_healing | engram-node04 | SOVEREIGN | RECOVERING |
| 324 | P6_healing | engram-node01 | RECOVERING | ANCHORED |
| 324 | P6_healing | engram-node02 | RECOVERING | ANCHORED |
| 324 | P6_healing | engram-node03 | RECOVERING | ANCHORED |
| 324 | P6_healing | engram-node04 | RECOVERING | ANCHORED |
