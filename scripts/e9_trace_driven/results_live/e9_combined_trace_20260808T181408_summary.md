# LIVE E9 combined-failure trace (BTC + DA + P2P churn, overlapping)

Total duration: 906s. Final phase reached ANCHORED: False

## Fault-injection / healing event markers

| t (s) | Event |
|---:|---|
| 17 | chaos-btc-delay started (BTC congestion pressure) |
| 80 | celestia-bridge stopped (DA outage layered on BTC pressure) |
| 142 | chaos-loss churn burst starting (3rd simultaneous fault class) |
| 241 | P2P churn burst complete |
| 273 | chaos-btc-delay cleaned up |
| 273 | celestia-bridge restarted |

## Real transitions observed

| t (s) | Phase | Node | From | To |
|---:|---|---|---|---|
