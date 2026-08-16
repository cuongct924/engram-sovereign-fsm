package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	cmtcfg "github.com/cometbft/cometbft/config"
	cmtcrypto "github.com/cometbft/cometbft/crypto"
	cmtlog "github.com/cometbft/cometbft/libs/log"
	"github.com/cometbft/cometbft/lp2p"
	"github.com/cometbft/cometbft/node"
	"github.com/cometbft/cometbft/p2p"
	"github.com/cometbft/cometbft/privval"
	"github.com/cometbft/cometbft/proxy"
	cmttypes "github.com/cometbft/cometbft/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cuongct220020/engram-sovereign-fsm/app"
	"github.com/cuongct220020/engram-sovereign-fsm/x/anchor"
	"github.com/cuongct220020/engram-sovereign-fsm/x/da"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper/sensors"
	sovereigntytypes "github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	sdklog "cosmossdk.io/log/v2"

	"github.com/cosmos/cosmos-sdk/server"
)

func defaultHome() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".engramd"
	}
	return filepath.Join(home, ".engramd")
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "engramd",
		Short: "Engram Sovereign FSM node -- Cosmos SDK + CometBFT prototype",
	}
	homeFlag := rootCmd.PersistentFlags().String("home", defaultHome(), "node home directory")

	rootCmd.AddCommand(initCmd(homeFlag))
	rootCmd.AddCommand(startCmd(homeFlag))
	rootCmd.AddCommand(testnetInitFilesCmd())

	rootCmd.AddCommand(queryStateCmd())
	rootCmd.AddCommand(queryRecoveryHeadersCmd())
	rootCmd.AddCommand(publishRecoveryWitnessCmd())
	rootCmd.AddCommand(txSubmitRecoveryProofCmd())
	rootCmd.AddCommand(txSubmitForcedTxCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// initCmd generates a single-validator home: CometBFT config, FilePV key,
// node key, and a genesis whose validator set is this one key (power 10) --
// no x/staking module, so the genesis validator list IS the validator set.
func initCmd(homeFlag *string) *cobra.Command {
	var moniker string
	cmd := &cobra.Command{
		Use:   "init [moniker]",
		Short: "Initialize a single-validator node home directory",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				moniker = args[0]
			} else if moniker == "" {
				moniker = "engram-node"
			}
			return initHome(*homeFlag, moniker)
		},
	}
	return cmd
}

func initHome(home, moniker string) error {
	config := cmtcfg.DefaultConfig()
	config.SetRoot(home)
	cmtcfg.EnsureRoot(home)

	config.Moniker = moniker
	// Enables the ABCI ingress peer filter (peer_filter.go's FilterPeerByAddr,
	// via baseapp.SetAddrPeerFilter) -- a stock CometBFT mechanism,
	// createCometTransport already builds the "/p2p/filter/addr/..." query
	// path when true, no fork changes needed.
	config.FilterPeers = true
	cmtcfg.WriteConfigFile(filepath.Join(home, "config", "config.toml"), config)

	pv := privval.LoadOrGenFilePV(config.PrivValidatorKeyFile(), config.PrivValidatorStateFile())
	pubKey, err := pv.GetPubKey()
	if err != nil {
		return fmt.Errorf("load validator pubkey: %w", err)
	}

	if _, err := p2p.LoadOrGenNodeKey(config.NodeKeyFile()); err != nil {
		return fmt.Errorf("load/gen node key: %w", err)
	}

	genesisFile := config.GenesisFile()
	if _, err := os.Stat(genesisFile); err == nil {
		fmt.Println("genesis file already exists:", genesisFile)
		return nil
	}

	params, err := paramsFromEnv()
	if err != nil {
		return fmt.Errorf("ENGRAM_PARAM_* override: %w", err)
	}
	appStateBytes, err := json.Marshal(map[string]json.RawMessage{
		"sovereignty": mustMarshalGenesis(sovereigntytypes.DefaultGenesisWithParams(params)),
	})
	if err != nil {
		return err
	}

	genDoc := cmttypes.GenesisDoc{
		ChainID:         "engram-dev-1",
		GenesisTime:     time.Now(),
		ConsensusParams: cmttypes.DefaultConsensusParams(),
		Validators: []cmttypes.GenesisValidator{
			{Address: pubKey.Address(), PubKey: pubKey, Power: 10, Name: moniker},
		},
		AppState: appStateBytes,
	}
	if err := genDoc.ValidateAndComplete(); err != nil {
		return fmt.Errorf("invalid genesis: %w", err)
	}
	if err := genDoc.SaveAs(genesisFile); err != nil {
		return fmt.Errorf("save genesis: %w", err)
	}

	fmt.Println("initialized node home:", home)
	fmt.Println("genesis file:", genesisFile)
	fmt.Println("validator address:", pubKey.Address())
	return nil
}

