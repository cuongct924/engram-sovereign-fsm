# S4/S5 real /net_info peer-count cross-check

Query.State never carries ActiveAnchors/p2p_healthy (documented stale field, logger.py's _decode_query_state) -- this cross-checks engram-node01's real connected-peer count via CometBFT's own /net_info RPC during the isolation window, independent of the app-level FSM state.

| t (s) | phase | n_peers |
|---:|---|---:|
| 1093 | S4_p2p_eclipse_partial | 6 |
| 1104 | S4_p2p_eclipse_partial | 6 |
| 1121 | S4_p2p_eclipse_partial | 6 |
| 1137 | S4_p2p_eclipse_partial | 6 |
| 1150 | S4_p2p_eclipse_partial | 6 |
| 1164 | S4_p2p_eclipse_partial | 6 |
| 1177 | S4_p2p_eclipse_partial | 6 |
| 1191 | S4_p2p_eclipse_partial | 6 |
| 1202 | S4_p2p_eclipse_partial | 6 |
| 1216 | S4_p2p_eclipse_partial | 6 |
| 1654 | S5_anchor_isolation | error: timed out |
| 1669 | S5_anchor_isolation | error: timed out |
| 1685 | S5_anchor_isolation | error: timed out |
| 1699 | S5_anchor_isolation | error: timed out |
| 1716 | S5_anchor_isolation | error: timed out |
| 1735 | S5_anchor_isolation | error: timed out |
| 1753 | S5_anchor_isolation | error: timed out |
| 1771 | S5_anchor_isolation | error: timed out |
| 1789 | S5_anchor_isolation | error: timed out |
| 1808 | S5_anchor_isolation | error: timed out |
| 1822 | S5_anchor_isolation | 6 |
| 1834 | S5_anchor_isolation | 6 |
