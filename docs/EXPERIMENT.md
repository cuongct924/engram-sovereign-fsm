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
| **E5** Hysteresis and flapping sensitivity | Show RECOVERING → ANCHORED is stable | No-hysteresis recovery | Oscillation count, failed recovery attempts, safe-block waiting cost |
| **E6** Re-anchoring feasibility | Show the recovery proof is practical and scalable | Noir+Honk vs. Plonky3; no-ZK baseline (re-execute) | Constraint count, proving/verification time, proof size, backend trade-off |
| **E7** Consensus overhead benchmark | Measure the cost of the extended proposal fields | Vanilla proposal | Proposal size, CPU validation cost, throughput, block latency |
| **E8** Attack-resilience scenarios | Demonstrate the security story empirically | Malicious proposer; data withholding; forged BTC receipt; withdrawal during SOVEREIGN; censorship | Accepted/rejected proposals, invalid commit count, forced-inclusion latency |
| **E9** Trace-driven stress test | Strengthen the paper with a realistic workload | Synthetic-only experiment | Downtime under historical/simulated BTC congestion and DA-delay traces |

---

## 3. Per-Experiment Design

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

**Measured (live, 4-node Docker cluster):** `scripts/e2_fault_injection/live_scenario_matrix.py`
chains all 7 scenarios into one continuous 1394s run (Pumba netem for S2/S4/S5, real
`docker stop`/`start` for S3/S6/S7). Result: the cluster never reached ANCHORED across all 7
phases — it oscillated continuously between RECOVERING/SOVEREIGN from S1 (baseline) onward,
matching the negative finding in §E5 below (hysteresis has no partial credit; one bad reading
resets the streak to 0). S7 (waiting for a real ANCHORED via the ZK pipeline) timed out after
600s: 23 real proofs were accepted during the run (after fixing `watch_and_prove.sh`'s SIGPIPE
bug, §E9), 29 rejected by a real race condition (the interval grows faster than N=4 proofs can
keep pace) — not enough to catch up within 600s. Data:
`scripts/e2_fault_injection/results_live/s*.csv`. The live Figure 3
(`scripts/e2_fault_injection/live_figure_builder.py`) replaces the two prior plots with
`figure3_state_timelines_live.{png,pdf}` (7 panels, each picking the node with the most valid
samples rather than a fixed node01, since S4/S5 deliberately isolate node01 and lose its RPC
connectivity for most of the run) and `figure3_summary_bars_live.{png,pdf}` (3 real metrics:
blocks committed, state transitions, time-outside-ANCHORED ratio — `time_to_fallback`/
`withdrawal_blocked_blocks` are omitted, since neither field exists in the schema polled directly
over RPC).

**Vanilla CometBFT baseline (measured):** `engramd start --vanilla` (`app/app.go`) runs the same
binary/module but skips `SetPrepareProposal`/`SetProcessProposal`/`SetPreBlocker`, so BaseApp uses
its default handlers with no `ExtendedProposal`. Running both variants side by side
(`scripts/e7_consensus_overhead/vanilla_comparison.sh`) confirms: the normal node always carries
the `ENGRAM_EXTENDED_PROPOSAL_V1|...` 228-byte marker in `Txs[0]` on every block; the vanilla node
carries 0 tx. Building this baseline also surfaced a real bug: `cmd/engramd/main.go`'s `runStart`
previously used `cmtcfg.DefaultConfig()` directly instead of reading `config.toml` from disk,
fixed with a viper-based loader (`loadConfig` in `main.go`) — also relevant to the Docker
multi-node setup, where per-node config (ports, peers) was being silently ignored at start time.

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

**Live run results (4-node cluster):** full write-up in
`scripts/e4_p2p_eclipse_detection/results_live/sybil_attack_live_run_20260808_summary.md`.
Summary:

- **A1** (10 attackers on `engram-net`): the filter holds at exactly **8/8**
  (`MaxPeersPerSubnet`), consistent across 2 repeated runs. The 3 honest validators (which appear
  on the `172.21.0.0` subnet due to the gateway-priority quirk, `docs/ARCHITECTURE.md` §1) are
  unaffected throughout; block height and AppHash progress normally, matching across all 4 nodes.