func mustMarshalGenesis(gs *sovereigntytypes.GenesisState) json.RawMessage {
	bz, err := json.Marshal(gs)
	if err != nil {
		panic(err)
	}
	return bz
}

// paramsFromEnv builds x/sovereignty's Params from ENGRAM_PARAM_* env vars,
// DefaultParams() per-field. Read ONLY at genesis-generation time
// (initHome/testnetInitFiles), never by `start`: every node copies the same
// generated genesis.json, so an override baked in stays uniform; re-reading
// env at `start` would let each node's local .env silently diverge -- a
// liveness/safety bug for consensus-compared values (see Params.Validate).
func paramsFromEnv() (sovereigntytypes.Params, error) {
	p := sovereigntytypes.DefaultParams()
	fields := []struct {
		env string
		dst *uint64
	}{
		{"ENGRAM_PARAM_SUSPICIOUS_THRESHOLD", &p.SuspiciousThreshold},
		{"ENGRAM_PARAM_SOVEREIGN_THRESHOLD", &p.SovereignThreshold},
		{"ENGRAM_PARAM_DA_THRESHOLD", &p.DAThreshold},
		{"ENGRAM_PARAM_HYSTERESIS_WAIT", &p.HysteresisWait},
		{"ENGRAM_PARAM_DOWN_HYSTERESIS_THRESHOLD", &p.DownHysteresisThreshold},
		{"ENGRAM_PARAM_MAX_DOWN_HYSTERESIS_THRESHOLD", &p.MaxDownHysteresisThreshold},
		{"ENGRAM_PARAM_SUSPICIOUS_HYSTERESIS_WAIT", &p.SuspiciousHysteresisWait},
		{"ENGRAM_PARAM_MIN_PEERS", &p.MinPeers},
		{"ENGRAM_PARAM_MIN_SUBNET_DIVERSITY", &p.MinSubnetDiversity},
		{"ENGRAM_PARAM_MIN_ANCHOR_PEERS", &p.MinAnchorPeers},
		{"ENGRAM_PARAM_MAX_CHURN_RATE", &p.MaxChurnRate},
		{"ENGRAM_PARAM_MIN_AVG_TENURE", &p.MinAvgTenure},
		{"ENGRAM_PARAM_MAX_PEER_LATENCY", &p.MaxPeerLatency},
		{"ENGRAM_PARAM_MAX_SUSPICIOUS_TIME", &p.MaxSuspiciousTime},
		{"ENGRAM_PARAM_MAX_IGNORE_ROUNDS", &p.MaxIgnoreRounds},
		{"ENGRAM_PARAM_K_DEEP_FINALITY", &p.KDeepFinality},
		{"ENGRAM_PARAM_MAX_UNPROVEN_TAIL_BLOCKS", &p.MaxUnprovenTailBlocks},
		{"ENGRAM_PARAM_MAX_PEERS_PER_SUBNET", &p.MaxPeersPerSubnet},
		{"ENGRAM_PARAM_MAX_SUSPICIOUS_FORCED_TX_QUEUE", &p.MaxSuspiciousForcedTxQueue},
	}
	for _, f := range fields {
		raw := os.Getenv(f.env)
		if raw == "" {
			continue
		}
		v, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return p, fmt.Errorf("%s=%q: %w", f.env, raw, err)
		}
		*f.dst = v
	}
	if err := p.Validate(); err != nil {
		return p, err
	}
	return p, nil
}

