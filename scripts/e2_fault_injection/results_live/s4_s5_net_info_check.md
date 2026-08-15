# S4/S5 real /net_info peer-count cross-check

Query.State never carries ActiveAnchors/p2p_healthy (documented stale field, logger.py's _decode_query_state) -- this cross-checks engram-node01's real connected-peer count via CometBFT's own /net_info RPC during the isolation window, independent of the app-level FSM state.

| t (s) | phase | n_peers |
|---:|---|---:|
| 351 | S4_p2p_eclipse_partial | 3 |
| 362 | S4_p2p_eclipse_partial | 3 |
| 373 | S4_p2p_eclipse_partial | 3 |
| 385 | S4_p2p_eclipse_partial | 3 |
| 401 | S4_p2p_eclipse_partial | 3 |
| 412 | S4_p2p_eclipse_partial | 3 |
| 428 | S4_p2p_eclipse_partial | 3 |
| 442 | S4_p2p_eclipse_partial | 3 |
| 457 | S4_p2p_eclipse_partial | 3 |
| 468 | S4_p2p_eclipse_partial | 3 |
| 509 | S5_anchor_isolation | error: timed out |
| 527 | S5_anchor_isolation | error: timed out |
| 545 | S5_anchor_isolation | error: timed out |
| 564 | S5_anchor_isolation | error: timed out |
| 582 | S5_anchor_isolation | error: timed out |
| 600 | S5_anchor_isolation | error: timed out |
| 618 | S5_anchor_isolation | error: timed out |
| 636 | S5_anchor_isolation | error: timed out |
| 654 | S5_anchor_isolation | error: timed out |
| 673 | S5_anchor_isolation | error: timed out |
| 683 | S5_anchor_isolation | 3 |
| 695 | S5_anchor_isolation | 3 |
