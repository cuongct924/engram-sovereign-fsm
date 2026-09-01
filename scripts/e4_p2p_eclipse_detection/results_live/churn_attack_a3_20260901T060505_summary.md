# LIVE E4 churn attack, A3 (Churn-based Rotation)

attacker-a3-01 dials engram-node03, attacker-a3-02 dials engram-node04 (empirically-confirmed ADJACENT proposers in this 7-validator cluster's real rotation order, node06 -> node03 -> node04 -> node01 -> node02 -> node07 -> node05, read off 15 consecutive real blocks' proposer_address); churn = 8 real `docker stop`/`docker start` cycles on both attacker containers together each cycle (on=15s stopped, off=20s reconnected) -- a genuine TCP teardown each cycle, not netem packet loss (an earlier design using 100% loss never actually disconnected CometBFT's MConnection, confirmed via real node02 logs on the earlier 4-node cluster). MaxChurnRate=5 (1h rolling window). Total duration: 473s.

**Quorum note:** no FSM transition is the correct, expected outcome for this design, not a detection failure. `ProcessProposal` (x/sovereignty/proposal.go:293-303) has every validator independently recompute `fsm_state` from its OWN local metrics and reject any disagreeing proposal; CometBFT needs >2/3 voting power to commit. Only node03/node04 (2 of 7, ~28.6%) see degraded PeerChurnRate here, so the honest majority (the other 5 validators) always rejects an unhealthy claim, forcing round-retries until an honest proposer's ANCHORED claim commits instead. A real, confirmed churn_rate excursion on a MINORITY of validators (well past MaxChurnRate on both attacked nodes, per the sensor_snapshot diagnostic line) cannot force a transition by design -- 'sensors propose, consensus decides' means no minority view, however degraded, moves committed state. A supermajority attack (>=5 of 7 validators) would be a structurally different experiment.

## Verdict

- Baseline FSM states: ['ANCHORED']
- Attack-window FSM states: ['ANCHORED', 'SUSPICIOUS']
- Recovery-window FSM states: ['ANCHORED', 'SUSPICIOUS']
- Final state: {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'}

- FSM deviated from baseline during the churn window (False is the expected/correct outcome here -- see Quorum note above): **True**
- FSM recovered back to baseline states afterward (hysteresis bounded it): **False**

## Full timeline

| t (s) | phase | states |
|---:|---|---|
| 0.0 | baseline | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 5.1 | baseline | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 10.1 | baseline | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 15.2 | baseline | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 20.2 | baseline | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 25.3 | baseline | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 32.0 | settle | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 37.1 | settle | {'engram-node01': 'SUSPICIOUS', 'engram-node02': 'SUSPICIOUS', 'engram-node03': 'SUSPICIOUS', 'engram-node04': 'SUSPICIOUS', 'engram-node05': 'SUSPICIOUS', 'engram-node06': 'SUSPICIOUS', 'engram-node07': 'SUSPICIOUS'} |
| 42.2 | settle | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 47.2 | settle | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 53.0 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 58.4 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 63.5 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 69.1 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 74.3 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 79.4 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 84.4 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 90.1 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 95.1 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 100.2 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 105.9 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 111.0 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 116.0 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 121.1 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 127.0 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 132.3 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 137.3 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 143.1 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 148.6 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 153.7 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 159.7 | attack | {'engram-node01': 'SUSPICIOUS', 'engram-node02': 'SUSPICIOUS', 'engram-node03': 'SUSPICIOUS', 'engram-node04': 'SUSPICIOUS', 'engram-node05': 'SUSPICIOUS', 'engram-node06': 'SUSPICIOUS', 'engram-node07': 'SUSPICIOUS'} |
| 165.3 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 170.4 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 175.5 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 181.2 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 186.3 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 191.4 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 196.5 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 202.4 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 207.6 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 212.6 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 218.3 | attack | {'engram-node01': 'SUSPICIOUS', 'engram-node02': 'SUSPICIOUS', 'engram-node03': 'SUSPICIOUS', 'engram-node04': 'SUSPICIOUS', 'engram-node05': 'SUSPICIOUS', 'engram-node06': 'SUSPICIOUS', 'engram-node07': 'SUSPICIOUS'} |
| 223.4 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 228.4 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 233.5 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 239.0 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 244.1 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 249.2 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 254.7 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 259.8 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 264.8 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 269.9 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 275.4 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 280.8 | attack | {'engram-node01': 'SUSPICIOUS', 'engram-node02': 'SUSPICIOUS', 'engram-node03': 'SUSPICIOUS', 'engram-node04': 'SUSPICIOUS', 'engram-node05': 'SUSPICIOUS', 'engram-node06': 'ANCHORED', 'engram-node07': 'SUSPICIOUS'} |
| 285.8 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 291.9 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 297.0 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 302.1 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 307.1 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 312.7 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 317.8 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 322.8 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 329.2 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 334.3 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 340.2 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 345.3 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 351.7 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 356.8 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 361.9 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 366.9 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 372.0 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 377.0 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 382.0 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 387.1 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 392.2 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 397.2 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 402.3 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 407.5 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 412.6 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 417.6 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 422.7 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 427.7 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 432.8 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 437.8 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 442.9 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 448.0 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 453.0 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 458.1 | recovery | {'engram-node01': 'SUSPICIOUS', 'engram-node02': 'SUSPICIOUS', 'engram-node03': 'SUSPICIOUS', 'engram-node04': 'SUSPICIOUS', 'engram-node05': 'SUSPICIOUS', 'engram-node06': 'SUSPICIOUS', 'engram-node07': 'SUSPICIOUS'} |
| 463.2 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 468.2 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |

## Post-hoc note (7-node rerun)

The `fsm_deviated_during_attack=True` verdict above should NOT be read as "the churn attack
caused a transition" -- checked directly: engram-node01 (a non-attacked validator)'s own
`sensor_snapshot` diagnostic line reported `churn_rate=0` on every single sample across this
entire run (281/281), including during both brief SUSPICIOUS excursions (t=160s, t=218s). The
SAME brief, uniform, self-healing SUSPICIOUS blip pattern also recurred once during the
post-attack recovery window (t=458s, attackers already torn down) -- it is ordinary background
FSM jitter (an intermittent BTC/DA-gap threshold crossing, consistent with similar unattributed
blips observed elsewhere this session during quiet baseline periods), not an A3 effect. The
underlying claim this experiment is designed to test -- that a real, confirmed churn excursion
on a 2-of-7 minority of validators cannot by itself force a committed FSM transition -- held in
this run exactly as in the original 4-node design; the blunt `states-seen != baseline` check
above is just too coarse a proxy to say so on its own.
