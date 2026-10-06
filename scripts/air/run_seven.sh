#!/usr/bin/env bash
# Seven-scenario run (S1-S7) x5 on fresh chains with the heuristic fallback and a 600 s wait for ANCHORED between scenarios,
# so that every scenario starts healthy. Takes about 5 hours. Each run's files are kept in
# scripts/e2_fault_injection/results_live/rep_l<N>; the tracked single-run files are restored afterwards.
source "$(dirname "$0")/common.sh"
export ENGRAM_SCENARIO_COOLDOWN_S=600
R=scripts/e2_fault_injection/results_live
cp .env "$LOGDIR/env.before_seven"
restore() { cp "$LOGDIR/env.before_seven" .env; }
trap restore EXIT
sed -i.bak 's/^ENGRAM_PARAM_PROOF_ONLY_RECOVERY=true/ENGRAM_PARAM_PROOF_ONLY_RECOVERY=false/' .env
say "seven start; heuristic lines in .env: $(grep -c 'PROOF_ONLY_RECOVERY=false' .env)"
for t in 1 2 3 4 5; do
  fresh engram "seven_t$t"
  python3 -u scripts/e2_fault_injection/live_scenario_matrix.py > "$LOGDIR/seven_t$t.log" 2>&1; say "seven_t$t matrix exit=$?"
  mkdir -p "$R/rep_l$t"
  for f in s1_normal s2_btc_congestion s3_da_unavailable s4_p2p_eclipse_partial s5_anchor_isolation s6_combined_btc_da_failure s7_recovery; do
    cp "$R/$f.csv" "$R/rep_l$t/" 2>/dev/null
  done
  cp "$R/s4_s5_net_info_check.md" "$R/s2e_throughput_latency.md" "$R/rep_l$t/" 2>/dev/null
  git checkout -- "$R"/s?_*.csv "$R/s4_s5_net_info_check.md" "$R/s2e_throughput_latency.md" 2>/dev/null
done
restore
say "seven done"
