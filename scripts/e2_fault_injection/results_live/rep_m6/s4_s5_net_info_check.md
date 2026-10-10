# S4/S5 real /net_info peer-count cross-check

Query.State never carries ActiveAnchors/p2p_healthy (documented stale field, logger.py's _decode_query_state) -- this cross-checks engram-node01's real connected-peer count via CometBFT's own /net_info RPC during the isolation window, independent of the app-level FSM state.

| t (s) | phase | n_peers |
|---:|---|---:|
| 1115 | S4_p2p_eclipse_partial | 6 |
| 1131 | S4_p2p_eclipse_partial | 6 |
| 1146 | S4_p2p_eclipse_partial | 6 |
| 1160 | S4_p2p_eclipse_partial | 6 |
| 1175 | S4_p2p_eclipse_partial | 6 |
| 1193 | S4_p2p_eclipse_partial | 6 |
| 1208 | S4_p2p_eclipse_partial | 6 |
| 1222 | S4_p2p_eclipse_partial | 6 |
| 1233 | S4_p2p_eclipse_partial | 6 |
| 1678 | S5_anchor_isolation | 6 |
| 1690 | S5_anchor_isolation | 6 |
| 1702 | S5_anchor_isolation | 6 |
| 1715 | S5_anchor_isolation | 6 |
| 1727 | S5_anchor_isolation | 6 |
| 1739 | S5_anchor_isolation | 6 |
| 1751 | S5_anchor_isolation | 6 |
| 1763 | S5_anchor_isolation | 6 |
| 1776 | S5_anchor_isolation | 6 |
| 1788 | S5_anchor_isolation | 6 |
| 1800 | S5_anchor_isolation | 6 |
| 1812 | S5_anchor_isolation | 6 |
| 1824 | S5_anchor_isolation | 6 |
| 1836 | S5_anchor_isolation | 6 |
| 1849 | S5_anchor_isolation | 6 |
| 1861 | S5_anchor_isolation | 6 |
