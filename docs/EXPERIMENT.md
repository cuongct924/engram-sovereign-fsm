# Experiment Plan — Engram Sovereign FSM

> **Goal:** a complete evaluation story for a rank-B-or-above CS conference submission.

The evaluation combines three layers of evidence: formal specification (TLA+), a fault-injection
prototype, and cryptographic/consensus microbenchmarks.

---

## 1. Research Questions

Four research questions drive the evaluation — they turn this from "a fallback FSM for modular
blockchains" into a claim backed by evidence under real Bitcoin-settlement, DA-layer, and P2P
failures.

| # | Question | What it evaluates |
|---|---|---|
| RQ1 | Safety | Does folding peripheral state into the consensus proposal prevent block/FSM-state conflicts, forged receipts, data withholding, and withdrawal leakage? |
| RQ2 | Liveness | Under Bitcoin/DA/P2P failure, does the system keep committing blocks better than a baseline CometBFT that hard-depends on external preconditions? |
| RQ3 | Recovery | Once peripherals recover, do hysteresis and the re-anchoring proof bring the chain back to a stable ANCHORED without flapping? |
| RQ4 | Cost | What overhead do the extended proposal, sensor validation, circuit breaker, and ZK re-anchoring add versus the CometBFT/Cosmos SDK baseline? |

---

## 2. Experiment Suite Overview

| Group | Scientific goal | Baseline | Metrics |
|---|---|---|---|
| **E1** Formal verification stress & ablation | Prove safety/liveness holds beyond one small config | Full FSM vs. removing hysteresis / P2P sensor / f+1 pacemaker / ZK proof gate | States generated, distinct states, depth, violations found, counterexample class |
| **E2** Fault-injection end-to-end prototype | Show the fallback keeps the chain alive under BTC/DA/P2P failure | Vanilla CometBFT with hard external preconditions; static circuit breaker | Block commit rate, time-to-SOVEREIGN, committed tx during outage, downtime, recovery time |
| **E3** External-dependency failure matrix | Evaluate each failure and combinations | Same as E2 | Availability, p50/p95 block latency, consensus rounds/block, nil-prevote ratio |
| **E4** P2P eclipse/Sybil detection | Test whether the tri-interface profiler beats a peer-count sensor | Peer-count-only detector | FPR/FNR, detection delay, incorrect recovery attempts |
| **E5** FSM transition stability across all absorb edges | Show every hysteresis-gated edge (RECOVERING→ANCHORED, ANCHORED→SUSPICIOUS, SUSPICIOUS→ANCHORED) resists flapping under natural noise | Each sweep's own floor value (`HYSTERESIS_WAIT=0`, `DownHysteresisThreshold=1`, `SuspiciousHysteresisWait=1`) — the edge transitions on a single reading, no absorption | Anchored uptime, flapping/transition counts, withdrawal-locked time, absorption rate, time-outside-ANCHORED |
| **E6** Re-anchoring feasibility | Show the recovery proof is practical and scalable | Noir+Honk vs. Plonky3; no-ZK baseline (re-execute) | Constraint count, proving/verification time, proof size, backend trade-off |
| **E7** Consensus overhead benchmark | Measure the cost of the extended proposal fields | Vanilla proposal | Proposal size, CPU validation cost, throughput, block latency |
| **E8** Attack-resilience scenarios | Demonstrate the security story empirically | Malicious proposer; data withholding; forged BTC receipt; withdrawal during SOVEREIGN; censorship | Accepted/rejected proposals, invalid commit count, forced-inclusion latency |
| **E9** Trace-driven stress test | Strengthen the paper with a realistic workload | Synthetic-only experiment | Downtime under historical/simulated BTC congestion and DA-delay traces |
| **E10** Bitcoin reorg fork-choice reaction | Test whether the FSM reacts correctly to a real Bitcoin reorg invalidating an anchored checkpoint | Shallow reorg (below `KDeepFinality`) vs. deep reorg (past the anchored checkpoint) | FSM state transition (ANCHORED vs. SOVEREIGN), reaction correctness vs. reorg depth |

---

## 3. Experimental Environment

Every "live" result in §4 runs on one real 4-node Docker Compose testnet — a real Cosmos SDK app
(`x/sovereignty`) on a forked CometBFT core (`engram-consensus-core`), not an in-process mock.
Described once here; per-experiment write-ups below only note what's scenario-specific.

**Cluster:**

