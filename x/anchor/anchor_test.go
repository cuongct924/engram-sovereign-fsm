package anchor

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// fakeBitcoindHandler routes RPCClient's JSON-RPC calls to a per-method
// canned response and counts calls -- a self-contained stand-in for real
// bitcoind so AnchorTracker's submit/confirm state machine can be
// unit-tested without docker/bitcoin-regtest-cluster.yml (that's
// rpc_smoke_test.go's job, gated behind -tags btcsmoke).
type fakeBitcoindHandler struct {
	mu     sync.Mutex
	calls  map[string]int
	handle map[string]func() (any, error)
}

func newFakeBitcoind(t *testing.T) (*httptest.Server, *fakeBitcoindHandler) {
	t.Helper()
	h := &fakeBitcoindHandler{calls: map[string]int{}, handle: map[string]func() (any, error){}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))

		h.mu.Lock()
		h.calls[req.Method]++
		fn, ok := h.handle[req.Method]
		h.mu.Unlock()

		resp := map[string]any{"id": req.ID}
		switch {
		case !ok:
			t.Errorf("fakeBitcoind: unexpected method %q (no handler registered)", req.Method)
			resp["error"] = map[string]any{"code": -32601, "message": "method not found"}
		default:
			result, err := fn()
			if err != nil {
				resp["error"] = map[string]any{"code": -1, "message": err.Error()}
			} else {
				resp["result"] = result
			}
		}
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(resp))
	}))
	t.Cleanup(srv.Close)
	return srv, h
}

func (h *fakeBitcoindHandler) on(method string, fn func() (any, error)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.handle[method] = fn
}

func (h *fakeBitcoindHandler) callCount(method string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.calls[method]
}

// wireSuccessfulSubmit registers the 5-RPC chain SubmitOpReturn needs
// (createrawtransaction -> fundrawtransaction -> decoderawtransaction ->
// signrawtransactionwithwallet -> sendrawtransaction), plus lockunspent's
// deferred unlock, all with fixed canned success responses -- the shape every
// MaybeSubmit test that reaches an actual broadcast needs.
func (h *fakeBitcoindHandler) wireSuccessfulSubmit(txid string) {
	h.on("createrawtransaction", func() (any, error) { return "deadbeef", nil })
	h.on("fundrawtransaction", func() (any, error) {
		return map[string]any{"hex": "funded-deadbeef", "fee": 0.0001, "changepos": 1}, nil
	})
	h.on("decoderawtransaction", func() (any, error) {
		return map[string]any{"vin": []map[string]any{{"txid": "prevtx", "vout": 0}}}, nil
	})
	h.on("signrawtransactionwithwallet", func() (any, error) {
		return map[string]any{"hex": "signed-deadbeef", "complete": true}, nil
	})
	h.on("sendrawtransaction", func() (any, error) { return txid, nil })
	h.on("lockunspent", func() (any, error) { return true, nil })
}

func (h *fakeBitcoindHandler) wireConfirmation(confirmations int64, blockHeight uint64) {
	h.on("gettransaction", func() (any, error) {
		return map[string]any{"confirmations": confirmations, "blockheight": blockHeight}, nil
	})
}

func TestAnchorTracker_MaybeSubmit_BroadcastsWhenNoPending(t *testing.T) {
	srv, h := newFakeBitcoind(t)
	h.wireSuccessfulSubmit("txid-1")
	tracker := NewAnchorTracker(NewRPCClient(srv.URL, "user", "pass"), 2)

	require.NoError(t, tracker.MaybeSubmit(context.Background(), 42))
	require.Equal(t, 1, h.callCount("sendrawtransaction"), "must broadcast exactly one submission")

	_, ok := tracker.ConfirmedAnchorHeight()
	require.False(t, ok, "must not report confirmed before any gettransaction check")
}

func TestAnchorTracker_MaybeSubmit_DoesNotResubmitWhilePending(t *testing.T) {
	srv, h := newFakeBitcoind(t)
	h.wireSuccessfulSubmit("txid-1")
	h.wireConfirmation(0, 0) // still unconfirmed in mempool
	tracker := NewAnchorTracker(NewRPCClient(srv.URL, "user", "pass"), 2)

	require.NoError(t, tracker.MaybeSubmit(context.Background(), 42)) // broadcasts
	require.NoError(t, tracker.MaybeSubmit(context.Background(), 42)) // pending -> confirmation check only

	require.Equal(t, 1, h.callCount("sendrawtransaction"), "must not broadcast a second time while one tx is pending")
	require.Equal(t, 1, h.callCount("gettransaction"))

	_, ok := tracker.ConfirmedAnchorHeight()
	require.False(t, ok)
}

func TestAnchorTracker_MaybeSubmit_NotConfirmedBelowKDeepPlusOne(t *testing.T) {
	srv, h := newFakeBitcoind(t)
	h.wireSuccessfulSubmit("txid-1")
	tracker := NewAnchorTracker(NewRPCClient(srv.URL, "user", "pass"), 2) // needs kDeepFinality+1=3

	require.NoError(t, tracker.MaybeSubmit(context.Background(), 42))
	h.wireConfirmation(2, 100) // exactly kDeepFinality, not kDeepFinality+1
	require.NoError(t, tracker.MaybeSubmit(context.Background(), 42))

	_, ok := tracker.ConfirmedAnchorHeight()
	require.False(t, ok, "must require kDeepFinality+1 confirmations, not kDeepFinality (bitcoind's count is inclusive)")
}