// testnetInitFilesCmd generates N validator homes sharing ONE genesis (all
// power 10) and per-node persistent_peers. initHome only makes a
// single-validator genesis -- correct for single-node but can't bootstrap a
// real multi-node testnet, where each node would otherwise generate a
// DIFFERENT single-validator genesis and never agree.
func testnetInitFilesCmd() *cobra.Command {
	var (
		numValidators  int
		outputDir      string
		chainID        string
		hostnamePrefix string
		startingPort   int
	)
	cmd := &cobra.Command{
		Use:   "testnet init-files",
		Short: "Generate N validator home directories sharing one genesis, for a real multi-node testnet",
		RunE: func(cmd *cobra.Command, args []string) error {
			return testnetInitFiles(outputDir, numValidators, chainID, hostnamePrefix, startingPort)
		},
	}
	cmd.Flags().IntVar(&numValidators, "v", 4, "number of validators")
	cmd.Flags().StringVar(&outputDir, "output-dir", "./testnet-data", "directory to write node homes into (one subdir per node)")
	cmd.Flags().StringVar(&chainID, "chain-id", "engram-dev-1", "chain ID for the shared genesis")
	cmd.Flags().StringVar(&hostnamePrefix, "hostname-prefix", "engram-node", "moniker/hostname prefix, nodes are named <prefix>01, <prefix>02, ...")
	cmd.Flags().IntVar(&startingPort, "p2p-port", 26656, "P2P port every node listens on (same port on every node -- each is a distinct Docker hostname, not distinct ports)")
	return cmd
}

// pairwiseLinkIndex returns the 0-indexed link for unordered pair {i,j} in
// row-major order: (0,1),(0,2),...,(0,n-1),(1,2),... -- must match
// docker/engram-validator-cluster.yml's validator-link-NN-MM networks 1:1
// (no shared source of truth between the YAML and this file).
func pairwiseLinkIndex(i, j, n int) int {
	if i > j {
		i, j = j, i
	}
	return i*n - i*(i+1)/2 + (j - i - 1)
}

// pairwiseLinkPeerIP returns validator `peer`'s static IP as dialed by `self`
// on their dedicated link (172.40.<link>.0/29, gateway .1, lower-indexed
// validator .2, higher .3) -- a literal IP, not a hostname, so real
// SubnetDiversity reads each peer from a genuinely distinct subnet. Peer auth
// is unaffected: CometBFT verifies nodeID via the secret handshake regardless
// of which address reached it.
func pairwiseLinkPeerIP(self, peer, n int) string {
	link := pairwiseLinkIndex(self, peer, n)
	if peer < self {
		return fmt.Sprintf("172.40.%d.2", link)
	}
	return fmt.Sprintf("172.40.%d.3", link)
}

