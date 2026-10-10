# S4/S5 real /net_info peer-count cross-check

Query.State never carries ActiveAnchors/p2p_healthy (documented stale field, logger.py's _decode_query_state) -- this cross-checks engram-node01's real connected-peer count via CometBFT's own /net_info RPC during the isolation window, independent of the app-level FSM state.

| t (s) | phase | n_peers |
|---:|---|---:|
| 1104 | S4_p2p_eclipse_partial | 6 |
| 1115 | S4_p2p_eclipse_partial | 6 |
| 1128 | S4_p2p_eclipse_partial | 6 |
| 1140 | S4_p2p_eclipse_partial | 6 |
| 1151 | S4_p2p_eclipse_partial | 6 |
| 1164 | S4_p2p_eclipse_partial | 6 |
| 1174 | S4_p2p_eclipse_partial | 6 |
| 1187 | S4_p2p_eclipse_partial | 6 |
| 1201 | S4_p2p_eclipse_partial | 6 |
| 1211 | S4_p2p_eclipse_partial | 6 |
| 1222 | S4_p2p_eclipse_partial | 6 |
| 1235 | S4_p2p_eclipse_partial | 6 |
| 1856 | S5_anchor_isolation | 6 |
| 1866 | S5_anchor_isolation | 6 |
| 1877 | S5_anchor_isolation | 6 |
| 1887 | S5_anchor_isolation | 6 |
| 1898 | S5_anchor_isolation | 6 |
| 1909 | S5_anchor_isolation | 6 |
| 1919 | S5_anchor_isolation | 6 |
| 1930 | S5_anchor_isolation | 6 |
| 1942 | S5_anchor_isolation | 6 |
| 1954 | S5_anchor_isolation | 6 |
| 1967 | S5_anchor_isolation | 6 |
| 1979 | S5_anchor_isolation | 6 |
| 1991 | S5_anchor_isolation | 6 |
| 2004 | S5_anchor_isolation | 6 |
| 2016 | S5_anchor_isolation | 6 |
| 2028 | S5_anchor_isolation | 6 |
| 2041 | S5_anchor_isolation | 6 |
