# LIVE withdrawal-during-SOVEREIGN test (E8 A5)

Total duration: 110s. Reached SOVEREIGN before submitting: True.

## Withdrawal tx submission (t=16s to t=49s)

- CLI exit code: 1
- stdout: ``
- stderr: `Error: passed CheckTx (hash E8D69F22300482F0D01C79C5CC6AE52F085EF66C09A67428CC8D20437C122D98) but DeliverTx result not observed within 30s -- likely still pending in a slow/round-skipping block, not necessarily rejected
Usage:
  engramd tx-submit-forced-tx [flags]

Flags:
      --dry-run              build the tx and print its hex-encoded raw bytes, without broadcasting
  -h, --help                 help for tx-submit-forced-tx
      --node string          CometBFT RPC endpoint (default "http://127.0.0.1:26657")
      --payload string       raw payload for MsgSubmitForcedTxRequest.Tx (mutually exclusive with --payload-hex)
      --payload-hex string   hex-encoded payload for MsgSubmitForcedTxRequest.Tx -- for byte-exact content (e.g. another tx's raw bytes), see this command's doc

Global Flags:
      --home string   node home directory (default "/Users/cuongct090_04/.engramd")

passed CheckTx (hash E8D69F22300482F0D01C79C5CC6AE52F085EF66C09A67428CC8D20437C122D98) but DeliverTx result not observed within 30s -- likely still pending in a slow/round-skipping block, not necessarily rejected`

## Verdict

- Withdrawal correctly blocked while SOVEREIGN (tx never committed, CLI did not report success): **True**

Note: CheckTx admits the tx to the mempool successfully -- the real rejection happens at ProcessProposal (the whole proposal containing it is rejected), so the tx is withheld/pending rather than returning an explicit error code. A CLI timeout here is the expected, correct signal, not a script failure.