func testnetInitFiles(outputDir string, n int, chainID, hostnamePrefix string, p2pPort int) error {
	if n < 1 {
		return fmt.Errorf("need at least 1 validator, got %d", n)
	}

	type nodeInfo struct {
		home    string
		moniker string
		pubKey  cmtcrypto.PubKey
		nodeID  p2p.ID
	}

	nodes := make([]nodeInfo, 0, n)
	for i := 1; i <= n; i++ {
		moniker := fmt.Sprintf("%s%02d", hostnamePrefix, i)
		home := filepath.Join(outputDir, moniker)
		config := cmtcfg.DefaultConfig()
		config.SetRoot(home)
		cmtcfg.EnsureRoot(home)
		config.Moniker = moniker

		pv := privval.LoadOrGenFilePV(config.PrivValidatorKeyFile(), config.PrivValidatorStateFile())
		pubKey, err := pv.GetPubKey()
		if err != nil {
			return fmt.Errorf("%s: load validator pubkey: %w", moniker, err)
		}
		nodeKey, err := p2p.LoadOrGenNodeKey(config.NodeKeyFile())
		if err != nil {
			return fmt.Errorf("%s: load/gen node key: %w", moniker, err)
		}

		nodes = append(nodes, nodeInfo{home: home, moniker: moniker, pubKey: pubKey, nodeID: nodeKey.ID()})
	}

	validators := make([]cmttypes.GenesisValidator, 0, n)
	for _, nd := range nodes {
		validators = append(validators, cmttypes.GenesisValidator{Address: nd.pubKey.Address(), PubKey: nd.pubKey, Power: 10, Name: nd.moniker})
	}
	params, err := paramsFromEnv()
	if err != nil {
		return fmt.Errorf("ENGRAM_PARAM_* override: %w", err)
	}
	appStateBytes, err := json.Marshal(map[string]json.RawMessage{
		"sovereignty": mustMarshalGenesis(sovereigntytypes.DefaultGenesisWithParams(params)),
	})
	if err != nil {
		return err
	}
	genDoc := cmttypes.GenesisDoc{
		ChainID:         chainID,
		GenesisTime:     time.Now(),
		ConsensusParams: cmttypes.DefaultConsensusParams(),
		Validators:      validators,
		AppState:        appStateBytes,
	}
	if err := genDoc.ValidateAndComplete(); err != nil {
		return fmt.Errorf("invalid shared genesis: %w", err)
	}

	for i, nd := range nodes {
		if err := genDoc.SaveAs(filepath.Join(nd.home, "config", "genesis.json")); err != nil {
			return fmt.Errorf("%s: save genesis: %w", nd.moniker, err)
		}

		config := cmtcfg.DefaultConfig()
		config.SetRoot(nd.home)
		config.Moniker = nd.moniker
		config.P2P.ListenAddress = fmt.Sprintf("tcp://0.0.0.0:%d", p2pPort)
		config.P2P.AddrBookStrict = false // container-internal IPs/hostnames aren't publicly routable
		config.P2P.AllowDuplicateIP = true
		// RPC defaults to 127.0.0.1, unreachable through a Docker port mapping
		// from the host -- 0.0.0.0 is safe on isolated Docker networks.
		config.RPC.ListenAddress = "tcp://0.0.0.0:26657"
		config.Instrumentation.Prometheus = true
		// See initHome's identical FilterPeers comment.
		config.FilterPeers = true

		var peers []string
		for j, other := range nodes {
			if j == i {
				continue
			}
			peerIP := pairwiseLinkPeerIP(i, j, n)
			peers = append(peers, fmt.Sprintf("%s@%s:%d", other.nodeID, peerIP, p2pPort))
		}
		config.P2P.PersistentPeers = strings.Join(peers, ",")

		cmtcfg.WriteConfigFile(filepath.Join(nd.home, "config", "config.toml"), config)
		fmt.Printf("%s: home=%s node_id=%s peers=%s\n", nd.moniker, nd.home, nd.nodeID, config.P2P.PersistentPeers)
	}

	fmt.Printf("generated %d-validator testnet under %s\n", n, outputDir)
	return nil
}

// startCmd boots a single CometBFT instance running EngramApp in-process
// (proxy.NewLocalClientCreator) -- no multi-node P2P here (see M6 for the
// Docker multi-node testnet path).
func startCmd(homeFlag *string) *cobra.Command {
	var vanilla bool
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the node",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStart(*homeFlag, vanilla)
		},
	}
	cmd.Flags().BoolVar(&vanilla, "vanilla", false, "run plain CometBFT/Cosmos SDK consensus with no ExtendedProposal (docs/EXPERIMENT.md's baseline)")
	return cmd
}

