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
| **E5** FSM transition stability across all absorb edges | Show every hysteresis-gated edge (RECOVERING→ANCHORED, ANCHORED→SUSPICIOUS, SUSPICIOUS→ANCHORED) resists flapping under natural noise | Hard reset on the first bad/good reading (no absorption) | Anchored uptime, flapping/transition counts, withdrawal-locked time, absorption rate, time-outside-ANCHORED |
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

- **BTC congestion** (E2 S2): `AnchorTracker.SetSubmissionPausedFile` pauses new checkpoint
  submission (a pending one still confirms), growing `btc_gap` directly — `btc_gap` is a
  block-height delta, not an RPC round-trip, so network delay can't reproduce it.
- **DA outage** (E2 S3, E3, E9): real `docker stop`/`start celestia-bridge`.
- **Combined BTC+DA** (E2 S6): both together.
- **Attacker/Byzantine infra** (E4, E8): non-validator `engramd` containers
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

### E1 — Formal Verification Stress & Ablation

Report this as a verification study with configurations, ablations, and counterexample traces —
not just "no error".

**Configurations:**

| Config | N | f | MaxRound | BTC height | Engram height | Goal |
|---|---|---|---|---|---|---|
| C1 | 4 | 1 | 2–3 | 2–3 | 2–3 | Reproduce the current result |
| C2 | 4 | 1 | 4–5 | 3–4 | 3–4 | Test deeper consensus rounds |
| C3 | 7 | 2 | 2–3 | 2–3 | 2–3 | Test larger quorum overlap |

**Ablations:**

| Ablation | Expected outcome |
|---|---|
| Remove hysteresis | Flapping or premature recovery may appear |
| Remove P2P health gate | An eclipsed node may trigger recovery incorrectly |
| Remove circuit breaker | Withdrawal leakage may appear during SOVEREIGN/RECOVERING |
| Remove f+1 timeout fast-forward | Liveness delay or round-stall increases |
| Remove DA receipt consistency | A data-withholding proposal may get committed |

At least 2-3 counterexample traces are needed from the significant ablations. If an ablation
produces no violation, report the state-space difference and explain why.

---

### E2 — Fault-Injection End-to-End Prototype

The most important experiment for a rank-B+ venue: demonstrate "graceful degradation rather than
halting" with a real prototype.

**Proposed setup:**

| Component | Suggestion |
|---|---|
| Validators | 4, 7, 10, 16 nodes |
| Consensus | CometBFT/Cosmos SDK prototype |
| Workload | 100–1000 tx/s synthetic; mix of normal and withdrawal tx |
| BTC sensor | Mock SPV/Babylon checkpoint service |
| DA sensor | Mock Celestia/Blobstream receipt service |
| P2P sensor | Controlled peer manager or network emulator |
| Fault injector | Docker Compose + tc/netem, iptables, service pause, artificial receipt delay |

**Scenarios:**

| Scenario | Description | Expected behavior |
|---|---|---|
| S1 Normal | BTC/DA/P2P healthy | Behaves like baseline |
| S2 BTC congestion | Growing checkpoint-confirmation delay | ANCHORED → SUSPICIOUS → SOVEREIGN, chain keeps committing |
| S3 DA unavailable | DA receipt missing/false | Rejects invalid DA blocks; falls back if sustained |
| S4 P2P eclipse (partial) | Reduced subnet diversity, high peer churn | Warns, does not recover early |
| S5 Anchor isolation | ActiveAnchors = 0 | Goes straight to SOVEREIGN |
| S6 Combined BTC+DA failure | Settlement and DA fail together | Chain still processes local tx, withdrawals locked |
| S7 Recovery | Failures clear, proof available | SOVEREIGN → RECOVERING → ANCHORED after hysteresis |

**Key metrics:** time-to-detection, time-to-fallback, availability during outage, throughput
degradation, consensus latency p50/p95/p99, recovery time, withdrawals blocked, incorrect state
transitions/flapping.

**Baselines:** vanilla CometBFT with strict external validity; static circuit breaker; FSM without
hysteresis; FSM with a peer-count-only P2P sensor.

**Measured (in-process):** `go test ./tests/e2e/...` runs all 7 scenarios (S1-S7) through
`x/sovereignty`'s real `BeginBlocker` (real FSM logic, only sensor inputs are mocked). Results in
`tests/e2e/results/s*.csv` + `e2_summary.md`. Figure 3 is built from the same data by
`scripts/e2_fault_injection/simulate_network_jitter.py` (state timeline across the 7 scenarios
plus withdrawal-lock shading).

**S2's mechanism.** `AnchorTracker.SetSubmissionPausedFile` (`x/anchor/anchor.go`) makes
`MaybeSubmit` skip broadcasting a *new* checkpoint while a marker file exists (checked fresh every
call, no restart needed) — an already-pending submission still confirms normally, only new ones
are withheld, freezing `h_btc_anchored` while `h_btc_current` keeps climbing (mempool congestion's
real-world effect: a specific tx's confirmation falling behind the wider chain's pace). Wired via
`ANCHOR_SUBMISSION_PAUSED_FILE` (`docker/engram-validator-cluster.yml`,
`/tmp/anchor_submission_paused`), touched on all 4 validators to grow `btc_gap` past
`SovereignThreshold`.

**Measured (live, 4-node Docker cluster, pairwise-link topology):**
`scripts/e2_fault_injection/live_scenario_matrix.py` chains all 7 scenarios into one continuous
run (Pumba netem for S2/S4/S5, real `docker stop`/`start` for S3/S6/S7). All 7 phases passed
cleanly in 818s, 32 real transitions, every one in lockstep across all 4 validators, zero
divergence. S2 (BTC congestion) showed the full `ANCHORED → SUSPICIOUS → SOVEREIGN` sequence as
three distinct observed states (t=88s, t=149s), recovering `SOVEREIGN → RECOVERING → ANCHORED`
within 6s of the cooldown phase starting; S3 (DA unavailable) independently reproduced the same
three-state shape (`ANCHORED → SUSPICIOUS` t=264s, `→ SOVEREIGN` t=294s, recovered within 2s); S6
(combined BTC+DA failure) drove a direct `ANCHORED → SOVEREIGN`; S7 completed real recovery
`SOVEREIGN → RECOVERING → ANCHORED` via the ZK re-anchoring pipeline. Data:
`scripts/e2_fault_injection/results_live/s{1..7}_*.csv`. The live Figure 3
(`scripts/e2_fault_injection/live_figure_builder.py`) produces
`figure3_state_timelines_live.{png,pdf}` (7 panels, each picking the node with the most valid
samples rather than a fixed node01, since S4/S5 deliberately isolate node01 and lose its RPC
connectivity for most of the run) and `figure3_summary_bars_live.{png,pdf}` (3 real metrics:
blocks committed, state transitions, time-outside-ANCHORED ratio).

**Vanilla CometBFT baseline (measured):** `engramd start --vanilla` (`app/app.go`) runs the same
binary/module but skips `SetPrepareProposal`/`SetProcessProposal`/`SetPreBlocker`, so BaseApp uses
its default handlers with no `ExtendedProposal`. Running both variants side by side
(`scripts/e7_consensus_overhead/vanilla_comparison.sh`) confirms: the normal node always carries
the `ENGRAM_EXTENDED_PROPOSAL_V1|...` 228-byte marker in `Txs[0]` on every block; the vanilla node
carries 0 tx.

---

### E3 — Failure Matrix

Shows precisely when the system may keep committing local blocks and when it must lock risky
actions like withdrawals.

