package da

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// namespaceUserIDLen mirrors Celestia's v0 namespace format: a 1-byte
// version prefix + 28-byte ID, where a v0 (user-specifiable) namespace's ID
// is left-padded with 18 zero bytes and only the last 10 bytes are
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
// bytes (e.g. "engramda01"), right-aligned into the 28-byte ID field per the
// v0 layout described above.
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

// RPCClient is a minimal celestia-node JSON-RPC 2.0 client -- deliberately
// stdlib-only (net/http, encoding/json), matching x/anchor/rpc.go's
// zero-dependency style rather than pulling in celestiaorg/celestia-node's
// full client SDK for the two calls (blob.Submit, blob.GetAll) this package
// actually needs.
type RPCClient struct {
	url       string
	authToken string
	client    *http.Client
}

// NewRPCClient builds a client against a celestia-node RPC endpoint (bridge
// or light node), e.g. the celestia-bridge service in
// docker/celestia-local-cluster.yml, http://127.0.0.1:26658. authToken is
// the node's admin/write JWT (celestia bridge auth admin) -- blob.Submit is
// a write call and celestia-node rejects it unauthenticated.
func NewRPCClient(url, authToken string) *RPCClient {
	return &RPCClient{url: url, authToken: authToken, client: &http.Client{}}
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

// blobSubmitParam mirrors the blob.Submit request shape for this
// celestia-node version (v0.14.1): [{namespace, data, share_version}], with
// gas price as a plain float parameter (not the `state.TxConfig` object
// newer upstream docs describe).
type blobSubmitParam struct {
	Namespace    string `json:"namespace"`
	Data         string `json:"data"`
	ShareVersion int    `json:"share_version"`
}

// defaultGasPrice is the flat gas price used for every blob submission --
// this is a local devnet/regtest-equivalent setup (see
// docker/celestia-local-cluster.yml), not a fee market, so a fixed value
// confirmed to work end-to-end is sufficient.
const defaultGasPrice = 0.002

// Submit calls blob.Submit with a single blob of data under namespace,
// returning the Celestia block height it landed in. This is the app's
// concrete counterpart of "publish this Engram block to the DA layer" --
// the precondition DANormalUpdate's h_engram_verified' = h_engram_current'
// (spec/core/EngramFSM.tla:196-201) depends on actually having happened.
func (c *RPCClient) Submit(ctx context.Context, ns Namespace, data []byte) (uint64, error) {
	param := blobSubmitParam{
		Namespace:    ns.base64(),
		Data:         base64.StdEncoding.EncodeToString(data),
		ShareVersion: 0,
	}
	var height uint64
	if err := c.call(ctx, "blob.Submit", []any{[]blobSubmitParam{param}, defaultGasPrice}, &height); err != nil {
		return 0, fmt.Errorf("blob.Submit: %w", err)
	}
	return height, nil
}

// blobResult is blob.GetAll's per-blob response shape -- only the fields
// this package actually reads.
type blobResult struct {
	Namespace string `json:"namespace"`
	Data      string `json:"data"`
}

// Available calls blob.GetAll(height, [ns]) and reports whether at least one
// blob is retrievable under ns at that Celestia height -- the concrete DAS
// check standing in for "the DA layer is publishing proofs within the
// allowed gap" (IsDAHealthy, spec/core/EngramFSM.tla:89). Unlike BTC's
// K-deep confirmation depth, Celestia's DAS is binary per spec (no depth
// concept in DANormalUpdate) -- a single successful retrieval is the whole
// check, matching the celestia-bridge blob.GetAll round-trip already
// confirmed manually to return the exact bytes submitted.
func (c *RPCClient) Available(ctx context.Context, celestiaHeight uint64, ns Namespace) (bool, error) {
	var blobs []blobResult
	err := c.call(ctx, "blob.GetAll", []any{celestiaHeight, []string{ns.base64()}}, &blobs)
	if err != nil {
		// celestia-node returns a "blob: not found" rpc error (not an empty
		// result) when nothing is available yet at that height/namespace --
		// treat that specific case as "not yet available", everything else
		// as a real error.
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
