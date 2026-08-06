package main

import (
	"context"
	"fmt"
	"math/big"
	"os"

	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	"github.com/cosmos/cosmos-sdk/codec"
	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	txsigning "github.com/cosmos/cosmos-sdk/x/tx/signing"
	"github.com/cosmos/gogoproto/proto"
	"github.com/spf13/cobra"

	sovereigntytypes "github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
)

// newSovereigntyInterfaceRegistry builds the same InterfaceRegistry shape as
// app.go's NewEngramApp (real bech32 address codec, "engram" HRP) -- needed
// here too since MsgSubmitRecoveryProofRequest's `authority` field has its
// GetSigners() auto-derived via address-codec decoding (see app.go's own doc
// on this, found by actually broadcasting a tx against a running node).
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

// queryRecoveryHeadersCmd dumps the CURRENT SOVEREIGN/RECOVERING interval's
// tracked header history via the real ABCI query path (CometBFT's
// /abci_query, which BaseApp routes internally to the GRPCQueryRouter
// registered in app.go -- no separate gRPC server needed), in a plain
// line-based format scripts/reanchoring_prover.sh (A6) parses directly to
// build the real Noir witness (see that script for the exact consumer).
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
// encoding: "2: SOVEREIGN, 3: RECOVERING" (the only two values HeaderHistory
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

// txSubmitRecoveryProofCmd builds and broadcasts a real
// MsgSubmitRecoveryProofRequest. There is no x/auth/x/bank in this
// prototype (app/app.go's own doc) and the ante chain
// (app.NewCircuitBreakerDecorator only) never checks a signature -- so
// rather than a real signed tx (which would need a keyring and an account
// this chain has no way to query), this builds the minimal structurally
// valid tx envelope BaseApp's TxDecoder requires: one SignerInfo/one
// (empty) signature, matching what nothing here actually verifies. Found
// and confirmed by actually broadcasting against a running node (not
// guessed): an empty AuthInfo/Fee decodes and routes fine once app.go's
// MsgServiceRouter/QueryServer registration and address codec (both fixed
// alongside this command, see app.go's doc) are in place.
func txSubmitRecoveryProofCmd() *cobra.Command {
	var nodeURL, proofFile, publicInputsFile string
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
				Authority:    authority,
				ZkProof:      proofBytes,
				PublicInputs: publicInputsBytes,
			}
			txBytes, err := buildMinimalTx(registry, msg)
			if err != nil {
				return err
			}

			client, err := rpchttp.New(nodeURL, "/websocket")
			if err != nil {
				return err
			}
			result, err := client.BroadcastTxCommit(context.Background(), txBytes)
			if err != nil {
				return err
			}
			if result.CheckTx.Code != 0 {
				return fmt.Errorf("rejected in CheckTx: %s", result.CheckTx.Log)
			}
			if result.TxResult.Code != 0 {
				return fmt.Errorf("rejected in DeliverTx: %s", result.TxResult.Log)
			}
			fmt.Printf("submitted at height %d, hash %s\n", result.Height, result.Hash)
			return nil
		},
	}
	cmd.Flags().StringVar(&nodeURL, "node", "http://127.0.0.1:26657", "CometBFT RPC endpoint")
	cmd.Flags().StringVar(&proofFile, "proof", "", "path to the bb-generated proof file (required)")
	cmd.Flags().StringVar(&publicInputsFile, "public-inputs", "", "path to the bb-generated public_inputs file (required)")
	cmd.MarkFlagRequired("proof")
	cmd.MarkFlagRequired("public-inputs")
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