| BTC | DA | P2P | Expected state | Withdrawals | Block production |
|---|---|---|---|---|---|
| healthy | healthy | healthy | ANCHORED | enabled | full |
| warning | healthy | healthy | SUSPICIOUS | restricted | moderate/full |
| critical | healthy | healthy | SOVEREIGN | locked | full local |
| healthy | failed | healthy | SUSPICIOUS/SOVEREIGN | locked if SOVEREIGN | local |
| healthy | healthy | eclipsed | SUSPICIOUS/SOVEREIGN | locked if critical | depends |
| critical | failed | eclipsed | SOVEREIGN | locked | local |
| recovered | recovered | healthy | RECOVERING → ANCHORED | locked until anchored | full |

**Measured:** `scripts/e3_failure_matrix/measure_latency.py` rebuilds this table from real data
(not hand-written) by reading the steady-state at the end of each S1-S7 scenario in
`tests/e2e/results/s*.csv`; result in
`scripts/e3_failure_matrix/results/table2_failure_matrix.md`. "Block production" is always
"continuous" because `Harness.Advance()` never stopped or errored in any scenario — that is
itself a measured result, not an assumption.

**Measured (live Docker, full lifecycle):** `scripts/e3_failure_matrix/live_lifecycle_test.py`
drives real `celestia-bridge` stop/start against the 4-node cluster through every edge of
ANCHORED ↔ SUSPICIOUS → SOVEREIGN ↔ RECOVERING → ANCHORED in one run. All 7 phases **passed**
in 344s, all 4 validators transitioning in lockstep at every edge: ANCHORED→SUSPICIOUS (12s),
SUSPICIOUS→ANCHORED on quick recovery with no escalation (24s), ANCHORED→SUSPICIOUS→SOVEREIGN via
the sustained-SUSPICIOUS gray-failure timeout (27s→58s), SOVEREIGN→RECOVERING (74s),
RECOVERING→SOVEREIGN regression on a brief re-outage (80s), and real recovery to ANCHORED (344s,
via `watch_and_prove.sh`/`reanchoring-prover`, with a couple of RECOVERING↔SOVEREIGN dips along
the way reflecting the prover's real proof cadence). Data:
`scripts/e3_failure_matrix/results_live/lifecycle_test_20260812T153523.csv` + `_summary.md`.

---

### E4 — P2P Eclipse/Sybil Detection

The P2P health profiler is the novelty: its output isn't just for monitoring, it becomes a
consensus input via proposal validation. This experiment compares two detectors directly to
quantify that advantage.

**Detector comparison:**

| Detector | Description | Weakness |
|---|---|---|
| **Peer-count-only** *(baseline)* | CometBFT's default peer count | Easily beaten by Sybil/slot-filling; very high FNR on most attacks |
| **Tri-interface profiler** *(proposed)* | Measures all 6 metrics: structural + behavioral + latency | — |

**Attack scenarios** *(feed Table 6):*

| # | Scenario | Description |
|---|---|---|
| A1 | Peer slot exhaustion | Fill connection slots with fake peers |
| A2 | Sybil via simulated multi-subnet (not literal BGP hijacking — see note below) | Peers spoofing shared-subnet routing |
| A3 | Churn-based rotation | Continuously rotate peers to avoid detection |
| A4 | Relay node attack | Insert a middle node to inflate latency |

**Methodology:** Chaos Engineering via **Pumba** + Docker Compose, injecting network delay/loss to
simulate realistic latency and connectivity changes.

**Metrics:** False Positive Rate (%), False Negative Rate (%), Detection Delay (ms/s).

**Table 6 — Detection accuracy, tri-interface profiler vs. peer-count baseline (target numbers):**

| Attack Scenario | Detector | FPR | FNR | Detection Delay |
|-----------------|----------|----:|----:|----------------:|
| Peer Slot Exhaustion | Peer-count | 1.5% | 98.2% | N/A |
| | **Tri-interface** | **0.8%** | **1.2%** | **450 ms** |
| Sybil / multi-subnet | Peer-count | 2.1% | 95.5% | N/A |
| | **Tri-interface** | **1.1%** | **0.5%** | **850 ms** |
| Churn-based Rotation | Peer-count | 85.0% | 15.0% | N/A |
| | **Tri-interface** | **2.5%** | **1.8%** | **1.2 s** |
| Relay Node Attack | Peer-count | 0.5% | 100.0% | N/A |
| | **Tri-interface** | **0.2%** | **0.0%** | **250 ms** |

**Measured (synthetic, not live-network — see live results below for the network-level test):**
the table above states target numbers, not yet reproduced on a live cluster.
`scripts/e4_p2p_eclipse_detection/simulate_eclipse_attack.py` runs a Monte Carlo simulation via
`go test ./tests/e2e/... -run TestE4_P2PDetectorComparison`: the real detector function
(`types.IsP2PQualityHealthy`) and the peer-count-only baseline are tested against synthetic peer
snapshots built to match each attack's signature (2000 trials/cell, fixed seed), at both
`DefaultParams()` (the small TLC-verification thresholds) and a more realistic
"production_scale" threshold set. Real result: FPR=0% for both detectors, FNR=100% for
peer-count-only across all 4 attacks (the attacks are designed to preserve clean-peer count while
breaking other signals), FNR=0% for the tri-interface profiler, detection delay 1.0-2.7 snapshots
depending on attack/threshold — see
`scripts/e4_p2p_eclipse_detection/results/table6_p2p_detector_accuracy.md`. This is real evidence
about the real detector function, on synthetic input — not equivalent to a live-network
measurement.

**A1/A2 live-Docker infrastructure:** the active defense under test is
`x/sovereignty/keeper/peer_filter.go`'s `FilterPeerByAddr` (registered via
`baseapp.SetAddrPeerFilter`), a real ingress filter that blocks a peer by subnet density
(`Params.MaxPeersPerSubnet`) *before* it's added to the peer set — distinct from `SubnetDiversity`,
which only reports after the fact. Infrastructure: `docker/attacker-peer-swarm.yml` (real,
non-validator `engramd` containers dialing `engram-node01`) +
`scripts/e4_p2p_eclipse_detection/live_sybil_attack.py` (leg `a1`: 10 attackers on the shared
`engram-net` subnet; leg `a2`: 12 attackers split across 4 simulated subnets
`attacker-subnet-a/b/c/d`, measured via CometBFT's real `/net_info` RPC, independent of the known
`Query.State` gap). A2 is named "Sybil via simulated multi-subnet" rather than "BGP hijacking"
because real BGP hijacking (Internet-layer route manipulation) can't and shouldn't be simulated in
a Docker testnet — what `MaxPeersPerSubnet`/`SubnetDiversity` defends against is BGP hijacking's
*consequence* (many peers appearing to come from different subnets but controlled by one
attacker), not its root cause, so simulating the consequence is a faithful enough test.

**Live run results (4-node cluster, pairwise-link mesh topology):**
`scripts/e4_p2p_eclipse_detection/results_live/sybil_attack_a1_20260812T154911_summary.md` and
`sybil_attack_a2_20260812T155359_summary.md`, measured against the current network design (§3's
`docs/ARCHITECTURE.md`: 6 dedicated `/29` pairwise links replacing the shared `engram-net` for
validator-to-validator gossip).

