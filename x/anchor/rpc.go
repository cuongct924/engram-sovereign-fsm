package anchor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"
)

// rpcTimeout bounds every call below. Every one of them runs synchronously
// inside RefreshMetrics (x/sovereignty/sensors_refresh.go), itself called
// from PrepareProposal/ProcessProposal -- an http.Client with no Timeout can
// hang on a stalled connection to a downed bitcoind indefinitely, stalling
// ABCI (and so all of consensus) rather than degrading through btc_gap as
// designed.
//
// Bounded well under CometBFT's timeout_propose (3s default,
// config.toml) -- a single RefreshMetrics call can chain multiple
// sequential RPC calls (CurrentHeight, VerifyAnchor, MaybeSubmit), each
// against this SAME client, so an individual timeout anywhere near
// timeout_propose still stalls the whole round if 2+ of them are slow at
// once (confirmed live: 5s here alongside x/da's 3s produced "ProposalBlock
// is nil" round-skips for the whole duration of a combined BTC+DA outage,
// not just a graceful degrade). 800ms is still >>40x regtest's normal
// sub-20ms latency.
const rpcTimeout = 800 * time.Millisecond

// RPCClient is a minimal Bitcoin Core JSON-RPC client -- deliberately
// stdlib-only (net/http, encoding/json), matching this package's existing
// zero-dependency style, rather than pulling in btcsuite/btcd's much larger
// dependency tree for what is currently a single call (getblockcount).
// Plays the observing role of Babylon's real Vigilante Monitor daemon
// (github.com/babylonlabs-io/vigilante) -- see CurrentHeight's doc.
type RPCClient struct {
	url      string
	user     string
	password string
	client   *http.Client
}

