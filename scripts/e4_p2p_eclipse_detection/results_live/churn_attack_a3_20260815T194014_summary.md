# LIVE E4 churn attack, A3 (Churn-based Rotation)

attacker-a3-01 dials engram-node02; churn = 8 real `docker stop`/`docker start` cycles on the attacker container itself (on=15s stopped, off=20s reconnected) -- a genuine TCP teardown each cycle, not netem packet loss (an earlier design using 100% loss never actually disconnected CometBFT's MConnection, confirmed via real node02 logs). MaxChurnRate=5 (1h rolling window). Total duration: 458s.

**Proposer-visibility caveat:** only node02 has this attacker as a peer, so the committed Healthy boolean only reflects the degraded view when node02 itself proposes -- an intermittent, not sustained, deviation is the honest expectation.

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
| 5.1 | baseline | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 10.1 | baseline | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 15.1 | baseline | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 20.2 | baseline | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 25.2 | baseline | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 31.2 | settle | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 36.2 | settle | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 41.3 | settle | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 46.3 | settle | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 51.6 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 56.6 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 61.6 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 66.9 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 71.9 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 76.9 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 82.0 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 87.2 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 92.2 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 97.2 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 102.5 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 107.5 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 112.6 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 117.6 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 122.8 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 127.8 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 132.8 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 138.1 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 143.1 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 148.1 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 153.2 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 158.4 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 163.4 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 168.5 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 173.7 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 178.7 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 183.7 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 188.8 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 194.0 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 199.0 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 204.1 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 209.3 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 214.3 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 219.4 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 224.4 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 229.6 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 234.6 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 239.7 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 244.9 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 249.9 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 254.9 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 260.0 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 265.2 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 270.3 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 275.3 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 280.6 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 285.6 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 290.6 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 295.6 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 300.9 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 305.9 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 310.9 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 316.2 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 321.2 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 326.3 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 331.3 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 337.5 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 342.7 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 347.7 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 352.8 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 357.8 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 362.8 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 367.8 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 372.8 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 377.9 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 382.9 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 387.9 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 392.9 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 398.0 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 403.0 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 408.0 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 413.0 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 418.1 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 423.1 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 428.1 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 433.1 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 438.2 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 443.2 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 448.2 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 453.2 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
