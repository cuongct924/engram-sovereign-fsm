BINARY_NAME=engramd
BUILD_DIR=./build

# Version from git tag, falling back to a commit hash if none exists.
VERSION=$(shell git describe --tags --always)

.PHONY: all build build-linux clean test lint proto-gen docker-build help \
	e1-guard e1-fsm-safety e1-fsm-liveness e1-tendermint-safety \
	e1-server-safety e1-server-liveness \
	testnet-up testnet-down testnet-status \
	byzantine-on byzantine-off double-sign-on double-sign-off \
	timeout-flood-on timeout-flood-off node04-honest \
	chaos-delay chaos-loss chaos-crash chaos-eclipse chaos-btc-delay chaos-stop \
	chaos-wan-latency chaos-wan-loss chaos-wan-stop \
	attacker-a1-up attacker-a1-down attacker-a2-up attacker-a2-down \
	prover-watch \
	e2 e2-figures e2-timeline \
	e3-lifecycle e3-p2p-lifecycle e3-recovery e3-latency \
	e4-sybil e4-churn e4-relay-latency \
	e5-spot-check e5-absorb-sweep \
	e6-prover-bench e6-plonky3-bench e6-stats e6-table6c \
	e7-overhead-scan e7-throughput e7-sensor-mismatch e7-vanilla-comparison \
	e8-byzantine e8-censorship e8-combined e8-double-signing e8-timeout-flood e8-withdrawal \
	e9-trace e9-figures \
	e10-reorg e10-figures

ENV_FILE := .env
COMPOSE_FILE := docker-compose.yml
# N=4 (just node01-04) or N=7 (the full cluster, default) selects
# `testnet-up`'s validator-count topology -- see docker/engram-
# validator-cluster.yml's "seven-node" profile comment for why node05-07
# and their celestia bridges are Compose-profile-gated rather than always
# started. Genesis (`testnet init-files --v N`) is generated to match.
N ?= 7
# PROVER=container (default) builds/runs docker/reanchoring-prover.yml's
# service; PROVER=host launches scripts/reanchoring_prover/watch_and_prove.sh
# in the background instead, logging to /tmp/watch_and_prove.log. The
# container's own circuit-compile happens at `docker build` time, but real
# N=256 proving still needs more RAM than reliably fits in Docker Desktop's
# VM alongside the live cluster (see docker/reanchoring-prover.yml) -- host
# mode shares the host's full RAM directly and has never OOM'd this session.
PROVER ?= container
ifeq ($(N),7)
	VALIDATOR_SERVICES_EXTRA := engram-node05 engram-node06 engram-node07
	BRIDGE_SERVICES_EXTRA := celestia-bridge-node05 celestia-bridge-node06 celestia-bridge-node07
	COMPOSE_PROFILE_FLAG := --profile seven-node
else
	VALIDATOR_SERVICES_EXTRA :=
	BRIDGE_SERVICES_EXTRA :=
	COMPOSE_PROFILE_FLAG :=
endif
# Stop/rm targets always name the full possible set regardless of which N a
# given testnet-up used -- stopping an already-absent service is a no-op,
# not an error, so this stays correct either way without tracking N here.
CORE_SERVICES := bitcoin-node01 bitcoin-node02 bitcoin-miner-loop celestia-app celestia-bridge-node01 celestia-bridge-prover \
	celestia-bridge-node02 celestia-bridge-node03 celestia-bridge-node04 celestia-bridge-node05 celestia-bridge-node06 celestia-bridge-node07 \
	engram-node01 engram-node02 engram-node03 engram-node04 engram-node05 engram-node06 engram-node07 reanchoring-prover
WAN_LATENCY_SERVICES := pumba-wan-latency-01 pumba-wan-latency-02 pumba-wan-latency-03 pumba-wan-latency-04 \
	pumba-wan-latency-05 pumba-wan-latency-06 pumba-wan-latency-07
