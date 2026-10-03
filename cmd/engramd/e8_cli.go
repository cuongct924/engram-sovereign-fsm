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
// arbitrary payload -- E8's live-test driver for A5 (--payload "TX_WITHDRAWAL",
// scanned by containsWithdrawal) and A7 (--payload a byzantine leader should
// censor via ENGRAM_BYZANTINE_BEHAVIOR=censor_tx:<payload>). Reuses
// buildMinimalTx/newSovereigntyInterfaceRegistry (reanchor_cli.go).
//
// --payload-hex/--dry-run drive A7: ForcedTxQueue matches msg.Tx verbatim
// against real req.Txs[1:] bytes, so the forced content must be another real
// tx's raw bytes -- --dry-run builds that tx and prints its hex so a driver
// can register and later broadcast byte-identical content.
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
			sender, err := addressCodec.BytesToString(make([]byte, 20)) // unused by SubmitForcedTx's handler; fixed placeholder is fine
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

			// Same BroadcastTxSync + manual poll as txSubmitRecoveryProofCmd
			// (see its doc).
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

// txGenForcedBatchCmd prints --count distinct, unsigned MsgSubmitForcedTx envelopes as hex, one per
// line, in a single process. The throughput sweep (scripts/e7_consensus_overhead/
// live_throughput_sweep.py) needs tens of thousands of transactions at 100+ tx/s; building each with a
// separate `tx-submit-forced-tx --dry-run` process made the generator, not the chain, the bottleneck.
func txGenForcedBatchCmd() *cobra.Command {
	var count, payloadBytes int
	var prefix string
	cmd := &cobra.Command{
		Use:   "tx-gen-forced-batch",
		Short: "Print --count distinct MsgSubmitForcedTx envelopes as hex, one per line (load generation)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if count <= 0 {
				return fmt.Errorf("--count must be positive")
			}
			registry, err := newSovereigntyInterfaceRegistry()
			if err != nil {
				return err
			}
			sender, err := addresscodec.NewBech32Codec("engram").BytesToString(make([]byte, 20))
			if err != nil {
				return err
			}
			for i := 0; i < count; i++ {
				payload := forcedBatchPayload(prefix, i, payloadBytes)
				txBytes, err := buildMinimalTx(registry, &sovereigntytypes.MsgSubmitForcedTxRequest{Sender: sender, Tx: payload})
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), hex.EncodeToString(txBytes))
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&count, "count", 0, "number of transactions")
	cmd.Flags().StringVar(&prefix, "prefix", "load", "payload prefix; keeps the batches of different runs distinct")
	cmd.Flags().IntVar(&payloadBytes, "payload-bytes", 32, "payload size in bytes (padded with 'x')")
	return cmd
}

// forcedBatchPayload is "<prefix>-<i>" padded to size bytes; it is never shorter than the label, so every
// index stays unique whatever size is asked for.
func forcedBatchPayload(prefix string, i, size int) []byte {
	label := fmt.Sprintf("%s-%d-", prefix, i)
	for len(label) < size {
		label += "x"
	}
	return []byte(label)
}
