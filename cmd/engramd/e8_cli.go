package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	sovereigntytypes "github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
	"github.com/spf13/cobra"

	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
)

// txSubmitForcedTxCmd broadcasts a real MsgSubmitForcedTxRequest carrying an
// arbitrary raw payload, for docs/EXPERIMENT.md's E8 live tests:
//
//   - A5 (Withdrawal During SOVEREIGN): --payload containing the literal
//     "TX_WITHDRAWAL" marker containsWithdrawal scans for.
//   - A7 (Censorship): --payload identifying tx content to force into
//     ForcedTxQueue, for a byzantine leader (ENGRAM_BYZANTINE_BEHAVIOR=
//     censor_tx:<payload>) to then be observed omitting.
//
// Reuses buildMinimalTx/newSovereigntyInterfaceRegistry (reanchor_cli.go) --
// same minimal, intentionally-unsigned tx envelope (no x/auth signing flow
// in this prototype).
//
// --payload-hex and --dry-run exist for A7's censorship driver: ForcedTxQueue
// stores msg.Tx verbatim, matched against real req.Txs[1:] bytes -- so the
// "forced" tx must be some OTHER real, independently decodable tx's raw
// bytes, not a plain string. --dry-run builds that other tx deterministically
// and prints its hex without broadcasting, so a driver script can capture
// byte-identical content for both a --payload-hex registration call and a
// separate real broadcast.
func txSubmitForcedTxCmd() *cobra.Command {
	var nodeURL, payload, payloadHex string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "tx-submit-forced-tx",
		Short: "Submit a MsgSubmitForcedTx carrying --payload (E8's A5/A7 live test driver)",
		RunE: func(cmd *cobra.Command, args []string) error {
			var payloadBytes []byte
			switch {
			case payloadHex != "":
				b, err := hex.DecodeString(payloadHex)
				if err != nil {
					return fmt.Errorf("--payload-hex: %w", err)
				}
				payloadBytes = b
			case payload != "":
				payloadBytes = []byte(payload)
			default:
				return fmt.Errorf("--payload or --payload-hex is required")
			}

			registry, err := newSovereigntyInterfaceRegistry()
			if err != nil {
				return err
			}
			addressCodec := addresscodec.NewBech32Codec("engram")
			sender, err := addressCodec.BytesToString(make([]byte, 20)) // unused by SubmitForcedTx's handler; a fixed placeholder is fine
			if err != nil {
				return err
			}

			msg := &sovereigntytypes.MsgSubmitForcedTxRequest{
				Sender: sender,
				Tx:     payloadBytes,
			}
			txBytes, err := buildMinimalTx(registry, msg)
			if err != nil {
				return err
			}

			if dryRun {
				fmt.Println(hex.EncodeToString(txBytes))
				return nil
			}

			client, err := rpchttp.New(nodeURL, "/websocket")
			if err != nil {
				return err
			}

			// Same BroadcastTxSync + manual poll pattern as
			// txSubmitRecoveryProofCmd, same reasoning (see its doc).
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
					fmt.Printf("submitted at height %d, hash %s, payload_hex %s\n",
						txResult.Height, syncResult.Hash, hex.EncodeToString(payloadBytes))
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
	cmd.Flags().StringVar(&payload, "payload", "", "raw payload for MsgSubmitForcedTxRequest.Tx (mutually exclusive with --payload-hex)")
	cmd.Flags().StringVar(&payloadHex, "payload-hex", "", "hex-encoded payload for MsgSubmitForcedTxRequest.Tx -- for byte-exact content (e.g. another tx's raw bytes), see this command's doc")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "build the tx and print its hex-encoded raw bytes, without broadcasting")
	return cmd
}