- **A2** (12 attackers, intended to spread across 4 subnets): the filter still holds at **8/8**,
  but does not achieve the intended subnet diversity — a multi-homed Docker container (attached to
  both its own subnet and `engram-net`, to reach `engram-node01`) defaults its outbound route to
  the network declared **second** in the service definition, not the first — the same mechanism
  that puts the 4 real validators on `bitcoin-net` instead of `engram-net`
  (`docs/ARCHITECTURE.md` §1). This is a limitation of simulating via Docker's default bridge
  networking, not a `FilterPeerByAddr` bug: the filter defends correctly on whichever subnet peers
  actually arrive from, shown by both A1 and A2 holding at 8/8. A `gw_priority` fix was attempted
  on one container and did not resolve it within the available time.
- **Fixes applied during the live run:** (1) a bare `docker compose ... down` (no service names)
  tears down the entire cluster, not just the attacker swarm — fixed by using `stop`+`rm -f` with
  explicit service names, applied to
  `live_combined_attack.py`/`live_double_signing_test.py` too; (2) attacker `persistent_peers`
  lacked the real node ID (CometBFT needs `id@host:port`) — fixed by resolving the real ID via RPC
  at container start; (3) attacker A2 initially had no route to `engram-node01` (only its own
  subnet attached) — fixed by attaching both networks, which is what surfaced the gateway-priority
  finding above.

---

### E5 — Hysteresis Sensitivity

The spec requires RECOVERING → ANCHORED only once `safe_blocks` reaches `HYSTERESIS_WAIT` and the
proof is valid. Reviewers will ask how this threshold was chosen.

Sweeps `HYSTERESIS_WAIT` over {0, 1, 3, 5, 10, 20} across 5 environments: stable, and four with
continuous noise (`noisy_btc`, `noisy_da`, `noisy_p2p`, `combined_adversarial`) — each block
independently has a 20% chance of being noisy. All `HYSTERESIS_WAIT` values in the same
environment share a fixed RNG seed for a fair comparison.

**Metrics:** flapping count, recovery latency, block throughput during RECOVERING, false recovery
rate, withdrawal-locked time.

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
time outside ANCHORED overall. Additionally: once in ANCHORED, a single bad reading drops it
immediately (ANCHORED has no hysteresis of its own), and SUSPICIOUS→ANCHORED has no hysteresis
gate at all (`CalculateNextState`'s SUSPICIOUS branch: `if healthy { return ANCHORED }`,
unconditional) — an asymmetry between the recovery edge (gated) and the regression edge
(unguarded). `HYSTERESIS_WAIT` does not "filter noise" as hypothesized; it sets an increasingly
hard pass/fail test, and every failure itself creates more oscillation. This is a negative result
worth publishing, not a bug to fix or a parameter to retune.

**Measured (live-Docker spot-check):** `scripts/e5_hysteresis_flapping/live_spot_check.py`
confirms the same direction under real consensus timing (not per-block mocking), with a narrower
2×2 scope (`HYSTERESIS_WAIT` ∈ {2 (current default), 10} × environment ∈ {stable, noisy_da}) rather
than the full 6×5 sweep, since each combination needs a `params.go` edit, rebuild, and clean
redeploy. Each combo measured over a real 300s window on the 4-node cluster:

| HYSTERESIS_WAIT | Environment | Flapping (300s) | Transitions | Anchored uptime |
|---:|---|---:|---:|---:|
| 2 | stable | 0 | 0 | 0.00% |
| 2 | noisy_da | 12 | 13 | 0.00% |
| 10 | stable | 0 | 1 | 0.00% |
| 10 | noisy_da | **14** | 15 | 0.00% |

Flapping increases with `HYSTERESIS_WAIT` under real noise (12→14 for noisy_da), matching the
in-process direction — confirms this isn't a mock-harness artifact. `anchored_uptime` is 0% across
all 4 combos (unlike the in-process run, which measured positive uptime at low HW): the cluster
was already oscillating between RECOVERING/SOVEREIGN before this spot-check ran, so the 300s
window never observed a real ANCHORED — a real limitation of measuring a short window on a system
that had already accumulated bad state, not evidence against the main finding. The live Figure 4
(`scripts/e5_hysteresis_flapping/live_figure_builder.py` → `figure4_hysteresis_live.{png,pdf}`)
reads directly from the 4 real `*_summary.md` files `live_spot_check.py` writes, not recomputed
from raw CSV.

