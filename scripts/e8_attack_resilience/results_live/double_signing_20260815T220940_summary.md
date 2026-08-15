# LIVE Double-signing detection test (E8 A9)

Total duration: 321s. Witnesses: ['engram-node01', 'engram-node02', 'engram-node03'].

## Verdict

- 3/3 witness nodes logged real `DuplicateVoteEvidence`: **True**
- Safety held (3 honest witnesses' AppHash never diverged at the same height during or after the double-signing window): **True**
- Divergence events: 0

## Detection latency

- engram-node01: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3823 detected_at_height=3824 latency=1 blocks
- engram-node01: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3824 detected_at_height=3825 latency=1 blocks
- engram-node01: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3825 detected_at_height=3826 latency=1 blocks
- engram-node01: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3825 detected_at_height=3826 latency=1 blocks
- engram-node01: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3826 detected_at_height=3827 latency=1 blocks
- engram-node01: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3826 detected_at_height=3827 latency=1 blocks
- engram-node01: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3827 detected_at_height=3828 latency=1 blocks
- engram-node02: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3823 detected_at_height=3824 latency=1 blocks
- engram-node02: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3824 detected_at_height=3825 latency=1 blocks
- engram-node02: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3825 detected_at_height=3826 latency=1 blocks
- engram-node02: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3825 detected_at_height=3826 latency=1 blocks
- engram-node02: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3826 detected_at_height=3827 latency=1 blocks
- engram-node02: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3826 detected_at_height=3827 latency=1 blocks
- engram-node02: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3827 detected_at_height=3828 latency=1 blocks
- engram-node02: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3827 detected_at_height=3829 latency=2 blocks
- engram-node02: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3828 detected_at_height=3829 latency=1 blocks
- engram-node02: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3829 detected_at_height=3830 latency=1 blocks
- engram-node02: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3829 detected_at_height=3830 latency=1 blocks
- engram-node02: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3830 detected_at_height=3831 latency=1 blocks
- engram-node02: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3830 detected_at_height=3831 latency=1 blocks
- engram-node02: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3831 detected_at_height=3833 latency=2 blocks
- engram-node02: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3832 detected_at_height=3833 latency=1 blocks
- engram-node02: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3832 detected_at_height=3833 latency=1 blocks
- engram-node02: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3833 detected_at_height=3834 latency=1 blocks
- engram-node03: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3823 detected_at_height=3824 latency=1 blocks
- engram-node03: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3824 detected_at_height=3825 latency=1 blocks
- engram-node03: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3825 detected_at_height=3826 latency=1 blocks
- engram-node03: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3825 detected_at_height=3826 latency=1 blocks
- engram-node03: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3826 detected_at_height=3827 latency=1 blocks
- engram-node03: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3826 detected_at_height=3827 latency=1 blocks
- engram-node03: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3827 detected_at_height=3828 latency=1 blocks
- engram-node03: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3827 detected_at_height=3829 latency=2 blocks
- engram-node03: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3828 detected_at_height=3829 latency=1 blocks
- engram-node03: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3829 detected_at_height=3830 latency=1 blocks
- engram-node03: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3829 detected_at_height=3830 latency=1 blocks
- engram-node03: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3830 detected_at_height=3831 latency=1 blocks
- engram-node03: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3830 detected_at_height=3831 latency=1 blocks
- engram-node03: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3831 detected_at_height=3833 latency=2 blocks
- engram-node03: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3832 detected_at_height=3833 latency=1 blocks
- engram-node03: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3832 detected_at_height=3833 latency=1 blocks
- engram-node03: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3833 detected_at_height=3834 latency=1 blocks
- engram-node03: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3834 detected_at_height=3835 latency=1 blocks
- engram-node03: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3834 detected_at_height=3835 latency=1 blocks
- engram-node03: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3835 detected_at_height=3836 latency=1 blocks
- engram-node03: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3835 detected_at_height=3836 latency=1 blocks
- engram-node03: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3836 detected_at_height=3837 latency=1 blocks
- engram-node03: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3837 detected_at_height=3838 latency=1 blocks
- engram-node03: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3838 detected_at_height=3839 latency=1 blocks
- engram-node03: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3838 detected_at_height=3839 latency=1 blocks
- engram-node03: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3839 detected_at_height=3840 latency=1 blocks
- engram-node03: type=DuplicateVote validator=41D62E3CE3EC5E0809A2E8C7676597D360DB5C8A offense_height=3840 detected_at_height=3841 latency=1 blocks
