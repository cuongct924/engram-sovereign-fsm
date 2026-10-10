# S4/S5 real /net_info peer-count cross-check

Query.State never carries ActiveAnchors/p2p_healthy (documented stale field, logger.py's _decode_query_state) -- this cross-checks engram-node01's real connected-peer count via CometBFT's own /net_info RPC during the isolation window, independent of the app-level FSM state.

| t (s) | phase | n_peers |
|---:|---|---:|
| 1145 | S4_p2p_eclipse_partial | 5 |
| 1161 | S4_p2p_eclipse_partial | 5 |
| 1175 | S4_p2p_eclipse_partial | 5 |
| 1188 | S4_p2p_eclipse_partial | 5 |
| 1200 | S4_p2p_eclipse_partial | 5 |
| 1212 | S4_p2p_eclipse_partial | 5 |
| 1225 | S4_p2p_eclipse_partial | 5 |
| 1238 | S4_p2p_eclipse_partial | 5 |
| 1251 | S4_p2p_eclipse_partial | 6 |
| 1266 | S4_p2p_eclipse_partial | 5 |
| 1680 | S5_anchor_isolation | error: timed out |
| 1695 | S5_anchor_isolation | error: timed out |
| 1710 | S5_anchor_isolation | error: timed out |
| 1724 | S5_anchor_isolation | error: timed out |
| 1742 | S5_anchor_isolation | error: timed out |
| 1760 | S5_anchor_isolation | error: timed out |
| 1778 | S5_anchor_isolation | error: timed out |
| 1797 | S5_anchor_isolation | error: timed out |
| 1815 | S5_anchor_isolation | error: timed out |
| 1833 | S5_anchor_isolation | error: timed out |
| 1848 | S5_anchor_isolation | 5 |
| 1860 | S5_anchor_isolation | 5 |
