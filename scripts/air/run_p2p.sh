#!/usr/bin/env bash
# P2P detection evidence on fresh chains: A1 control (no attacker) x3, P3 churn x3, P4 relay latency x3, then B8 x3.
# Takes about 4-5 hours. Results land in the experiments' own results_live directories.
source "$(dirname "$0")/common.sh"
say "p2p start"
for t in 1 2 3; do
  fresh engram "control_t$t"
  python3 -u scripts/e4_p2p_eclipse_detection/live_sybil_attack.py a1 --control > "$LOGDIR/control_t$t.log" 2>&1; say "control_t$t exit=$?"
done
for t in 1 2 3; do
  fresh engram "p3_t$t"
  python3 -u scripts/e4_p2p_eclipse_detection/live_churn_attack.py > "$LOGDIR/p3_t$t.log" 2>&1; say "p3_t$t exit=$?"
  fresh engram "p4_t$t"
  python3 -u scripts/e4_p2p_eclipse_detection/live_relay_latency_attack.py > "$LOGDIR/p4_t$t.log" 2>&1; say "p4_t$t exit=$?"
done
python3 -u scripts/e8_attack_resilience/live_collusion_suite.py b8 --trials 3 > "$LOGDIR/b8.log" 2>&1; say "b8 exit=$?"
say "p2p done"
