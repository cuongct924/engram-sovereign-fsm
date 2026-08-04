package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	sdklog "cosmossdk.io/log/v2"
	"github.com/celestiaorg/smt"
	cmtcfg "github.com/cometbft/cometbft/config"
	cmtcrypto "github.com/cometbft/cometbft/crypto"
	cmtlog "github.com/cometbft/cometbft/libs/log"
	"github.com/cometbft/cometbft/node"
	"github.com/cometbft/cometbft/p2p"
	"github.com/cometbft/cometbft/privval"
	"github.com/cometbft/cometbft/proxy"
	cmttypes "github.com/cometbft/cometbft/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/server"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/cuongct220020/engram-sovereign-fsm/app"
	sovereigntytypes "github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
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

	appStateBytes, err := json.Marshal(map[string]json.RawMessage{
		"sovereignty": mustMarshalGenesis(sovereigntytypes.DefaultGenesis()),
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
	appStateBytes, err := json.Marshal(map[string]json.RawMessage{
		"sovereignty": mustMarshalGenesis(sovereigntytypes.DefaultGenesis()),
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
		// RPC defaults to 127.0.0.1 (cmtcfg.DefaultConfig()), which is
		// unreachable through a Docker port mapping from the host -- found by
		// actually running this in Docker: `docker compose ps` showed the
		// port mapped and the container healthy (the healthcheck runs
		// curl from inside the same container, so 127.0.0.1 works there),
		// but curl from the host got "Connection reset by peer" on the
		// mapped port. 0.0.0.0 is safe here since these are isolated
		// Docker networks, not public-internet-facing.
		config.RPC.ListenAddress = "tcp://0.0.0.0:26657"
		config.Instrumentation.Prometheus = true

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
// cmtcfg's defaults via viper -- cmtcfg.DefaultConfig() alone does NOT read
// anything from disk, so per-node customization (RPC/P2P ports, seeds,
// persistent_peers, instrumentation) written by `engramd init` or hand-edited
// for a multi-node testnet was previously silently ignored at `start` time
// (found by actually running two nodes with different config.toml ports
// side by side: the second one failed to bind, still trying the hardcoded
// default port). This mirrors CometBFT's own cmd/cometbft/commands bootstrap
// pattern (viper + mapstructure tags already on cmtcfg.Config's fields).
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
	engramApp := app.NewEngramApp(sdkLogger, db, genDoc.ChainID, vanilla)

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

	if err := n.Start(); err != nil {
		return fmt.Errorf("start node: %w", err)
	}
	fmt.Println("engramd started -- RPC:", config.RPC.ListenAddress)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	return n.Stop()
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
