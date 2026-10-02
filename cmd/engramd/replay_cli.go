package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	rpcclient "github.com/cometbft/cometbft/rpc/client"
	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	cmttypes "github.com/cometbft/cometbft/types"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty"
	sovereigntytypes "github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"
)

// debugCmd groups operator/experiment diagnostics that read a running chain.
func debugCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "debug", Short: "Diagnostics over a running chain (E8's semantic checks)"}
	cmd.AddCommand(replayFSMCmd())
	return cmd
}

// replayFSMCmd re-derives every committed block's FSM transition from its
// Txs[0] and reports violations of the protocol's own semantics. "All honest
// nodes agree on AppHash" only shows they ran the same code; this shows the
// committed transitions are the ones that code allows for the signed
// observations in the block (docs/EXPERIMENT.md's E8 collusion suite).
func replayFSMCmd() *cobra.Command {
	var rpcURL, genesisPath, honestList, byzantineList, outPath string
	var from, to, reportFrom int64
	var checkCommitted, failOnViolation bool
	cmd := &cobra.Command{
		Use:   "replay-fsm",
		Short: "Replay committed blocks through the FSM and report semantic violations",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := rpchttp.New(rpcURL, "/websocket")
			if err != nil {
				return err
			}
			ctx := context.Background()

			doc, err := loadReplayGenesis(ctx, client, genesisPath)
			if err != nil {
				return err
			}
			gs, err := sovereigntyGenesis(doc.AppState)
			if err != nil {
				return err
			}
			honest, err := honestAddresses(doc.Validators, honestList, byzantineList)
			if err != nil {
				return err
			}
			replayer, err := sovereignty.NewReplayer(doc.ChainID, gs, doc.Validators, honest)
			if err != nil {
				return err
			}

			if to == 0 {
				st, err := client.Status(ctx)
				if err != nil {
					return err
				}
				to = st.SyncInfo.LatestBlockHeight
			}
			if from < doc.InitialHeight || from < 1 {
				from = max(doc.InitialHeight, 1)
			}

			var violations []sovereignty.ReplayViolation
			committedWarned := false
			for h := from; h <= to; h++ {
				blk, err := client.Block(ctx, &h)
				if err != nil {
					return fmt.Errorf("block %d: %w", h, err)
				}
				txs := make([][]byte, len(blk.Block.Txs))
				for i, tx := range blk.Block.Txs {
					txs[i] = tx
				}
				vs, err := replayer.ApplyBlock(h, blk.Block.AppHash, txs)
				if err != nil {
					return err
				}
				violations = append(violations, vs...)

				if checkCommitted {
					onChain, qerr := committedStateAt(ctx, client, h)
					switch {
					case qerr != nil:
						if !committedWarned {
							fmt.Fprintf(os.Stderr, "replay-fsm: cannot read committed state at height %d (%v); skipping --check-committed\n", h, qerr)
							committedWarned = true
						}
					case onChain != replayer.FSMState():
						violations = append(violations, sovereignty.ReplayViolation{
							Height: h,
							Kind:   sovereignty.ViolationCommittedStateDiffer,
							Detail: fmt.Sprintf("on-chain %s, replay %s", onChain, replayer.FSMState()),
						})
					}
				}
			}

			// Replay always starts at genesis (the FSM carries state), but a case
			// that only misbehaved from some height reports from there on.
			violations = slices.DeleteFunc(violations, func(v sovereignty.ReplayViolation) bool { return v.Height < reportFrom })

			if err := writeViolationsCSV(outPath, violations); err != nil {
				return err
			}
			fmt.Printf("replay-fsm: heights %d..%d, %d honest validators, final state %s, %d violations\n",
				from, to, len(honest), replayer.FSMState(), len(violations))
			for _, v := range violations {
				fmt.Printf("  height=%d kind=%s %s\n", v.Height, v.Kind, v.Detail)
			}
			if failOnViolation && len(violations) > 0 {
				return fmt.Errorf("%d violations", len(violations))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&rpcURL, "rpc", "http://127.0.0.1:26657", "CometBFT RPC endpoint of an honest node")
	cmd.Flags().StringVar(&genesisPath, "genesis", "", "genesis.json (default: fetched from --rpc)")
	cmd.Flags().Int64Var(&from, "from", 1, "first height to replay")
	cmd.Flags().Int64Var(&reportFrom, "report-from", 0, "only report violations at heights >= this (the replay itself still runs from --from)")
	cmd.Flags().Int64Var(&to, "to", 0, "last height to replay (0 = latest)")
	cmd.Flags().StringVar(&honestList, "honest", "", "comma-separated monikers of the honest validators the range check trusts")
	cmd.Flags().StringVar(&byzantineList, "byzantine", "", "comma-separated monikers of the Byzantine validators (honest = every other validator)")
	cmd.Flags().StringVar(&outPath, "out", "replay_violations.csv", "violations CSV (height,kind,detail)")
	cmd.Flags().BoolVar(&checkCommitted, "check-committed", true, "also compare each replayed state with the node's committed State query at that height")
	cmd.Flags().BoolVar(&failOnViolation, "fail-on-violation", false, "exit non-zero when any violation is found")
	return cmd
}

func loadReplayGenesis(ctx context.Context, client *rpchttp.HTTP, path string) (*cmttypes.GenesisDoc, error) {
	if path != "" {
		return cmttypes.GenesisDocFromFile(path)
	}
	res, err := client.Genesis(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch genesis from RPC: %w", err)
	}
	return res.Genesis, nil
}

// sovereigntyGenesis parses the app_state the same way app.newInitChainer does.
func sovereigntyGenesis(appState json.RawMessage) (*sovereigntytypes.GenesisState, error) {
	gs := sovereigntytypes.DefaultGenesis()
	if len(appState) == 0 {
		return gs, nil
	}
	var parsed struct {
		Sovereignty json.RawMessage `json:"sovereignty"`
	}
	if err := json.Unmarshal(appState, &parsed); err != nil || len(parsed.Sovereignty) == 0 {
		return gs, nil
	}
	gs = &sovereigntytypes.GenesisState{}
	if err := json.Unmarshal(parsed.Sovereignty, gs); err != nil {
		return nil, fmt.Errorf("genesis sovereignty state: %w", err)
	}
	return gs, nil
}

// honestAddresses resolves monikers (genesis validator names) to addresses.
// Neither flag set disables the honest-range check.
func honestAddresses(validators []cmttypes.GenesisValidator, honestList, byzantineList string) ([][]byte, error) {
	if honestList != "" && byzantineList != "" {
		return nil, fmt.Errorf("--honest and --byzantine are mutually exclusive")
	}
	if honestList == "" && byzantineList == "" {
		return nil, nil
	}
	byName := map[string][]byte{}
	for _, v := range validators {
		byName[v.Name] = v.Address
	}
	named := func(list string) (map[string]bool, error) {
		set := map[string]bool{}
		for _, n := range strings.Split(list, ",") {
			if n = strings.TrimSpace(n); n == "" {
				continue
			}
			if _, ok := byName[n]; !ok {
				return nil, fmt.Errorf("unknown validator moniker %q", n)
			}
			set[n] = true
		}
		return set, nil
	}
	var out [][]byte
	if honestList != "" {
		set, err := named(honestList)
		if err != nil {
			return nil, err
		}
		for _, v := range validators {
			if set[v.Name] {
				out = append(out, v.Address)
			}
		}
		return out, nil
	}
	set, err := named(byzantineList)
	if err != nil {
		return nil, err
	}
	for _, v := range validators {
		if !set[v.Name] {
			out = append(out, v.Address)
		}
	}
	return out, nil
}

func committedStateAt(ctx context.Context, client *rpchttp.HTTP, height int64) (string, error) {
	reqBytes, err := proto.Marshal(&sovereigntytypes.QueryStateRequest{})
	if err != nil {
		return "", err
	}
	resp, err := client.ABCIQueryWithOptions(ctx, "/engram.sovereignty.v1.Query/State", reqBytes, rpcclient.ABCIQueryOptions{Height: height})
	if err != nil {
		return "", err
	}
	if resp.Response.Code != 0 {
		return "", fmt.Errorf("query failed: %s", resp.Response.Log)
	}
	var out sovereigntytypes.QueryStateResponse
	if err := proto.Unmarshal(resp.Response.Value, &out); err != nil {
		return "", err
	}
	return out.FsmState, nil
}

func writeViolationsCSV(path string, violations []sovereignty.ReplayViolation) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.Write([]string{"height", "kind", "detail"}); err != nil {
		return err
	}
	for _, v := range violations {
		if err := w.Write([]string{strconv.FormatInt(v.Height, 10), v.Kind, v.Detail}); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}