// TestAnchorTracker_MaybeSubmit_ConfirmsAndImmediatelySubmitsNextInSameCall
// covers MaybeSubmit's "resolved -- free to submit the next one below"
// comment literally: the call that CONFIRMS a pending submission falls
// through, unconditionally, to broadcasting the next one in that same
// invocation -- callers relying on a separate later call to see the
// resubmission would be off by one.
func TestAnchorTracker_MaybeSubmit_ConfirmsAndImmediatelySubmitsNextInSameCall(t *testing.T) {
	srv, h := newFakeBitcoind(t)
	h.wireSuccessfulSubmit("txid-1")
	tracker := NewAnchorTracker(NewRPCClient(srv.URL, "user", "pass"), 2)

	require.NoError(t, tracker.MaybeSubmit(context.Background(), 42)) // submits txid-1
	h.wireConfirmation(3, 100)                                        // kDeepFinality+1
	require.NoError(t, tracker.MaybeSubmit(context.Background(), 43)) // confirms txid-1, then immediately resubmits

	height, ok := tracker.ConfirmedAnchorHeight()
	require.True(t, ok)
	require.Equal(t, uint64(100), height)
	require.Equal(t, 2, h.callCount("sendrawtransaction"), "confirming a pending submission must submit the next one in the SAME call")
}

func TestAnchorTracker_MaybeSubmit_PausedSkipsNewSubmissionWhenNoneIsPending(t *testing.T) {
	srv, h := newFakeBitcoind(t)
	tracker := NewAnchorTracker(NewRPCClient(srv.URL, "user", "pass"), 2)

	pauseFile := filepath.Join(t.TempDir(), "paused")
	require.NoError(t, os.WriteFile(pauseFile, nil, 0o644))
	tracker.SetSubmissionPausedFile(pauseFile)

	require.NoError(t, tracker.MaybeSubmit(context.Background(), 42))
	require.Equal(t, 0, h.callCount("sendrawtransaction"), "paused submission must never broadcast a new tx")
}

func TestAnchorTracker_MaybeSubmit_PausedStillConfirmsAlreadyPendingSubmission(t *testing.T) {
	srv, h := newFakeBitcoind(t)
	h.wireSuccessfulSubmit("txid-1")
	tracker := NewAnchorTracker(NewRPCClient(srv.URL, "user", "pass"), 2)
	require.NoError(t, tracker.MaybeSubmit(context.Background(), 42)) // broadcasts before pause

	pauseFile := filepath.Join(t.TempDir(), "paused")
	require.NoError(t, os.WriteFile(pauseFile, nil, 0o644))
	tracker.SetSubmissionPausedFile(pauseFile)

	h.wireConfirmation(3, 100)
	require.NoError(t, tracker.MaybeSubmit(context.Background(), 42))

	height, ok := tracker.ConfirmedAnchorHeight()
	require.True(t, ok, "an already-broadcast tx must still confirm while paused -- only NEW submissions are withheld")
	require.Equal(t, uint64(100), height)
}

func TestAnchorTracker_VerifyAnchor_AcceptsRealTaggedKDeepConfirmedBlock(t *testing.T) {
	srv, h := newFakeBitcoind(t)
	tagHex := fmt.Sprintf("%x", AnchorTag)
	h.on("getblockcount", func() (any, error) { return 105, nil })
	h.on("getblockhash", func() (any, error) { return "hash-100", nil })
	h.on("getblock", func() (any, error) {
		return map[string]any{
			"tx": []map[string]any{{
				"vout": []map[string]any{{
					"scriptPubKey": map[string]any{"hex": "6a04" + tagHex},
				}},
			}},
		}, nil
	})
	tracker := NewAnchorTracker(NewRPCClient(srv.URL, "user", "pass"), 2) // 105-100=5 >= kDeepFinality=2

	ok, err := tracker.VerifyAnchor(context.Background(), 100)
	require.NoError(t, err)
	require.True(t, ok)
}

func TestAnchorTracker_VerifyAnchor_RejectsBelowKDeepFinality(t *testing.T) {
	srv, h := newFakeBitcoind(t)
	h.on("getblockcount", func() (any, error) { return 101, nil }) // only 1 block ahead
	tracker := NewAnchorTracker(NewRPCClient(srv.URL, "user", "pass"), 2)

	ok, err := tracker.VerifyAnchor(context.Background(), 100)
	require.NoError(t, err)
	require.False(t, ok, "current-height=101 - claimed=100 = 1 < kDeepFinality=2, and must short-circuit before even reading the block")
}

func TestAnchorTracker_VerifyAnchor_RejectsBlockWithoutOurTag(t *testing.T) {
	srv, h := newFakeBitcoind(t)
	h.on("getblockcount", func() (any, error) { return 105, nil })
	h.on("getblockhash", func() (any, error) { return "hash-100", nil })
	h.on("getblock", func() (any, error) {
		return map[string]any{
			"tx": []map[string]any{{
				"vout": []map[string]any{{
					"scriptPubKey": map[string]any{"hex": "76a914deadbeef88ac"}, // ordinary P2PKH, not OP_RETURN
				}},
			}},
		}, nil
	})
	tracker := NewAnchorTracker(NewRPCClient(srv.URL, "user", "pass"), 2)

	ok, err := tracker.VerifyAnchor(context.Background(), 100)
	require.NoError(t, err)
	require.False(t, ok, "a real, deep-enough block with no matching OP_RETURN tag must not verify")
}
