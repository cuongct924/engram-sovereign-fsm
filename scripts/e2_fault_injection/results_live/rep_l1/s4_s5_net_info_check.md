# S4/S5 real /net_info peer-count cross-check

Query.State never carries ActiveAnchors/p2p_healthy (documented stale field, logger.py's _decode_query_state) -- this cross-checks engram-node01's real connected-peer count via CometBFT's own /net_info RPC during the isolation window, independent of the app-level FSM state.

| t (s) | phase | n_peers |
|---:|---|---:|
| 1404 | S4_p2p_eclipse_partial | 6 |
| 1419 | S4_p2p_eclipse_partial | 6 |
| 1430 | S4_p2p_eclipse_partial | 6 |
| 1440 | S4_p2p_eclipse_partial | 6 |
| 1451 | S4_p2p_eclipse_partial | 6 |
| 1462 | S4_p2p_eclipse_partial | 6 |
| 1473 | S4_p2p_eclipse_partial | 6 |
| 1484 | S4_p2p_eclipse_partial | 6 |
| 1495 | S4_p2p_eclipse_partial | 6 |
| 1506 | S4_p2p_eclipse_partial | 6 |
| 1517 | S4_p2p_eclipse_partial | 6 |
| 1529 | S4_p2p_eclipse_partial | 6 |
| 1634 | S5_anchor_isolation | 6 |
| 1646 | S5_anchor_isolation | 6 |
| 1659 | S5_anchor_isolation | 6 |
| 1671 | S5_anchor_isolation | 6 |
| 1683 | S5_anchor_isolation | 6 |
| 1695 | S5_anchor_isolation | 6 |
| 1708 | S5_anchor_isolation | 6 |
| 1720 | S5_anchor_isolation | 6 |
| 1732 | S5_anchor_isolation | 6 |
| 1744 | S5_anchor_isolation | 6 |
| 1756 | S5_anchor_isolation | 6 |
| 1769 | S5_anchor_isolation | 6 |
| 1781 | S5_anchor_isolation | 6 |
| 1793 | S5_anchor_isolation | 6 |
| 1805 | S5_anchor_isolation | 6 |
| 1818 | S5_anchor_isolation | 6 |