// NewRPCClient builds a client against a Bitcoin Core-compatible JSON-RPC
// endpoint (e.g. the bitcoin-node01 regtest service in
// docker/bitcoin-regtest-cluster.yml, http://127.0.0.1:18443).
func NewRPCClient(url, user, password string) *RPCClient {
	return &RPCClient{
		url:      url,
		user:     user,
		password: password,
		client:   &http.Client{Timeout: rpcTimeout},
	}
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      string `json:"id"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *rpcError       `json:"error"`
}

func (c *RPCClient) call(ctx context.Context, method string, params []any, result any) error {
	body, err := json.Marshal(rpcRequest{JSONRPC: "1.0", ID: "engram-anchor", Method: method, Params: params})
	if err != nil {
		return fmt.Errorf("marshal rpc request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build rpc request: %w", err)
	}
	req.SetBasicAuth(c.user, c.password)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("rpc request %s: %w", method, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read rpc response: %w", err)
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusInternalServerError {
		// Bitcoin Core returns 500 with a JSON-RPC error body for RPC-level
		// errors (e.g. block not found) -- only a genuinely unexpected status
		// (auth failure, wrong port) short-circuits here.
		return fmt.Errorf("rpc request %s: unexpected status %d: %s", method, resp.StatusCode, string(respBody))
	}

	var rpcResp rpcResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return fmt.Errorf("unmarshal rpc response: %w", err)
	}
	if rpcResp.Error != nil {
		return fmt.Errorf("rpc error %s: %d %s", method, rpcResp.Error.Code, rpcResp.Error.Message)
	}
	if result != nil {
		if err := json.Unmarshal(rpcResp.Result, result); err != nil {
			return fmt.Errorf("unmarshal rpc result: %w", err)
		}
	}
	return nil
}

// Reachable reports whether c's host:port currently accepts a TCP
// connection -- a raw, stateless liveness probe deliberately independent of
// getblockcount/gettransaction's own request/response cycle, mirroring
// x/da/rpc.go's RPCClient.Reachable (see that doc for the full rationale:
// every honest validator dialing the same bitcoind at roughly the same
// instant observes the same binary up/down fact, unlike a JSON-RPC call
// whose success can depend on which validator's connection happens to be
// stale vs freshly (re)established -- confirmed live as the source of
// cross-validator btc_gap disagreement during a real bitcoind outage).
//
// Self-bounded by rpcTimeout, same as every other call in this file --
// net.Dialer.DialContext has no timeout of its own beyond whatever deadline
// ctx already carries, and the real PrepareProposal/ProcessProposal ctx
// carries none. Confirmed live: with an unbounded ctx, a stopped bitcoind
// left connect() hanging on the OS's own default TCP connect timeout
// (~90-100s observed), stalling every proposer's round -- not a graceful
// btc_gap degrade as designed, a full liveness halt (docs/EXPERIMENT.md's
// E2 S6). x/da/rpc.go's own Reachable has the identical gap; d.Publisher.
// ProbeHealthy already wraps its call in a bounded context, which is why
// this bug only ever showed up on the BTC side.
func (c *RPCClient) Reachable(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, rpcTimeout)
	defer cancel()

	u, err := url.Parse(c.url)
	if err != nil {
		return false
	}
	host := u.Host
	if u.Port() == "" {
		switch u.Scheme {
		case "https":
			host = net.JoinHostPort(u.Hostname(), "443")
		default:
			host = net.JoinHostPort(u.Hostname(), "80")
		}
	}
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", host)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// CurrentHeight calls getblockcount, returning the connected node's current
// chain-tip height -- H_current in spec/README.md §4.1's finality-gap
// formula. Implements sensors.BTCHeightSource.
func (c *RPCClient) CurrentHeight(ctx context.Context) (uint64, error) {
	var height uint64
	if err := c.call(ctx, "getblockcount", nil, &height); err != nil {
		return 0, err
	}
	return height, nil
}

// BlockHashAt calls getblockhash(height), returning the real Bitcoin block
// hash at that height. Not consulted by VerifySPVProof (see that function's
// doc for why ExpectedBlockHash's abstracted <<"BTC_BLOCK", height>> hash
// stays as the spec-fidelity consensus-level check) -- used by AnchorTracker
// instead, whose independent-per-validator verification is real, not
// spec-abstracted (see anchor.go).
func (c *RPCClient) BlockHashAt(ctx context.Context, height uint64) (string, error) {
	var hash string
	if err := c.call(ctx, "getblockhash", []any{height}, &hash); err != nil {
		return "", err
	}
	return hash, nil
}

// GetNewAddress calls getnewaddress -- regtest/dev tooling only (mining
// rewards need somewhere to go); not used by any production sensor path.
func (c *RPCClient) GetNewAddress(ctx context.Context) (string, error) {
	var addr string
	if err := c.call(ctx, "getnewaddress", nil, &addr); err != nil {
		return "", err
	}
	return addr, nil
}

// GenerateToAddress calls generatetoaddress(n, address) -- regtest/dev
// tooling only, mines n blocks instantly (regtest has no real PoW
// difficulty); not used by any production sensor path.
func (c *RPCClient) GenerateToAddress(ctx context.Context, n int, address string) error {
	return c.call(ctx, "generatetoaddress", []any{n, address}, nil)
}

// utxoRef identifies one input by (txid, vout) -- the shape `lockunspent`
// and `decoderawtransaction`'s vin entries both use.
type utxoRef struct {
	Txid string `json:"txid"`
	Vout uint32 `json:"vout"`
}

// SubmitOpReturn broadcasts a zero-value transaction carrying payload in an
// OP_RETURN output -- this fork's minimal stand-in for Babylon's Vigilante
// Submitter. Requires a wallet loaded on the connected bitcoind with
// spendable funds (regtest: `createwallet` + mine to one of its addresses).
// Returns the broadcast transaction's txid.
//
// This repo's 4 validators share one bitcoind wallet, so every validator
// calls this every block -- fundrawtransaction's coin selection and the
// lock protecting it must be ATOMIC, or two validators can select the same
// UTXO before either one locks it (the previous version called
// fundrawtransaction, THEN decoderawtransaction, THEN a separate
// lockunspent -- a real TOCTOU window between "select" and "lock" that let
// concurrent submissions race on the same input in practice, live-confirmed
// via "lockunspent: -8 Invalid parameter, output already locked"
// and every submission failing/retrying forever, never actually anchoring).
// Passing lockUnspents to fundrawtransaction itself makes bitcoind select
// AND lock the chosen inputs as one atomic RPC call -- no separate
// lockunspent(false, ...) needed, and no window for another validator's
// concurrent fundrawtransaction to pick the same input.
func (c *RPCClient) SubmitOpReturn(ctx context.Context, payload []byte) (string, error) {
	dataHex := fmt.Sprintf("%x", payload)

	var rawHex string
	if err := c.call(ctx, "createrawtransaction", []any{[]any{}, []any{map[string]any{"data": dataHex}}}, &rawHex); err != nil {
		return "", fmt.Errorf("createrawtransaction: %w", err)
	}

	var funded struct {
		Hex string `json:"hex"`
	}
	if err := c.call(ctx, "fundrawtransaction", []any{rawHex, map[string]any{"lockUnspents": true}}, &funded); err != nil {
		return "", fmt.Errorf("fundrawtransaction: %w", err)
	}

	var decoded struct {
		Vin []utxoRef `json:"vin"`
	}
	if err := c.call(ctx, "decoderawtransaction", []any{funded.Hex}, &decoded); err != nil {
		return "", fmt.Errorf("decoderawtransaction: %w", err)
	}
	// Inputs are already locked (lockUnspents above) -- this only unlocks
	// them once we're done, whether signing/broadcasting below succeeds or
	// fails, so a failed attempt's inputs are free for the next retry.
	if len(decoded.Vin) > 0 {
		defer func() {
			_ = c.call(context.Background(), "lockunspent", []any{true, decoded.Vin}, nil)
		}()
	}

	var signed struct {
		Hex      string `json:"hex"`
		Complete bool   `json:"complete"`
	}
	if err := c.call(ctx, "signrawtransactionwithwallet", []any{funded.Hex}, &signed); err != nil {
		return "", fmt.Errorf("signrawtransactionwithwallet: %w", err)
	}
	if !signed.Complete {
		return "", fmt.Errorf("signrawtransactionwithwallet: incomplete signature")
	}

	var txid string
	if err := c.call(ctx, "sendrawtransaction", []any{signed.Hex}, &txid); err != nil {
		return "", fmt.Errorf("sendrawtransaction: %w", err)
	}
	return txid, nil
}

// TxConfirmation calls gettransaction(txid), returning how many
// confirmations it has and (once mined) the height of the block it's in.
// mined is false while the transaction is still unconfirmed in the mempool
// (Confirmations 0), in which case height is meaningless.
func (c *RPCClient) TxConfirmation(ctx context.Context, txid string) (confirmations int64, height uint64, mined bool, err error) {
	var resp struct {
		Confirmations int64  `json:"confirmations"`
		BlockHeight   uint64 `json:"blockheight"`
	}
	if err := c.call(ctx, "gettransaction", []any{txid}, &resp); err != nil {
		return 0, 0, false, fmt.Errorf("gettransaction: %w", err)
	}
	return resp.Confirmations, resp.BlockHeight, resp.Confirmations > 0, nil
}
