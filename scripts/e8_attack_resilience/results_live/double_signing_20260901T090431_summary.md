# LIVE Double-signing detection test (E8 A9)

Total duration: 107s. Witnesses: ['engram-node01', 'engram-node02', 'engram-node03', 'engram-node05', 'engram-node06', 'engram-node07'].

## Verdict

- 6/6 witness nodes logged real `DuplicateVoteEvidence`: **True**
- Safety held (6 honest witnesses' AppHash never diverged at the same height during or after the double-signing window): **True**
- Divergence events: 0

## Detection latency

- engram-node01: type=DuplicateVote validator=4B322E5153B01CD3AD477FFCF1E70DC388595136 offense_height=2507 detected_at_height=2508 latency=1 blocks
- engram-node02: type=DuplicateVote validator=4B322E5153B01CD3AD477FFCF1E70DC388595136 offense_height=2507 detected_at_height=2508 latency=1 blocks
- engram-node03: type=DuplicateVote validator=4B322E5153B01CD3AD477FFCF1E70DC388595136 offense_height=2507 detected_at_height=2508 latency=1 blocks
- engram-node05: type=DuplicateVote validator=4B322E5153B01CD3AD477FFCF1E70DC388595136 offense_height=2507 detected_at_height=2508 latency=1 blocks
- engram-node06: type=DuplicateVote validator=4B322E5153B01CD3AD477FFCF1E70DC388595136 offense_height=2507 detected_at_height=2508 latency=1 blocks
- engram-node07: type=DuplicateVote validator=4B322E5153B01CD3AD477FFCF1E70DC388595136 offense_height=2507 detected_at_height=2508 latency=1 blocks