- **A1** (10 attackers on `engram-net`): filter holds at exactly **8/8** (`MaxPeersPerSubnet`).
  The 3 honest peers, on their own dedicated pairwise-link `/29`s (one per peer, real
  `SubnetDiversity=3`), are completely absent from `engram-net`'s peer count throughout. FSM stays
  ANCHORED the whole run; height and peer counts on the 3 pairwise links (1 each) never move.
- **A2** (12 attackers across `attacker-subnet-a/b/c/d`): filter still holds at exactly **8/8** on
  `engram-net` — attacker admission and honest peer connectivity are structurally independent,
  since `engram-net` now carries only non-gossip traffic. Same clean FSM/height behavior as A1.

---

### E5 — FSM Transition Stability: Hysteresis Across All Absorb Edges

Three hysteresis-gated edges guard the FSM against flapping under transient peripheral noise:
`HYSTERESIS_WAIT` on RECOVERING→ANCHORED, `DownHysteresisThreshold` on ANCHORED→SUSPICIOUS, and
`SuspiciousHysteresisWait` on SUSPICIOUS→ANCHORED (`x/sovereignty/keeper/circuit_breaker.go`'s
`CalculateNextState`). Each absorbs a run of non-critical bad or good readings up to its threshold
before actually transitioning, instead of reacting to the first one. A fourth mechanism —
exponential backoff on RECOVERING→SOVEREIGN (`EffectiveDownHysteresisThreshold`) — is out of scope
here: it defends against an adversary who times disruptions precisely, not against random noise, so
a memoryless-noise sweep can't validate its actual claim; it belongs under E8 as an attack scenario
(not yet built).

**Metrics**, applied per sub-scenario below:

- `AnchoredUptime` — share of the run spent in ANCHORED, meaningful regardless of which edge or
  starting state is under test.
- `FlappingCount` + `TotalTransitions` — `tests/e2e/harness.go`'s existing `ComputeMetrics`
  definition (an immediate A→B→A reversal). Each sub-scenario's noise targets only one edge, so
  this blended, whole-run count stays a clean signal without needing a per-edge split.
- `WithdrawalBlocked` — blocks where `WithdrawLocked` was true, from the same shared `E2Metrics`
  struct — the concrete cost the asymmetric-hardening discussion below rests on.
- `AbsorbedEvents` / `RealTransitions` at the edge under test, and the derived `AbsorptionRate` —
  measures how much of the near-threshold noise the mechanism actually filters directly, instead of
  inferring it indirectly from `FlappingCount` alone.
- `TimeOutsideAnchored` + `DemotionCount` — for a scenario starting already in ANCHORED, the
  meaningful question is how much and how often it left, not whether it "reached" ANCHORED.

### 5a — Up-hysteresis (RECOVERING → ANCHORED)

The spec requires RECOVERING → ANCHORED only once `safe_blocks` reaches `HYSTERESIS_WAIT` and the
proof is valid.

Sweeps `HYSTERESIS_WAIT` over {0, 1, 3, 5, 10, 20} across 5 environments: stable, and four with
continuous noise (`noisy_btc`, `noisy_da`, `noisy_p2p`, `combined_adversarial`) — each block
independently has a 20% chance of being noisy. All `HYSTERESIS_WAIT` values in the same
environment share a fixed RNG seed for a fair comparison.

**Measured (5/5 environments):** `go test ./tests/e2e/... -run TestE5_HysteresisSweep` sweeps
`HYSTERESIS_WAIT` through the real Harness/BeginBlocker under the 5 environments above. Results:
`tests/e2e/results/e5_hysteresis_sweep.csv`, Figure 4 (3 panels: stability/flapping/
time-to-first-recovery) at `scripts/e5_hysteresis_flapping/results/figure4_hysteresis.{png,pdf}`.

**Result:** under continuous noise, both measured metrics move opposite to the initial hypothesis
(`HYSTERESIS_WAIT=3-5` as a sweet spot): `anchored_uptime` (share of time in ANCHORED over a
100-block window) decreases monotonically as `HYSTERESIS_WAIT` increases (e.g. `noisy_btc`: 59.8%
at HW=0 → 0.0% at HW=20), and `flapping_count` increases monotonically instead of decreasing (same
environment: 10 at HW=0 → 37 at HW=20) — no sweet spot on either metric.

The cause is architectural, and explains both directions: `x/sovereignty/keeper/circuit_breaker.go`'s
`CalculateNextState`/`NextSafeBlocks` sends RECOVERING's `!healthy` branch straight back to
SOVEREIGN on a single bad block (not just a local counter reset), and `NextSafeBlocks` only
accumulates across consecutive healthy RECOVERING blocks — a hard streak counter with no partial
credit. Under fixed per-block noise, the probability of completing an uninterrupted streak of
`HYSTERESIS_WAIT` blocks falls exponentially as the threshold grows, so a larger value forces more
RECOVERING→SOVEREIGN→RECOVERING retry cycles (more flapping) before any success, and spends more
time outside ANCHORED overall. `HYSTERESIS_WAIT` does not "filter noise" as hypothesized; it sets
an increasingly hard pass/fail test, and every failure itself creates more oscillation. This is a
negative result worth publishing, not a bug to fix or a parameter to retune. 5b (below) applies the
same absorb-then-transition pattern to the RECOVERING→SOVEREIGN regression edge specifically to
answer this finding.

**Security hardening — exponential backoff against a flapping DoS attack.** A distinct concern
from the natural-noise finding above: a network adversary that can time a single disruptive block
precisely (wait until `safe_blocks` is one block from `HYSTERESIS_WAIT`, then inject noise) could
otherwise repeat the same cheap attack indefinitely, holding the chain in a
SOVEREIGN/RECOVERING loop without ever violating a safety property or triggering slashing. 5b's
down-hysteresis mechanism, applied to this same RECOVERING→SOVEREIGN edge, already raises this
attack's cost from "one precisely-timed block" to "`DownHysteresisThreshold` *consecutive*
precisely-timed blocks" — a materially harder bar, since sustaining disruption over multiple
consecutive blocks is a stronger adversary assumption than a single blip. Exponential backoff
hardens the *repeated*-attack case specifically: a `FailedRecoveryAttempts` counter
(RECOVERING→SOVEREIGN regressions since the last successful recovery) doubles
`EffectiveDownHysteresisThreshold` on every consecutive failed attempt, capped at
`MaxDownHysteresisThreshold` — a single genuine network fault still only pays the plain
`DownHysteresisThreshold` cost, unaffected, but a repeated attacker faces a progressively harder
bar each cycle instead of the same fixed cost every time. Formally specified in
`spec/core/EngramFSM.tla` (`EffectiveDownHysteresisThreshold`, a recursive `Pow2` helper) and
ported to `x/sovereignty/keeper/circuit_breaker.go`; covered by unit tests for the doubling
formula, the cap, saturation, and the end-to-end `CalculateNextState` behavior change across a
failed attempt — not yet exercised by a live or in-process end-to-end scenario (an E8 candidate,
per the intro above).

**Measured (live-Docker spot-check, 5×2):** `scripts/e5_hysteresis_flapping/live_spot_check.py`
confirms the in-process sweep's finding under real consensus timing (not per-block mocking) at
`HYSTERESIS_WAIT` ∈ {0, 2, 5, 10, 20} × environment ∈ {stable, noisy_da} — matching
`tests/e2e/hysteresis_sweep_test.go`'s own value set (`{0,1,3,5,10,20}`, minus 1) rather than an
arbitrary live-only subset, so each live point is a direct confirmatory check of a specific
in-process prediction. Each combo required its own genesis (`ENGRAM_PARAM_HYSTERESIS_WAIT` at
genesis-generation time, `docs/DEVELOPMENT.md` §3) and a real 300s window on the 4-node cluster:

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

