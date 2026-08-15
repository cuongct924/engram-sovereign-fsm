# LIVE E4 relay-latency attack, A4 (Relay Node Attack)

350ms +-50ms netem delay held on engram-node04's real P2P interfaces (eth3, eth4, eth5, the pairwise validator-link networks -- not eth0/Pumba's default, see module doc) for the whole attack window -- not toggled, a sustained degradation. MaxPeerLatency=200ms. Total duration: 400s.

**Quorum note:** unlike A3's single-peer-link churn (visible to only 1-2 of 4 validators' own local view, see live_churn_attack.py's module doc), delaying all of engram-node04's real P2P links degrades every OTHER validator's own RTT measurement of its connection to engram-node04 too -- all 4 validators should independently compute Healthy=false, past the >2/3 quorum `ProcessProposal` (x/sovereignty/proposal.go:293-303) requires to commit. A real FSM transition is therefore the expected outcome here, unlike A3's minority-visible design.

## Verdict

- Baseline FSM states: ['ANCHORED']
- Attack-window FSM states: ['ANCHORED', 'SOVEREIGN', 'SUSPICIOUS']
- Recovery-window FSM states: ['ANCHORED', 'RECOVERING', 'SOVEREIGN']
- Final state: {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'}

- FSM deviated from baseline during the delay window (True is the expected/correct outcome here -- see Quorum note above): **True**
- FSM recovered back to baseline states afterward (hysteresis bounded it): **False**

## Full timeline

| t (s) | phase | states |
|---:|---|---|
| 0.8 | baseline | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 5.8 | baseline | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 10.9 | baseline | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 16.1 | baseline | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 21.1 | baseline | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 26.3 | baseline | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 33.2 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 38.2 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 43.3 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 48.3 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 53.3 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 58.3 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 63.4 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 68.4 | attack | {'engram-node01': 'SUSPICIOUS', 'engram-node02': 'SUSPICIOUS', 'engram-node03': 'SUSPICIOUS', 'engram-node04': 'SUSPICIOUS'} |
| 73.4 | attack | {'engram-node01': 'SUSPICIOUS', 'engram-node02': 'SUSPICIOUS', 'engram-node03': 'SUSPICIOUS', 'engram-node04': 'SUSPICIOUS'} |
| 78.4 | attack | {'engram-node01': 'SUSPICIOUS', 'engram-node02': 'SUSPICIOUS', 'engram-node03': 'SUSPICIOUS', 'engram-node04': 'SUSPICIOUS'} |
| 83.5 | attack | {'engram-node01': 'SUSPICIOUS', 'engram-node02': 'SUSPICIOUS', 'engram-node03': 'SUSPICIOUS', 'engram-node04': 'SUSPICIOUS'} |
| 88.5 | attack | {'engram-node01': 'SUSPICIOUS', 'engram-node02': 'SUSPICIOUS', 'engram-node03': 'SUSPICIOUS', 'engram-node04': 'SUSPICIOUS'} |
| 93.5 | attack | {'engram-node01': 'SUSPICIOUS', 'engram-node02': 'SUSPICIOUS', 'engram-node03': 'SUSPICIOUS', 'engram-node04': 'SUSPICIOUS'} |
| 98.8 | attack | {'engram-node01': 'SUSPICIOUS', 'engram-node02': 'SUSPICIOUS', 'engram-node03': 'SUSPICIOUS', 'engram-node04': 'SUSPICIOUS'} |
| 103.8 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN'} |
| 108.9 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN'} |
| 113.9 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN'} |
| 119.0 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN'} |
| 124.0 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN'} |
| 129.0 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN'} |
| 134.1 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN'} |
| 139.2 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN'} |
| 144.2 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN'} |
| 149.3 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN'} |
| 154.3 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN'} |
| 159.3 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN'} |
| 164.4 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN'} |
| 169.4 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN'} |
| 174.4 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN'} |
| 179.4 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN'} |
| 184.5 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN'} |
| 189.9 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN'} |
| 195.0 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN'} |
| 200.0 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN'} |
| 205.0 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN'} |
| 210.1 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN'} |
| 215.1 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN'} |
| 220.1 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN'} |
| 225.5 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN'} |
| 230.5 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN'} |
| 235.6 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN'} |
| 240.6 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN'} |
| 245.7 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN'} |
| 250.8 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN'} |
| 255.9 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN'} |
| 260.9 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN'} |
| 266.0 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN'} |
| 271.0 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN'} |
| 278.7 | recovery | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN'} |
| 283.9 | recovery | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN'} |
| 288.9 | recovery | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN'} |
| 294.0 | recovery | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'RECOVERING', 'engram-node03': 'RECOVERING', 'engram-node04': 'SOVEREIGN'} |
| 299.0 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 304.0 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 309.0 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 314.1 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 319.1 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 324.4 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 329.4 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 334.5 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 339.5 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 344.6 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 349.6 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 354.7 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 359.7 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 364.7 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 369.8 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 374.8 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 379.9 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 384.9 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 389.9 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
| 394.9 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED'} |
