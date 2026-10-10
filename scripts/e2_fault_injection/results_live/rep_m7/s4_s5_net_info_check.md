# S4/S5 real /net_info peer-count cross-check

Query.State never carries ActiveAnchors/p2p_healthy (documented stale field, logger.py's _decode_query_state) -- this cross-checks engram-node01's real connected-peer count via CometBFT's own /net_info RPC during the isolation window, independent of the app-level FSM state.

| t (s) | phase | n_peers |
|---:|---|---:|
| 1076 | S4_p2p_eclipse_partial | 6 |
| 1087 | S4_p2p_eclipse_partial | 6 |
| 1105 | S4_p2p_eclipse_partial | 6 |
| 1122 | S4_p2p_eclipse_partial | 6 |
| 1141 | S4_p2p_eclipse_partial | 6 |
| 1154 | S4_p2p_eclipse_partial | 6 |
| 1165 | S4_p2p_eclipse_partial | 6 |
| 1178 | S4_p2p_eclipse_partial | 6 |
| 1191 | S4_p2p_eclipse_partial | 6 |
| 1203 | S4_p2p_eclipse_partial | 6 |
| 1637 | S5_anchor_isolation | 6 |
| 1652 | S5_anchor_isolation | error: timed out |
| 1667 | S5_anchor_isolation | error: timed out |
| 1682 | S5_anchor_isolation | error: timed out |
| 1697 | S5_anchor_isolation | error: timed out |
| 1712 | S5_anchor_isolation | error: timed out |
| 1728 | S5_anchor_isolation | error: timed out |
| 1743 | S5_anchor_isolation | error: timed out |
| 1758 | S5_anchor_isolation | error: timed out |
| 1773 | S5_anchor_isolation | error: timed out |
| 1788 | S5_anchor_isolation | error: timed out |
| 1803 | S5_anchor_isolation | error: timed out |
| 1814 | S5_anchor_isolation | 6 |
| 1826 | S5_anchor_isolation | 6 |
