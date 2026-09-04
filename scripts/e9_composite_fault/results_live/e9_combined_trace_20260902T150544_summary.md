# LIVE E9 combined-failure trace (BTC + DA + P2P churn, overlapping)

Total duration: 951s. Final phase reached ANCHORED: False. WAN-realism baseline: chaos-wan-latency.

## Fault-injection / healing event markers

| t (s) | Event |
|---:|---|
| 21 | chaos-btc-delay started (BTC congestion pressure) |
| 94 | all 7 celestia bridges stopped (DA outage layered on BTC pressure) |
| 162 | chaos-loss churn burst starting (3rd simultaneous fault class) |
| 275 | P2P churn burst complete |
| 309 | chaos-btc-delay cleaned up |
| 313 | all 7 celestia bridges restarted |

## Real transitions observed

| t (s) | Phase | Node | From | To |
|---:|---|---|---|---|
