package main

import (
	"testing"

	sovereigntytypes "github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/codec"
	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
)

// testEngramAddress returns a valid "engram"-HRP bech32 address as the
// placeholder Authority/Sender (mirroring the CLI's fixed placeholder), rather
// than a hand-typed literal whose checksum could be wrong.
func testEngramAddress(t *testing.T) string {
	t.Helper()
	addr, err := addresscodec.NewBech32Codec("engram").BytesToString(make([]byte, 20))
	require.NoError(t, err)
	return addr
}

func TestFsmStateToField(t *testing.T) {
	require.Equal(t, 3, fsmStateToField(sovereigntytypes.StateRecovering))
	require.Equal(t, 2, fsmStateToField(sovereigntytypes.StateSovereign),
		"HeaderHistory only ever tracks SOVEREIGN/RECOVERING (types.WithdrawLocked gate), so anything else falls to 2")
}

func TestBoolToField(t *testing.T) {
	require.Equal(t, 1, boolToField(true))
	require.Equal(t, 0, boolToField(false))
}

// TestBuildMinimalTx_IsDecodableByTheRealAppTxDecoder confirms buildMinimalTx's
// output round-trips through the SAME authtx.NewTxConfig TxDecoder app.go wires
// into BaseApp -- the actual real-world requirement.
func TestBuildMinimalTx_IsDecodableByTheRealAppTxDecoder(t *testing.T) {
	registry, err := newSovereigntyInterfaceRegistry()
	require.NoError(t, err)

	msg := &sovereigntytypes.MsgSubmitRecoveryProofRequest{
		Authority:    testEngramAddress(t),
		ZkProof:      []byte{0x01, 0x02, 0x03},
		PublicInputs: []byte{0x04, 0x05},
	}
	txBytes, err := buildMinimalTx(registry, msg)
	require.NoError(t, err)
	require.NotEmpty(t, txBytes)

	cdc := codec.NewProtoCodec(registry)
	txConfig := authtx.NewTxConfig(cdc, authtx.DefaultSignModes)
	decoded, err := txConfig.TxDecoder()(txBytes)
	require.NoError(t, err, "must decode via the same TxDecoder app.go's BaseApp uses")

	msgs := decoded.GetMsgs()
	require.Len(t, msgs, 1)
	got, ok := msgs[0].(*sovereigntytypes.MsgSubmitRecoveryProofRequest)
	require.True(t, ok)
	require.Equal(t, msg.Authority, got.Authority)
	require.Equal(t, msg.ZkProof, got.ZkProof)
	require.Equal(t, msg.PublicInputs, got.PublicInputs)
}

// TestBuildMinimalTx_IsDeterministic backs --dry-run's assumption
// (e8_cli.go): buildMinimalTx has no timestamp/nonce/random field, so encoding
// the same msg twice must give byte-identical output -- required for a driver
// to capture --dry-run's printed hex and later register/broadcast it as a
// separate, matching tx.
func TestBuildMinimalTx_IsDeterministic(t *testing.T) {
	registry, err := newSovereigntyInterfaceRegistry()
	require.NoError(t, err)

	msg := &sovereigntytypes.MsgSubmitForcedTxRequest{
		Sender: testEngramAddress(t),
		Tx:     []byte("TX_WITHDRAWAL"),
	}
	first, err := buildMinimalTx(registry, msg)
	require.NoError(t, err)
	second, err := buildMinimalTx(registry, msg)
	require.NoError(t, err)
	require.Equal(t, first, second)
}
