package main

import (
	"crypto/sha256"
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

	"github.com/celestiaorg/smt"
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
	rootCmd.AddCommand(demoSMTCmd())
	rootCmd.AddCommand(queryRecoveryHeadersCmd())
	rootCmd.AddCommand(txSubmitRecoveryProofCmd())
	rootCmd.AddCommand(txSubmitForcedTxCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// initCmd generates a single-validator home directory: CometBFT config,
// a FilePV validator key, a node key, and a genesis file whose
// consensus-layer validator set is this one key (power 10) -- there is no
// x/staking module in this prototype (see app/app.go's package doc), so the
// genesis validator list IS the validator set, not a starting point for
// staking-driven rotation.
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
	// Enables the ABCI ingress peer filter (x/sovereignty/keeper/peer_filter.go's
	// FilterPeerByAddr, wired via app.go's baseapp.SetAddrPeerFilter) -- a
	// stock cosmos-sdk/CometBFT mechanism, node/setup.go's
	// createCometTransport already builds the "/p2p/filter/addr/..." ABCI
	// query path whenever this is true, no fork changes needed.
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

// paramsFromEnv builds x/sovereignty's consensus-critical Params from
// ENGRAM_PARAM_* env vars, falling back to DefaultParams() per-field when
// unset. Read ONLY here, at genesis-generation time (initHome/
// testnetInitFiles) -- never by `start`. Every validator's genesis.json is
// generated once and copied identically to all nodes, so baking an
// override in at generation time is safe (guaranteed uniform across the
// validator set); reading these same env vars again at `start` time would
// let each node's own local .env silently diverge from the others, which
// for a consensus-compared value like Params is a liveness/safety bug, not
// a convenience -- see Params.Validate's doc.
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

// testnetInitFilesCmd generates N validator home directories sharing ONE
// genesis (all N validators listed, power 10 each) and per-node
// persistent_peers -- initCmd/initHome above only produce a single-validator
// genesis (this app has no x/staking module, so the genesis validator list
// IS the validator set), which is correct for M5's single-node path but
// cannot bootstrap a real multi-validator testnet (docs/EXPERIMENT.md's E2's
// "4, 7, 10, 16 nodes" / M7's Docker testnet) -- every node would otherwise
// generate a DIFFERENT single-validator genesis and never agree.
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

func testnetInitFiles(outputDir string, n int, chainID, hostnamePrefix string, p2pPort int) error {
	if n < 1 {
		return fmt.Errorf("need at least 1 validator, got %d", n)
	}

	type nodeInfo struct {
		home     string
		moniker  string
		hostname string
		pubKey   cmtcrypto.PubKey
		nodeID   p2p.ID
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

		nodes = append(nodes, nodeInfo{home: home, moniker: moniker, hostname: moniker, pubKey: pubKey, nodeID: nodeKey.ID()})
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
		// RPC defaults to 127.0.0.1 (cmtcfg.DefaultConfig()), unreachable
		// through a Docker port mapping from the host -- 0.0.0.0 is safe
		// here since these are isolated Docker networks, not
		// public-internet-facing.
		config.RPC.ListenAddress = "tcp://0.0.0.0:26657"
		config.Instrumentation.Prometheus = true
		// See initHome's identical FilterPeers comment.
		config.FilterPeers = true

		var peers []string
		for j, other := range nodes {
			if j == i {
				continue
			}
			peers = append(peers, fmt.Sprintf("%s@%s:%d", other.nodeID, other.hostname, p2pPort))
		}
		config.P2P.PersistentPeers = strings.Join(peers, ",")

		cmtcfg.WriteConfigFile(filepath.Join(nd.home, "config", "config.toml"), config)
		fmt.Printf("%s: home=%s node_id=%s peers=%s\n", nd.moniker, nd.home, nd.nodeID, config.P2P.PersistentPeers)
	}

	fmt.Printf("generated %d-validator testnet under %s\n", n, outputDir)
	return nil
}

// startCmd boots a single-node CometBFT instance running EngramApp
// in-process (proxy.NewLocalClientCreator) -- no multi-node P2P networking
// is exercised here (see M6 for the Docker multi-node testnet path).
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

// loadConfig reads config/config.toml from home and unmarshals it onto
// cmtcfg's defaults via viper -- cmtcfg.DefaultConfig() alone does not read
// anything from disk, so per-node customization (RPC/P2P ports, seeds,
// persistent_peers) written by `engramd init` would otherwise be silently
// ignored at `start` time. Mirrors CometBFT's own cmd/cometbft/commands
// bootstrap pattern.
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
	// only ever set by docker/engram-validator-node04-byzantine.yml. See
	// x/sovereignty/proposal.go's applyByzantineBehavior for recognized
	// values.
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
// (github.com/cuongct090_04/engram-consensus-core, M0a) into the
// field-compatible sensors.P2PSnapshot shape x/sovereignty expects --
// implements sensors.P2PHealthSource. Lives here, not in x/sovereignty,
// because this is the only layer that imports both the fork's lp2p package
// and x/sovereignty's sensors package (see P2PHealthSource's doc).
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
// carrying consensus traffic in every deployment of this repo to date
// (libp2p is disabled in every generated config.toml). Reads real,
// already-flowing p2p.Switch data (Peers(), IsPersistent(),
// Status().Duration); does not touch the active consensus data plane, only
// observes it.
//
// ActiveAnchors mirrors persistent_peers (config.toml's trusted bootstrap
// set every validator dials on startup): a known, pre-designated peer, as
// opposed to one discovered dynamically via PEX and therefore more exposed
// to Sybil/Eclipse manipulation. Counted by matching each connected peer's
// ID against persistentPeerIDs, NOT via p2p.Peer's own IsPersistent() --
// IsPersistent() matches by NetAddress (IP:port), and only the DIALING side
// of a connection has a NetAddress that can match a persistent_peers entry;
// the ACCEPTING side sees the remote peer's ephemeral source port instead,
// so IsPersistent() undercounts asymmetrically depending on who dialed whom.
// ID-based matching is symmetric.
//
// CleanPeers has no independent "blacklist" concept in this app yet (the
// fork's own HealthMonitor.Blacklist is likewise never called) -- counts
// total connected peers, a documented simplification matching that.
//
// Latency piggybacks on MConnection's existing PacketPing/PacketPong
// keep-alive (p2p.Peer.RTT()) rather than a new reactor/protobuf message --
// 0 per-peer until that peer's first exchange completes.
type vanillaP2PHealthAdapter struct {
	sw                *p2p.Switch
	persistentPeerIDs map[p2p.ID]bool

	mu          sync.Mutex
	firstSeen   map[p2p.ID]time.Time
	churnEvents []time.Time
}

// parsePersistentPeerIDs extracts just the "id@host:port" -> id portion of
// config.toml's persistent_peers string, for ActiveAnchors' ID-based
// matching (see vanillaP2PHealthAdapter's doc for why raw IsPersistent()
// isn't reliable). Malformed entries (no "@") are skipped rather than
// erroring -- this is a best-effort health signal, not consensus-critical
// parsing.
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
			a.churnEvents = append(a.churnEvents, now)
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
			a.churnEvents = append(a.churnEvents, now)
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
// mock to a live source. n.Switch() only exists after node.NewNode()
// returns, so this late-binds the source in rather than passing it at
// construction time -- runs before n.Start(), so no real proposal is ever
// processed with the mock source still active.
//
// Tries the fork's lp2p.Switch first (for if libp2p networking is ever
// enabled); falls back to vanillaP2PHealthAdapter against the standard
// p2p.Switch, which every real deployment has actually used to date.
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

// wirePeerFilter upgrades engramApp's ingress peer filter
// (peer_filter.go's FilterPeerByAddr) from its fail-open default to a live
// PeerFilterSource, mirroring wireP2PSensor's late-binding pattern. Only the
// vanilla *p2p.Switch case is wired (lp2p is dormant on every real
// deployment) -- stays a no-op (fail-open) if lp2p is ever enabled instead.
//
// Builds its own lightweight vanillaP2PHealthAdapter rather than reusing
// wireP2PSensor's instance, since PeerCountInSubnet only reads a.sw.Peers()
// fresh and needs none of the stateful churn-tracking fields.
func wirePeerFilter(engramApp *app.EngramApp, n *node.Node) {
	sw, ok := n.Switch().(*p2p.Switch)
	if !ok {
		return
	}
	engramApp.SovereigntyKeeper.SetPeerFilterSource(&vanillaP2PHealthAdapter{sw: sw})
}

// wireBTCSensor upgrades engramApp's BTC sensor from its static SetGap mock
// to a real bitcoind JSON-RPC connection, and wires an AnchorTracker against
// the same connection so h_btc_anchored has a real submission-and-confirmation
// pipeline behind it -- when BITCOIN_HOST is set. Left unwired (silent
// no-op) when unset.
//
// Requires a wallet with spendable funds already loaded on the connected
// bitcoind. This repo does not run the real babylonlabs/babylond vigilante
// images -- those expect a real Babylon chain's checkpointing modules this
// app doesn't have; AnchorTracker is a minimal in-process stand-in achieving
// the same submit-and-confirm goal without that dependency.
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
	engramApp.Sensors.Anchor = anchor.NewAnchorTracker(client, engramApp.SovereigntyKeeper.Params.KDeepFinality)
}

// wireDASensor upgrades engramApp's DA sensor from its static SetAvailable
// mock to a real celestia-bridge JSON-RPC connection, and wires a Publisher
// against the same connection so h_engram_verified has a real
// submission-and-retrieval pipeline behind it -- when CELESTIA_BRIDGE_URL is
// set. Left unwired (silent no-op) when unset.
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

// demoSMTCmd is the original SMT proof-of-concept (unrelated to the
// sovereignty Keeper's own SMT usage) -- kept as a standalone command
// instead of deleted.
func demoSMTCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "demo-smt",
		Short: "Standalone Sparse Merkle Tree proof-of-concept (unrelated to node state)",
		RunE: func(cmd *cobra.Command, args []string) error {
			nodes := smt.NewSimpleMap()
			values := smt.NewSimpleMap()
			tree := smt.NewSparseMerkleTree(nodes, values, sha256.New())

			key := []byte("user_1")
			val := []byte("state_anchored")
			if _, err := tree.Update(key, val); err != nil {
				return err
			}

			root := tree.Root()
			fmt.Printf("Current Root: %x\n", root)

			proof, err := tree.Prove(key)
			if err != nil {
				return err
			}

			valid := smt.VerifyProof(proof, root, key, val, sha256.New())
			fmt.Printf("Proof valid: %v\n", valid)
			return nil
		},
	}
}
