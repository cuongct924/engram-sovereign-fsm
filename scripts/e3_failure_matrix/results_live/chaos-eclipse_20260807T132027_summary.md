# LIVE chaos experiment: chaos-eclipse

netem 100% packet loss on engram-node01 for 3m

Samples: 172, CSV: `chaos-eclipse_20260807T132027.csv`

| Node | Height range | RPC errors observed |
|---|---|---:|
| engram-node01 | 187..189 | 36 |
| engram-node02 | 187..199 | 0 |
| engram-node03 | 187..199 | 0 |
| engram-node04 | 187..199 | 0 |

Target container `engram-node01` final status: `running`