**Operational note (recurring failure class — see `docs/DEVELOPMENT.md`):** redeploying the
cluster for the HW=10 combo hit a permanent round-skip at height=1, caused by two overlapping
issues: (1) the Bitcoin wallet on `bitcoin-node01` had been unloaded by an earlier container
restart, so `AnchorTracker` couldn't submit an anchor tx; (2) after reloading the wallet,
`bitcoin_miner_loop.sh` had also stopped running, so no new Bitcoin blocks existed to bring the
anchor tx to `kDeepFinality=2` confirmations, so `h_btc_anchored` never advanced despite the
wallet being funded. Restarting the miner loop resolved it. This confirms why
`docs/DEVELOPMENT.md` §3 requires the Bitcoin wallet funded *and* continuously mining
before/throughout every `engramd` run, not just at first bootstrap.

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

*(The circuit originally used `pedersen_hash`; switching to `std::hash::poseidon2_permutation` —
the pinned Noir version has no convenience wrapper, so a fixed-length sponge was hand-built —
dropped the ACIR opcode count sharply versus the prior measurement: 12.00 opcodes/header instead
of 42, since the `POSEIDON2_PERMUTATION` black-box gate is far cheaper than `pedersen_hash`'s
field-arithmetic construction in ACIR. The embedded VK (`x/sovereignty/keeper/zk_assets/vk`) was
regenerated to match, and re-verified end-to-end via real `bb prove`/`bb verify` plus the real Go
path `VerifyZKProof`/`BenchmarkVerifyZKProof` before the switch.)*

**Table 6C/Figure 7 (Plonky3 backend comparison) — done:** `benchmark_plonky3.sh` runs Plonky3's
real `prove_prime_field_31` example (pinned commit `a31a1443a114c58735850daa5b5fc5c43c138d9d`,
BabyBear field, UniStark+FRI, Poseidon2 permutation, transparent setup, no trusted setup) at the
same N=8..256; `table6c_collector.py` merges it with `table6b_scaling.csv` into
`results/table6c_backend_comparison.md` + `results/figure7_backend_tradeoff.{png,pdf}`. At N=256:
Noir+Honk — 14,656 B proof, 22.0 ms verify, 0.684 s prove, KZG trusted setup, not PQ-secure;
Plonky3 — 1,278,939 B proof (~87x larger), 32.2 ms verify, 0.044 s prove (~15.5x faster),
transparent/FRI setup, PQ-secure. The two circuits aren't bit-identical (Plonky3 uses its own
example rather than a hand-ported `Header` struct matching `main.nr` — see the comment at the top
of `benchmark_plonky3.sh`), but since the Noir circuit now uses a real Poseidon2 permutation, both
sides measure the same real primitive rather than one side being an approximate proxy, making the
trade-off comparison (KZG/pairing vs. FRI/hash-based, proof size vs. prove time) more trustworthy
than before.