All 4 validators identical throughout every run, zero divergence at any sample. `stable` shows
zero flapping and full uptime at every `HYSTERESIS_WAIT` regardless of value, as expected (no noise
to filter). Under `noisy_da`, the shape is **not** the in-process sweep's clean monotonic decrease:
`anchored_uptime` rises slightly from HW=0 to HW=5 (11.70% → 15.29% → 18.75%), then falls off a
cliff and saturates near zero at HW=10-20 (1.06%, 1.04% — nearly identical). `flapping_count` is
similarly non-monotonic (4 → 13 → 7 → 11 → 11). This genuinely differs from the in-process sweep's
smooth curve, and the most likely reason is the RNG seed itself: `hysteresis_sweep_test.go` fixes
one noise sequence shared identically across every `HysteresisWait` value it tests, isolating the
parameter's effect from run-to-run variance by construction — the live spot-check has no such
control, since each `HYSTERESIS_WAIT` value here is a **separate** genesis reset and a separate
real WAN-chaos noise injection, so real timing/network variance between runs is a genuine
uncontrolled confound live testing can't remove the way a fixed RNG seed can in-process. The
qualitative conclusion the in-process sweep supports — no interior sweet spot, and large
`HYSTERESIS_WAIT` values are markedly worse, not better — still holds (HW=10/20's uptime collapse
is the clearest live evidence of this): the *shape* of the curve at small `HYSTERESIS_WAIT` is
where live and in-process genuinely disagree, worth flagging rather than smoothing over. The live
Figure 4 (`scripts/e5_hysteresis_flapping/live_figure_builder.py` →
`figure4_hysteresis_live.{png,pdf}`) reads directly from all 10 real `*_summary.md` files
`live_spot_check.py` writes (auto-discovered by filename, not hardcoded to a fixed value set), not
recomputed from raw CSV.

### 5b — Down-hysteresis (ANCHORED → SUSPICIOUS)

`CalculateNextState`'s ANCHORED branch gates the demotion to SUSPICIOUS on `UnhealthyStreak`, a
counter of consecutive non-critical (never critical) warning/unhealthy blocks: a single noisy
block is absorbed, not an instant demotion, symmetric with the recovery edge's own
`HYSTERESIS_WAIT` gate. `NextSafeBlocks` leaks by 1 on an absorbed block instead of hard-resetting
to 0, preserving partial hysteresis progress through sporadic noise. This is a pure extension of
the existing model (`DownHysteresisThreshold`, default 2, capped by `MaxDownHysteresisThreshold`,
default 8) — `HysteresisSafety`/`StrictFSMTransitionSafety` are unaffected (only *when* a
transition fires changes, never which edges are legal), confirmed by a full, exhaustive TLC
re-verification: `MC_FSMSafety` (514M states generated, 1.7M distinct, depth 16, zero violations)
and `MC_FSMLiveness` (17,283 states, depth 10, zero violations, all of
`CircuitBreakerLiveness`/`RecoveryAttemptLiveness`/`CompleteRecoveryLiveness` held). A critical
reading always bypasses this and demotes immediately, so a noise model for this edge must stay
strictly at WARNING level (`btc_gap = SuspiciousThreshold`, not `SovereignThreshold`) to actually
exercise the absorb path — unlike 5a's noise, which is deliberately critical-level since it targets
a different edge.

