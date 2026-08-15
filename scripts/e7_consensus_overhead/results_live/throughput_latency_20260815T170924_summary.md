# LIVE E7 throughput/latency A/B under real load

Real MsgSubmitForcedTxRequest load, 5.0 tx/s target, 45s per mode, against two local real `engramd` processes (extended vs. `--vanilla`), not the shared docker testnet.

| Mode | Sent | Accepted | Blocks/s | Tx-accepted/s | Mean interval (s) | p50 (s) | p95 (s) |
|---|---:|---:|---:|---:|---:|---:|---:|
| extended | 225 | 225 | 0.756 | 5.00 | 1.001 | 1.001 | 1.022 |
| vanilla | 225 | 225 | 0.955 | 5.00 | 1.000 | 1.003 | 1.015 |
