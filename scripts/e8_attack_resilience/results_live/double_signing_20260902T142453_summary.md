# LIVE Double-signing detection test (E8 A9)

Total duration: 127s. Witnesses: ['engram-node01', 'engram-node02', 'engram-node03', 'engram-node05', 'engram-node06', 'engram-node07'].

## Verdict

- 6/6 witness nodes logged real `DuplicateVoteEvidence`: **True**
- Safety held (6 honest witnesses' AppHash never diverged at the same height during or after the double-signing window): **True**
- Divergence events: 0

## Detection latency

- engram-node01: type=DuplicateVote validator=5B113185BFD9ED2CF6C24A805AFE515698468D8E offense_height=1460 detected_at_height=1461 latency=1 blocks
- engram-node02: type=DuplicateVote validator=5B113185BFD9ED2CF6C24A805AFE515698468D8E offense_height=1460 detected_at_height=1461 latency=1 blocks
- engram-node03: type=DuplicateVote validator=5B113185BFD9ED2CF6C24A805AFE515698468D8E offense_height=1460 detected_at_height=1461 latency=1 blocks
- engram-node05: type=DuplicateVote validator=5B113185BFD9ED2CF6C24A805AFE515698468D8E offense_height=1460 detected_at_height=1461 latency=1 blocks
- engram-node06: type=DuplicateVote validator=5B113185BFD9ED2CF6C24A805AFE515698468D8E offense_height=1460 detected_at_height=1461 latency=1 blocks
- engram-node07: type=DuplicateVote validator=5B113185BFD9ED2CF6C24A805AFE515698468D8E offense_height=1460 detected_at_height=1461 latency=1 blocks
