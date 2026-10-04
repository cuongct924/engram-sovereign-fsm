---------------- MODULE MC_HealthCertificate_Apalache ----------------
(*
 * MC_HealthCertificate_Apalache -- Apalache driver for Lemma 1 on the numeric
 * health certificate (EngramFSM.tla's AggregateHealthCertificate /
 * WeightedMedianOf, EngramServer.tla's AggWithinHonestRange).
 *
 * The property is a pure statement about an arbitrary observation vector, so
 * the initial `sensor_view` is left fully symbolic (`\in [Nodes -> ReadingSpace]`)
 * and the transition relation only stutters: one check at length 1 covers
 * every observation vector, not just the ones ServerExtendVote can reach.
 * EXTENDS EngramServer directly for the same reason MC_ServerSafety_Apalache
 * does (see its header: no EngramServerRefinement, no RECURSIVE SumStakeOp).
 *
 * Two constant blocks:
 *   ApalacheCInit               -- N=4, f=1 (one Byzantine): expect no violation.
 *   ApalacheCInit_TwoByzantine  -- two Byzantine in the same N=4 network
 *                                  (more than T): expect a violation of
 *                                  Sanity_AggWithinHonestRange_NoByzantineBound.
 *)
EXTENDS EngramServer

CONSTANTS
    \* @type: Str;
    n1,
    \* @type: Str;
    n2,
    \* @type: Str;
    n3,
    \* @type: Str;
    n4

\* @type: Int -> Str;
MC_Proposer == [r \in 0..2 |-> IF r % 2 = 0 THEN n1 ELSE n2]

\* @type: Set({ is_healthy: Bool, is_warning: Bool, is_critical: Bool, btc_gap_obs: Int, da_gap_obs: Int });
ReadingSpace ==
    [is_healthy: BOOLEAN, is_warning: BOOLEAN, is_critical: BOOLEAN,
     btc_gap_obs: ObsDomain, da_gap_obs: ObsDomain]

HCInit ==
    /\ TendermintInit
    /\ FSMInit
    /\ quorum_certs = {}
    /\ timeout_certs = {}
    /\ health_certs = {}
    /\ sensor_view \in [Nodes -> ReadingSpace]

HCNext ==
    /\ UNCHANGED <<tendermintCoreVars, temporalVars, bookkeepingVars, invariantVars,
                   networkSensorVars, fsmVars, censorshipVars, certificateVars, sensorAggVars>>

ApalacheCInit ==
    /\ n1 = "n1" /\ n2 = "n2" /\ n3 = "n3" /\ n4 = "n4"
    /\ Nodes = {"n1", "n2", "n3", "n4"}
    /\ Method = {"TX_NORMAL", "TX_WITHDRAWAL"}
    /\ ByzantineNodes = {"n1"}
    /\ HonestNodes = {"n2", "n3", "n4"}
    /\ Proposer = MC_Proposer
    \* Bounds shrunk from MC_ServerRefinementSafety.cfg's TLC values (3/3/20)
    \* -- `valid_value[p] \in ProposalsOrNil` (TendermintTypeOK, inside
    \* CoreTendermintInvariant) forces Apalache to enumerate the full
    \* Proposals Cartesian product; TLC's own values would be ~880K
    \* elements, past what's practical for a first pilot pass.
    /\ MAX_ROUND = 2
    /\ MAX_BTC_HEIGHT = 1
    /\ MAX_ENGRAM_HEIGHT = 1
    /\ MAX_TIMESTAMP = 2
    /\ MAX_IGNORE_ROUNDS = 1
    /\ MAX_CENSORSHIP_ROUNDS = 3
    /\ RESET_TIME = 2
    /\ SUSPICIOUS_THRESHOLD = 1
    /\ SOVEREIGN_THRESHOLD = 2
    /\ DA_THRESHOLD = 1
    /\ BTC_THRESHOLD = 0
    /\ RECOVERY_HYSTERESIS_WAIT = 1
    /\ SUSPICIOUS_HYSTERESIS_WAIT = 1
    /\ MAX_SUSPICIOUS_TIME = 1
    /\ DOWN_HYSTERESIS_THRESHOLD = 1
    /\ MAX_DOWN_HYSTERESIS_THRESHOLD = 8
    /\ STABLE_WINDOW = 2
    /\ MIN_PEERS = 2
    /\ MIN_SUBNET_DIVERSITY = 2
    /\ MIN_ANCHOR_PEERS = 1
    /\ MAX_CHURN_RATE = 1
    /\ MIN_AVG_TENURE = 1
    /\ MAX_PEER_LATENCY = 1
    /\ N = 4
    /\ T = 1
    /\ MIN_TIMESTAMP = 0
    /\ PRECISION = 1
    /\ DELAY = 1
    /\ TIMEOUT_DURATION = 2


ApalacheCInit_TwoByzantine ==
    /\ n1 = "n1" /\ n2 = "n2" /\ n3 = "n3" /\ n4 = "n4"
    /\ Nodes = {"n1", "n2", "n3", "n4"}
    /\ Method = {"TX_NORMAL", "TX_WITHDRAWAL"}
    /\ ByzantineNodes = {"n1", "n2"}
    /\ HonestNodes = {"n3", "n4"}
    /\ Proposer = MC_Proposer
    \* Bounds shrunk from MC_ServerRefinementSafety.cfg's TLC values (3/3/20)
    \* -- `valid_value[p] \in ProposalsOrNil` (TendermintTypeOK, inside
    \* CoreTendermintInvariant) forces Apalache to enumerate the full
    \* Proposals Cartesian product; TLC's own values would be ~880K
    \* elements, past what's practical for a first pilot pass.
    /\ MAX_ROUND = 2
    /\ MAX_BTC_HEIGHT = 1
    /\ MAX_ENGRAM_HEIGHT = 1
    /\ MAX_TIMESTAMP = 2
    /\ MAX_IGNORE_ROUNDS = 1
    /\ MAX_CENSORSHIP_ROUNDS = 3
    /\ RESET_TIME = 2
    /\ SUSPICIOUS_THRESHOLD = 1
    /\ SOVEREIGN_THRESHOLD = 2
    /\ DA_THRESHOLD = 1
    /\ BTC_THRESHOLD = 0
    /\ RECOVERY_HYSTERESIS_WAIT = 1
    /\ SUSPICIOUS_HYSTERESIS_WAIT = 1
    /\ MAX_SUSPICIOUS_TIME = 1
    /\ DOWN_HYSTERESIS_THRESHOLD = 1
    /\ MAX_DOWN_HYSTERESIS_THRESHOLD = 8
    /\ STABLE_WINDOW = 2
    /\ MIN_PEERS = 2
    /\ MIN_SUBNET_DIVERSITY = 2
    /\ MIN_ANCHOR_PEERS = 1
    /\ MAX_CHURN_RATE = 1
    /\ MIN_AVG_TENURE = 1
    /\ MAX_PEER_LATENCY = 1
    /\ N = 4
    /\ T = 1
    /\ MIN_TIMESTAMP = 0
    /\ PRECISION = 1
    /\ DELAY = 1
    /\ TIMEOUT_DURATION = 2

=============================================================================
