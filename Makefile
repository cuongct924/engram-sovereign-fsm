BINARY_NAME=engramd
BUILD_DIR=./build

# Version from git tag, falling back to a commit hash if none exists.
VERSION=$(shell git describe --tags --always)

.PHONY: all build build-linux clean test lint proto-gen docker-build help \
	testnet-up testnet-down testnet-status \
	byzantine-on byzantine-off double-sign-on double-sign-off \
	timeout-flood-on timeout-flood-off \
	chaos-delay chaos-loss chaos-crash chaos-eclipse chaos-btc-delay chaos-stop \
	chaos-wan-latency chaos-wan-loss chaos-wan-stop \
	attacker-a1-up attacker-a1-down attacker-a2-up attacker-a2-down

ENV_FILE := .env
MINER_PID_FILE := .bitcoin_miner_loop.pid
CORE_SERVICES := bitcoin-node01 bitcoin-node02 celestia-app celestia-bridge \
	engram-node01 engram-node02 engram-node03 engram-node04
WAN_LATENCY_SERVICES := pumba-wan-latency-01 pumba-wan-latency-02 pumba-wan-latency-03 pumba-wan-latency-04
WAN_LOSS_SERVICES := pumba-wan-loss-01 pumba-wan-loss-02 pumba-wan-loss-03 pumba-wan-loss-04
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
# LOCAL DOCKER TESTNET CONTROL
# ==============================================================================
# Wraps docs/DEVELOPMENT.md §3's manual walkthrough. Requires $(ENV_FILE)
# (cp .env.example .env, fill in BITCOIN_RPC_USER/BITCOIN_RPC_PASSWORD first).

testnet-up: build
	@test -f $(ENV_FILE) || (echo "Missing $(ENV_FILE) -- cp .env.example $(ENV_FILE) and fill in BITCOIN_RPC_USER/BITCOIN_RPC_PASSWORD first" && exit 1)
	@echo "--> Wiping testnet-data/ and regenerating genesis (a stale priv_validator_state.json blocks restarts)"
	rm -rf testnet-data/
	set -a && . $(ENV_FILE) && set +a && ./$(BUILD_DIR)/$(BINARY_NAME) testnet init-files --v 4
	@echo "--> Starting Bitcoin + Celestia (must be funded/mature BEFORE any engramd container)"
	docker compose --env-file $(ENV_FILE) -f compose.yml up -d bitcoin-node01 bitcoin-node02
	docker compose --env-file $(ENV_FILE) -f compose.yml up -d celestia-app celestia-bridge
	set -a && . $(ENV_FILE) && set +a && ./scripts/testnet_fund_wallet.sh
	@if [ -f $(MINER_PID_FILE) ] && kill -0 "$$(cat $(MINER_PID_FILE))" 2>/dev/null; then \
		echo "--> Miner loop already running (pid $$(cat $(MINER_PID_FILE)))"; \
	else \
		echo "--> Starting steady miner loop (never manual bursts once engramd is up)"; \
		( set -a && . $(ENV_FILE) && set +a && nohup ./scripts/bitcoin_miner_loop.sh > .bitcoin_miner_loop.log 2>&1 & echo $$! > $(MINER_PID_FILE) ); \
	fi
	@echo "--> Fetching celestia-bridge admin JWT into $(ENV_FILE)"
	./scripts/testnet_fetch_celestia_token.sh $(ENV_FILE)
	@echo "--> Starting the 4 validators"
	docker compose --env-file $(ENV_FILE) -f compose.yml up -d --build engram-node01 engram-node02 engram-node03 engram-node04
	@echo "--> Up. Verify: curl -s http://localhost:26657/status | jq .result.sync_info"

testnet-down:
	@echo "--> Stopping miner loop"
	@if [ -f $(MINER_PID_FILE) ]; then kill "$$(cat $(MINER_PID_FILE))" 2>/dev/null || true; rm -f $(MINER_PID_FILE); fi
	@echo "--> Stopping core services by name (never a bare 'docker compose down' -- see docs/DEVELOPMENT.md's gotcha)"
	docker compose --env-file $(ENV_FILE) stop $(CORE_SERVICES)
	docker compose --env-file $(ENV_FILE) rm -f $(CORE_SERVICES)

testnet-status:
	docker compose ps

# --- Byzantine mode (docs/EXPERIMENT.md's E8 A3/A4/A6/A7) --------------------
# BEHAVIOR: fake_fsm_state:<STATE> | forge_btc_hash | false_da_attestation | censor_tx:<hex>
# (recognized values: x/sovereignty/proposal.go's applyByzantineBehavior)
byzantine-on:
	@test -n "$(BEHAVIOR)" || (echo "Usage: make byzantine-on BEHAVIOR=fake_fsm_state:SOVEREIGN|forge_btc_hash|false_da_attestation|censor_tx:<hex>" && exit 1)
	ENGRAM_BYZANTINE_BEHAVIOR=$(BEHAVIOR) docker compose --env-file $(ENV_FILE) -f compose.yml -f docker/engram-node04-byzantine.yml up -d --no-deps engram-node04
	@echo "--> node04 is byzantine: $(BEHAVIOR) -- never leave this set on a real validator"

byzantine-off:
	docker compose --env-file $(ENV_FILE) -f compose.yml up -d --no-deps engram-node04
	@echo "--> node04 reverted to honest"

# --- Double-signing harness (docs/EXPERIMENT.md's E8 Double-signing) --------
double-sign-on:
	./scripts/testnet_double_sign_toggle.sh on

double-sign-off:
	./scripts/testnet_double_sign_toggle.sh off

# --- Timeout-flooding harness (docs/EXPERIMENT.md's E8 "Timeout flooding") --
# INTERVAL_MS: how often node04 re-broadcasts a signed Timeout (default 50ms,
# far faster than TIMEOUT_DURATION's real precommit-wait timer).
timeout-flood-on:
	ENGRAM_TIMEOUT_FLOOD_INTERVAL_MS=$(or $(INTERVAL_MS),50) docker compose --env-file $(ENV_FILE) -f compose.yml -f docker/engram-node04-timeout-flood.yml up -d --no-deps engram-node04
	@echo "--> node04 is flooding Timeout messages every $(or $(INTERVAL_MS),50)ms -- never leave this set on a real validator"

timeout-flood-off:
	docker compose --env-file $(ENV_FILE) -f compose.yml up -d --no-deps engram-node04
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

# --- WAN realism profiles (per-node distinct latency/loss, compose.yml) -----
# Multi-service profiles (one pumba-wan-* per validator) -- bypass
# injector.py's one-service-per-profile framework, same reasoning as the
# attacker swarm targets above: brought up/down directly via `docker
# compose --profile`, not measured/orchestrated by scripts/framework.
#
# ALWAYS pass explicit $(WAN_*_SERVICES) to stop/rm, never a bare
# `--profile X stop` with no service list -- without an explicit service
# list, `docker compose --profile X stop/rm` treats every service in the
# whole project as in scope (not just profile X's own services) and
# stops/removes the entire cluster, not just the chaos containers. Hit this
# for real once already; the explicit lists below are the fix, not a
# stylistic choice.
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
	@echo "Local docker testnet:"
	@echo "  testnet-up          - Fresh genesis + bring up bitcoin/celestia/4 validators"
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
