package main

import (
	"bytes"
	"encoding/hex"
	"io"
	"os"
	"strings"
	"testing"

	sovereigntytypes "github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/codec"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
)

// runForcedTxDryRun invokes txSubmitForcedTxCmd's RunE directly (bypassing
// cobra's arg-parsing/usage machinery, matching reanchor_cli_test.go's
// preference for exercising real behavior over mocks) and captures --dry-run's
// output, since it writes via fmt.Println directly to os.Stdout rather than
// cmd.OutOrStdout().
func runForcedTxDryRun(t *testing.T, flags map[string]string) (string, error) {
	t.Helper()
	cmd := txSubmitForcedTxCmd()
	for name, val := range flags {
		require.NoError(t, cmd.Flags().Set(name, val))
	}

	r, w, err := os.Pipe()
	require.NoError(t, err)
	origStdout := os.Stdout
	os.Stdout = w

	runErr := cmd.RunE(cmd, nil)

	w.Close()
	os.Stdout = origStdout
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	return strings.TrimSpace(buf.String()), runErr
}

// decodeForcedTx decodes dry-run's printed hex via the same TxDecoder
// app.go's BaseApp uses (mirroring
// TestBuildMinimalTx_IsDecodableByTheRealAppTxDecoder in reanchor_cli_test.go)
// and returns the single MsgSubmitForcedTxRequest it carries.
func decodeForcedTx(t *testing.T, hexOut string) *sovereigntytypes.MsgSubmitForcedTxRequest {
	t.Helper()
	txBytes, err := hex.DecodeString(hexOut)
	require.NoError(t, err)

	registry, err := newSovereigntyInterfaceRegistry()
	require.NoError(t, err)
	cdc := codec.NewProtoCodec(registry)
	txConfig := authtx.NewTxConfig(cdc, authtx.DefaultSignModes)
	decoded, err := txConfig.TxDecoder()(txBytes)
	require.NoError(t, err, "must decode via the same TxDecoder app.go's BaseApp uses")

	msgs := decoded.GetMsgs()
	require.Len(t, msgs, 1)
	got, ok := msgs[0].(*sovereigntytypes.MsgSubmitForcedTxRequest)
	require.True(t, ok)
	return got
}

func TestSubmitForcedTx_RequiresPayload(t *testing.T) {
	_, err := runForcedTxDryRun(t, map[string]string{"dry-run": "true"})
	require.ErrorContains(t, err, "--payload or --payload-hex is required")
}

func TestSubmitForcedTx_InvalidPayloadHex(t *testing.T) {
	_, err := runForcedTxDryRun(t, map[string]string{"dry-run": "true", "payload-hex": "not-hex"})
	require.ErrorContains(t, err, "--payload-hex:")
}

// TestSubmitForcedTx_DryRunPayload_ProducesDecodableTx confirms --payload's
// raw string bytes land unmodified in MsgSubmitForcedTxRequest.Tx -- A5's
// live driver matches on this exact marker (e8_cli.go's doc: "TX_WITHDRAWAL").
func TestSubmitForcedTx_DryRunPayload_ProducesDecodableTx(t *testing.T) {
	out, err := runForcedTxDryRun(t, map[string]string{"dry-run": "true", "payload": "TX_WITHDRAWAL"})
	require.NoError(t, err)

	got := decodeForcedTx(t, out)
	require.Equal(t, []byte("TX_WITHDRAWAL"), got.Tx)
}

// TestSubmitForcedTx_DryRunPayloadHex_MatchesDecodedBytes confirms
// --payload-hex round-trips as raw bytes, not the hex string itself -- A7's
// censorship driver needs byte-exact content matching another real tx.
func TestSubmitForcedTx_DryRunPayloadHex_MatchesDecodedBytes(t *testing.T) {
	out, err := runForcedTxDryRun(t, map[string]string{"dry-run": "true", "payload-hex": "deadbeef"})
	require.NoError(t, err)

	got := decodeForcedTx(t, out)
	require.Equal(t, []byte{0xde, 0xad, 0xbe, 0xef}, got.Tx)
}

// TestSubmitForcedTx_PayloadHexTakesPrecedence documents the switch
// statement's order in e8_cli.go: --payload-hex is checked first, so passing
// both is not an error -- --payload-hex silently wins.
func TestSubmitForcedTx_PayloadHexTakesPrecedence(t *testing.T) {
	out, err := runForcedTxDryRun(t, map[string]string{
		"dry-run":     "true",
		"payload":     "ignored",
		"payload-hex": "deadbeef",
	})
	require.NoError(t, err)

	got := decodeForcedTx(t, out)
	require.Equal(t, []byte{0xde, 0xad, 0xbe, 0xef}, got.Tx)
}

// TestSubmitForcedTx_DryRunIsDeterministic backs the same assumption as
// reanchor_cli_test.go's TestBuildMinimalTx_IsDeterministic: no
// timestamp/nonce/random field, so a driver can capture --dry-run's hex once
// and later register/broadcast byte-identical content.
func TestSubmitForcedTx_DryRunIsDeterministic(t *testing.T) {
	first, err := runForcedTxDryRun(t, map[string]string{"dry-run": "true", "payload": "TX_WITHDRAWAL"})
	require.NoError(t, err)
	second, err := runForcedTxDryRun(t, map[string]string{"dry-run": "true", "payload": "TX_WITHDRAWAL"})
	require.NoError(t, err)
	require.Equal(t, first, second)
}
