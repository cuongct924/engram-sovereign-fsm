# S4/S5 real /net_info peer-count cross-check

Query.State never carries ActiveAnchors/p2p_healthy (documented stale field, logger.py's _decode_query_state) -- this cross-checks engram-node01's real connected-peer count via CometBFT's own /net_info RPC during the isolation window, independent of the app-level FSM state.

| t (s) | phase | n_peers |
|---:|---|---:|
| 1106 | S4_p2p_eclipse_partial | 6 |
| 1123 | S4_p2p_eclipse_partial | 6 |
| 1138 | S4_p2p_eclipse_partial | 6 |
| 1152 | S4_p2p_eclipse_partial | 6 |
| 1166 | S4_p2p_eclipse_partial | 6 |
| 1178 | S4_p2p_eclipse_partial | 6 |
| 1195 | S4_p2p_eclipse_partial | 6 |
| 1210 | S4_p2p_eclipse_partial | 6 |
| 1222 | S4_p2p_eclipse_partial | 6 |
| 1238 | S4_p2p_eclipse_partial | 6 |
| 1649 | S5_anchor_isolation | 6 |
| 1661 | S5_anchor_isolation | 6 |
| 1674 | S5_anchor_isolation | 6 |
| 1686 | S5_anchor_isolation | 6 |
| 1698 | S5_anchor_isolation | 6 |
| 1711 | S5_anchor_isolation | 6 |
| 1723 | S5_anchor_isolation | 6 |
| 1735 | S5_anchor_isolation | 6 |
| 1747 | S5_anchor_isolation | 6 |
| 1760 | S5_anchor_isolation | 6 |
| 1772 | S5_anchor_isolation | 6 |
| 1784 | S5_anchor_isolation | 6 |
| 1796 | S5_anchor_isolation | 6 |
| 1809 | S5_anchor_isolation | 6 |
| 1821 | S5_anchor_isolation | 6 |
| 1833 | S5_anchor_isolation | 6 |