// loadConfig reads config/config.toml from home onto cmtcfg's defaults via
// viper -- cmtcfg.DefaultConfig() alone reads nothing from disk, so per-node
// customization (RPC/P2P ports, persistent_peers) written by `engramd init`
// would otherwise be silently ignored at `start`. Mirrors CometBFT's own
// command bootstrap pattern.
func loadConfig(home string) (*cmtcfg.Config, error) {
	v := viper.New()
	v.SetConfigFile(filepath.Join(home, "config", "config.toml"))
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config.toml (did you run `engramd init` first?): %w", err)
	}

	config := cmtcfg.DefaultConfig()
	if err := v.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("parse config.toml: %w", err)
	}
	config.SetRoot(home)
	if err := config.ValidateBasic(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	return config, nil
}

func runStart(home string, vanilla bool) error {
	config, err := loadConfig(home)
	if err != nil {
		return err
	}

	cmtLogger := cmtlog.NewTMLogger(cmtlog.NewSyncWriter(os.Stdout))
	sdkLogger := sdklog.NewLogger(os.Stdout)

	db, err := dbm.NewDB("application", dbm.GoLevelDBBackend, filepath.Join(home, "data"))
	if err != nil {
		return fmt.Errorf("open application db: %w", err)
	}

	genDoc, err := cmttypes.GenesisDocFromFile(config.GenesisFile())
	if err != nil {
		return fmt.Errorf("load genesis (did you run `engramd init` first?): %w", err)
	}
	// ENGRAM_BYZANTINE_BEHAVIOR: unset ("") on every real validator --
	// deliberate misbehavior for docs/EXPERIMENT.md's E8 A3/A4/A6/A7 rows,
	// only ever set by docker/engram-node04-byzantine.yml. See
	// x/sovereignty/proposal.go's applyByzantineBehavior for recognized values.
	byzantineBehavior := os.Getenv("ENGRAM_BYZANTINE_BEHAVIOR")
	if byzantineBehavior != "" {
		fmt.Println("engramd: WARNING -- ENGRAM_BYZANTINE_BEHAVIOR is set:", byzantineBehavior,
			"(this node will deliberately misbehave; never set this on a real validator)")
	}
	engramApp := app.NewEngramApp(sdkLogger, db, genDoc.ChainID, vanilla, byzantineBehavior)

	pv := privval.LoadFilePV(config.PrivValidatorKeyFile(), config.PrivValidatorStateFile())
	nodeKey, err := p2p.LoadNodeKey(config.NodeKeyFile())
	if err != nil {
		return fmt.Errorf("load node key (did you run `engramd init` first?): %w", err)
	}

	n, err := node.NewNode(
		config,
		pv,
		nodeKey,
		proxy.NewLocalClientCreator(server.NewCometABCIWrapper(engramApp)),
		node.DefaultGenesisDocProviderFunc(config),
		cmtcfg.DefaultDBProvider,
		node.DefaultMetricsProvider(config.Instrumentation),
		cmtLogger,
	)
	if err != nil {
		return fmt.Errorf("construct node: %w", err)
	}

	wireP2PSensor(engramApp, n, config.P2P.PersistentPeers)
	wirePeerFilter(engramApp, n)
	wireBTCSensor(engramApp)
	wireDASensor(engramApp)

	if err := n.Start(); err != nil {
		return fmt.Errorf("start node: %w", err)
	}
	fmt.Println("engramd started -- RPC:", config.RPC.ListenAddress)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	return n.Stop()
}

// lp2pHealthAdapter converts the CometBFT fork's raw lp2p.HealthSnapshot
// (github.com/cuongct090_04/engram-consensus-core) into the field-compatible
// sensors.P2PSnapshot shape x/sovereignty expects -- implements
// sensors.P2PHealthSource. Lives here because it's the only layer importing
// both the fork's lp2p package and x/sovereignty's sensors package.
type lp2pHealthAdapter struct {
	sw *lp2p.Switch
}

