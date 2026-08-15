package main

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"time"

	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	"github.com/cosmos/gogoproto/proto"
	"github.com/cuongct220020/engram-sovereign-fsm/x/da"
	sovereigntytypes "github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/codec"
	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	txsigning "github.com/cosmos/cosmos-sdk/x/tx/signing"
)

// newSovereigntyInterfaceRegistry builds the same InterfaceRegistry shape as
// app.go (real bech32 address codec, "engram" HRP) -- required because
// MsgSubmitRecoveryProofRequest's `authority` field has GetSigners()
// auto-derived via address-codec decoding.
func newSovereigntyInterfaceRegistry() (codectypes.InterfaceRegistry, error) {
	registry, err := codectypes.NewInterfaceRegistryWithOptions(codectypes.InterfaceRegistryOptions{
		ProtoFiles: proto.HybridResolver,
		SigningOptions: txsigning.Options{
			AddressCodec:          addresscodec.NewBech32Codec("engram"),
			ValidatorAddressCodec: addresscodec.NewBech32Codec("engramvaloper"),
		},
	})
	if err != nil {
		return nil, err
	}
	sovereigntytypes.RegisterInterfaces(registry)
	return registry, nil
}

// queryStateCmd dumps this node's current FSM snapshot via the same real
// ABCI query path as queryRecoveryHeadersCmd (no separate gRPC server) --
// previously only reachable by hand-decoding /abci_query's raw protobuf (see
// scripts/framework/logger.py's _decode_query_state, which exists solely
// because no CLI did this).
func queryStateCmd() *cobra.Command {
	var nodeURL string
	cmd := &cobra.Command{
		Use:   "query-state",
		Short: "Query this node's current FSM state and peripheral metrics",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := rpchttp.New(nodeURL, "/websocket")
			if err != nil {
				return err
			}
			reqBytes, err := proto.Marshal(&sovereigntytypes.QueryStateRequest{})
			if err != nil {
				return err
			}
			resp, err := client.ABCIQuery(context.Background(), "/engram.sovereignty.v1.Query/State", reqBytes)
			if err != nil {
				return err
			}
			if resp.Response.Code != 0 {
				return fmt.Errorf("query failed: %s", resp.Response.Log)
			}
			var out sovereigntytypes.QueryStateResponse
			if err := proto.Unmarshal(resp.Response.Value, &out); err != nil {
				return err
			}

			fmt.Printf("fsm_state=%s safe_blocks=%d suspicious_duration=%d reanchoring_proof_valid=%t\n",
				out.FsmState, out.SafeBlocks, out.SuspiciousDuration, out.ReanchoringProofValid)
			if m := out.Metrics; m != nil {
				fmt.Printf("btc_gap=%d is_btc_spv_failed=%t da_gap=%d is_das_failed=%t is_attestation_failed=%t\n",
					m.BtcGap, m.IsBtcSpvFailed, m.DaGap, m.IsDasFailed, m.IsAttestationFailed)
				fmt.Printf("subnet_diversity=%d active_anchors=%d clean_peers=%d peer_churn_rate=%d avg_peer_tenure=%d peer_latency=%d\n",
					m.SubnetDiversity, m.ActiveAnchors, m.CleanPeers, m.PeerChurnRate, m.AvgPeerTenure, m.PeerLatency)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&nodeURL, "node", "http://127.0.0.1:26657", "CometBFT RPC endpoint")
	return cmd
}

// queryRecoveryHeadersCmd dumps the current SOVEREIGN/RECOVERING interval's
// tracked header history via the real ABCI query path (CometBFT's
// /abci_query, routed by BaseApp to the GRPCQueryRouter registered in
// app.go -- no separate gRPC server), in a line-based format
// scripts/reanchoring_prover.sh (A6) parses to build the real Noir witness.
func queryRecoveryHeadersCmd() *cobra.Command {
	var nodeURL string
	cmd := &cobra.Command{
		Use:   "query-recovery-headers",
		Short: "Query this node's currently tracked ZK re-anchoring header history",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := rpchttp.New(nodeURL, "/websocket")
			if err != nil {
				return err
			}
			reqBytes, err := proto.Marshal(&sovereigntytypes.QueryRecoveryHeadersRequest{})
			if err != nil {
				return err
			}
			resp, err := client.ABCIQuery(context.Background(), "/engram.sovereignty.v1.Query/RecoveryHeaders", reqBytes)
			if err != nil {
				return err
			}
			if resp.Response.Code != 0 {
				return fmt.Errorf("query failed: %s", resp.Response.Log)
			}
			var out sovereigntytypes.QueryRecoveryHeadersResponse
			if err := proto.Unmarshal(resp.Response.Value, &out); err != nil {
				return err
			}

			fmt.Printf("rt_last=%s\n", new(big.Int).SetBytes(out.LastAnchoredRoot).String())
			for _, h := range out.Headers {
				fmt.Printf("height=%d fsm_state=%d withdrawal_locked=%d state_root=%s\n",
					h.Height, fsmStateToField(h.FsmState), boolToField(h.WithdrawalLocked),
					new(big.Int).SetBytes(h.StateRoot).String())
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&nodeURL, "node", "http://127.0.0.1:26657", "CometBFT RPC endpoint")
	return cmd
}

// fsmStateToField mirrors circuit/reanchoring/src/main.nr's Header.fsm_state
// encoding: 2: SOVEREIGN, 3: RECOVERING (the only two values HeaderHistory
// ever stores, per preblock.go's types.WithdrawLocked gate).
func fsmStateToField(state string) int {
	if state == sovereigntytypes.StateRecovering {
		return 3
	}
	return 2
}

func boolToField(b bool) int {
	if b {
		return 1
	}
	return 0
}

// publishRecoveryWitnessCmd publishes a proof's real header-chain witness
// (the same data prove_and_submit.sh feeds the circuit) to Celestia DA before
// the proof is submitted. Pure audit trail: SubmitRecoveryProof never verifies
// this blob, and HeaderHistory (the on-chain source of this data) gets pruned
// once the proof is accepted -- without this, no late joiner/auditor could
// retrieve the real header chain a past proof was built from. Concrete-only,
// no spec line (see msg_server.go's RecoveryProofDAHeights doc).
func publishRecoveryWitnessCmd() *cobra.Command {
	var headersFile string
	cmd := &cobra.Command{
		Use:   "publish-recovery-witness",
		Short: "Publish a recovery proof's real header-chain witness to Celestia DA, for audit after HeaderHistory is pruned",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(headersFile)
			if err != nil {
				return fmt.Errorf("reading headers file: %w", err)
			}

			url := os.Getenv("CELESTIA_BRIDGE_URL")
			if url == "" {
				return fmt.Errorf("CELESTIA_BRIDGE_URL not set")
			}
			authToken := os.Getenv("CELESTIA_BRIDGE_AUTH_TOKEN")
			namespaceID := os.Getenv("CELESTIA_RECOVERY_NAMESPACE_ID")
			if namespaceID == "" {
				namespaceID = "engramrp01" // distinct from block-data's engramda01
			}
			ns, err := da.NewNamespace(namespaceID)
			if err != nil {
				return fmt.Errorf("invalid CELESTIA_RECOVERY_NAMESPACE_ID: %w", err)
			}

			client := da.NewRPCClient(url, authToken)
			height, err := client.Submit(context.Background(), ns, data)
			if err != nil {
				return fmt.Errorf("blob.Submit: %w", err)
			}
			fmt.Printf("da_celestia_height=%d\n", height)
			return nil
		},
	}
	cmd.Flags().StringVar(&headersFile, "headers", "", "path to a file with the real header-chain witness data (required)")
	_ = cmd.MarkFlagRequired("headers")
	return cmd
}

// txSubmitRecoveryProofCmd builds and broadcasts a real
// MsgSubmitRecoveryProofRequest. There is no x/auth/x/bank in this prototype
// and the ante chain never checks a signature, so instead of a real signed tx
// (which would need a keyring/account this chain can't query), it builds the
// minimal structurally-valid envelope BaseApp's TxDecoder requires: one
// SignerInfo, one empty signature.
func txSubmitRecoveryProofCmd() *cobra.Command {
	var nodeURL, proofFile, publicInputsFile string
	var daCelestiaHeight uint64
	cmd := &cobra.Command{
		Use:   "tx-submit-recovery-proof",
		Short: "Submit a real ZK re-anchoring proof (spec/README.md's Proof of Recovery)",
		RunE: func(cmd *cobra.Command, args []string) error {
			proofBytes, err := os.ReadFile(proofFile)
			if err != nil {
				return fmt.Errorf("reading proof file: %w", err)
			}
			publicInputsBytes, err := os.ReadFile(publicInputsFile)
			if err != nil {
				return fmt.Errorf("reading public inputs file: %w", err)
			}

			registry, err := newSovereigntyInterfaceRegistry()
			if err != nil {
				return err
			}
			addressCodec := addresscodec.NewBech32Codec("engram")
			authority, err := addressCodec.BytesToString(make([]byte, 20)) // unused by any check today (see doc above); a fixed placeholder is fine
			if err != nil {
				return err
			}

			msg := &sovereigntytypes.MsgSubmitRecoveryProofRequest{
				Authority:        authority,
				ZkProof:          proofBytes,
				PublicInputs:     publicInputsBytes,
				DaCelestiaHeight: daCelestiaHeight,
			}
			txBytes, err := buildMinimalTx(registry, msg)
			if err != nil {
				return err
			}

			client, err := rpchttp.New(nodeURL, "/websocket")
			if err != nil {
				return err
			}

			// BroadcastTxCommit blocks the RPC server ~10s waiting for
			// DeliverTx -- a real bottleneck against this testnet's occasional
			// round-skip stalls. BroadcastTxSync returns as soon as CheckTx
			// passes; DeliverTx's real result is polled here directly with a
			// timeout matched to this testnet's worst-case block latency.
			syncResult, err := client.BroadcastTxSync(context.Background(), txBytes)
			if err != nil {
				return err
			}
			if syncResult.Code != 0 {
				return fmt.Errorf("rejected in CheckTx: %s", syncResult.Log)
			}

			const pollTimeout = 30 * time.Second
			const pollInterval = 300 * time.Millisecond
			deadline := time.Now().Add(pollTimeout)
			for {
				txResult, txErr := client.Tx(context.Background(), syncResult.Hash, false)
				if txErr == nil {
					if txResult.TxResult.Code != 0 {
						return fmt.Errorf("rejected in DeliverTx: %s", txResult.TxResult.Log)
					}
					fmt.Printf("submitted at height %d, hash %s\n", txResult.Height, syncResult.Hash)
					return nil
				}
				if time.Now().After(deadline) {
					return fmt.Errorf("passed CheckTx (hash %s) but DeliverTx result not observed within %s -- likely still pending in a slow/round-skipping block, not necessarily rejected", syncResult.Hash, pollTimeout)
				}
				time.Sleep(pollInterval)
			}
		},
	}
	cmd.Flags().StringVar(&nodeURL, "node", "http://127.0.0.1:26657", "CometBFT RPC endpoint")
	cmd.Flags().StringVar(&proofFile, "proof", "", "path to the bb-generated proof file (required)")
	cmd.Flags().StringVar(&publicInputsFile, "public-inputs", "", "path to the bb-generated public_inputs file (required)")
	cmd.Flags().Uint64Var(&daCelestiaHeight, "da-height", 0, "Celestia height this proof's witness header chain was published at (optional, audit-only, see publish-recovery-witness)")
	_ = cmd.MarkFlagRequired("proof")
	_ = cmd.MarkFlagRequired("public-inputs")
	return cmd
}

// buildMinimalTx encodes msg into the minimal structurally-valid sdk.Tx
// envelope this chain's TxDecoder (authtx.NewTxConfig, app.go) accepts --
// see txSubmitRecoveryProofCmd's doc for why a real signature isn't needed.
func buildMinimalTx(registry codectypes.InterfaceRegistry, msg sdk.Msg) ([]byte, error) {
	cdc := codec.NewProtoCodec(registry)

	msgAny, err := codectypes.NewAnyWithValue(msg)
	if err != nil {
		return nil, err
	}
	body := &txtypes.TxBody{Messages: []*codectypes.Any{msgAny}}
	bodyBytes, err := cdc.Marshal(body)
	if err != nil {
		return nil, err
	}

	authInfo := &txtypes.AuthInfo{
		SignerInfos: []*txtypes.SignerInfo{
			{
				ModeInfo: &txtypes.ModeInfo{
					Sum: &txtypes.ModeInfo_Single_{Single: &txtypes.ModeInfo_Single{Mode: signingtypes.SignMode_SIGN_MODE_DIRECT}},
				},
				Sequence: 0,
			},
		},
		Fee: &txtypes.Fee{},
	}
	authInfoBytes, err := cdc.Marshal(authInfo)
	if err != nil {
		return nil, err
	}

	txRaw := &txtypes.TxRaw{
		BodyBytes:     bodyBytes,
		AuthInfoBytes: authInfoBytes,
		Signatures:    [][]byte{{}},
	}
	return cdc.Marshal(txRaw)
}