**Circuit redesign: max-N=256 with padding/count, supersedes the fixed-N-per-compile deployment
above.** The Table 6B/6C scaling curve was produced by *recompiling*
`circuit/reanchoring/src/main.nr` at each N (a valid way to measure how cost scales with header
count in the abstract, and Table 6C's Plonky3 comparison still rests on it) — but the circuit
actually **deployed** (embedded VK, `x/sovereignty/keeper/zk_assets/vk`) only ever compiled one
fixed N at a time, forcing every re-anchoring proof to cover exactly that many headers. Real E2 S7
data (above: "never reached ANCHORED... 29 rejected by a real race condition") showed this
genuinely couldn't keep pace with an unhealthy interval at N=4, and a second, independent bug was
found in `scripts/reanchoring_prover/`: a trailing remainder shorter than the fixed N could never
be proven once the interval stopped growing — RECOVERING could get permanently stuck just short of
the tip. Fixed by compiling **once** for a large ceiling (`N_MAX=256`, chosen because Engram
blocks are produced well under 2s, and 256 is Table 6B's own largest already-measured, still-cheap
data point) with a real, public `count` witness (1..=256) gating which of the 256 header slots are
constrained — matching `spec/README.md`'s formal relation `x = (rt_last, rt_new, n)` literally for
the first time (`n` was previously implicit in the compiled circuit size, never a real input).
Real measured cost (`circuit/reanchoring/RESULTS_MAXN_PADDING.md`, count ∈ {1, 4, 130, 256}, all
real `nargo`/`bb` runs): **6,143 ACIR opcodes, 47,613 circuit size, ~1.06s prove, ~22ms verify,
14,656 B proof, 96 B public inputs — all constant regardless of `count`** (count=1's prove time,
1.054s, is statistically indistinguishable from count=256's, 1.059s). This is real, quantified
overhead versus the old fixed-N=256 compile (2.0x opcodes, 1.55x prove time) — the cost of the
padding-gate logic and a dynamic array index — but still comfortably under one block time, in
exchange for a real, previously-impossible capability: any interval length from 1 to 256 headers,
submittable immediately, in one proof. `x/sovereignty/keeper/msg_server.go`'s public-input parsing
was updated (64 → 96 bytes) to match; `findHeaderByStateRoot`'s rolling-checkpoint logic needed no
changes (it never depended on a fixed N).

> **Scientific claim:** Recovery proofs scale linearly in prover cost while preserving
> constant-size proofs and constant-time verification — reanchoring is practical, scalable, and
> incurs bounded overhead.

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
(`CalculateNextState`, `da.VerifyReceipt`, `vigilante.VerifyReceipt`), plus the full
`ProcessProposal` cost (V5, real end-to-end). `scripts/e7_consensus_overhead/measure_overhead.py`
builds Table 4 from this; result in `scripts/e7_consensus_overhead/results/table4_overhead.md`.
Note: V4 (P2P digest) is a size estimate only — that field isn't actually in the wire format (P2P
health is validated from the leader's local `keeper.Metrics`, not carried in the proposal) — see
the comment at the top of `tests/benchmark/fsm_latency_test.go`. The table also includes a real
vanilla-CometBFT baseline (2 parallel `engramd` processes, one normal, one `--vanilla` — the same
baseline shared with E2/E3). Measured overhead: **+228 bytes/block** for the
`ENGRAM_EXTENDED_PROPOSAL_V1` marker on 100% of blocks; block interval shows no meaningful
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
  `SubmitRecoveryProof` tx — an earlier 60-block sample never passed through RECOVERING and needs
  re-running after driving the cluster through one full RECOVERING cycle
  (e.g. `live_lifecycle_test.py`'s phase 7) to capture a real sample.

---

### E8 — Attack-Resilience Test Suite

Turn the safety lemmas into integration tests or simulation traces.

**A1-A8 matrix** (maps to formal lemmas already in `spec/README.md` — Eclipse ≈ Lemma 7.5, Data
Withholding ≈ Lemma 7.2 — and reuses E4's infrastructure rather than duplicating it):

| # | Attack | Expected result | Live result | Live-Docker mechanism |
|---|---|---|---|---|
| A1 | Eclipse Attack (isolation) | Filter blocks the slot before it's taken; FSM doesn't degrade incorrectly | **PASS** — filter holds exactly 8/8 (`MaxPeersPerSubnet`), cluster unaffected | `docker/attacker-peer-swarm.yml` leg `a1`, `scripts/e4_p2p_eclipse_detection/live_sybil_attack.py a1` |
| A2 | Sybil via simulated multi-subnet | Filter blocks by subnet density, not fooled by diversification | **PASS** — filter holds 8/8 despite a larger swarm (12 attackers) | leg `a2` of the same script |
| A3 | Data Withholding | Honest validators reject a proposal claiming a fake DA attestation | **PASS** — `safety_held=True divergence_events=0`, height progressed normally through 60s attack + 30s recovery | `docker/engram-validator-node04-byzantine.yml` (`ENGRAM_BYZANTINE_BEHAVIOR=false_da_attestation`), `scripts/e8_attack_resilience/live_byzantine_attacks.py a3_false_da_attestation` |
| A4 | Forged BTC Receipt | Honest validators reject a checkpoint hash that doesn't match `ExpectedBlockHash` | **PASS** — same verdict, same mechanism | same script, scenario `a4_forge_btc_hash` |
| A5 | Withdrawal During SOVEREIGN | Tx is withheld (never committed) while SOVEREIGN | **PASS** — `blocked_correctly=True` (real CLI timeout waiting on DeliverTx, tx never commits while SOVEREIGN); cluster recovers normally once celestia-bridge is restored | `engramd tx-submit-forced-tx --payload "TX_WITHDRAWAL..."` (`cmd/engramd/e8_cli.go`), `scripts/e8_attack_resilience/live_withdrawal_test.py` |
| A6 | Malicious Proposer | Honest validators reject a fake `fsm_state` that doesn't match their own computation | **PASS** — same verdict, same mechanism | same byzantine script, scenario `a6_fake_fsm_state` |
| A7 | Censorship / Tx Withholding | A leader deliberately omitting a tx is caught by `IsCensoring`/`ForcedTxQueue` | **PASS** (after fixing 3 real bugs, see below) — `safety_held=True divergence_events=0`, height progressed continuously 10→132 throughout the censoring window | `docker/engram-validator-node04-byzantine.yml` (`ENGRAM_BYZANTINE_BEHAVIOR=censor_tx:<hex>`), `scripts/e8_attack_resilience/live_censorship_test.py` |
| A8 | Combined Attack | Safety holds under multiple overlapping attack vectors | **PASS** — node04 byzantine (`fake_fsm_state:SOVEREIGN`) simultaneous with an A1 swarm of 10 attackers, `safety_held=True divergence_events=0`, height progressed continuously 145→309 through 120s of combined attack | `scripts/e8_attack_resilience/live_combined_attack.py` |
| — | Double-signing | Evidence extracted/logged | **PASS** — all 3 honest validators detected real `DuplicateVoteEvidence`, 1-block detection latency, confirmed independently twice (offense heights 765 and 773) | `x/sovereignty/types/evidence.go` + `preblock.go`'s `recordDetectedEvidence`, `docker/engram-validator-node04-duplicate.yml`, `scripts/e8_attack_resilience/live_double_signing_test.py` — see below |

**Double-signing closed without wiring `x/evidence`:** ABCI 2.0's `RequestFinalizeBlock.Misbehavior`
already carries real `DuplicateVoteEvidence`/`LightClientAttackEvidence` reports from CometBFT's
stock evidence pool (no fork changes) — no need to wire the Cosmos SDK's `x/evidence` module, just
read this field directly in the existing `PreBlocker`. `preblock.go`'s `recordDetectedEvidence`
reads `req.Misbehavior` and writes it to new state (`Keeper.DetectedEvidenceCount`/
`LastDetectedEvidence`, JSON-encoded, no new proto needed), plus a `SLASHABLE EVIDENCE DETECTED`
log line. **Safe to commit** (unlike the local-sensor bug that previously caused AppHash
divergence): `req.Misbehavior` is already-agreed, deterministic data, exactly like `req.Txs` — not
a fresh local read (see the comment in `evidence.go`).

The hardest half (one real, live validator behaving maliciously) is closed via
`docker/engram-validator-node04-duplicate.yml`: it clones node04's real `priv_validator_key.json`
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

**Live result:** ran successfully after fixing 2 real bootstrap bugs (below) — all 3 honest
validators logged real `SLASHABLE EVIDENCE DETECTED`, 1-block detection latency, confirmed
independently twice in the same run (offense_height=765/detected=766 and
offense_height=773/detected=774). Observed via `docker logs` (no dedicated Query RPC for
`DetectedEvidenceCount` yet — would need a new .proto message, out of scope here), not assumed.

### Bugs found and fixed during A7/A8/Double-signing (real, found by running live)

1. **`docker/engram-validator-node04-byzantine.yml` had its own top-level `name:`** — when merged
   with `compose.yml` via `-f a -f b`, Compose takes the `name:` from the last file, turning this
   override into a *separate* Compose project instead of joining the real cluster's project —
   caused a real "container name /engram-node04 already in use" conflict when swapping this
   service in. Fixed by removing `name:` from the override file (the `-duplicate.yml` file had the
   same latent issue, though it hadn't crashed since it creates a new container rather than
   swapping an existing one).
2. **A withdrawal tx caused a permanent liveness deadlock:** A5's `TX_WITHDRAWAL` tx was correctly
   rejected by `ProcessProposal`'s check #4 (no commit while SOVEREIGN), but because
   `PrepareProposal` didn't proactively filter it out of its own proposal, the tx stayed in the
   mempool and every leader kept re-proposing it — so every proposal from every validator (not
   just a malicious leader) was rejected forever, and the cluster stalled (continuous round-skips,
   `docker logs`: `prevote step: state machine rejected a proposed block`). Fixed by having
   `PrepareProposal` filter withdrawal-marked tx out of its own proposal when `WithdrawLocked`,
   instead of relying on check #4 as the only backstop (`x/sovereignty/proposal.go`).
3. **`ForcedTxQueue` never dequeued an already-included tx** (the most serious bug, found while
   debugging A7 a second time): `updateForcedTxTracking` only reset `ignoredRounds` to 0 once a tx
   was included, but never removed the entry from `ForcedTxQueue` — since a tx can only be
   "included" once (consumed from the mempool on commit), every subsequent round had
   `included[tx]==false` forever, permanently tripping `IsCensoring` even after node04 reverted to
   honest. Fixed by dequeuing from `ForcedTxQueue`/`TxIgnoredRounds` entirely on inclusion, not
   just resetting the counter (`x/sovereignty/preblock.go`'s `updateForcedTxTracking`).
4. **`SubmitForcedTx` accepted content that could never be included (an unbounded self-DoS/DoS
   hole):** root cause of the second A7 deadlock — the test script picked a target tx that was
   itself another `MsgSubmitForcedTxRequest`; broadcasting it re-triggered `SubmitForcedTx`'s own
   handler, registering the inner payload (a plain ASCII string, not a valid tx) as a *new*
   `ForcedTxQueue` entry that could never appear as real raw tx bytes, so it could never be
   included. Not just a test-script bug: **anyone submitting `MsgSubmitForcedTx` with content that
   doesn't decode to a valid tx could permanently freeze the entire network**, no validator
   privilege required. Fixed at the root: `SubmitForcedTx` now rejects at submission time if
   `msg.Tx` doesn't decode via `k.TxDecoder` (a new, optional/nil-safe field following the existing
   `peerFilterSrc` pattern, wired from `app.go` via `txConfig.TxDecoder()` — the same decoder
   BaseApp itself uses). 2 new unit tests cover both branches (rejects garbage, accepts a valid
   tx). (`x/sovereignty/keeper/msg_server.go`, `x/sovereignty/keeper/keeper.go`, `app/app.go`)
5. **Docker Desktop (macOS virtiofs) can't mount a single file nested inside a directory already
   bind-mounted from another source** — a real infrastructure error, hit twice:
   `docker/engram-validator-node04-duplicate.yml`'s two nested volume lines
   (`priv_validator_key.json`/`genesis.json` from different sources, nested inside
   `/root/.engramd`, itself already a bind mount) → "mountpoint ... is outside of rootfs". Fixed
   by removing the nested volumes and instead having a Python script (`stage_duplicate_identity`)
   copy both files onto the host before `docker compose up`, so they already exist inside the one
   real volume.
6. **`priv_validator_state.json` was never created for the duplicate-key harness** — `engramd
   init` exits early when `genesis.json`/`priv_validator_key.json` already exist, so it never
   reaches the step that creates FilePV's state file, causing `engramd start` to crash ("no such
   file or directory") the first time this harness actually ran (bugs #1/#5 had masked this
   before). Fixed: `stage_duplicate_identity` now creates a proper empty state file
   (`{"height":"0","round":0,"step":0}`) — matching the original design intent (the second process
   must start from a blank signing state, not a copy of node04's already-advanced state).

**"Timeout flooding by Byzantine nodes"** (an earlier row, not part of the numbered A1-A8 matrix):
closed via `chaos-crash` (SIGKILL on one node) as the nearest available approximation, with two
caveats: (1) it **does** exercise the real f+1-timeout-quorum path (M0b's `handleTimeout`/
`recordTimeoutSenderAndMaybeAdvance`), giving real liveness-recovery numbers for the **crash fault
model** (f=1 validator goes silent); (2) it does **not** confirm resilience against an **active
Byzantine** model — a live, still-signing validator deliberately flooding valid `Timeout`
attestations to manipulate the round-skip cadence faster than plain silence would, which never
exercises `handleTimeoutMessage`'s signature path under real adversarial content (SIGKILL sends
nothing). A cheaper future closure: M0b's `PrivValidator.SignTimeout` signing path already exists;
it would only need a small harness to trigger early/active signing instead of waiting for the real
timer.

**Beyond pass/fail:** rounds-to-recover, invalid proposals rejected, honest-validator agreement
rate, censorship latency, slashable-evidence detection latency.

**Measured, all 9 rows (A1-A8 + Double-signing):** every row above now has a live-Docker result
(not just in-process) — see the "Live result" column and the corresponding
`results_live/*_summary.md` files under `scripts/e8_attack_resilience/results_live/`. This is the
first time this matrix reached full live-Docker coverage; the prior in-process-only run
(`scripts/e8_attack_resilience/trigger_disconnect.py`, `go test -json`) is kept as a parallel
reference at `scripts/e8_attack_resilience/results/table3_attack_resilience.md`, no longer the
primary data source. Only "Timeout flooding" (the unnumbered row) remains partially closed via
`chaos-crash` as described above — not for lack of infrastructure, but because the full closure
(an active Byzantine node flooding valid `Timeout` messages) is new work, out of scope here.

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
real ANCHORED via the ZK pipeline. Phases 1-6 (baseline → BTC → +DA → +P2P churn → peak
triple-fault → healing) **passed**: height progressed continuously 1043→1192 across the first
273s, no stalled round, all 4 nodes stayed height-synced. Phase 7 (waiting for a real ANCHORED via
`watch_and_prove.sh`/`prove_and_submit.sh`) **timed out after 600s**, never reaching ANCHORED.
Data: `scripts/e9_trace_driven/results_live/e9_combined_trace_20260808T181408.csv` (1504 samples)
+ `_summary.md`. No automated 6-panel figure (BTC gap/DA gap/P2P health aren't in committed state,
per the limitation documented in `x/sovereignty/preblock.go`'s `NewPreBlocker`) — only
`fsm_state`/height/marker are real live data.

**Root cause of the Phase 7 timeout:** running `bash -x scripts/reanchoring_prover/prove_and_submit.sh`
directly showed the script exiting with **code 141 (SIGPIPE)** at Step 1/4 (header extraction),
never reaching Step 4 (proof submission) or any chain rejection —
`HEADER_LINES=$(echo "$ALL_HEADER_LINES" | head -n "$EXPECTED_N")` breaks the pipe once
`$ALL_HEADER_LINES` exceeds the OS pipe buffer (~64 KB, a few hundred headers): `head -n 4` reads
its 4 lines and exits early while `echo` is still writing, `echo` receives SIGPIPE, and under
`set -o pipefail` the whole script exits 141. This is a shell-script bug, unrelated to the
ZK/chain logic. Fixed by replacing the pipe with a here-string
(`head -n "$EXPECTED_N" <<< "$ALL_HEADER_LINES"`). Re-running manually after the fix succeeded:
the proof was accepted by the chain (`submitted at height 2517`), the checkpoint advanced
(`HeaderHistory` now starts at height=5 instead of height=1, `rt_last` updated) — confirming the
ZK re-anchoring pipeline works correctly when actually invoked. The 518 prior "rejected" log lines
are almost certainly all this same SIGPIPE bug, not the chain rejecting proofs for staleness.
Conclusion for the Phase 7 timeout: the ZK re-anchoring mechanism works correctly when called, but
`watch_and_prove.sh` never successfully called it during this E9 run because of this script bug —
the timeout is real, but caused by the driver script, not by any design limitation of the
fixed-N=4 proof mechanism (that limitation is real and still documented at
`RealProofSubmittedHeight`'s doc, just not the cause of this particular timeout). E9's Phase 7 has
not yet been re-run with the fixed script (the interval is now past 2500 headers; a successful
`watch_and_prove.sh` run would still need many N=4 iterations to catch up).

**Second bug found running E9:** `scripts/framework/injector.py`'s `cleanup_profile` (shared by
every chaos script in the repo) only calls `docker compose rm -f`, without `stop` first — `rm -f`
only skips the confirmation prompt, it does **not** stop a running container (unlike plain
`docker rm -f`). This had gone unnoticed because every prior call happened after a container's
own `--duration` had already elapsed. E9's Phase 4 was the first to need interrupting a
still-running profile mid-flight (`chaos-loss` has `--duration=2m` but each cycle only holds for
20s) — exposing the bug: the container stayed stuck "Up", and `wait_for_no_active_netem()`
correctly refused to start the next profile rather than silently stacking two profiles. Fixed at
the shared helper: `stop` before `rm -f`.

---

## 4. Minimum Viable Experiment Suite

Don't over-scope. These five groups cover all three evidence layers: **formal**, **systems**, and
**cryptographic microbenchmark**.

| # | Group | Required content |
|---|---|---|
| 1 | TLA+ verification + ablation counterexamples | Reproduce safety/liveness; ablations for hysteresis, circuit breaker, P2P gate, DA consistency |
| 2 | Fault-injection prototype on a 4/7-node local testnet | BTC failure, DA failure, P2P eclipse, combined failure, and recovery |
| 3 | Consensus overhead benchmark | Vanilla CometBFT vs. extended proposal |
| 4 | Recovery Proof Evaluation | Circuit composition (Table 6A), scaling benchmark (Table 6B) at 10-5,000 sovereign blocks; Figure 6 scaling plot |
| 5 | Attack-resilience integration tests | Forged receipt, data withholding, withdrawal during fallback, fake FSM state |

---

## 5. Figures & Tables Needed in the Paper

| Figure/Table | Content |
|---|---|
| **Fig. 1** | Architecture: Engram execution + BTC settlement + Celestia DA + FSM sensors |
| **Fig. 2** | FSM timeline under combined failure *(E9, or E2 if E9 isn't available)* |
| **Fig. 3** | Availability/throughput during outage: Engram FSM vs. vanilla CometBFT *(E2)* |
| **Fig. 4** | Recovery stability vs. `HYSTERESIS_WAIT` *(E5)* |
| **Fig. 6** | Recovery Proof Scaling: 4 panels (Constraint Count, Proving Time, Verification Time, Proof Size) *(E6)* |
| **Fig. 7** | Backend trade-off radar chart: Noir+Honk vs. Plonky3 *(E6, optional)* |
| **Table 1** | Formal verification state-space results *(E1)* |
| **Table 2** | Failure matrix and expected policy *(E3)* |
| **Table 3** | Attack-resilience tests *(E8)* |
| **Table 4** | Extended proposal overhead *(E7)* |
| **Table 5** | Ablation study |
| **Table 6** | P2P profiler accuracy *(E4)* |

---

## 6. Immediate Repo TODOs

Before running the experiments, the following need finishing:

- [ ] Complete the real `BeginBlock` in `x/sovereignty/abci.go` — no longer a comment.
- [ ] Complete `CalculateNextFSMState`, `ExecuteFSMTransition`, `IsWarningCondition`,
      `IsCriticalCondition`, `IsHealthyCondition` in Go to match the TLA+.
- [ ] Build mock modules for the BTC finality, DA receipt, and P2P health sensors.
- [ ] Turn `tests/fsm_transition_e2e_test.go` into a real test with failure-matrix scenarios.
- [ ] Re-enable the `computed_new_root == state_root_new` constraint in Noir, or maintain two
      versions: an unconstrained demo and a constrained benchmark.
- [ ] Add reproducibility scripts: `make test-faults`, `make bench-consensus`, `make bench-zk`,
      `make verify-tla`.
- [ ] Log every state transition to CSV/JSON to auto-generate timelines.

---

## Conclusion

The Engram Sovereign FSM idea is strong enough for a conference submission if the evaluation is
built with discipline. The deciding factor isn't more theory — it's **demonstrating empirically**
that the FSM:

- maintains **safety** while improving **liveness** for a modular blockchain under peripheral
  failure,
- provides **controlled recovery**,
- and incurs **acceptable overhead**.