func (a lp2pHealthAdapter) PeerHealthSnapshot() sensors.P2PSnapshot {
	snap := a.sw.PeerHealthSnapshot()
	return sensors.P2PSnapshot{
		SubnetDiversity: snap.SubnetDiversity,
		ActiveAnchors:   snap.ActiveAnchors,
		CleanPeers:      snap.CleanPeers,
		ChurnRate:       snap.PeerChurnRate,
		AvgTenure:       snap.AvgPeerTenure,
		Latency:         snap.PeerLatencyMs,
	}
}

// vanillaP2PHealthAdapter computes real P2PSnapshot readings from the
// STANDARD (non-libp2p) CometBFT p2p.Switch -- the transport actually
// carrying consensus traffic in every deployment (libp2p is disabled in every
// generated config.toml). Reads real, already-flowing switch data
// (Peers(), IsPersistent(), Status().Duration); observes the data plane,
// never touches it.
//
// ActiveAnchors mirrors persistent_peers (the trusted bootstrap set, vs.
// dynamically PEX-discovered peers more exposed to Sybil/Eclipse), matched by
// peer ID -- NOT p2p.Peer.IsPersistent(), which matches by NetAddress and
// only on the DIALING side (the ACCEPTING side sees the remote's ephemeral
// source port), undercounting asymmetrically; ID matching is symmetric.
//
// CleanPeers has no blacklist yet (the fork's HealthMonitor.Blacklist is
// likewise never called) -- counts total connected peers, a documented
// simplification. Latency piggybacks MConnection's PacketPing/PacketPong
// keep-alive (p2p.Peer.RTT()); 0 per-peer until the first exchange.
type vanillaP2PHealthAdapter struct {
	sw                *p2p.Switch
	persistentPeerIDs map[p2p.ID]bool

	mu          sync.Mutex
	firstSeen   map[p2p.ID]time.Time
	churnEvents []time.Time
}

// parsePersistentPeerIDs extracts just the "id@host:port" -> id portion of
// persistent_peers, for ActiveAnchors' ID-based matching (see the adapter doc
// for why raw IsPersistent() isn't reliable). Malformed entries (no "@") are
// skipped, not erroring -- best-effort health, not consensus-critical.
func parsePersistentPeerIDs(raw string) map[p2p.ID]bool {
	ids := make(map[p2p.ID]bool)
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		at := strings.Index(entry, "@")
		if at <= 0 {
			continue
		}
		ids[p2p.ID(entry[:at])] = true
	}
	return ids
}

const p2pChurnWindow = time.Hour

// countsAsChurn reports whether a peer-set change for id should count toward
// ChurnRate. A reconnect of an already-known genesis validator (tracked via
// persistentPeerIDs, the same set ActiveAnchors uses) is routine -- a
// restart, config toggle, or upgrade -- categorically different from churn
// by an unknown/untrusted peer, which is what MaxChurnRate exists to catch
// (E4's real Sybil/eclipse churn attack always uses a fresh, non-validator
// node key -- never appears in persistentPeerIDs, so this exemption cannot
// blind that detection). Divergence from spec/core/EngramFSM.tla's
// peer_churn_rate (a bare Nat with no peer-identity concept) -- concrete-only,
// no spec line, same pattern as MaxSuspiciousForcedTxQueue.
func countsAsChurn(id p2p.ID, persistentPeerIDs map[p2p.ID]bool) bool {
	return !persistentPeerIDs[id]
}

