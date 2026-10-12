package sovereignty

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper/sensors"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// slowBTC answers like a healthy bitcoind at height 5 until stall is set,
// after which every call blocks for delay (a hung or very slow RPC).
type slowBTC struct {
	stall atomic.Bool
	delay time.Duration
}

func (b *slowBTC) wait(ctx context.Context) {
	if b.stall.Load() {
		select {
		case <-time.After(b.delay):
		case <-ctx.Done():
		}
	}
}
func (b *slowBTC) Reachable(ctx context.Context) bool { b.wait(ctx); return true }
func (b *slowBTC) CurrentHeight(ctx context.Context) (uint64, error) {
	b.wait(ctx)
	return 5, nil
}

func extendOnce(t *testing.T, h func() (*abci.ResponseExtendVote, error)) (*types.PeripheralMetrics, time.Duration) {
	t.Helper()
	start := time.Now()
	resp, err := h()
	d := time.Since(start)
	require.NoError(t, err)
	var m types.PeripheralMetrics
	require.NoError(t, proto.Unmarshal(resp.VoteExtension, &m))
	return &m, d
}

func TestExtendVoteSnapshot_BoundedUnderStalledRPC_StaleReadsDegradedThenRecovers(t *testing.T) {
	t.Setenv("ENGRAM_SENSOR_INTERVAL_MS", "20")
	t.Setenv("ENGRAM_SENSOR_MAX_AGE_MS", "300")
	k, ctx := newVoteExtTestKeeperCtx(t)
	ctx = ctx.WithBlockHeight(10)
	src := &slowBTC{delay: 30 * time.Second}
	s := &Sensors{BTC: sensors.NewBTCSensor(), DA: sensors.NewDASensor(), P2P: sensors.NewP2PSensor()}
	s.BTC.SetSource(src)
	handler := NewExtendVoteHandler(k, s, "")
	call := func() (*abci.ResponseExtendVote, error) { return handler(ctx, &abci.RequestExtendVote{Height: 10}) }

	// Nothing observed yet: unknown reads as the worst case, not healthy.
	m, _ := extendOnce(t, call)
	require.Equal(t, k.Params.SovereignThreshold, m.BtcGap)

	// Once a reading lands, the real gap is reported.
	require.Eventually(t, func() bool {
		m, _ = extendOnce(t, call)
		return m.BtcGap < k.Params.SovereignThreshold
	}, 3*time.Second, 20*time.Millisecond)
	require.Equal(t, uint64(5), m.BtcCurrentHeight)

	// RPC hangs completely: ExtendVote stays fast, and once the last healthy
	// reading ages out the sensor reads as degraded, never healthy forever.
	src.stall.Store(true)
	require.Eventually(t, func() bool {
		var d time.Duration
		m, d = extendOnce(t, call)
		require.Less(t, d, 100*time.Millisecond)
		return m.BtcGap == k.Params.SovereignThreshold
	}, 3*time.Second, 20*time.Millisecond)
	require.Zero(t, m.BtcCurrentHeight)

	// Sensor data returns: the next fresh reading is reported again.
	src.stall.Store(false)
	require.Eventually(t, func() bool {
		m, _ = extendOnce(t, call)
		return m.BtcGap < k.Params.SovereignThreshold
	}, 40*time.Second, 50*time.Millisecond)
}
