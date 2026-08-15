# LIVE E4 churn attack, A3 (Churn-based Rotation)

attacker-a3-01 dials engram-node02, attacker-a3-02 dials engram-node04 (the real, empirically-confirmed next proposer after node02 in this cluster's rotation order, node02 -> node04 -> node01 -> node03, per /dump_consensus_state); churn = 8 real `docker stop`/`docker start` cycles on both attacker containers together each cycle (on=15s stopped, off=20s reconnected) -- a genuine TCP teardown each cycle, not netem packet loss (an earlier design using 100% loss never actually disconnected CometBFT's MConnection, confirmed via real node02 logs). MaxChurnRate=5 (1h rolling window). Total duration: 475s.

**Proposer-visibility caveat:** only node02 and node04 have an attacker as a peer, so the committed Healthy boolean only reflects the degraded view when one of THEM proposes -- an intermittent, not sustained, deviation is still the honest expectation, but degrading two ADJACENT proposers (vs. the single-attacker design's one proposer in four) gives DownHysteresisThreshold's 2-consecutive-unhealthy requirement a real chance to accumulate, since a single-target run showed a confirmed real churn_rate excursion (22-25 vs MaxChurnRate=5) that never transitioned the FSM -- the other 3 honest proposers reset the streak every time.

## Verdict

- Baseline FSM states: ['ANCHORED']
- Attack-window FSM states: ['ANCHORED']
- Recovery-window FSM states: ['ANCHORED']
- Final state: {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'}

- FSM deviated from baseline during the churn window (expected/correct outcome): **False**
- FSM recovered back to baseline states afterward (hysteresis bounded it): **True**

## Full timeline

| t (s) | phase | states |
|---:|---|---|
| 0.0 | baseline | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 5.0 | baseline | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 10.1 | baseline | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 15.1 | baseline | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 20.1 | baseline | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 25.2 | baseline | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 42.5 | settle | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 47.6 | settle | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 52.6 | settle | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 57.7 | settle | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 63.9 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 69.0 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 74.0 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 79.4 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 84.5 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 89.5 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 94.6 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 100.0 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 105.0 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 110.1 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 115.5 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 120.5 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 125.5 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 130.5 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 136.0 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 141.0 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 146.1 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 151.5 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 156.6 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 161.6 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 166.7 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 172.0 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 177.1 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 182.1 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 187.5 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 192.6 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 197.6 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 202.6 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 208.1 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 213.1 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 218.1 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 223.5 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 228.6 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 233.6 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 238.7 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 244.3 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 249.4 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 254.4 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 260.0 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 265.1 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 270.1 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 275.1 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 280.6 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 285.6 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 290.7 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 296.1 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 301.1 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 306.2 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 311.2 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 316.6 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 321.6 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 326.7 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 332.1 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 337.1 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 342.2 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 347.2 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 353.9 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 358.9 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 364.0 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 369.0 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 374.0 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 379.0 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 384.1 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 389.1 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 394.1 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 399.3 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 404.3 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 409.3 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 414.4 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 419.4 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 424.5 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 429.5 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 434.5 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 439.6 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 444.6 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 449.6 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 454.7 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 459.7 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 464.7 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 469.8 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