**Design (not yet measured):** start from ANCHORED, apply WARNING-level per-block noise at the same
20% probability as 5a, sweep `DownHysteresisThreshold` ∈ {1, 2, 4, 6, 8} (1 disables absorption —
every warning demotes immediately; 8 is `MaxDownHysteresisThreshold`'s cap). Measure
`AnchoredUptime`, `TimeOutsideAnchored`, `DemotionCount`, `AbsorptionRate`, and `WithdrawalBlocked`
(expected near-zero throughout, since SUSPICIOUS alone doesn't lock withdrawals — a useful
cross-check against 5c's and 5a's non-zero values).

### 5c — SUSPICIOUS-exit hysteresis (SUSPICIOUS → ANCHORED)

`CalculateNextState`'s SUSPICIOUS branch gates the exit to ANCHORED on
`SuspiciousSafeBlocks+1 >= SuspiciousHysteresisWait` consecutive healthy blocks via the leaky
`NextSuspiciousSafeBlocks` counter, absorbing a single healthy blip instead of exiting on it, while
`suspicious_duration` keeps accumulating through the whole absorption window rather than resetting
on every healthy reading. This closes the "Gray Failure Arbitrage" attack shape: without this gate,
`suspicious_duration` hard-resets to 0 the instant the FSM leaves SUSPICIOUS, so an attacker who
nudges sensors healthy for exactly one block right before `MaxSuspiciousTime` would escape
SUSPICIOUS for free and restart the gray-failure clock, repeatable indefinitely, holding the
network in a throttled SUSPICIOUS/ANCHORED loop without ever reaching SOVEREIGN. Formally specified
in `spec/core/EngramFSM.tla` (`SuspiciousHysteresisSafety`, mirroring `HysteresisSafety`) and
ported to `x/sovereignty/keeper/circuit_breaker.go`; covered by unit tests for the
absorb/exit/leak/critical-bypass cases. TLC re-verification (`MC_FSMSafety`, `MC_FSMLiveness`,
`MC_ServerRefinementSafety`) against the complete spec — down-hysteresis, backoff, and this fix
together with the separate `is_btc_spv_failed` `IsCriticalCondition` fix (`spec/README.md` §4.1) —
is pending, run once against the whole set rather than once per mechanism.

**Design (not yet measured):** start from SUSPICIOUS, apply healthy/warning noise at the same 20%
probability, sweep `SuspiciousHysteresisWait` ∈ {1, 2, 4, 6, 8}. Measure `AnchoredUptime`,
`FlappingCount`, `TotalTransitions`, `AbsorptionRate`; `suspicious_duration`'s own trajectory
(distance from `MaxSuspiciousTime`) is worth logging alongside these, since it's this mechanism's
actual defended quantity.

### Asymmetric hardening: an open question

Exponential backoff (5a) exists only on RECOVERING→SOVEREIGN. The same fixed-cost, repeatable-attack
shape it defends against — an attacker who can force a demotion at a constant, non-escalating cost
every time — exists structurally on ANCHORED↔SUSPICIOUS too: re-entering SUSPICIOUS costs a fixed
`DownHysteresisThreshold` streak (5b), escaping it costs a fixed `SuspiciousHysteresisWait` streak
(5c), and neither grows with repetition the way `EffectiveDownHysteresisThreshold` does. The
difference in consequence is real — `CircuitBreakerSafety` locks withdrawals only in
SOVEREIGN/RECOVERING, while SUSPICIOUS only restricts them (E3's failure matrix, above) — which argues
for lower priority on this edge, not for no defense at all. Whether ANCHORED↔SUSPICIOUS also
warrants an escalating-cost mechanism is an open question, not one resolved by the current design.

---

### E6 — Reanchoring Feasibility Evaluation

**Goal:** show the recovery proof is practical and scalable, not just a proving-system benchmark.
Answers RQ4 and its sub-questions:

- **RQ4.1** — How does proving cost scale?
- **RQ4.2** — Does verification remain succinct?
- **RQ4.3** — What are the trade-offs between PLONK-like and STARK-like backends?

**Circuit input:** a recovery interval from `checkpoint_old` → sovereign execution →
`checkpoint_new`, with five components:

| # | Component | What it proves |
|---|---|---|
| C1 | Header continuity | `H_i → H_{i+1}` is valid |
| C2 | FSM legality | The SOVEREIGN → RECOVERING → ANCHORED chain matches the spec |
| C3 | Withdrawal lock invariant | `withdrawal_locked = true` throughout the interval |
| C4 | SMT root progression | `root_old → root_new` through the state transitions |
| C5 | Policy binding | `policy_hash` is consistent |

**Table 6A — Circuit composition (target design):**

| Component | Constraints | Share |
| --- | ---: | ---: |
| Header verification | 12k | 22% |
| FSM transition | 2k | 4% |
| Withdrawal lock check | 1k | 2% |
| SMT inclusion proof | 18k | 33% |
| SMT update proof | 20k | 37% |
| Policy binding | 1k | 2% |
| **Total** | **54k** | 100% |

**Table 6B — Scaling benchmark (target):**

| Sovereign Blocks | Constraints | Prove (s) | Verify (ms) | Proof Size | Blocks/s |
|---:|---:|---:|---:|---:|---:|
| 10 | 54k | 0.8 | 7 | 410 B | 20.4 |
| 100 | 540k | 4.9 | 8 | 410 B | 23.2 |
| 1,000 | 5.4M | 43 | 8 | 410 B | 24.8 |
| 5,000 | 27M | 201 | 8 | 410 B | 26.3 |

**Table 6C — Backend comparison (target):**

| Metric | Noir + Honk | Plonky3 |
|---|---|---|
| Proof size | 400 B | 150 KB |
| Verify time | 8 ms | 28 ms |
| Prove time | 43 s | 22 s |
| Trusted setup | Yes | No |
| PQ secure | No | Yes |
| Recursion support | Good | Excellent |

**Figures needed:**

- **Figure 6** — Recovery Proof Scaling: 4 panels — (A) Constraint Count, (B) Proving Time (both
  linear); (C) Verification Time (near-flat); (D) Proof Size (near-constant).
- **Figure 7** *(optional)* — Backend Trade-off: radar or grouped-bar chart comparing Noir+Honk
  vs. Plonky3 across 6 criteria.

**Measured (real, not the target numbers above):**
`scripts/e6_zk_reanchoring_benchmark/benchmark_prover.sh` runs the full pipeline (`nargo compile`
→ `bb gates` → `nargo execute` → `bb prove` → `bb verify`, Noir 1.0.0-beta.22 + Barretenberg
5.0.0-nightly.20260522, UltraHonk) on `circuit/reanchoring/src/main.nr` at N = 4..256 headers; raw
results in `scripts/e6_zk_reanchoring_benchmark/results/table6b_scaling.csv`, tables/plots built
by `stats_collector.py` into `results/table6a_6b.md` + `results/figure6_scaling.{png,pdf}`. The
real circuit is simpler than Table 6A/6B's target design (chain continuity via a Poseidon2 hash
rather than a real SMT inclusion/update proof — see the comment at the top of `main.nr`), so the
measured numbers aren't meant to match the target figures above; they confirm the scaling shape E6
sets out to show: constraint count grows perfectly linearly (12.00 ACIR opcodes/header, R²=1.0000,
~0 fixed overhead), proving time grows near-linearly (0.130s → 0.684s from N=4 to 256),
verification time is flat (22-33ms, independent of N), proof size is constant (14,656 B at every
N).

**Table 6C/Figure 7 (Plonky3 backend comparison):** `benchmark_plonky3.sh` runs Plonky3's real
`prove_prime_field_31` example (pinned commit `a31a1443a114c58735850daa5b5fc5c43c138d9d`, BabyBear
field, UniStark+FRI, Poseidon2 permutation, transparent setup, no trusted setup) at the same
N=8..256; `table6c_collector.py` merges it with `table6b_scaling.csv` into
`results/table6c_backend_comparison.md` + `results/figure7_backend_tradeoff.{png,pdf}`. At N=256:
Noir+Honk — 14,656 B proof, 22.0 ms verify, 0.684 s prove, KZG trusted setup, not PQ-secure;
Plonky3 — 1,278,939 B proof (~87x larger), 32.2 ms verify, 0.044 s prove (~15.5x faster),
transparent/FRI setup, PQ-secure. The two circuits aren't bit-identical (Plonky3 uses its own
example rather than a hand-ported `Header` struct matching `main.nr`), but since the Noir circuit
uses a real Poseidon2 permutation, both sides measure the same real primitive rather than one side
being an approximate proxy, making the trade-off comparison (KZG/pairing vs. FRI/hash-based, proof
size vs. prove time) trustworthy.

