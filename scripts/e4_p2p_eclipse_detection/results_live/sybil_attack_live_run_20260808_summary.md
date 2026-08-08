# LIVE Sybil/slot-exhaustion attack -- real 4-node Docker cluster, both legs (2026-08-08)

Real live run against the real 4-node testnet (rebuilt with all of this session's changes:
Part 1a's ingress filter, Part 1b's real RTT, Part 3's ZKProofRef, Part 4a's byzantine-mode
infra, Part 4c's evidence recording). Confirms Part 1a's `FilterPeerByAddr` (`x/sovereignty/
keeper/peer_filter.go`, `Params.MaxPeersPerSubnet=8`) against real attacker traffic, not
synthetic Monte Carlo data.

## A1 -- Peer Slot Exhaustion (10 attackers, all on engram-net)

Run twice (first run's swarm-teardown step had a real bug destroying the whole cluster,
see "Bugs found" below -- re-run after fixing it, both runs agree):

| Metric | Run 1 | Run 2 |
|---|---:|---:|
| Peak attacker-subnet (172.28.0.0) peer count | 8 | 8 |
| Real validator subnet (172.21.0.0) peer count, throughout | 3 (unaffected) | 3 (unaffected) |
| `MaxPeersPerSubnet` limit | 8 | 8 |
| Filter held (peak <= limit) | **true** | **true** |
| Cluster height progressed normally during attack | yes (200->218) | yes (210->218) |
| AppHash matched across all 4 real validators throughout | yes | yes |

**Verdict: the real ingress filter correctly capped the attacker swarm's subnet at exactly
MaxPeersPerSubnet=8, even with 10 attacker containers attempting to connect. The real
validator mesh (a structurally different observed subnet, see "Real finding" below) was
completely unaffected the entire time.**

## A2 -- Sybil via simulated multi-subnet swarm (12 attackers, intended across 4 subnets)

| Metric | Value |
|---|---:|
| Peak observed peer count on 172.28.0.0 | 8 |
| `MaxPeersPerSubnet` limit | 8 |
| Filter held | **true** |
| Genuine 4-subnet diversity achieved | **no -- see real finding below** |
| Cluster height progressed normally during attack | yes (230->234) |
| AppHash matched across all 4 real validators throughout | yes |

**Verdict: the filter held (capped at 8) even with a larger, 12-container swarm. However,
A2's original goal -- testing whether spreading attackers across 4 distinct subnets
(attacker-subnet-a/b/c/d) evades the same-subnet cap -- was NOT actually achieved, for a
real, documented reason below, not a filter bug.**

## Real finding: Docker multi-homed container gateway-priority quirk

Confirmed live via `docker inspect`/`/net_info`, not guessed: for a container attached to
MULTIPLE Docker networks with no explicit `gw_priority` override, Docker's default outbound
route consistently prefers whichever network was NOT the container's own dedicated subnet --
concretely:

- The 4 REAL validators (multi-homed on engram-net + bitcoin-net + celestia-net) show up to
  each other via their **bitcoin-net** IPs (172.21.0.0/24), not engram-net, despite engram-net
  being their intended shared P2P subnet and being declared FIRST in their compose service
  definitions.
- The 12 A2 attacker containers (multi-homed on their own attacker-subnet-a/b/c/d PLUS
  engram-net, added so they could reach engram-node01 at all) show up via **engram-net**
  (172.28.0.0/24) -- the SAME subnet as A1's attackers -- not their own dedicated
  attacker-subnet-X, despite that being declared FIRST too.

Both cases show the SECOND-declared network winning as the effective route, a consistent
(if undocumented) Docker Compose default-gateway behavior. A partial fix was attempted via
Compose's `gw_priority` field (confirmed to set `GwPriority` correctly via `docker inspect`)
on one test container, but it did not change the observed connection subnet within the time
available to debug further -- left as a genuine, real, unresolved limitation of this
docker-based simulation approach, not a flaw in `FilterPeerByAddr` itself (which correctly
enforces its cap on whatever subnet peers actually arrive from, real validators included --
demonstrated safe by A1/A2 both holding at exactly 8).

**Practical implication for E4/E8's A2 row**: this session's docker topology cannot cleanly
demonstrate "genuine multi-subnet Sybil evades a single-subnet cap" as originally scoped --
what WAS demonstrated live is that the filter's cap holds under both a same-subnet swarm (A1)
and a larger swarm that (unintentionally, due to this Docker quirk) also landed on one
subnet (A2) -- a real, valid safety result, just not the specific subnet-diversity-evasion
scenario A2 set out to test. A real fix would need either resolving the gw_priority behavior
fully, or moving off Docker's default bridge networking (e.g. macvlan) for genuinely
independent source IPs per attacker -- out of scope for this pass.

## Bugs found and fixed live during this run (real, not hypothetical)

1. **`docker compose ... down` (no service names) tears down the ENTIRE project**, not just
   the profile-gated attacker services -- confirmed live: an earlier version of
   `live_sybil_attack.py`'s `swarm_down()` used this and destroyed the running real 4-node
   cluster mid-experiment (all containers removed, including bitcoin/celestia). Fixed to use
   `docker compose stop <services>` + `rm -f <services>` with explicit service name lists,
   matching `scripts/framework/injector.py`'s existing `cleanup_profile` convention. Same bug
   also existed in `live_combined_attack.py` and `live_double_signing_test.py`, fixed
   identically in both.
2. **`docker/attacker-peer-swarm.yml`'s `persistent_peers` was missing the target's node
   ID** (`engram-node01:26656` instead of `<real-id>@engram-node01:26656`) -- CometBFT
   requires the ID-prefixed form; every attacker container crash-looped on startup with
   `address (engram-node01:26656) does not contain ID`. Fixed by having each attacker's
   entrypoint script resolve the real target ID live via `curl .../status | jq -r
   .result.node_info.id` at container startup (the ID changes every redeploy since it's
   derived from a freshly-generated node_key.json, so it can't be hardcoded).
3. **A2 attacker containers had no route to engram-node01 at all** -- they were only attached
   to their own `attacker-subnet-X`, never to `engram-net` (where engram-node01 actually
   listens), so the node-ID lookup and the P2P dial both failed. Fixed by adding `engram-net`
   to each A2 service's `networks:` block (which is what then surfaced the gateway-priority
   quirk documented above).
4. **Self-inflicted bitcoind desync (not a repo bug)**: burst-mining 101 blocks to fund a test
   wallet WHILE the engramd containers were already live pushed `h_btc_current` far outside
   `vigilante.VerifyReceipt`'s tolerance window, permanently stalling consensus (every
   proposal rejected at check #3, confirmed via temporary debug logging). This is the exact
   documented failure mode in this repo's own operational history
   (`scripts/bitcoin_miner_loop.sh`'s own comment) -- re-confirmed live, fixed by always
   funding + maturing the mining wallet BEFORE starting any engramd container, never mid-run.
   Also found: `bitcoin_miner_loop.sh` defaults to wallet name `engramwallet`, but the wallet
   created ad hoc during this incident was named `minerwallet` -- the mismatch silently killed
   the miner loop (`set -e` + RPC error), which combined with (4) above to fully stall the
   chain for several minutes before being root-caused and fixed.