func (a *vanillaP2PHealthAdapter) PeerHealthSnapshot() sensors.P2PSnapshot {
	peers := a.sw.Peers().Copy()

	subnets := make(map[string]bool, len(peers))
	var activeAnchors, cleanPeers uint64
	var tenureSum time.Duration
	var maxRTT time.Duration

	a.mu.Lock()
	defer a.mu.Unlock()

	seen := make(map[p2p.ID]bool, len(peers))
	now := time.Now()
	for _, p := range peers {
		id := p.ID()
		seen[id] = true
		if _, ok := a.firstSeen[id]; !ok {
			a.firstSeen[id] = now
			if countsAsChurn(id, a.persistentPeerIDs) {
				a.churnEvents = append(a.churnEvents, now)
			}
		}

		if ip := p.RemoteIP(); ip != nil {
			subnets[sovereigntytypes.SubnetOf(ip)] = true
		}
		if a.persistentPeerIDs[id] {
			activeAnchors++
		}
		cleanPeers++
		tenureSum += p.Status().Duration
		// Worst-case (max, not average) across peers: a single
		// degraded/attacked peer is itself the signal IsP2PQualityHealthy
		// exists to catch, not something an average should dilute. 0 until
		// a peer's first ping/pong exchange completes.
		if rtt := p.RTT(); rtt > maxRTT {
			maxRTT = rtt
		}
	}
	// Any previously-seen peer no longer in the current set disconnected --
	// also counts as a churn event, and stops accumulating tenure/anchors.
	for id := range a.firstSeen {
		if !seen[id] {
			delete(a.firstSeen, id)
			if countsAsChurn(id, a.persistentPeerIDs) {
				a.churnEvents = append(a.churnEvents, now)
			}
		}
	}

	cutoff := now.Add(-p2pChurnWindow)
	kept := a.churnEvents[:0]
	for _, t := range a.churnEvents {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	a.churnEvents = kept

	var avgTenure uint64
	if cleanPeers > 0 {
		avgTenure = uint64((tenureSum / time.Duration(cleanPeers)).Seconds())
	}

	return sensors.P2PSnapshot{
		SubnetDiversity: uint64(len(subnets)),
		ActiveAnchors:   activeAnchors,
		CleanPeers:      cleanPeers,
		ChurnRate:       uint64(len(a.churnEvents)),
		AvgTenure:       avgTenure,
		Latency:         uint64(maxRTT.Milliseconds()),
	}
}

// PeerCountInSubnet implements sovereigntykeeper.PeerFilterSource for the
// vanilla p2p.Switch transport -- counts currently-connected peers whose
// types.SubnetOf matches subnet, using the same live peer set
// PeerHealthSnapshot reads. Used by FilterPeerByAddr (x/sovereignty/keeper/
// peer_filter.go) to decide whether a NEW connection attempt would push that
// subnet's population to or above Params.MaxPeersPerSubnet.
func (a *vanillaP2PHealthAdapter) PeerCountInSubnet(subnet string) uint64 {
	var count uint64
	for _, p := range a.sw.Peers().Copy() {
		if ip := p.RemoteIP(); ip != nil && sovereigntytypes.SubnetOf(ip) == subnet {
			count++
		}
	}
	return count
}

// wireP2PSensor upgrades engramApp's P2P sensor from its static SetSnapshot
// mock to a live source. n.Switch() only exists after node.NewNode() returns,
// so the source is late-bound rather than passed at construction -- runs
// before n.Start(), so no real proposal is ever processed with the mock
// still active. Tries the fork's lp2p.Switch first; falls back to the
// vanilla adapter against the standard switch, which every deployment uses.
func wireP2PSensor(engramApp *app.EngramApp, n *node.Node, persistentPeers string) {
	if lsw, ok := n.Switch().(*lp2p.Switch); ok {
		engramApp.Sensors.P2P.SetSource(lp2pHealthAdapter{sw: lsw})
		return
	}
	sw, ok := n.Switch().(*p2p.Switch)
	if !ok {
		return
	}
	engramApp.Sensors.P2P.SetSource(&vanillaP2PHealthAdapter{
		sw:                sw,
		persistentPeerIDs: parsePersistentPeerIDs(persistentPeers),
		firstSeen:         make(map[p2p.ID]time.Time),
	})
}

// wirePeerFilter upgrades engramApp's ingress peer filter (peer_filter.go's
// FilterPeerByAddr) from its fail-open default to a live PeerFilterSource,
// mirroring wireP2PSensor's late-binding. Only the vanilla *p2p.Switch case
// is wired (lp2p is dormant) -- stays fail-open if lp2p is ever enabled.
//
// Builds its own lightweight adapter rather than reusing wireP2PSensor's
// instance: PeerCountInSubnet only reads a.sw.Peers() fresh and needs none
// of the stateful churn-tracking fields.
func wirePeerFilter(engramApp *app.EngramApp, n *node.Node) {
	sw, ok := n.Switch().(*p2p.Switch)
	if !ok {
		return
	}
	engramApp.SovereigntyKeeper.SetPeerFilterSource(&vanillaP2PHealthAdapter{sw: sw})
}

// wireBTCSensor upgrades engramApp's BTC sensor from its static SetGap mock
// to a real bitcoind JSON-RPC connection, and wires an AnchorTracker against
// it so h_btc_anchored has a real submit-and-confirm pipeline -- when
// BITCOIN_HOST is set; silent no-op when unset.
//
// Requires a wallet with spendable funds on the connected bitcoind. This repo
// doesn't run the real babylonlabs/babylond vigilante images (they need a
// real Babylon chain this app lacks); AnchorTracker is a minimal in-process
// stand-in for the same submit-and-confirm goal.
func wireBTCSensor(engramApp *app.EngramApp) {
	host := os.Getenv("BITCOIN_HOST")
	if host == "" {
		return
	}
	port := os.Getenv("BITCOIN_RPC_PORT")
	if port == "" {
		port = "18443"
	}
	user := os.Getenv("BITCOIN_RPC_USER")
	pass := os.Getenv("BITCOIN_RPC_PASSWORD")

	client := anchor.NewRPCClient(fmt.Sprintf("http://%s:%s", host, port), user, pass)
	engramApp.Sensors.BTC.SetSource(client)
	tracker := anchor.NewAnchorTracker(client, engramApp.SovereigntyKeeper.Params.KDeepFinality)
	// ANCHOR_SUBMISSION_PAUSED_FILE lets a fault-injection script pause new
	// checkpoint submissions mid-run (see SetSubmissionPausedFile's doc) --
	// unset on every real validator, only used by
	// scripts/e2_fault_injection/live_scenario_matrix.py's S2 phase.
	if pauseFile := os.Getenv("ANCHOR_SUBMISSION_PAUSED_FILE"); pauseFile != "" {
		tracker.SetSubmissionPausedFile(pauseFile)
	}
	engramApp.Sensors.Anchor = tracker
}

// wireDASensor upgrades engramApp's DA sensor from its static SetAvailable
// mock to a real celestia-bridge JSON-RPC connection, and wires a Publisher
// against it so h_engram_verified has a real submit-and-retrieve pipeline --
// when CELESTIA_BRIDGE_URL is set; silent no-op when unset.
//
// CELESTIA_BRIDGE_AUTH_TOKEN must be the bridge's admin/write JWT --
// blob.Submit is a write call celestia-node rejects unauthenticated.
func wireDASensor(engramApp *app.EngramApp) {
	url := os.Getenv("CELESTIA_BRIDGE_URL")
	if url == "" {
		return
	}
	authToken := os.Getenv("CELESTIA_BRIDGE_AUTH_TOKEN")
	namespaceID := os.Getenv("CELESTIA_NAMESPACE_ID")
	if namespaceID == "" {
		namespaceID = "engramda01"
	}

	ns, err := da.NewNamespace(namespaceID)
	if err != nil {
		fmt.Println("engramd: skipping DA sensor wiring, invalid CELESTIA_NAMESPACE_ID:", err)
		return
	}

	client := da.NewRPCClient(url, authToken)
	engramApp.Sensors.DAPublisher = da.NewPublisher(client, ns)
	engramApp.Sensors.DA.SetSource(engramApp.Sensors.DAPublisher)
}