**Deployed circuit design: max-N=256 with padding/count.** The Table 6B/6C scaling curve above was
produced by *recompiling* `circuit/reanchoring/src/main.nr` at each N (a valid way to measure how
cost scales with header count in the abstract, and Table 6C's Plonky3 comparison still rests on
it). The circuit actually **deployed** (embedded VK, `x/sovereignty/keeper/zk_assets/vk`) instead
compiles **once** for a large ceiling (`N_MAX=256`, chosen because Engram blocks are produced well
under 2s, and 256 is Table 6B's own largest already-measured, still-cheap data point) with a real,
public `count` witness (1..=256) gating which of the 256 header slots are constrained — matching
`spec/README.md`'s formal relation `x = (rt_last, rt_new, n)` literally, and letting a single
compiled circuit cover any interval length from 1 to 256 headers, submittable immediately, in one
proof. Real measured cost (`circuit/reanchoring/RESULTS_MAXN_PADDING.md`, count ∈ {1, 4, 130, 256},
all real `nargo`/`bb` runs): **6,143 ACIR opcodes, 47,613 circuit size, ~1.06s prove, ~22ms verify,
14,656 B proof, 96 B public inputs — all constant regardless of `count`** (count=1's prove time,
1.054s, is statistically indistinguishable from count=256's, 1.059s). This is a measured, bounded
overhead versus a fixed-N=256 compile with no padding logic (2.0x opcodes, 1.55x prove time) — the
cost of the padding-gate logic and a dynamic array index — still comfortably under one block time.
`x/sovereignty/keeper/msg_server.go`'s public-input parsing matches this format (96 bytes);
`findHeaderByStateRoot`'s rolling-checkpoint logic works for any interval length, never depending
on a fixed N.

> **Scientific claim:** Recovery proofs scale linearly in prover cost while preserving
> constant-size proofs and constant-time verification — reanchoring is practical, scalable, and
> incurs bounded overhead.

**Follow-up needed: prover throughput vs. a genuinely long BTC outage.** The deployed batching
policy (`BATCH_THRESHOLD=256`, `STABLE_POLLS_REQUIRED=3` debounce,
`scripts/reanchoring_prover/watch_and_prove.sh`) measures two different regimes, only one of which
has actually been stress-tested live:

- **Catching up a pre-existing backlog.** Measured throughput is 256 headers / 23.61s ≈ 10.84
  headers/s once a full batch fires — roughly an order of magnitude ahead of Engram's own
  block-production rate (<2s/block, ≈0.5-1 blocks/s), confirmed live (proof accepted, 0 rejected,
  `HeaderHistory` count climbing cleanly toward the next 256-header batch).
- **Keeping pace with an ongoing SOVEREIGN period in real time.** The prover cannot outrun block
  production — it can only prove blocks that already exist. This is a physical, liveness-preserving
  limit, not a code defect, but it means total recovery wall-clock time is bounded below by
  `blocks_in_interval / block_production_rate`, not by prover speed alone.

Neither regime has been measured against a BTC outage lasting **hours** — only against outages on
the order of minutes (E3/E9's live BTC stop/restart cycles, E10's reorg tests). Two things are
unknown at that timescale and need a dedicated run before citing this design as solving the general
case:

1. Whether `HeaderHistory` growth stays linear in wall-clock outage duration, or whether some other
   resource (state DB size, block time itself, mempool behavior) degrades once the tracked header
   count sits in the thousands rather than the hundreds this design has been exercised against.
2. Whether the measured ~10x throughput headroom holds at that scale, or is an artifact of the
   specific backlog size (a few hundred headers) exercised so far — a multi-hour outage could
   plausibly accumulate a backlog two to three orders of magnitude larger.

Needs a dedicated experiment: hold Bitcoin submission down (`AnchorTracker.
SetSubmissionPausedFile`, the mechanism S2 already uses) for several hours under continuous block
production, and measure `HeaderHistory` size, per-proof latency, and total recovery wall-clock time
as a function of outage duration — not yet run.

**Priority:**

| Level | Artifact |
|---|---|
| Required | Figure 6, Table 6A, Table 6B |
| Optional | Table 6C, Figure 7 |

---

### E7 — Extended Proposal Consensus Overhead

Extended proposal adds `fsm_state`, `da_receipt`, `btc_receipt`, `zk_proof_ref`. Question: does
the FSM mechanism reduce throughput or increase latency too much versus plain CometBFT?

| Variant | Description |
|---|---|
| V0 | Vanilla CometBFT |
| V1 | + `fsm_state` only |
| V2 | + DA receipt |
| V3 | + BTC receipt |
| V4 | + P2P sensor digest |
| V5 | + ZK proof ref / verification flag |

**Metrics:** proposal size overhead, block validation CPU, commit latency, throughput,
rounds/block, bandwidth per validator, nil-prevote ratio under sensor mismatch.

> **Target result:** low overhead in the normal case; overhead rises mainly when receipt
> verification or a sensor mismatch occurs.

**Measured:** `go test ./tests/benchmark/... -bench=. -benchmem` measures the real cumulative JSON
size of each V0-V5 payload and the real cumulative CPU cost of each validation step
(`CalculateNextState`, `da.VerifyReceipt`, `anchor.VerifyReceipt`), plus the full
`ProcessProposal` cost (V5, real end-to-end). `scripts/e7_consensus_overhead/measure_overhead.py`
builds Table 4 from this; result in `scripts/e7_consensus_overhead/results/table4_overhead.md`.
Note: V4 (P2P digest) is a size estimate only — that field isn't actually in the wire format (P2P
health is validated from the leader's local `keeper.Metrics`, not carried in the proposal). The
table also includes a real vanilla-CometBFT baseline (2 parallel `engramd` processes, one normal,
one `--vanilla` — the same baseline shared with E2/E3). Measured overhead: **+228 bytes/block** for
the `ENGRAM_EXTENDED_PROPOSAL_V1` marker on 100% of blocks; block interval shows no meaningful
difference at idle (CometBFT's default `timeout_commit` dominates both, not `ExtendedProposal`).

**Two overhead regimes (steady-state vs. recovery-event), not one blended average:** the V0→V5
table and the +228B/block figure above only cover fields present on every block
(`fsm_state`/`da_receipt`/`btc_receipt`) — they exclude the real cost of the ZK proof entirely,
since `ZKProofRef` (even after switching to a hash, see E6) never carries the real proof bytes
(~14,656 bytes UltraHonk, measured in E6's `table6b_scaling.csv`) inside `ExtendedProposal` — the
real proof travels through a separate `SubmitRecoveryProof` tx. A single average both
underestimates the near-zero cost of the healthy path and hides the real, bounded, rare cost of
the recovery path:

- **Steady-state tax** (every block, always paid): ~230 B/block (measured above), negligible CPU.
- **Recovery-event cost** (only blocks carrying a `SubmitRecoveryProof` tx — rare, self-limiting):
  the real proof, ~14,656 B (E6), plus the real CPU cost of `bb verify` inside `DeliverTx`
  (`x/sovereignty/keeper/reanchor.go`'s `VerifyZKProof`) — measured via
  `go test ./tests/benchmark/... -bench=BenchmarkVerifyZKProof`, running the real `bb verify` on
  the real N=4 proof produced in E6: **~18.77 ms/verify**
  (`BenchmarkVerifyZKProof-8: 3 iterations, 18,771,861 ns/op`). This cost is only paid once
  `RealProofSubmittedHeight` catches up to the tip, and doesn't persist — matching the "graceful
  degradation, near-zero tax on the healthy path" design intent, a stronger claim than one blended
  average.
- `scripts/e7_consensus_overhead/live_overhead_scan.py` aggregates per-block overhead by the real
  `fsm_state` and flags (by size heuristic, not exact protobuf decoding) blocks likely carrying a
  `SubmitRecoveryProof` tx. **Live result (real RECOVERING sample, heights 5-380 on a fresh
  genesis):** 376 blocks scanned, 100% marker coverage — 184 SOVEREIGN (avg 248.2 B), 6 RECOVERING
  (avg 270.0 B), 186 ANCHORED (avg 247.0 B). One block (height 194) matched the recovery-proof
  heuristic — `other_tx_bytes=14881`, consistent with E6's measured ~14,656 B UltraHonk proof plus
  envelope overhead. Confirms the steady-state tax stays flat (~247-270 B/block) across all three
  states, with the one-off recovery-proof cost isolated to the single block that actually carries
  it — matching the "two regimes" design intent, not inferred from a healthy-only sample.
  Data: `scripts/e7_consensus_overhead/results_live/table4_live_overhead.{csv,md}`.

---

### E8 — Attack-Resilience Test Suite

Turn the safety lemmas into integration tests or simulation traces.

**A1-A8 matrix** (maps to formal lemmas already in `spec/README.md` — Eclipse ≈ Lemma 7.5, Data
Withholding ≈ Lemma 7.2 — and reuses E4's infrastructure rather than duplicating it):

| # | Attack | Expected result | Live result | Live-Docker mechanism |
|---|---|---|---|---|
| A1 | Eclipse Attack (isolation) | Filter blocks the slot before it's taken; FSM doesn't degrade incorrectly | **PASS** — filter holds exactly 8/8 (`MaxPeersPerSubnet`), cluster unaffected | `docker/attacker-peer-swarm.yml` leg `a1`, `scripts/e4_p2p_eclipse_detection/live_sybil_attack.py a1` |
| A2 | Sybil via simulated multi-subnet | Filter blocks by subnet density, not fooled by diversification | **PASS** — filter holds 8/8 despite a larger swarm (12 attackers) | leg `a2` of the same script |
| A3 | Data Withholding | Honest validators reject a proposal claiming a fake DA attestation | **PASS** — `safety_held=True divergence_events=0`, height progressed normally through attack + recovery | `docker/engram-node04-byzantine.yml` (`ENGRAM_BYZANTINE_BEHAVIOR=false_da_attestation`), `scripts/e8_attack_resilience/live_byzantine_attacks.py a3_false_da_attestation` |
| A4 | Forged BTC Receipt | Honest validators reject a checkpoint hash that doesn't match `ExpectedBlockHash` | **PASS** — same verdict, same mechanism | same script, scenario `a4_forge_btc_hash` |
| A5 | Withdrawal During SOVEREIGN | Tx is withheld (never committed) while SOVEREIGN | **PASS** — `blocked_correctly=True` (real CLI timeout waiting on DeliverTx, tx never commits while SOVEREIGN); cluster recovers normally once celestia-bridge is restored | `engramd tx-submit-forced-tx --payload "TX_WITHDRAWAL..."` (`cmd/engramd/e8_cli.go`), `scripts/e8_attack_resilience/live_withdrawal_test.py` |
| A6 | Malicious Proposer | Honest validators reject a fake `fsm_state` that doesn't match their own computation | **PASS** — same verdict, same mechanism | same byzantine script, scenario `a6_fake_fsm_state` |
| A7 | Censorship / Tx Withholding | A leader deliberately omitting a tx is caught by `IsCensoring`/`ForcedTxQueue` | **PASS** — `safety_held=True divergence_events=0`, height progressed continuously throughout the censoring window | `docker/engram-node04-byzantine.yml` (`ENGRAM_BYZANTINE_BEHAVIOR=censor_tx:<hex>`), `scripts/e8_attack_resilience/live_censorship_test.py` |
| A8 | Combined Attack | Safety holds under multiple overlapping attack vectors | **PASS** — node04 byzantine (`fake_fsm_state:SOVEREIGN`) simultaneous with an A1 swarm of attackers, `safety_held=True divergence_events=0`, height progressed continuously through the combined-attack window | `scripts/e8_attack_resilience/live_combined_attack.py` |
| — | Double-signing | Evidence extracted/logged | **PASS** — all 3 honest validators detected real `DuplicateVoteEvidence`, 1-block detection latency | `x/sovereignty/types/evidence.go` + `preblock.go`'s `recordDetectedEvidence`, `docker/engram-node04-double-sign.yml`, `scripts/e8_attack_resilience/live_double_signing_test.py` — see below |

**Double-signing closed without wiring `x/evidence`:** ABCI 2.0's `RequestFinalizeBlock.Misbehavior`
already carries real `DuplicateVoteEvidence`/`LightClientAttackEvidence` reports from CometBFT's
stock evidence pool (no fork changes) — no need to wire the Cosmos SDK's `x/evidence` module, just
read this field directly in the existing `PreBlocker`. `preblock.go`'s `recordDetectedEvidence`
reads `req.Misbehavior` and writes it to new state (`Keeper.DetectedEvidenceCount`/
`LastDetectedEvidence`, JSON-encoded, no new proto needed), plus a `SLASHABLE EVIDENCE DETECTED`
log line. Safe to commit: `req.Misbehavior` is already-agreed, deterministic data, exactly like
`req.Txs` — not a fresh local read.

The hardest half (one real, live validator behaving maliciously) is closed via
`docker/engram-node04-double-sign.yml`: it clones node04's real `priv_validator_key.json`
into a second container, but deliberately does **not** share `priv_validator_state.json` (FilePV's
last-signed height/round tracker, its built-in double-sign guard) — sharing it would make the
second process unable to double-sign, by design. 3 unit tests (`evidence_test.go`) confirm
`recordDetectedEvidence` behaves correctly.

**Definitional note (double-signing ≠ AppHash divergence):** double-signing/equivocation means
**one validator** (one private key) signs **2 conflicting votes** (different `BlockID`) for the
**same height + round + vote type** — different from "AppHash divergence" (state-machine
non-determinism, where honest validators compute different state roots, checked via `safety_held`
in A3-A8 above). This harness produces real equivocation by running 2 independent processes
holding one private key with no shared signing history — when both independently vote differently
for the same height/round, CometBFT's evidence pool detects it with no additional simulation
needed.

**Attack-hardening: rate-limited Timeout messages.** `ENGRAM_TIMEOUT_FLOOD_INTERVAL_MS`
(`timeoutFloodRoutine`, `engram-consensus-core`'s `consensus/state.go`) makes node04 actively
re-broadcast a signed Timeout every 50ms — genuinely signed, individually valid messages, bypassing
the real precommit-wait timer entirely — via `make timeout-flood-on`/
`docker/engram-node04-timeout-flood.yml`, exercising an active Byzantine validator deliberately
flooding valid attestations to manipulate round-skip cadence (distinct from the crash fault model
`chaos-crash`/SIGKILL already covers). `handleTimeoutMessage` bounds accepted rounds to
`(cs.Round, cs.Round+5]` before signature verification; `enterNewRound` evicts stale
`cs.timeoutSenders` entries on every round advance, not just on height change; `PeerState.
allowTimeoutMessage` caps each peer to 20 `TimeoutMessage`s/second at the Reactor level. Covered by
`consensus/round_skip_test.go`'s `TestStateTimeoutFloodCannotForceRoundSkipAlone` (one validator's
repeated flood still counts as only its single vote toward f+1) and the full `consensus/...` suite.

**Live result (2 independent runs, moderate rate):** `scripts/e8_attack_resilience/live_timeout_flood_test.py`,
4-node cluster, 30s baseline + 60s flood (node04, 50ms interval = 20 msgs/s) + 30s recovery. Both
runs **PASS** — `safety_held=True divergence_events=0` both times, `cadence_held=True` both times:
block rate during the flood was never degraded versus baseline (run 1: 0.800 vs. 0.771 blocks/s;
run 2: 0.816 vs. 0.661 blocks/s) — if flooding could force extra round-skips, the flood-phase rate
would have dropped, not held or risen. Data: `results_live/timeout_flood_20260812T072615{,_summary}`
and `..._20260812T073456{,_summary}`.

**Live result (extreme rate, 25x the rate limiter's threshold, with resource measurement):** same
script, `--interval-ms 2 --sample-stats` — node04 attempts 500 signed `Timeout`s/s, 25x
`allowTimeoutMessage`'s 20/s cap. **PASS** — `safety_held=True divergence_events=0`,
`cadence_held=True` (baseline 0.825, flood 0.789 blocks/s). Direct, not inferred, evidence the
hardening in place actually does its job:

- **Rate limiter engaged hard:** real "rate limit exceeded" drops per honest node — **28,071 /
  29,238 / 30,376** over the 60s window (of ~30,000 attempted messages), i.e. the limiter rejected
  essentially all traffic past its 20/s allowance, letting through only what
  `recordTimeoutSenderAndMaybeAdvance` needs.
- **CPU cost stayed bounded, not explosive:** honest-node CPU roughly tripled under flood (avg
  2.4-3.3% baseline → 7.85-9.51% flood, max 22.40% momentary) but never approached saturation;
  node04 itself (paying the real signing cost for 500 `SignTimeout` calls/s) topped out at 28.47%.
  All three honest nodes' CPU returned to baseline-range within the recovery window.
- **Memory stayed flat:** 78.7-92.2 MiB baseline vs. 82.7-92.2 MiB flood (a few MiB at most) — no
  unbounded growth from thousands of dropped-but-still-network-received messages.

Full CPU/memory table and drop counts: `results_live/timeout_flood_20260812T073739_summary.md`.

**Re-run on the pairwise-link topology:** A3/A4/A6 (byzantine), A5 (withdrawal), A7 (censorship),
A8 (combined), and both timeout-flood rates all re-confirmed **PASS** against a
freshly-redeployed cluster (`safety_held=True divergence_events=0` on every one; A5's
`blocked_correctly=True`; timeout-flood's `cadence_held=True` at both the moderate rate, baseline
0.808 vs. flood 0.608 blocks/s, and the extreme 25x rate, baseline 0.586 vs. flood 0.813 blocks/s,
26,563-28,777 messages rate-limited). A1/A2 reuse E4's infrastructure per this section's own note
above, not re-run separately — E4's own re-run already covers them on this topology. Double-signing
re-confirmed **PASS** at height ~15,150: all 3 witnesses independently detected real
`DuplicateVoteEvidence` (offense heights 15156/15160, 1-block detection latency both times),
matching the original result's shape exactly.

**Beyond pass/fail:** rounds-to-recover, invalid proposals rejected, honest-validator agreement
rate, censorship latency, slashable-evidence detection latency.

**Measured, all 10 rows (A1-A8 + Double-signing + Timeout flooding):** every row above now has a
live-Docker result (not just in-process) — see the "Live result" column and the corresponding
`results_live/*_summary.md` files under `scripts/e8_attack_resilience/results_live/`. This is the
first time this matrix reached full live-Docker coverage; the prior in-process-only run
(`scripts/e8_attack_resilience/trigger_disconnect.py`, `go test -json`) is kept as a parallel
reference at `scripts/e8_attack_resilience/results/table3_attack_resilience.md`, no longer the
primary data source.

---

### E9 — Trace-Driven Stress Test

If time allows, a trace-driven experiment goes beyond a synthetic-only benchmark. Replay traces
simulating Bitcoin congestion, DA outage, P2P churn, and mixed failure into the FSM prototype.

Present results as a timeline of ANCHORED → SUSPICIOUS → SOVEREIGN → RECOVERING → ANCHORED,
plotted alongside:

- BTC finality gap
- DA gap
- P2P health score
- Block commit rate
- Withdrawal lock status
- Proof generation status

**Measured (in-process):** `go test ./tests/e2e/... -run TestE9_TraceDrivenCombinedFailure`
replays one continuous real trace (not 7 separate scenarios like E2) through the real
Harness/BeginBlocker: growing BTC congestion → layered DA outage while still SOVEREIGN → layered
P2P churn spike (all 3 failures at once) → sequential recovery → RECOVERING → ANCHORED. Data:
`tests/e2e/results/e9_trace_driven.csv` (48 blocks), 6-panel Figure 2 at
`scripts/e9_trace_driven/results/figure2_trace_timeline.{png,pdf}` — confirms the chain keeps
committing blocks even with all 3 failures layered simultaneously.

**Measured (live Docker):** `scripts/e9_trace_driven/live_combined_trace.py` runs one continuous
real trace on the 4-node cluster (not mocked): `chaos-btc-delay` (BTC congestion) → layered
`docker stop celestia-bridge` (DA outage, still under BTC pressure) → layered 3 cycles of
`chaos-loss` (P2P churn burst, all 3 failures at once) → recovery in reverse order → wait for a
real ANCHORED via the ZK pipeline, under a `chaos-wan-latency` per-validator WAN-realism baseline
and the pairwise-link P2P mesh (§3's `MinSubnetDiversity=2` topology). All 7 phases **passed** in
a single 319s run: height progressed continuously throughout with no stalled round and all 4 nodes
height-synced at every sample; all 4 validators transitioned ANCHORED → SOVEREIGN together at
t=152s (the triple-fault peak), then SOVEREIGN → RECOVERING → ANCHORED together at t=308s/315s
once healing completed — zero divergence across validators at any transition. Data:
`scripts/e9_trace_driven/results_live/e9_combined_trace_20260812T140833.csv` (384 samples) +
`_summary.md`. No automated 6-panel figure (BTC gap/DA gap/P2P health aren't in committed state,
per the limitation documented in `x/sovereignty/preblock.go`'s `NewPreBlocker`) — only
`fsm_state`/height/marker are real live data.

---

### E10 — Bitcoin Reorg Fork-Choice Reaction

Formally specified (`spec/core/EngramConsensus.tla`'s `BitcoinReorg` action; `spec/README.md`
SS7.3 "Threat Model Boundary: Bitcoin Reorg Depth"; `CanElect`'s FSM-state-dependent branch —
`IsKDeep` for ANCHORED/SUSPICIOUS, `IsMaxStakeBranch` for SOVEREIGN).

**Mechanism:** `bitcoin-node02` (node01's regtest sync peer, never dialed directly by `engramd`)
is isolated via `setnetworkactive false`, `invalidateblock` forces it to rebuild an alternate chain
from a target height, it mines past node01's (frozen, via `bitcoin_miner_loop.sh`'s
`MINER_INTERVAL_OVERRIDE_FILE`) height, then reconnecting forces a real reorg on node01 — and every
validator watching it. `scripts/e10_bitcoin_reorg/live_reorg_test.py` drives this and independently
verifies via `getblockhash` that a reorg actually happened, not just assumed from height catching
up.

**Shallow reorg (depth=1, < `KDeepFinality`=2):** `scripts/e10_bitcoin_reorg/results_live/
reorg_shallow_20260812T140516.csv` — all 4 validators stayed `ANCHORED` throughout a verified real
1-block reorg. Matches the spec's own safety claim: `IsKDeep`/`CanElect`'s ANCHORED/SUSPICIOUS
branch structurally can't re-elect a certificate lost this shallow.

**Deep reorg (depth >= `KDeepFinality`), targeting the actually-anchored checkpoint:**
`scripts/e10_bitcoin_reorg/results_live/reorg_deep_20260812T143632.csv` — a verified real 15-block
reorg (heights 472-486), deliberately sized to guarantee orphaning the checkpoint height
`h_btc_anchored` was actually pointing to at the time (477). Result: **all 4 validators
transitioned to `SOVEREIGN`**, in lockstep. `spec/README.md` §7.3 calls deep-reorg behavior
explicitly out of scope/unguaranteed — this is a concrete finding beyond what the spec promises,
not a violation of it: `AnchorTracker.VerifyAnchor` (`x/anchor/anchor.go`, "no spec line")
re-derives `BlockContainsTag` via `getblockhash(height)` fresh on every call rather than caching,
so a reorg replacing the block at the previously-anchored height is caught on the very next
re-check — `is_btc_spv_failed` goes true, `IsCriticalCondition` fires, forcing SOVEREIGN. A real
defense-in-depth mechanism the spec doesn't require but the concrete implementation provides.

**Follow-up needed before treating this as a settled result.** The direction (fail-safe to
SOVEREIGN rather than silently trusting an invalidated anchor) is a genuine positive finding, but
the measurement itself has real limits, honestly flagged rather than glossed over:

- **Single run.** One depth-15 trial, not repeated -- no variance/repeatability check.
- **Boundary untested.** Depth-15 is far past `KDeepFinality`=2 with a wide safety margin. The
  precise minimum depth at which `VerifyAnchor` starts catching a reorg -- particularly right at
  `KDeepFinality`+1, the smallest case `spec/README.md` §7.3 classifies as "deep" -- is still
  unmeasured. `--reorg-depth 3` (the smallest deep case at the current default `KDeepFinality`=2)
  is the natural next run.
- **Not formally verified.** `AnchorTracker.VerifyAnchor` has no spec line and isn't covered by any
  TLC/Apalache model -- its correctness here rests on this one live observation plus code reading,
  not exhaustive proof.
- **Recovery path after a deep reorg not separately measured.** The cluster was still `SOVEREIGN`
  when this run ended; how long recovery back to `ANCHORED` takes *specifically following a deep
  reorg event* (as opposed to the DA/BTC-outage recovery paths E3/E9 already measure) is unmeasured
  -- worth chaining a settle-and-recover phase onto a future run, mirroring E3/E9's structure.

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
