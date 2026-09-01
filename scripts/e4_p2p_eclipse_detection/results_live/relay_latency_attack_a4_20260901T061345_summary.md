# LIVE E4 relay-latency attack, A4 (Relay Node Attack)

350ms +-50ms netem delay held on engram-node04's real P2P interfaces (eth1, eth2, eth3, eth4, eth5, eth8, the pairwise validator-link networks -- not eth0/Pumba's default, see module doc) for the whole attack window -- not toggled, a sustained degradation. MaxPeerLatency=200ms. Total duration: 400s.

**Quorum note:** unlike A3's single-peer-link churn (visible to only 2 of 7 validators' own local view, see live_churn_attack.py's module doc), delaying all of engram-node04's real P2P links degrades every OTHER validator's own RTT measurement of its connection to engram-node04 too -- all 7 validators should independently compute Healthy=false, past the >2/3 quorum `ProcessProposal` (x/sovereignty/proposal.go:293-303) requires to commit. A real FSM transition is therefore the expected outcome here, unlike A3's minority-visible design.

## Verdict

- Baseline FSM states: ['ANCHORED']
- Attack-window FSM states: ['ANCHORED', 'SOVEREIGN', 'SUSPICIOUS']
- Recovery-window FSM states: ['ANCHORED', 'RECOVERING', 'SOVEREIGN']
- Final state: {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'}

- FSM deviated from baseline during the delay window (True is the expected/correct outcome here -- see Quorum note above): **True**
- FSM recovered back to baseline states afterward (hysteresis bounded it): **False**

## Full timeline

| t (s) | phase | states |
|---:|---|---|
| 0.6 | baseline | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 5.8 | baseline | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 10.9 | baseline | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 15.9 | baseline | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 21.0 | baseline | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 26.0 | baseline | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 34.5 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 41.0 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 47.7 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 54.1 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 60.7 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 67.2 | attack | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 73.7 | attack | {'engram-node01': 'SUSPICIOUS', 'engram-node02': 'SUSPICIOUS', 'engram-node03': 'SUSPICIOUS', 'engram-node04': 'SUSPICIOUS', 'engram-node05': 'SUSPICIOUS', 'engram-node06': 'SUSPICIOUS', 'engram-node07': 'SUSPICIOUS'} |
| 80.2 | attack | {'engram-node01': 'SUSPICIOUS', 'engram-node02': 'SUSPICIOUS', 'engram-node03': 'SUSPICIOUS', 'engram-node04': 'SUSPICIOUS', 'engram-node05': 'SUSPICIOUS', 'engram-node06': 'SUSPICIOUS', 'engram-node07': 'SUSPICIOUS'} |
| 86.7 | attack | {'engram-node01': 'SUSPICIOUS', 'engram-node02': 'SUSPICIOUS', 'engram-node03': 'SUSPICIOUS', 'engram-node04': 'SUSPICIOUS', 'engram-node05': 'SUSPICIOUS', 'engram-node06': 'SUSPICIOUS', 'engram-node07': 'SUSPICIOUS'} |
| 93.3 | attack | {'engram-node01': 'SUSPICIOUS', 'engram-node02': 'SUSPICIOUS', 'engram-node03': 'SUSPICIOUS', 'engram-node04': 'SUSPICIOUS', 'engram-node05': 'SUSPICIOUS', 'engram-node06': 'SUSPICIOUS', 'engram-node07': 'SUSPICIOUS'} |
| 99.6 | attack | {'engram-node01': 'SUSPICIOUS', 'engram-node02': 'SUSPICIOUS', 'engram-node03': 'SUSPICIOUS', 'engram-node04': 'SUSPICIOUS', 'engram-node05': 'SUSPICIOUS', 'engram-node06': 'SUSPICIOUS', 'engram-node07': 'SUSPICIOUS'} |
| 106.6 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN', 'engram-node05': 'SOVEREIGN', 'engram-node06': 'SOVEREIGN', 'engram-node07': 'SOVEREIGN'} |
| 113.1 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN', 'engram-node05': 'SOVEREIGN', 'engram-node06': 'SOVEREIGN', 'engram-node07': 'SOVEREIGN'} |
| 119.6 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN', 'engram-node05': 'SOVEREIGN', 'engram-node06': 'SOVEREIGN', 'engram-node07': 'SOVEREIGN'} |
| 126.1 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN', 'engram-node05': 'SOVEREIGN', 'engram-node06': 'SOVEREIGN', 'engram-node07': 'SOVEREIGN'} |
| 132.5 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN', 'engram-node05': 'SOVEREIGN', 'engram-node06': 'SOVEREIGN', 'engram-node07': 'SOVEREIGN'} |
| 139.0 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN', 'engram-node05': 'SOVEREIGN', 'engram-node06': 'SOVEREIGN', 'engram-node07': 'SOVEREIGN'} |
| 145.5 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN', 'engram-node05': 'SOVEREIGN', 'engram-node06': 'SOVEREIGN', 'engram-node07': 'SOVEREIGN'} |
| 152.0 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN', 'engram-node05': 'SOVEREIGN', 'engram-node06': 'SOVEREIGN', 'engram-node07': 'SOVEREIGN'} |
| 158.4 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN', 'engram-node05': 'SOVEREIGN', 'engram-node06': 'SOVEREIGN', 'engram-node07': 'SOVEREIGN'} |
| 164.8 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN', 'engram-node05': 'SOVEREIGN', 'engram-node06': 'SOVEREIGN', 'engram-node07': 'SOVEREIGN'} |
| 171.2 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN', 'engram-node05': 'SOVEREIGN', 'engram-node06': 'SOVEREIGN', 'engram-node07': 'SOVEREIGN'} |
| 177.6 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN', 'engram-node05': 'SOVEREIGN', 'engram-node06': 'SOVEREIGN', 'engram-node07': 'SOVEREIGN'} |
| 184.1 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN', 'engram-node05': 'SOVEREIGN', 'engram-node06': 'SOVEREIGN', 'engram-node07': 'SOVEREIGN'} |
| 190.6 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN', 'engram-node05': 'SOVEREIGN', 'engram-node06': 'SOVEREIGN', 'engram-node07': 'SOVEREIGN'} |
| 197.1 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN', 'engram-node05': 'SOVEREIGN', 'engram-node06': 'SOVEREIGN', 'engram-node07': 'SOVEREIGN'} |
| 203.6 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN', 'engram-node05': 'SOVEREIGN', 'engram-node06': 'SOVEREIGN', 'engram-node07': 'SOVEREIGN'} |
| 210.0 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN', 'engram-node05': 'SOVEREIGN', 'engram-node06': 'SOVEREIGN', 'engram-node07': 'SOVEREIGN'} |
| 216.4 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN', 'engram-node05': 'SOVEREIGN', 'engram-node06': 'SOVEREIGN', 'engram-node07': 'SOVEREIGN'} |
| 222.9 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN', 'engram-node05': 'SOVEREIGN', 'engram-node06': 'SOVEREIGN', 'engram-node07': 'SOVEREIGN'} |
| 229.4 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN', 'engram-node05': 'SOVEREIGN', 'engram-node06': 'SOVEREIGN', 'engram-node07': 'SOVEREIGN'} |
| 235.8 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN', 'engram-node05': 'SOVEREIGN', 'engram-node06': 'SOVEREIGN', 'engram-node07': 'SOVEREIGN'} |
| 242.3 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN', 'engram-node05': 'SOVEREIGN', 'engram-node06': 'SOVEREIGN', 'engram-node07': 'SOVEREIGN'} |
| 248.7 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN', 'engram-node05': 'SOVEREIGN', 'engram-node06': 'SOVEREIGN', 'engram-node07': 'SOVEREIGN'} |
| 255.1 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN', 'engram-node05': 'SOVEREIGN', 'engram-node06': 'SOVEREIGN', 'engram-node07': 'SOVEREIGN'} |
| 261.7 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN', 'engram-node05': 'SOVEREIGN', 'engram-node06': 'SOVEREIGN', 'engram-node07': 'SOVEREIGN'} |
| 268.2 | attack | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN', 'engram-node05': 'SOVEREIGN', 'engram-node06': 'SOVEREIGN', 'engram-node07': 'SOVEREIGN'} |
| 278.3 | recovery | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN', 'engram-node05': 'SOVEREIGN', 'engram-node06': 'SOVEREIGN', 'engram-node07': 'SOVEREIGN'} |
| 283.6 | recovery | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN', 'engram-node05': 'SOVEREIGN', 'engram-node06': 'SOVEREIGN', 'engram-node07': 'SOVEREIGN'} |
| 288.6 | recovery | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN', 'engram-node05': 'SOVEREIGN', 'engram-node06': 'SOVEREIGN', 'engram-node07': 'SOVEREIGN'} |
| 293.7 | recovery | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN', 'engram-node05': 'SOVEREIGN', 'engram-node06': 'SOVEREIGN', 'engram-node07': 'SOVEREIGN'} |
| 298.8 | recovery | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN', 'engram-node05': 'SOVEREIGN', 'engram-node06': 'SOVEREIGN', 'engram-node07': 'SOVEREIGN'} |
| 303.8 | recovery | {'engram-node01': 'SOVEREIGN', 'engram-node02': 'SOVEREIGN', 'engram-node03': 'SOVEREIGN', 'engram-node04': 'SOVEREIGN', 'engram-node05': 'SOVEREIGN', 'engram-node06': 'SOVEREIGN', 'engram-node07': 'SOVEREIGN'} |
| 308.9 | recovery | {'engram-node01': 'RECOVERING', 'engram-node02': 'RECOVERING', 'engram-node03': 'RECOVERING', 'engram-node04': 'RECOVERING', 'engram-node05': 'RECOVERING', 'engram-node06': 'RECOVERING', 'engram-node07': 'RECOVERING'} |
| 313.9 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 319.0 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 324.1 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 329.1 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 334.2 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 339.2 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 344.3 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 349.3 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 354.4 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 359.5 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 364.5 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 369.6 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 374.7 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 379.7 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 384.8 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 389.8 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
| 394.8 | recovery | {'engram-node01': 'ANCHORED', 'engram-node02': 'ANCHORED', 'engram-node03': 'ANCHORED', 'engram-node04': 'ANCHORED', 'engram-node05': 'ANCHORED', 'engram-node06': 'ANCHORED', 'engram-node07': 'ANCHORED'} |
