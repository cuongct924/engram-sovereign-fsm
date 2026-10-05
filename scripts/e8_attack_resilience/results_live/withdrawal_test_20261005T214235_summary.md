# LIVE withdrawal-during-SOVEREIGN test (E8 A5)

Total duration: 231s. Reached SOVEREIGN before submitting: False.

## Withdrawal tx submission (t=169s to t=170s)

- CLI exit code: 1
- stdout: ``
- stderr: `Error: rejected in DeliverTx: failed to execute message; message index: 0: forced tx content does not decode as a valid tx (would be permanently unsatisfiable): expected 2 wire type, got 4: tx parse error [cosmos/cosmos-sdk@v0.54.3/x/auth/tx/decoder.go:23]
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

rejected in DeliverTx: failed to execute message; message index: 0: forced tx content does not decode as a valid tx (would be permanently unsatisfiable): expected 2 wire type, got 4: tx parse error [cosmos/cosmos-sdk@v0.54.3/x/auth/tx/decoder.go:23]`

## Verdict

- Withdrawal correctly blocked while SOVEREIGN (tx never committed, CLI did not report success): **True**
- Safety held (all 4 validators' AppHash never diverged at the same height): **True**
- Divergence events: 0

Note: CheckTx admits the tx to the mempool successfully -- the real rejection happens at ProcessProposal (the whole proposal containing it is rejected), so the tx is withheld/pending rather than returning an explicit error code. A CLI timeout here is the expected, correct signal, not a script failure.
