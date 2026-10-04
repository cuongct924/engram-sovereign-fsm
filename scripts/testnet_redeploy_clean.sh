#!/usr/bin/env bash
# Clean redeploy of the N-validator testnet: removes every cluster container, wipes testnet-data/ and data/ (a genesis
# against a far-advanced Bitcoin regtest starts with btc_gap in the hundreds and sits in SOVEREIGN for good), starts
# bitcoind and celestia-app first and waits for celestia-app (a bridge started in the same instant loses its
# one-time TrustedHash race and crash-loops), then runs `make testnet-up`. testnet-up's own sensor-health wait is
# allowed to fail (proof-only start-up legitimately does); the caller waits for ANCHORED.
#
#   scripts/testnet_redeploy_clean.sh [MODE]      MODE = engram (default) | vanilla
set -uo pipefail
cd "$(dirname "$0")/.."
MODE="${1:-engram}"
N="${N:-7}"

docker ps -aq --filter name=engram- --filter name=celestia- --filter name=bitcoin- --filter name=reanchoring | xargs -r docker rm -f >/dev/null 2>&1
rm -rf data testnet-data
docker compose --env-file .env -f docker-compose.yml up -d bitcoin-node01 bitcoin-node02 celestia-app
until [ "$(docker inspect -f '{{.State.Health.Status}}' celestia-app 2>/dev/null)" = healthy ]; do sleep 5; done
sleep 60
for attempt in 1 2 3 4; do
	make testnet-up MODE="$MODE" N="$N" || echo "--> testnet-up's health wait failed; checking the chain directly"
	[ -n "$(docker ps -q --filter name=^engram-node01$)" ] && python3 scripts/testnet_verify_consensus.py --wait-s 180 && exit 0
	echo "--> validators are not running (attempt $attempt), retrying in 60 s"
	sleep 60
done
exit 1