WAN_LOSS_SERVICES := pumba-wan-loss-01 pumba-wan-loss-02 pumba-wan-loss-03 pumba-wan-loss-04 \
	pumba-wan-loss-05 pumba-wan-loss-06 pumba-wan-loss-07
ATTACKER_A1_SERVICES := $(shell printf 'attacker-a1-%02d ' $$(seq 1 10))
ATTACKER_A2_SERVICES := attacker-a2-a1 attacker-a2-a2 attacker-a2-a3 \
	attacker-a2-b1 attacker-a2-b2 attacker-a2-b3 \
	attacker-a2-c1 attacker-a2-c2 attacker-a2-c3 \
	attacker-a2-d1 attacker-a2-d2 attacker-a2-d3

all: build

build:
	@echo "--> Building engramd..."
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/engramd

build-linux:
	@echo "--> Building engramd for Linux..."
	GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-linux ./cmd/engramd

# -race catches proto-generated structs (e.g. PeripheralMetrics, embedding
# sync.Mutex via protoimpl.MessageState) copied by value.
test:
	@echo "--> Running tests..."
	go test -v -race ./...

lint:
	@echo "--> Running golangci-lint..."
	golangci-lint run

# buf.gen.yaml generates into .tmp-proto-gen/ (mirroring the proto package
# path) since the proto package (engram.sovereignty.v1) and the Go package
# (x/sovereignty/types) intentionally differ -- this recipe copies the
# generated files into place and removes the staging directory.
proto-gen:
	@echo "--> Generating protobuf code..."
	rm -rf .tmp-proto-gen
	buf generate
	cp .tmp-proto-gen/engram/sovereignty/v1/*.go x/sovereignty/types/
	rm -rf .tmp-proto-gen

zk-compile:
	@echo "--> Compiling ZK circuits..."
	cd circuit/reanchoring && nargo compile

docker-build:
	@echo "--> Building docker image..."
	docker build -t engram/node:$(VERSION) .

# ==============================================================================
# E1 FORMAL VERIFICATION (TLA+/TLC, spec/core)
# ==============================================================================
# Requires Java JDK 11+ and TLA_JAR pointing at tla2tools.jar (TLA+ releases
# page: https://github.com/tlaplus/tlaplus/releases). Runnable from the repo
# root -- TLC resolves EXTENDS relative to the main .tla file's own
# directory, not the shell's cwd, so the spec/-prefixed paths below just
# work (verified directly, see README.md's E1 section).
#
# Apalache's complementary bounded-safety checks (separate toolchain, JDK
# 17+, per-layer --cinit/--init/--next flags, used for the E1 ablations) are
# not wrapped here -- see spec/README.md's "Apalache" section.

e1-guard:
	@test -n "$(TLA_JAR)" || (echo "Usage: make <e1-target> TLA_JAR=/path/to/tla2tools.jar [WORKERS=8]" && exit 1)

e1-fsm-safety: e1-guard
	java -cp $(TLA_JAR) tlc2.TLC -workers $(or $(WORKERS),8) -config spec/core/MC_FSMSafety.cfg spec/core/MC_FSMSafety.tla

e1-fsm-liveness: e1-guard
	java -cp $(TLA_JAR) tlc2.TLC -workers $(or $(WORKERS),8) -config spec/core/MC_FSMLiveness.cfg spec/core/MC_FSMLiveness.tla

e1-tendermint-safety: e1-guard
	java -cp $(TLA_JAR) tlc2.TLC -workers $(or $(WORKERS),8) -config spec/core/MC_TendermintSafety.cfg spec/core/MC_TendermintSafety.tla

e1-server-safety: e1-guard
	java -cp $(TLA_JAR) tlc2.TLC -workers $(or $(WORKERS),8) -config spec/core/MC_ServerRefinementSafety.cfg spec/core/MC_ServerRefinementSafety.tla

e1-server-liveness: e1-guard
	java -cp $(TLA_JAR) tlc2.TLC -workers $(or $(WORKERS),8) -config spec/core/MC_ServerRefinementLiveness.cfg spec/core/MC_ServerRefinementLiveness.tla

# ==============================================================================
# LOCAL DOCKER TESTNET CONTROL
# ==============================================================================
# Wraps docs/DEVELOPMENT.md §3's manual walkthrough. Requires $(ENV_FILE)
# (cp .env.example .env, fill in BITCOIN_RPC_USER/BITCOIN_RPC_PASSWORD first).

# N=4 or N=7 (default), e.g. `make testnet-up N=4` -- see the N variable's
# own comment above CORE_SERVICES for what each brings up.
testnet-up: build
	@test -f $(ENV_FILE) || (echo "Missing $(ENV_FILE) -- cp .env.example $(ENV_FILE) and fill in BITCOIN_RPC_USER/BITCOIN_RPC_PASSWORD first" && exit 1)
	@echo "--> Wiping testnet-data/ and regenerating a $(N)-validator genesis (a stale priv_validator_state.json blocks restarts)"
	rm -rf testnet-data/
	set -a && . $(ENV_FILE) && set +a && ./$(BUILD_DIR)/$(BINARY_NAME) testnet init-files --v $(N)
	@echo "--> Starting Bitcoin + Celestia (must be funded/mature BEFORE any engramd container)"
	docker compose --env-file $(ENV_FILE) -f $(COMPOSE_FILE) up -d bitcoin-node01 bitcoin-node02
	docker compose --env-file $(ENV_FILE) -f $(COMPOSE_FILE) $(COMPOSE_PROFILE_FLAG) up -d celestia-app celestia-bridge-node01 celestia-bridge-prover celestia-bridge-node02 celestia-bridge-node03 celestia-bridge-node04 $(BRIDGE_SERVICES_EXTRA)
	set -a && . $(ENV_FILE) && set +a && ./scripts/testnet_fund_wallet.sh
	@echo "--> Starting the containerized miner loop (engramwallet must already be funded above)"
	docker compose --env-file $(ENV_FILE) -f $(COMPOSE_FILE) up -d bitcoin-miner-loop
	@echo "--> Fetching each validator's own celestia bridge admin JWT into $(ENV_FILE) (docker/engram-validator-cluster.yml: node01 uses celestia-bridge-node01, node02-0$(N) each get their own dedicated bridge)"
	./scripts/testnet_fetch_celestia_token.sh $(ENV_FILE) celestia-bridge-node01 CELESTIA_BRIDGE_NODE01_AUTH_TOKEN
	./scripts/testnet_fetch_celestia_token.sh $(ENV_FILE) celestia-bridge-prover CELESTIA_BRIDGE_PROVER_AUTH_TOKEN
	./scripts/testnet_fetch_celestia_token.sh $(ENV_FILE) celestia-bridge-node02 CELESTIA_BRIDGE_NODE02_AUTH_TOKEN
	./scripts/testnet_fetch_celestia_token.sh $(ENV_FILE) celestia-bridge-node03 CELESTIA_BRIDGE_NODE03_AUTH_TOKEN
	./scripts/testnet_fetch_celestia_token.sh $(ENV_FILE) celestia-bridge-node04 CELESTIA_BRIDGE_NODE04_AUTH_TOKEN
ifeq ($(N),7)
	./scripts/testnet_fetch_celestia_token.sh $(ENV_FILE) celestia-bridge-node05 CELESTIA_BRIDGE_NODE05_AUTH_TOKEN
	./scripts/testnet_fetch_celestia_token.sh $(ENV_FILE) celestia-bridge-node06 CELESTIA_BRIDGE_NODE06_AUTH_TOKEN
	./scripts/testnet_fetch_celestia_token.sh $(ENV_FILE) celestia-bridge-node07 CELESTIA_BRIDGE_NODE07_AUTH_TOKEN
endif
	@echo "--> Waiting for every validator's own celestia bridge to actually serve DA requests (its auth-token file lands well before that -- starting validators too early reliably trips MaxSuspiciousTime and forces an unwanted SOVEREIGN escalation on every fresh deploy)"
	./scripts/testnet_wait_healthy.sh celestia-bridge-node01
	./scripts/testnet_wait_healthy.sh celestia-bridge-node02
	./scripts/testnet_wait_healthy.sh celestia-bridge-node03
	./scripts/testnet_wait_healthy.sh celestia-bridge-node04
ifeq ($(N),7)
	./scripts/testnet_wait_healthy.sh celestia-bridge-node05
	./scripts/testnet_wait_healthy.sh celestia-bridge-node06
	./scripts/testnet_wait_healthy.sh celestia-bridge-node07
endif
	@echo "--> Starting the $(N) validators"
	docker compose --env-file $(ENV_FILE) -f $(COMPOSE_FILE) $(COMPOSE_PROFILE_FLAG) up -d --build engram-node01 engram-node02 engram-node03 engram-node04 $(VALIDATOR_SERVICES_EXTRA)
ifeq ($(PROVER),host)
	@echo "--> Starting the ZK re-anchoring prover host-side (PROVER=host) -- log: /tmp/watch_and_prove.log"
	set -a && . $(ENV_FILE) && set +a && \
		NODE_URL=http://127.0.0.1:26657 MAX_CHECKS=999999 \
		nohup ./scripts/reanchoring_prover/watch_and_prove.sh > /tmp/watch_and_prove.log 2>&1 &
else
	@echo "--> Starting the ZK re-anchoring prover as a container (PROVER=container; needed for RECOVERING -> ANCHORED via a real submitted proof; the heuristic path alone is enough to reach a baseline ANCHORED)"
	docker compose --env-file $(ENV_FILE) -f $(COMPOSE_FILE) up -d --build reanchoring-prover
endif
	@echo "--> Up. Verify: curl -s http://localhost:26657/status | jq .result.sync_info"

testnet-down:
	@echo "--> Stopping core services by name (never a bare 'docker compose down' -- see docs/DEVELOPMENT.md's gotcha)"
	docker compose --env-file $(ENV_FILE) -f $(COMPOSE_FILE) --profile seven-node stop $(CORE_SERVICES)
	docker compose --env-file $(ENV_FILE) -f $(COMPOSE_FILE) --profile seven-node rm -f $(CORE_SERVICES)

testnet-status:
	docker compose ps

# --- Byzantine mode (docs/EXPERIMENT.md's E8 A3/A4/A6/A7) --------------------
# BEHAVIOR: fake_fsm_state:<STATE> | forge_btc_hash | false_da_attestation | censor_tx:<hex>
# (recognized values: x/sovereignty/proposal.go's applyByzantineBehavior)
byzantine-on:
	@test -n "$(BEHAVIOR)" || (echo "Usage: make byzantine-on BEHAVIOR=fake_fsm_state:SOVEREIGN|forge_btc_hash|false_da_attestation|censor_tx:<hex>" && exit 1)
	ENGRAM_BYZANTINE_BEHAVIOR=$(BEHAVIOR) docker compose --env-file $(ENV_FILE) -f $(COMPOSE_FILE) -f docker/engram-node04-byzantine.yml up -d --no-deps engram-node04
	@echo "--> node04 is byzantine: $(BEHAVIOR) -- never leave this set on a real validator"

byzantine-off: node04-honest

# --- Double-signing harness (docs/EXPERIMENT.md's E8 Double-signing) --------
double-sign-on:
	./scripts/testnet_double_sign_toggle.sh on

double-sign-off:
	./scripts/testnet_double_sign_toggle.sh off

# --- Timeout-flooding harness (docs/EXPERIMENT.md's E8 "Timeout flooding") --
# INTERVAL_MS: how often node04 re-broadcasts a signed Timeout (default 50ms,
# far faster than TIMEOUT_DURATION's real precommit-wait timer).
timeout-flood-on:
	ENGRAM_TIMEOUT_FLOOD_INTERVAL_MS=$(or $(INTERVAL_MS),50) docker compose --env-file $(ENV_FILE) -f $(COMPOSE_FILE) -f docker/engram-node04-timeout-flood.yml up -d --no-deps engram-node04
	@echo "--> node04 is flooding Timeout messages every $(or $(INTERVAL_MS),50)ms -- never leave this set on a real validator"

timeout-flood-off: node04-honest

# Shared by every harness that swaps node04's compose file (byzantine,
# timeout-flood) -- reverting is the same "recreate from plain
# docker-compose.yml" regardless of which override put it into a non-honest
# state.
node04-honest:
	docker compose --env-file $(ENV_FILE) -f $(COMPOSE_FILE) up -d --no-deps engram-node04
	@echo "--> node04 reverted to honest"

# --- Chaos profiles (Pumba, docs/DEVELOPMENT.md §5) --------------------------
# Self-exit on their own --duration; chaos-stop is only needed to interrupt
# one early (see scripts/framework/injector.py's cleanup_profile doc).
chaos-delay chaos-loss chaos-crash chaos-eclipse chaos-btc-delay:
	python3 scripts/framework/injector.py start $@

chaos-stop:
	@for p in chaos-delay chaos-loss chaos-crash chaos-eclipse chaos-btc-delay; do \
		python3 scripts/framework/injector.py stop $$p || true; \
	done

# --- WAN realism profiles (per-node distinct latency/loss, docker-compose.yml) --
# One pumba-wan-* container per validator; scripts/framework/injector.py
# has its own Python-side start/cleanup for these (used by live E5/E9), the
# targets below are for manual `make chaos-wan-*` use.
#
# ALWAYS pass explicit $(WAN_*_SERVICES) to stop/rm -- a bare
# `--profile X stop` with no service list stops the ENTIRE cluster, not
# just profile X's containers. Hit this for real once.
chaos-wan-latency:
	docker compose --env-file $(ENV_FILE) --profile chaos-wan-latency up -d $(WAN_LATENCY_SERVICES)

chaos-wan-loss:
	docker compose --env-file $(ENV_FILE) --profile chaos-wan-loss up -d $(WAN_LOSS_SERVICES)

chaos-wan-stop:
	docker compose --env-file $(ENV_FILE) --profile chaos-wan-latency --profile chaos-wan-loss stop $(WAN_LATENCY_SERVICES) $(WAN_LOSS_SERVICES)
	docker compose --env-file $(ENV_FILE) --profile chaos-wan-latency --profile chaos-wan-loss rm -f $(WAN_LATENCY_SERVICES) $(WAN_LOSS_SERVICES)

# --- Attacker swarm (E4/E8's A1/A2, docker/attacker-peer-swarm.yml) ---------
attacker-a1-up:
	docker compose --env-file $(ENV_FILE) --profile attacker-swarm-a1 up -d $(ATTACKER_A1_SERVICES)

attacker-a1-down:
	docker compose --env-file $(ENV_FILE) --profile attacker-swarm-a1 stop $(ATTACKER_A1_SERVICES)
	docker compose --env-file $(ENV_FILE) --profile attacker-swarm-a1 rm -f $(ATTACKER_A1_SERVICES)

attacker-a2-up:
	docker compose --env-file $(ENV_FILE) --profile attacker-swarm-a2 up -d $(ATTACKER_A2_SERVICES)

attacker-a2-down:
	docker compose --env-file $(ENV_FILE) --profile attacker-swarm-a2 stop $(ATTACKER_A2_SERVICES)
	docker compose --env-file $(ENV_FILE) --profile attacker-swarm-a2 rm -f $(ATTACKER_A2_SERVICES)

# ==============================================================================
# LIVE EXPERIMENTS (E2-E10)
# ==============================================================================
# Methodology + real-vs-synthetic status: docs/EXPERIMENT.md. Full command
# reference (incl. which need `prover-watch` running): README.md.
# All of these expect `make testnet-up` already running.

prover-watch:
	./scripts/reanchoring_prover/watch_and_prove.sh

e2:
	python3 -u scripts/e2_fault_injection/live_scenario_matrix.py

e2-figures:
	python3 scripts/e2_fault_injection/live_figure_builder.py

e2-timeline:
	python3 scripts/e2_fault_injection/live_single_timeline.py

e3-lifecycle:
	python3 scripts/e3_failure_matrix/live_lifecycle_test.py

e3-p2p-lifecycle:
	python3 scripts/e3_failure_matrix/live_p2p_lifecycle_test.py

e3-recovery:
	python3 scripts/e3_failure_matrix/live_recovery_flow.py --minutes $(or $(MINUTES),20)

e3-latency:
	python3 scripts/e3_failure_matrix/measure_latency_live.py

e4-sybil:
	@test -n "$(LEG)" || (echo "Usage: make e4-sybil LEG=a1|a2" && exit 1)
	python3 -u scripts/e4_p2p_eclipse_detection/live_sybil_attack.py $(LEG)

e4-churn:
	python3 -u scripts/e4_p2p_eclipse_detection/live_churn_attack.py

e4-relay-latency:
	python3 -u scripts/e4_p2p_eclipse_detection/live_relay_latency_attack.py

e5-spot-check:
	@test -n "$(HYSTERESIS_WAIT)" && test -n "$(ENV)" || (echo "Usage: make e5-spot-check HYSTERESIS_WAIT=... ENV=stable|noisy_da -- set ENGRAM_PARAM_HYSTERESIS_WAIT and redeploy first, see README.md" && exit 1)
	python3 -u scripts/e5_hysteresis_flapping/live_spot_check.py --hysteresis-wait $(HYSTERESIS_WAIT) --env $(ENV)

e5-absorb-sweep:
	python3 -u scripts/e5_hysteresis_flapping/run_absorb_sweep.py

e6-prover-bench:
	./scripts/e6_zk_reanchoring_benchmark/benchmark_prover.sh

e6-plonky3-bench:
	./scripts/e6_zk_reanchoring_benchmark/benchmark_plonky3.sh

e6-stats:
	python3 scripts/e6_zk_reanchoring_benchmark/stats_collector.py

e6-table6c:
	python3 scripts/e6_zk_reanchoring_benchmark/table6c_collector.py

e7-overhead-scan:
	python3 scripts/e7_consensus_overhead/live_overhead_scan.py

e7-throughput:
	python3 -u scripts/e7_consensus_overhead/live_throughput_latency.py

e7-sensor-mismatch:
	python3 -u scripts/e7_consensus_overhead/live_sensor_mismatch.py

e7-vanilla-comparison:
	./scripts/e7_consensus_overhead/vanilla_comparison.sh

e8-byzantine:
	@test -n "$(ATTACK)" || (echo "Usage: make e8-byzantine ATTACK=a6_fake_fsm_state|a4_forge_btc_hash|a3_false_da_attestation|--all" && exit 1)
	python3 -u scripts/e8_attack_resilience/live_byzantine_attacks.py $(ATTACK)

e8-censorship:
	python3 -u scripts/e8_attack_resilience/live_censorship_test.py

e8-combined:
	python3 -u scripts/e8_attack_resilience/live_combined_attack.py

e8-double-signing:
	python3 -u scripts/e8_attack_resilience/live_double_signing_test.py

e8-timeout-flood:
	python3 -u scripts/e8_attack_resilience/live_timeout_flood_test.py

e8-withdrawal:
	python3 -u scripts/e8_attack_resilience/live_withdrawal_test.py

e9-trace:
	python3 -u scripts/e9_trace_driven/live_combined_trace.py

e9-figures:
	python3 scripts/e9_trace_driven/live_figure_builder.py

e10-reorg:
	@test -n "$(DEPTH)" || (echo "Usage: make e10-reorg DEPTH=shallow|deep [REORG_DEPTH=N]" && exit 1)
	python3 -u scripts/e10_bitcoin_reorg/live_reorg_test.py --depth $(DEPTH) $(if $(REORG_DEPTH),--reorg-depth $(REORG_DEPTH))

e10-figures:
	python3 scripts/e10_bitcoin_reorg/live_figure_builder.py

clean:
	rm -rf $(BUILD_DIR)

help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build         - Build the node binary (engramd)"
	@echo "  test          - Run all tests"
	@echo "  proto-gen     - Generate protobuf code"
	@echo "  zk-compile    - Compile the Noir circuits"
	@echo "  lint          - Run golangci-lint"
	@echo "  docker-build  - Build the docker image"
	@echo ""
	@echo "E1 formal verification (TLA+/TLC, requires TLA_JAR=/path/to/tla2tools.jar):"
	@echo "  e1-fsm-safety / e1-fsm-liveness           - FSM layer"
	@echo "  e1-tendermint-safety                      - Tendermint layer"
	@echo "  e1-server-safety / e1-server-liveness     - Full refinement bridge (all layers)"
	@echo ""
	@echo "Local docker testnet:"
	@echo "  testnet-up [N=4|7] [PROVER=container|host]  - Fresh genesis + bring up bitcoin/celestia/N validators (default N=7, PROVER=container)"
	@echo "                        (honors ENGRAM_PARAM_* in .env -- see .env.example)"
	@echo "  testnet-down        - Stop the miner loop + core services (not a bare down)"
	@echo "  testnet-status      - docker compose ps"
	@echo "  byzantine-on BEHAVIOR=... - Swap node04 to a byzantine build (E8 A3/A4/A6/A7)"
	@echo "  byzantine-off       - Revert node04 to honest"
	@echo "  double-sign-on      - Start the duplicate-key double-signing harness (E8)"
	@echo "  double-sign-off     - Stop it"
	@echo "  timeout-flood-on INTERVAL_MS=... - node04 floods signed Timeout msgs (E8)"
	@echo "  timeout-flood-off  - Revert node04 to honest"
	@echo "  chaos-delay/-loss/-crash/-eclipse/-btc-delay - Start a Pumba fault profile"
	@echo "  chaos-stop          - Stop all Pumba fault profiles"
	@echo "  attacker-a1-up/-down - Peer-slot-exhaustion swarm (E4/E8 A1)"
	@echo "  attacker-a2-up/-down - Simulated multi-subnet Sybil swarm (E4/E8 A2)"
	@echo ""
	@echo "Live experiments (E2-E10, see README.md for the full reference):"
	@echo "  prover-watch        - Run the ZK re-anchoring prover loop (needed for RECOVERING -> ANCHORED legs)"
	@echo "  e2 / e2-figures / e2-timeline       - Fault-injection scenario matrix (S1-S7)"
	@echo "  e3-lifecycle / e3-p2p-lifecycle / e3-recovery / e3-latency - Full-lifecycle failure matrix"
	@echo "  e4-sybil LEG=a1|a2 / e4-churn / e4-relay-latency - P2P eclipse-detection attacks"
	@echo "  e5-spot-check HYSTERESIS_WAIT=... ENV=... / e5-absorb-sweep - Hysteresis flapping"
	@echo "  e6-prover-bench / e6-plonky3-bench / e6-stats / e6-table6c - ZK re-anchoring benchmark"
	@echo "  e7-overhead-scan / e7-throughput / e7-sensor-mismatch / e7-vanilla-comparison - Consensus overhead"
	@echo "  e8-byzantine ATTACK=... / e8-censorship / e8-combined / e8-double-signing / e8-timeout-flood / e8-withdrawal - Attack resilience"
	@echo "  e9-trace / e9-figures  - Trace-driven combined-failure capstone"
	@echo "  e10-reorg DEPTH=shallow|deep / e10-figures - Bitcoin reorg"