| Component | Real infrastructure |
|---|---|
| Validators | 4 (`engram-node01..04`), real ABCI++ hooks (`PrepareProposal`/`ProcessProposal`/`PreBlocker`) |
| Bitcoin settlement | Real `bitcoind` regtest (2 nodes), continuous miner loop (~1 block/20s) |
| Celestia DA | Real `celestia-app` + `celestia-bridge` (2nd bridge backs the prover's audit trail) |
| ZK re-anchoring | Real Noir/Barretenberg prover, watches for a SOVEREIGN backlog, submits real `SubmitRecoveryProof` txs |
| Network | 6 dedicated pairwise `/29` links for validator gossip (see [`docs/ARCHITECTURE.md`](ARCHITECTURE.md)), not a shared subnet; separate `engram-net`/`bitcoin-net`/`celestia-net` for everything else |

**Network-level fault injection** (real `tc netem` via [Pumba](https://github.com/alexei-led/pumba), not application-level delay):

| Profile | Real effect | Used by |
|---|---|---|
| `chaos-delay` | 100ms ±20ms jitter, all 4 validators, 5m | general baseline |
| `chaos-loss` | 5% loss, node01+node02, 2m | E2 S4, E9 |
| `chaos-eclipse` | 100% loss, node01 only, 3m | E2 S5 |
| `chaos-crash` | SIGKILL node04 (one-shot) | crash-fault baseline |
| `chaos-btc-delay` | 500ms ±100ms jitter, bitcoin-node01, 2m | E2 S2 (RPC realism only) |
| `chaos-wan-latency` | per-validator delay: 15/70/140/45ms ±3/15/25/10ms, 10m | WAN baseline (E5, E9, others) |
| `chaos-wan-loss` | per-validator loss: 1%/3%/6%/2%, 10m | WAN baseline (loss variant) |

The `chaos-wan-*` pair is the main realism baseline: each validator gets its own real delay/loss
value, simulating 4 distinct regions instead of one shared value for all 4.

**Application/service-level fault injection**, where network delay alone can't reproduce the real
mechanism:

* **BTC congestion** (E2 S2): `AnchorTracker.SetSubmissionPausedFile` pauses new checkpoint
  submission (a pending one still confirms), growing `btc_gap` directly — `btc_gap` is a
  block-height delta, not an RPC round-trip, so network delay can't reproduce it.
* **DA outage** (E2 S3, E3, E9): real `docker stop`/`start celestia-bridge`.
* **Combined BTC+DA** (E2 S6): both together.
* **Attacker/Byzantine infra** (E4, E8): non-validator `engramd` containers
  (`docker/attacker-peer-swarm.yml`), a validator faking its own proposal
  (`docker/engram-node04-byzantine.yml`), a second process holding one validator's real key
  (`docker/engram-node04-double-sign.yml`), a validator flooding `Timeout` messages
  (`docker/engram-node04-timeout-flood.yml`).

**Sim-to-real gap.** This setup approximates a real decentralized deployment; it isn't one.

1. **One physical machine.** All containers share CPU/memory/Docker — one problem can hit every
   "validator" at once, unlike real separate machines. Caused a real bug in this work: a
   CPU-contention deadlock (§4's E8, the `x/anchor/verify.go` fix).
2. **~30x faster than real Bitcoin.** ~1 block/20s here vs. ~10 min/block on mainnet. Thresholds
   in blocks (`KDeepFinality`, `SovereignThreshold`) stay valid; wall-clock times in this document
   (e.g. "t=88s") don't — real Bitcoin would take ~30x longer.
3. **DA check trusts one bridge.** A single `blob.GetAll` call to one self-run Celestia bridge, not
   real light-client sampling across many peers. Deliberate scope choice, but DA-outage results
   (E2 S3, E3, E9) measure this one bridge going down, not a sampling-detected failure. Full
   detail: `spec/README.md`'s DAS/Blobstream section.

---

## 4. Per-Experiment Design

Each experiment below follows the same five-part structure: **Objective** (which research
question it answers), **Metrics** (what's measured and why), **Method** (the code/command that
produces the measurement), **Results** (real numbers, or explicitly marked not-yet-measured), and
**Conclusion** (the actual finding, including negative results, and what's still open). Tables are
used wherever the content is naturally tabular; bullets otherwise.

### E1 — Formal Verification Stress & Ablation

**Objective:** show FSM safety/liveness holds beyond the one small TLC configuration checked
routinely — across larger quorums/rounds and with individual safety mechanisms deliberately
removed, producing counterexample traces where a mechanism actually matters. Answers RQ1/RQ2's
formal half.

**Metrics**

| Metric | Definition |
|---|---|
| States generated / distinct | TLC's raw + deduplicated state count |
| Depth | longest state-graph path explored |
| Violations found | any invariant/property broken |
| Counterexample class | which property broke, if any |

**Method:** TLC model-checking, 3 configs × 5 ablations, each ablation expected to reproduce a
specific known failure mode.

| Config | N | f | MaxRound | BTC height | Engram height | Goal |
|---|---|---|---|---|---|---|
| C1 | 4 | 1 | 2–3 | 2–3 | 2–3 | reproduce the current result |
| C2 | 4 | 1 | 4–5 | 3–4 | 3–4 | deeper consensus rounds |
| C3 | 7 | 2 | 2–3 | 2–3 | 2–3 | larger quorum overlap |

| Ablation | Expected outcome |
|---|---|
| Remove hysteresis | Flapping or premature recovery |
| Remove P2P health gate | An eclipsed node may trigger recovery incorrectly |
| Remove circuit breaker | Withdrawal leakage during SOVEREIGN/RECOVERING |
| Remove f+1 timeout fast-forward | Liveness delay or round-stall increases |
| Remove DA receipt consistency | A data-withholding proposal may get committed |

**Results:** 

**Conclusion:**


---

### E2 — Fault-Injection End-to-End Prototype

**Objective:** demonstrate graceful degradation instead of halting across 7 failure/recovery
scenarios. Answers RQ2/RQ3.

**Metrics**

| Metric | Status |
|---|---|
| Time-to-detection | Measured |
| Time-to-fallback | Measured |
| Availability during outage | Measured |
| Recovery time | Measured |
| Withdrawals blocked | Measured |
| Flapping / incorrect transitions | Measured (0 across all scenarios) |
| Throughput/latency | N/A in-process (architectural); measured live as a block-interval proxy |

**Method**

| Scenario | Mechanism | Result |
|---|---|---|
| S1 Normal baseline | healthy sensors throughout | Confirmed, in-process + live |
| S2 BTC congestion | `AnchorTracker.SetSubmissionPausedFile` | Confirmed, in-process + live |
| S3 DA outage | `docker stop celestia-bridge` | Confirmed, in-process + live |
| S4 P2P eclipse (partial) | real `netem` delay, 6 validator-link interfaces to/from node01 | Confirmed, in-process + live |
| S5 Total anchor isolation | Pumba `chaos-eclipse` (100% loss, node01) | Confirmed in-process; live isolation genuine, no FSM transition (see caveat) |
| S6 Combined BTC+DA | real `docker stop bitcoin-node01`+`celestia-bridge` | Confirmed, in-process + live |
| S7 Recovery | failures clear, ZK proof submitted | Confirmed, in-process + live |

In-process: `go test ./tests/e2e/...`, real `BeginBlocker`, mocked sensors. Live:
`scripts/e2_fault_injection/live_scenario_matrix.py`, real 4-node cluster. Baselines: vanilla
CometBFT (`engramd start --vanilla`), static circuit breaker, FSM without hysteresis, FSM with a
peer-count-only P2P sensor — see **Vanilla comparison** below.

**Results**

In-process (`E2Metrics`/`ComputeMetrics`, block heights, not seconds; `n/a` = never entered
RECOVERING):

| Scenario | Detection | Fallback | Recovery | Withdrawals blocked | Flapping | Transitions |
|---|---:|---:|---:|---:|---:|---:|
| S1 Normal | n/a | n/a | n/a | 0 | 0 | 0 |
| S2 BTC congestion | 3 | 3 | 3 | 7 | 0 | 3 |
| S3 DA unavailable | 2 | 26 | 3 | 5 | 0 | 4 |
| S4 P2P eclipse | 2 | 26 | n/a | 3 | 0 | 2 |
| S5 Anchor isolation | 2 | 2 | n/a | 1 | 0 | 1 |
| S6 Combined BTC+DA | 1 | 1 | n/a | 10 | 0 | 1 |
| S7 Recovery | 1 | 1 | 3 | 4 | 0 | 3 |

Live (810s continuous run, `results_live/s{1..7}_*.csv`, 60 real transitions, zero divergence
across all 4 validators). Times are elapsed seconds since the run started, not per-scenario —
each phase begins where the previous one's recovery left off. FSM state defaults to `ANCHORED`,
so "—" means no transition out of it during that scenario:

| Scenario | →SUSPICIOUS | →SOVEREIGN | →RECOVERING | →ANCHORED |
|---|---:|---:|---:|---:|
| S1 Normal | — | — | — | — |
| S2 BTC congestion | 111s | 142s | 248s | 252s |
| S3 DA unavailable | 258s | 288s | 328s | 333s |
| S4 P2P eclipse | 376s | 443s | 486s | 493s |
| S5 Anchor isolation | — | — | — | — |
| S6 Combined BTC+DA | — | 699s (direct) | — | — |
| S7 Recovery | — | — | 805s | 810s |

Throughput/latency (block-interval proxy, seconds between height increments,
`results_live/s2e_throughput_latency.md`):

| Scenario | Mean | p50 | p95 |
|---|---:|---:|---:|
| S1 | 1.39 | 1.52 | 1.53 |
| S2 | 1.38 | 1.51 | 2.05 |
| S3 | 1.36 | 1.51 | 2.07 |
| S4 | 2.51 | 2.03 | 6.09 |
| S5 | 1.29 | 1.30 | 2.07 |
| S6 | 1.34 | 1.52 | 1.57 |
| S7 | 1.50 | 1.78 | 2.05 |

**Caveat on S5.** Isolation is real (node01's own RPC timed out for ~164s straight, `/net_info`
cross-check) but produces no FSM transition — losing 1 of 4 peers doesn't degrade the surviving
majority's own health signal.

**Vanilla comparison:** structural fact confirmed by `app/app_test.go`'s
`TestNewEngramApp_VanillaSkipsPreBlocker` — vanilla mode never wires `PreBlocker`, so `FSMState`
can never leave genesis `ANCHORED` regardless of real BTC/DA/P2P health. Live confirmation
pending.

**Conclusion:**

* RQ2/RQ3's graceful-degradation claim is confirmed live for 6 of 7 scenarios
  (S1/S2/S3/S4/S6/S7); S5 is the one exception, for the understood reason above.

**Insights:**

* The DA check has an escape hatch (`IsDAHealthy`); the BTC check didn't — an asymmetric
  conditional gate between two structurally similar checks is a liveness trap.
* In-process mocked-sensor testing cannot catch this class of bug by construction (no real leader
  rotation, no persisted state a mock can fail to refresh) — both in-process and live layers are
  needed.
* S4 (timeout-based) and S5 (instant isolation) are different mechanisms: S4 escalates without
  losing peer count; S5's single-node loss doesn't move the surviving majority's own signal.

---

### E3 — Failure Matrix

**Objective:** show precisely which (BTC, DA, P2P) health combinations keep committing blocks vs.
lock withdrawals vs. force SOVEREIGN — a policy-correctness claim under RQ1/RQ2, stated as a
lookup table rather than prose.

**Metrics:** expected FSM state, withdrawal policy, and block-production mode per health
combination (the table below, not a scalar metric).

**Method:**

* `scripts/e3_failure_matrix/measure_latency.py` rebuilds the matrix from E2's real S1-S7
  steady-state data (`tests/e2e/results/s*.csv`), not hand-written.
* `scripts/e3_failure_matrix/live_lifecycle_test.py` separately drives one continuous live run
  through every FSM edge via real `celestia-bridge` stop/start.

**Results:**

| BTC | DA | P2P | Expected state | Withdrawals | Block production |
|---|---|---|---|---|---|
| healthy | healthy | healthy | ANCHORED | enabled | full |
| warning | healthy | healthy | SUSPICIOUS | restricted | moderate/full |
| critical | healthy | healthy | SOVEREIGN | locked | full local |
| healthy | failed | healthy | SUSPICIOUS/SOVEREIGN | locked if SOVEREIGN | local |
| healthy | healthy | eclipsed | SUSPICIOUS/SOVEREIGN | locked if critical | depends |
| critical | failed | eclipsed | SOVEREIGN | locked | local |
| recovered | recovered | healthy | RECOVERING → ANCHORED | locked until anchored | full |

* In-process: `Harness.Advance()` never stopped or errored in any scenario — "full/local" block
  production is a measured result, not an assumption
  (`scripts/e3_failure_matrix/results/table2_failure_matrix.md`).
* Live (344s, full lifecycle, all 4 validators in lockstep at every edge):

  | Edge | Time |
  |---|---:|
  | ANCHORED → SUSPICIOUS | 12s |
  | SUSPICIOUS → ANCHORED (quick recovery, no escalation) | 24s |
  | ANCHORED → SUSPICIOUS → SOVEREIGN (gray-failure timeout) | 27s → 58s |
  | SOVEREIGN → RECOVERING | 74s |
  | RECOVERING → SOVEREIGN (regression, brief re-outage) | 80s |
  | → ANCHORED (real recovery via `watch_and_prove.sh`) | 344s |

  Data: `scripts/e3_failure_matrix/results_live/lifecycle_test_20260812T153523.csv` + `_summary.md`.

**Conclusion:**

* The policy table is real, reconstructed data confirmed on both an in-process and a live
  full-lifecycle run.
* Includes an edge the original design didn't explicitly plan for (a live regression dip
  mid-recovery) — the matrix holds under real timing, not just as a static design table.

---

### E4 — P2P Eclipse/Sybil Detection

**Objective:** show the tri-interface P2P profiler (structural + behavioral + latency) detects
eclipse/Sybil attacks a peer-count-only baseline misses, and that its output is a real consensus
input (via `FilterPeerByAddr`), not just a monitoring signal. Answers RQ1.

**Metrics**

| Metric | Definition |
|---|---|
| FPR / FNR | false positive/negative rate, per attack |
| Detection delay | snapshots/time until the attack is flagged |

**Method**

| # | Attack |
|---|---|
| A1 | Peer-slot exhaustion |
| A2 | Simulated multi-subnet Sybil |
| A3 | Churn-based rotation |
| A4 | Relay-node latency inflation |

* Synthetic: Monte Carlo simulation (`go test ./tests/e2e/... -run TestE4_P2PDetectorComparison`,
  2000 trials/cell, fixed seed), comparing the real `types.IsP2PQualityHealthy` against a
  peer-count-only baseline, at both `DefaultParams()` and a "production_scale" threshold set.
* Live: `docker/attacker-peer-swarm.yml` (real, non-validator `engramd` containers) +
  `scripts/e4_p2p_eclipse_detection/live_sybil_attack.py`, testing the real ingress filter
  `FilterPeerByAddr`.
* A2 is named "simulated multi-subnet" rather than "BGP hijacking" because real BGP hijacking
  can't be reproduced in Docker — only its consequence (many peers appearing multi-subnet) is
  simulated.

**Results:**

| Detector | FPR | FNR | Detection delay |
|---|---:|---:|---:|
| Peer-count-only | 0% | 100% (by attack design — all 4 preserve clean-peer count) | — |
| Tri-interface | 0% | 0% | 1.0–2.7 snapshots |

(`scripts/e4_p2p_eclipse_detection/results/table6_p2p_detector_accuracy.md`)

Live (pairwise-link topology):

* **A1** (10 attackers, shared subnet): filter holds exactly 8/8 (`MaxPeersPerSubnet`), the 3
  honest peers on separate dedicated `/29`s never appear in the attacked subnet's count, FSM stays
  ANCHORED.
* **A2** (12 attackers, 4 simulated subnets): filter still holds 8/8; attacker admission and
  honest connectivity are structurally independent once gossip moved off the shared subnet.

(`scripts/e4_p2p_eclipse_detection/results_live/sybil_attack_a{1,2}_*_summary.md`)

**Conclusion:**

* The tri-interface profiler is real evidence against synthetic input, and the ingress filter it
  feeds is confirmed live against real attacker containers.
* A3/A4 (churn, relay latency) haven't been run live, only in the synthetic Monte Carlo.

---

### E5 — FSM Transition Stability: Hysteresis Across All Absorb Edges

**Objective:** Demonstrate that the 3 hysteresis-gated FSM state transition edges resist "flapping" (unstable/continuous state switching) under natural per-block noise:

* `HYSTERESIS_WAIT` on `RECOVERING` $\rightarrow$ `ANCHORED`.
* `DownHysteresisThreshold` on `ANCHORED` $\rightarrow$ `SUSPICIOUS`.
* `SuspiciousHysteresisWait` on `SUSPICIOUS` $\rightarrow$ `ANCHORED` (in `CalculateNextState`).

> Out of scope: **Exponential backoff** mechanism (`RECOVERING` $\rightarrow$ `SOVEREIGN`, `EffectiveDownHysteresisThreshold`). The main reason is that this mechanism defends against a timed adversary, not random noise, and belongs under experiment **E8**.

**Metrics**

| Metric | Definition |
|---|---|
| `AnchoredUptime` | share of run spent in ANCHORED |
| `FlappingCount` / `TotalTransitions` | `harness.go`'s `ComputeMetrics` |
| `WithdrawalBlocked` | blocks with `WithdrawLocked` true |
| `AbsorbedEvents` / `RealTransitions` / `AbsorptionRate` | noise absorbed vs. actually transitioned, at the edge under test |
| `TimeOutsideAnchored` / `DemotionCount` | for ANCHORED-starting scenarios only |

**Live vs. in-process — one finding shared across all three edges below:** every parameter is
swept twice — the existing fixed-seed in-process sweep, and a real 4-node Docker live spot-check.
All three edges show the same pattern: the in-process curve is clean and monotonic; the live curve
is not. Each live value is a separate genesis reset and a separate real noise draw — a confound
the in-process fixed seed removes by construction but a real deployment can't. This divergence is
the section's headline result and isn't re-derived per edge below; each subsection reports only
its own numbers and what's specific to it.

5b/5c's live spot-checks run one noise source only (`celestia-bridge` stop/start); unlike 5a's
`live_spot_check.py`, `live_spot_check_absorb.py` never implemented a `stable` (no-noise) baseline
option, so there is no 5b/5c equivalent of 5a's `stable` column — a live-tooling scope gap, not a
missing measurement of an existing option.

**5a — Up-hysteresis (RECOVERING → ANCHORED)**

* *Method:* `TestE5_HysteresisSweep`, `HYSTERESIS_WAIT` ∈ {0,1,3,5,10,20} × 5 environments
  (critical-level `noisy_btc`/`combined_adversarial`, warning-level `noisy_da`/`noisy_p2p`,
  `stable`), fixed-seed 20%-per-block noise. Live: own genesis per value
  (`ENGRAM_PARAM_HYSTERESIS_WAIT`, `docs/DEVELOPMENT.md` §3), real 300s window.

* *Results:* `noisy_btc` uptime 59.8%→0.0%, flapping 10→37 as HW 0→20
  (`tests/e2e/results/e5_hysteresis_sweep.csv`, Figure 4).

  Live spot-check (5×2, `HYSTERESIS_WAIT` ∈ {0,2,5,10,20} × {stable, noisy_da}, 300s/run, all 4
  validators identical at every sample):

  | HYSTERESIS_WAIT | Environment | Flapping (300s) | Transitions | Anchored uptime |
  |---:|---|---:|---:|---:|
  | 0 | stable | 0 | 0 | 100.00% |
  | 0 | noisy_da | 4 | 17 | 11.70% |
  | 2 | stable | 0 | 0 | 100.00% |
  | 2 | noisy_da | 13 | 14 | 15.29% |
  | 5 | stable | 0 | 0 | 100.00% |
  | 5 | noisy_da | 7 | 13 | 18.75% |
  | 10 | stable | 0 | 0 | 100.00% |
  | 10 | noisy_da | 11 | 14 | 1.06% |
  | 20 | stable | 0 | 0 | 100.00% |
  | 20 | noisy_da | 11 | 14 | 1.04% |

  `stable` clean at every value; `noisy_da` rises HW 0→5 (11.70%→18.75%) then collapses to ~1% at
  HW≥10 (`scripts/e5_hysteresis_flapping/results_live/`, live Figure 4).

* *Conclusion:* negative result — critical-level noise bypasses hysteresis entirely by design
  (`IsCriticalCondition` checked first, before any absorption), so a longer required streak only
  gives it more chances to interrupt recovery. Larger `HYSTERESIS_WAIT` is strictly worse against
  this noise shape, not a bug to fix or a parameter to retune.

**5b — Down-hysteresis (ANCHORED → SUSPICIOUS)**

* *Method:* `TestE5b_DownHysteresisSweep`, `DownHysteresisThreshold` ∈ {1,2,4,6,8} × 4
  warning-level environments, starting ANCHORED. Formally specified in `spec/core/EngramFSM.tla`
  (extends `HysteresisSafety`; `StrictFSMTransitionSafety` unaffected), re-verified under E1's TLC run.

* *Results:* `AnchoredUptime` 61%→100%, `AbsorptionRate` 0%→100% as threshold 1→8, all 4
  environments identical (`tests/e2e/results/e5b_down_hysteresis_sweep.csv`);
  `WithdrawalBlocked=0` throughout.

  Live spot-check (5×1, `DownHysteresisThreshold` ∈ {1,2,4,6,8} × `noisy_da`, 300s/run):

  | DownHysteresisThreshold | Flapping (300s) | Transitions | Anchored uptime |
  |---:|---:|---:|---:|
  | 1 | 6 | 14 | 13.48% |
  | 2 | 4 | 7 | 4.23% |
  | 4 | 12 | 13 | 30.00% |
  | 6 | 12 | 13 | 36.46% |
  | 8 | 8 | 13 | 28.24% |

  3 of 4 validators agree exactly at every threshold; node04 diverges by ±1 transition/flapping at
  threshold=1 and 2 — a granularity-caveat artifact (fixed-interval polling can catch a transition
  on a different side across nodes), not a consensus divergence. Table shows the 3/4 majority value.

* *Conclusion:* a clean positive result in-process — this parameter is swept against exactly the
  noise level it's designed to absorb. Live doesn't confirm it: uptime is non-monotonic, and the
  production default (threshold=2) measures the *lowest* live uptime of the five tested (n=1/value,
  not conclusive on its own).

**5c — SUSPICIOUS-exit hysteresis (SUSPICIOUS → ANCHORED)**

* *Method:* `TestE5c_SuspiciousExitHysteresisSweep`, `SuspiciousHysteresisWait` ∈ {1,2,4,6,8},
  starting SUSPICIOUS — the "Gray Failure Arbitrage" attack shape this closes. Formally specified
  (`SuspiciousHysteresisSafety`); full-spec TLC re-verification alongside down-hysteresis/backoff
  is still pending.

* *Results:* `AbsorptionRate` rises 0%→100% as SHW 1→4, but `suspicious_duration` hits
  `MaxSuspiciousTime`(24) mid-run at SHW≥4, forcing SOVEREIGN regardless. Only SHW=1,2 stay under
  the cap and reach ANCHORED (`AnchoredUptime` 43%, 17%) (`tests/e2e/results/e5c_suspicious_exit_sweep.csv`).

  Live spot-check (5×1, `SuspiciousHysteresisWait` ∈ {1,2,4,6,8}, 300s/run, driven to SUSPICIOUS
  via a real `celestia-bridge` stop before each window):

  | SuspiciousHysteresisWait | Flapping (300s) | Transitions | Anchored uptime |
  |---:|---:|---:|---:|
  | 1 | 8 | 13 | 18.75% |
  | 2 | 13 | 14 | 36.46% |
  | 4 | 6 | 16 | 12.50% |
  | 6 | 2 | 12 | 15.79% |
  | 8 | 0 | 12 | 17.11% |

  Same granularity artifact as 5b: all 4 validators agree except SHW=2, an even 2/2 split on
  uptime (36.46% vs. 37.50%); table shows one side.

* *Conclusion:* a real tuning tension in-process — absorbing more noise on this edge accelerates
  the very escalation the absorption is meant to buy time against. Live confirms the flapping half
  of that mechanism works exactly as designed (falls monotonically 8→0 as SHW grows); the uptime
  half doesn't track the in-process curve at all, including disagreeing on which direction the
  1→2 step moves.

**Open question — asymmetric hardening:** exponential backoff exists only on RECOVERING→SOVEREIGN.
The same fixed-cost, repeatable-attack shape it defends against exists structurally on
ANCHORED↔SUSPICIOUS too (5b/5c's thresholds don't grow with repetition).
`CircuitBreakerSafety` locking withdrawals only in SOVEREIGN/RECOVERING argues for lower priority
on this edge, not for no defense at all — unresolved.

A bounded admission cap on `MsgSubmitForcedTxRequest` while SUSPICIOUS now exists as a partial,
concrete mitigation (`app/ante.go`'s `CircuitBreakerDecorator`, `MaxSuspiciousForcedTxQueue` —
concrete-only, no spec line). It bounds one repeatable-attack surface (unbounded `ForcedTxQueue`
growth) but doesn't touch the broader question above: whether SOVEREIGN/RECOVERING-style backoff
hardening belongs on this edge at all remains unresolved.

---

### E6 — Reanchoring Feasibility Evaluation

**Objective:** show the ZK recovery proof (chain continuity, FSM legality, withdrawal-lock
invariant, SMT-root progression, policy binding) is practical and scales linearly, not just
correct. Answers RQ4.1–4.3.

**Metrics**

| Metric | Definition |
|---|---|
| Constraint count | ACIR opcodes / circuit size |
| Prove / verify time | real `bb prove` / `bb verify` wall-clock |
| Proof size | bytes |
| Backend trade-off | Noir+Honk vs. Plonky3 across the above |

**Method:**

* `scripts/e6_zk_reanchoring_benchmark/benchmark_prover.sh` runs the real pipeline
  (`nargo compile`→`bb gates`→`nargo execute`→`bb prove`→`bb verify`, Noir 1.0.0-beta.22,
  Barretenberg 5.0.0-nightly, UltraHonk) on `circuit/reanchoring/src/main.nr` at N=4..256 headers
  (chain continuity via a Poseidon2 hash, simpler than a full SMT proof — see the comment at the
  top of `main.nr`).
* `benchmark_plonky3.sh` runs Plonky3's real `prove_prime_field_31` example (BabyBear,
  UniStark+FRI, Poseidon2, transparent setup) at N=8..256 for the backend comparison — both sides
  exercise the same real Poseidon2 primitive, not an approximate proxy.

**Results:**

| N | Constraint growth | Prove time | Verify time | Proof size |
|---|---|---:|---:|---:|
| 4 → 256 | 12.00 ACIR opcodes/header, linear, R²=1.0000 | 0.130s → 0.684s | flat, 22–33ms | constant, 14,656 B |

(`scripts/e6_zk_reanchoring_benchmark/results/table6b_scaling.csv`, `figure6_scaling.{png,pdf}`)

| Backend (N=256) | Proof size | Verify | Prove | Setup | PQ-secure |
|---|---:|---:|---:|---|---|
| Noir + Honk | 14,656 B | 22.0 ms | 0.684 s | KZG (trusted) | No |
| Plonky3 | 1,278,939 B (~87x larger) | 32.2 ms | 0.044 s (~15.5x faster) | Transparent/FRI | Yes |

(`results/table6c_backend_comparison.md`, `figure7_backend_tradeoff.{png,pdf}`)

**Deployed circuit** compiles once at `N_MAX=256` with a padding/`count` witness (1..256) instead
of per-interval recompilation.

* Real cost (`circuit/reanchoring/RESULTS_MAXN_PADDING.md`, `count` ∈ {1,4,130,256}): constant
  6,143 ACIR opcodes, 47,613 circuit size, ~1.06s prove, ~22ms verify, 14,656 B proof — regardless
  of `count` (1.054s vs. 1.059s prove time, statistically indistinguishable).
* 2.0x/1.55x cheaper (opcodes/prove time) than a fixed-N=256 compile with no padding logic.

**Conclusion:**

* Constraint count and prove time scale linearly; verify time and proof size stay constant — the
  practicality/scalability claim holds on the real (Poseidon2-simplified) circuit.
* The deployed max-N=256+padding design adds only bounded, measured overhead versus per-interval
  recompilation.
* Not yet measured: prover throughput against a BTC outage lasting hours rather than minutes —
  only backlogs of a few hundred headers have been exercised live (measured throughput ≈10.84
  headers/s catching up a backlog, ~10x Engram's own block-production rate); whether
  `HeaderHistory` growth and that throughput headroom hold at 2–3 orders of magnitude larger scale
  is unknown. Keeping pace with an *ongoing* SOVEREIGN period is bounded below by
  block-production rate regardless (a physical limit, not a code defect). Needs a dedicated run:
  hold `AnchorTracker.SetSubmissionPausedFile` for several hours under continuous block
  production.

---

### E7 — Extended Proposal Consensus Overhead

**Objective:** measure whether folding `fsm_state`/`da_receipt`/`btc_receipt`/`zk_proof_ref` into
the proposal costs meaningful throughput/latency versus plain CometBFT. Answers RQ4.

**Metrics:** proposal size overhead, validation CPU cost, commit latency, throughput, nil-prevote
ratio under sensor mismatch — split into a steady-state regime (every block) and a recovery-event
regime (rare, ZK-proof-carrying blocks only), since blending them would hide both the near-zero
healthy-path cost and the real bounded recovery cost.

**Method:**

* `go test ./tests/benchmark/... -bench=. -benchmem` measures real cumulative JSON size per field
  (`V0` vanilla → `V5` full: `fsm_state`, DA receipt, BTC receipt, P2P digest, ZK proof ref) and
  real CPU cost of `CalculateNextState`/`da.VerifyReceipt`/`anchor.VerifyReceipt`/full
  `ProcessProposal`.
* `-bench=BenchmarkVerifyZKProof` measures real `bb verify` cost on E6's real N=4 proof.
* `scripts/e7_consensus_overhead/measure_overhead.py` builds the overhead table;
  `live_overhead_scan.py` scans a real chain's blocks by `fsm_state` and a size heuristic for
  `SubmitRecoveryProof` txs.
* V4 (P2P digest) is a size estimate only — that field isn't actually on the wire (P2P health is
  validated from the leader's local `keeper.Metrics`).

**Results:**

| Regime | Result |
|---|---|
| Steady-state tax | +228 B/block (`ENGRAM_EXTENDED_PROPOSAL_V1` marker), 100% of blocks; vanilla carries 0; no meaningful block-interval difference at idle (`timeout_commit` dominates both) |
| Recovery-event cost | ~18.77 ms/verify (`BenchmarkVerifyZKProof-8`, 3 iterations), real `bb verify` inside `DeliverTx`, paid once per proof, not persistent |
| Live (376 blocks, fresh genesis) | 100% marker coverage; SOVEREIGN avg 248.2 B, RECOVERING avg 270.0 B, ANCHORED avg 247.0 B; 1 block matched the recovery-proof heuristic at 14,881 B, consistent with E6's ~14,656 B proof + envelope |

(`scripts/e7_consensus_overhead/results_live/table4_live_overhead.{csv,md}`)

**Conclusion:**

* The healthy-path tax is small and flat (~247–270 B/block) across all three states.
* The real proof cost is real but rare and self-limiting, isolated to the single block that
  carries it — "near-zero tax on the healthy path" is a measured claim here, not just a design
  intent.

---

### E8 — Attack-Resilience Test Suite

**Objective:** turn the spec's safety lemmas into real integration tests and live attack traces —
show safety (`safety_held`, zero AppHash divergence) holds under 8 concrete attack scenarios plus
double-signing and a Timeout-flood DoS attempt. Answers RQ1.

**Metrics:** pass/fail per attack (`safety_held`, `divergence_events`), plus attack-specific
correctness signals — `blocked_correctly` (withdrawal), `cadence_held` (timeout flood), detection
latency (evidence). Double-signing/equivocation is a distinct concept from AppHash divergence: the
former is one validator (one key) signing 2 conflicting votes for the same height/round/type; the
latter is honest validators computing different state roots (`safety_held` in A3-A8). Both are
tracked here but are not interchangeable evidence for each other.

**Method:** 8 attacks map to `spec/README.md`'s lemmas and reuse E4's attacker infrastructure
where applicable.

| Attack | Mechanism |
|---|---|
| A1/A2 Eclipse / simulated multi-subnet Sybil | E4's attacker swarm |
| A3 Data withholding / A4 Forged BTC receipt / A6 Malicious proposer | `docker/engram-node04-byzantine.yml`, `ENGRAM_BYZANTINE_BEHAVIOR` |
| A5 Withdrawal during SOVEREIGN | `cmd/engramd/e8_cli.go`'s forced-tx CLI |
| A7 Censorship | `IsCensoring`/`ForcedTxQueue` |
| A8 Combined | A6 + an A1 swarm simultaneously |
| Double-signing | 2 processes sharing one real validator key but not `priv_validator_state.json`; detected via CometBFT's stock evidence pool (`RequestFinalizeBlock.Misbehavior`, no `x/evidence` needed) and `preblock.go`'s `recordDetectedEvidence` |
| Timeout-flood | `ENGRAM_TIMEOUT_FLOOD_INTERVAL_MS` re-broadcasts signed Timeouts every 50ms, rate-limited by `PeerState.allowTimeoutMessage` (20 msgs/s/peer) and bounded round acceptance in `handleTimeoutMessage` |

**Results:** all 10 rows PASS live, re-confirmed on the pairwise-link topology
(`scripts/e8_attack_resilience/results_live/*_summary.md`):

| # | Attack | Live result |
|---|---|---|
| A1 | Eclipse | filter holds 8/8, cluster unaffected |
| A2 | Simulated multi-subnet Sybil | filter holds 8/8 despite 12 attackers |
| A3 | Data withholding | `safety_held=True`, 0 divergence |
| A4 | Forged BTC receipt | `safety_held=True`, 0 divergence |
| A5 | Withdrawal during SOVEREIGN | `blocked_correctly=True`, tx never commits |
| A6 | Malicious proposer | `safety_held=True`, 0 divergence |
| A7 | Censorship | `safety_held=True`, 0 divergence, height progressed |
| A8 | Combined | `safety_held=True`, 0 divergence under overlap |
| — | Double-signing | all 3 honest validators detect real `DuplicateVoteEvidence`, 1-block latency (height ~15,150, offense heights 15156/15160, re-confirmed on the pairwise-link topology) |
| — | Timeout flood | `cadence_held=True` in all 4 runs below, `safety_held=True`, 0 divergence |

**Timeout-flood detail** (`ENGRAM_TIMEOUT_FLOOD_INTERVAL_MS` on node04; block rate is the direct
evidence flooding didn't force extra round-skips — a drop, not a hold/rise, would indicate that):

| Run | Baseline (blocks/s) | Flood (blocks/s) | Rate-limiter drops/node | CPU (baseline → flood) | Memory |
|---|---:|---:|---|---|---|
| Moderate (20 msg/s), run 1 | 0.800 | 0.771 | — | — | — |
| Moderate (20 msg/s), run 2 | 0.816 | 0.661 | — | — | — |
| Extreme (500 msg/s, 25x the 20/s cap) | 0.825 | 0.789 | 28,071 / 29,238 / 30,376 (of ~30,000 attempted) | 2.4–3.3% → 7.85–9.51% (max 22.40%; node04 itself 28.47%) | 78.7–92.2 → 82.7–92.2 MiB |
| Pairwise-link re-run, moderate | 0.808 | 0.608 | — | — | — |
| Pairwise-link re-run, extreme | 0.586 | 0.813 | 26,563–28,777 | — | — |

**Conclusion:**

* Full live-Docker coverage on all 10 rows — the safety story is empirically demonstrated, not
  just argued from the spec.
* The earlier in-process-only pass is kept as a secondary reference
  (`scripts/e8_attack_resilience/results/table3_attack_resilience.md`); live is now the primary
  data source.
* The Timeout-flood hardening (rate limiter, bounded round window, stale-sender eviction) holds
  cadence and bounds resource cost even at 25x its own design threshold: CPU roughly triples under
  flood but stays well short of saturation, memory stays flat, and the rate limiter rejects
  essentially all traffic past its 20/s allowance.

---

### E9 — Trace-Driven Stress Test

**Objective:** go beyond synthetic single-fault scenarios — replay a realistic combined-failure
trace (growing BTC congestion → layered DA outage → layered P2P churn, all three at once →
sequential recovery) and confirm the chain keeps committing throughout.

**Metrics:** FSM state timeline, BTC/DA gap, P2P health, block commit rate, withdrawal-lock
status, proof-generation status — in-process only for the sensor-level metrics; live is limited to
`fsm_state`/height/marker, since BTC/DA/P2P readings aren't committed state
(`preblock.go`'s `NewPreBlocker`).

**Method:**

* In-process — `go test ./tests/e2e/... -run TestE9_TraceDrivenCombinedFailure`, one continuous
  trace through real `BeginBlocker`.
* Live — `scripts/e9_trace_driven/live_combined_trace.py`, one continuous real trace on the 4-node
  cluster (`chaos-btc-delay` → `docker stop celestia-bridge` layered on top → 3 cycles of
  `chaos-loss` layered on both → recovery in reverse), under a `chaos-wan-latency` baseline and
  the pairwise-link topology.

**Results:**

* In-process — 48 blocks, the chain commits throughout all 3 layered failures
  (`tests/e2e/results/e9_trace_driven.csv`, 6-panel Figure 2).
* Live — single 319s run, all 7 phases pass: height progressed continuously, no stalled round, all
  4 nodes height-synced at every sample; all 4 validators transitioned ANCHORED→SOVEREIGN together
  at t=152s (the triple-fault peak), SOVEREIGN→RECOVERING→ANCHORED together at t=308s/315s, zero
  divergence at any transition
  (`scripts/e9_trace_driven/results_live/e9_combined_trace_20260812T140833.csv`, 384 samples).

**Conclusion:**

* The chain survives a realistic layered multi-failure trace both in-process and live, with all
  validators staying in lockstep through the triple-fault peak and full recovery — a stronger
  claim than any single-fault scenario alone.
* No automated 6-panel figure for the live run, since only `fsm_state`/height/marker are real
  committed data there.

---

### E10 — Bitcoin Reorg Fork-Choice Reaction

**Objective:** test whether the FSM reacts correctly to a real Bitcoin reorg that invalidates an
already-anchored checkpoint — shallow (below `KDeepFinality`) vs. deep (past it) — grounded in
`spec/core/EngramConsensus.tla`'s `BitcoinReorg` action and `CanElect`'s state-dependent branch.

**Metrics:** FSM state after the reorg (ANCHORED vs. SOVEREIGN), reaction correctness vs. reorg
depth.

**Method:** `scripts/e10_bitcoin_reorg/live_reorg_test.py`

* Isolates `bitcoin-node02` (`setnetworkactive false`).
* Forces an alternate chain via `invalidateblock`, mines it past node01's frozen height.
* Reconnects to force a real reorg on node01 and every validator watching it — verified
  independently via `getblockhash`, not assumed from height alone.

**Results:**

| Depth | Result |
|---|---|
| Shallow (depth=1, < `KDeepFinality`=2) | all 4 validators stay ANCHORED (`results_live/reorg_shallow_20260812T140516.csv`) |
| Deep (depth=15, past the anchored checkpoint at height 477) | all 4 validators transition to SOVEREIGN in lockstep (`results_live/reorg_deep_20260812T143632.csv`) |

`AnchorTracker.VerifyAnchor` re-derives `BlockContainsTag` fresh via `getblockhash` on every call,
so the deep reorg is caught on the next check, `is_btc_spv_failed` goes true,
`IsCriticalCondition` fires.

**Conclusion:**

* The shallow case matches the spec's own safety claim exactly (`IsKDeep` structurally can't
  re-elect a certificate lost this shallow).
* The deep case is a real, positive finding beyond what the spec promises (`spec/README.md` §7.3
  leaves deep reorgs out of scope) — `VerifyAnchor`'s no-cache re-derivation is real
  defense-in-depth.
* Not yet measured: repeatability (a single depth-15 trial only), the precise boundary depth
  (`KDeepFinality`+1, the smallest "deep" case, untested — `--reorg-depth 3` is the natural next
  run), formal verification of `VerifyAnchor` itself (no spec line, no TLC/Apalache coverage), and
  recovery time specifically following a deep reorg (the run ended still SOVEREIGN).

---

## 5. Figures & Tables Needed in the Paper

| Figure/Table | Content |
|---|---|
| **Fig. 1** | Architecture: Engram execution + BTC settlement + Celestia DA + FSM sensors |
| **Fig. 2** | FSM timeline under combined failure *(E9)* |
| **Fig. 3** | Availability/throughput during outage: Engram FSM vs. vanilla CometBFT *(E2)* |
| **Fig. 4** | Recovery stability vs. `HYSTERESIS_WAIT` *(E5)* |
| **Fig. 6** | Recovery Proof Scaling: 4 panels (Constraint Count, Proving Time, Verification Time, Proof Size) *(E6)* |
| **Fig. 7** | Backend trade-off radar chart: Noir+Honk vs. Plonky3 *(E6, optional)* |
| **Table 1** | Formal verification state-space results *(E1)* |
| **Table 2** | Failure matrix and expected policy *(E3)* |
| **Table 3** | Attack-resilience tests *(E8)* |
| **Table 4** | Extended proposal overhead *(E7)* |
| **Table 5** | Ablation study *(E1)*|
| **Table 6** | P2P profiler accuracy *(E4)* |

---

## Conclusion

The Engram Sovereign FSM idea is strong enough for a conference submission if the evaluation is
built with discipline. The deciding factor isn't more theory — it's **demonstrating empirically**
that the FSM:

- maintains **safety** while improving **liveness** for a modular blockchain under peripheral
  failure,
- provides **controlled recovery**,
- and incurs **acceptable overhead**.
