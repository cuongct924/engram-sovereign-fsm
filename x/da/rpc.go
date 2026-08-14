package da

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// rpcTimeout bounds every RPC call below, including publisher.go's background
// Submit that waits out Celestia's ~12s block time -- otherwise a stalled
// connection to a downed bridge hangs forever. Available() gets its own
// shorter deadline (see that call site) to stay within a consensus round.
const rpcTimeout = 20 * time.Second

// namespaceUserIDLen mirrors Celestia's v0 namespace layout: 1 version byte +
// 28-byte ID, left-padded with 18 zeros so only the last 10 bytes are
// caller-chosen.
const (
	namespaceVersion    = 0
	namespaceTotalLen   = 29
	namespaceUserIDLen  = 10
	namespacePaddingLen = namespaceTotalLen - 1 - namespaceUserIDLen // 18
)

// Namespace is a real 29-byte Celestia v0 namespace ID.
type Namespace [namespaceTotalLen]byte

// NewNamespace builds a v0 namespace from a caller-chosen id of at most 10
// bytes (e.g. "engramda01"), right-aligned into the ID field.
func NewNamespace(id string) (Namespace, error) {
	if len(id) > namespaceUserIDLen {
		return Namespace{}, fmt.Errorf("namespace id %q longer than %d bytes", id, namespaceUserIDLen)
	}
	var ns Namespace
	ns[0] = namespaceVersion
	copy(ns[namespaceTotalLen-len(id):], id)
	return ns, nil
}

func (n Namespace) base64() string {
	return base64.StdEncoding.EncodeToString(n[:])
}

// RPCClient is a minimal stdlib-only JSON-RPC 2.0 client for celestia-node,
// matching x/anchor/rpc.go's zero-dependency style.
type RPCClient struct {
	url       string
	authToken string
	client    *http.Client
}

// NewRPCClient builds a client against a celestia-node RPC endpoint (the
// docker bridge at http://127.0.0.1:26658). authToken is the admin/write JWT:
// blob.Submit is a write call celestia-node rejects unauthenticated.
func NewRPCClient(url, authToken string) *RPCClient {
	return &RPCClient{url: url, authToken: authToken, client: &http.Client{Timeout: rpcTimeout}}
}

// Reachable is a raw, stateless TCP liveness probe -- independent of the
// JSON-RPC cycle and Publisher's async bookkeeping, so every validator dialing
// the bridge at once sees the same up/down fact.
//
// Self-bounded by availableCheckTimeout, not rpcTimeout: DialContext has no
// timeout of its own, and 20s is too slow for a consensus-round check (a
// missing self-bound stalled consensus live, docs/EXPERIMENT.md E2 S6).
func (c *RPCClient) Reachable(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, availableCheckTimeout)
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

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
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
	body, err := json.Marshal(rpcRequest{JSONRPC: "2.0", ID: 1, Method: method, Params: params})
	if err != nil {
		return fmt.Errorf("marshal rpc request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build rpc request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("rpc request %s: %w", method, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read rpc response: %w", err)
	}

	var rpcResp rpcResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return fmt.Errorf("unmarshal rpc response (status %d): %w", resp.StatusCode, err)
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

// blobSubmitParam mirrors blob.Submit's request shape for celestia-node
// v0.14.1: [{namespace, data, share_version}], gas price as a plain float
// (not newer docs' state.TxConfig object).
type blobSubmitParam struct {
	Namespace    string `json:"namespace"`
	Data         string `json:"data"`
	ShareVersion int    `json:"share_version"`
}

// defaultGasPrice is the fixed gas price for every submission -- this is a
// local devnet (docker/celestia-local-cluster.yml), not a fee market.
const defaultGasPrice = 0.002

// maxSubmitSequenceRetries/submitSequenceRetryBackoff handle blob.Submit's
// "account sequence mismatch": all submitters share one bridge account, and
// concurrent Submits race its sequence tracking. A bounded backoff retry
// resyncs it instead of failing on the first race.
const (
	maxSubmitSequenceRetries   = 5
	submitSequenceRetryBackoff = 400 * time.Millisecond
)

func isSequenceMismatch(err error) bool {
	return err != nil && strings.Contains(err.Error(), "account sequence mismatch")
}

// Submit publishes data as a blob under namespace via blob.Submit, returning
// the Celestia block height it landed in -- the concrete counterpart of
// "publish this block to DA", the precondition of DANormalUpdate's
// h_engram_verified' = h_engram_current' (spec/core/EngramFSM.tla:196-201).
func (c *RPCClient) Submit(ctx context.Context, ns Namespace, data []byte) (uint64, error) {
	param := blobSubmitParam{
		Namespace:    ns.base64(),
		Data:         base64.StdEncoding.EncodeToString(data),
		ShareVersion: 0,
	}
	var height uint64
	var err error
	for attempt := 0; attempt <= maxSubmitSequenceRetries; attempt++ {
		err = c.call(ctx, "blob.Submit", []any{[]blobSubmitParam{param}, defaultGasPrice}, &height)
		if err == nil {
			return height, nil
		}
		if !isSequenceMismatch(err) || attempt == maxSubmitSequenceRetries {
			break
		}
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(submitSequenceRetryBackoff):
		}
	}
	return 0, fmt.Errorf("blob.Submit: %w", err)
}

// blobResult is blob.GetAll's per-blob response shape -- only the fields
// this package actually reads.
type blobResult struct {
	Namespace string `json:"namespace"`
	Data      string `json:"data"`
}

// Available calls blob.GetAll(height, [ns]) and reports whether any blob is
// retrievable under ns at that height -- the DAS check behind IsDAHealthy
// (spec/core/EngramFSM.tla:89). Unlike BTC's K-deep depth, DAS is binary:
// one successful retrieval is the whole check.
func (c *RPCClient) Available(ctx context.Context, celestiaHeight uint64, ns Namespace) (bool, error) {
	var blobs []blobResult
	err := c.call(ctx, "blob.GetAll", []any{celestiaHeight, []string{ns.base64()}}, &blobs)
	if err != nil {
		// celestia-node reports "blob: not found" (not an empty result) when
		// nothing is available yet -- treat that as "not yet available".
		if isBlobNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("blob.GetAll: %w", err)
	}
	return len(blobs) > 0, nil
}

func isBlobNotFound(err error) bool {
	return err != nil && bytes.Contains([]byte(err.Error()), []byte("blob: not found"))
}
