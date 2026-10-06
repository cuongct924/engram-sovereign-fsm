// Package weighted tests AggregateHealthCertificates with unequal voting power. The live testbed and the TLA+ model use
// equal power, where "weight" and "count" coincide, so a defect that confuses them cannot show there. Lemma 1 (aggregate
// within the honest range while the Byzantine weight is below W/3) and the 2W/3 quorum are checked here against an
// independent reference: the weighted median is the element at index floor(T/2) of the multiset in which each value is
// repeated by its power.
package weighted

import (
	"math/rand"
	"sort"
	"testing"

	"github.com/cometbft/cometbft/crypto/ed25519"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	cmttypes "github.com/cometbft/cometbft/types"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
	"google.golang.org/protobuf/proto"

	"cosmossdk.io/collections/colltest"
	log "cosmossdk.io/log/v2"

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	chainID = "sovereignty-weighted"
	height  = int64(10)
)

// distributions: the first is the one proposed for the paper; the others put the 2W/3 line between reachable sums.
var distributions = map[string][]int64{
	"skewed":    {30, 20, 15, 10, 10, 10, 5},
	"irregular": {26, 19, 17, 13, 11, 8, 6},
	"three":     {34, 33, 33},
	"equal":     {1, 1, 1, 1, 1, 1, 1},
}

type fixture struct {
	k      *keeper.Keeper
	ctx    sdk.Context
	powers []int64
	keys   []ed25519.PrivKey
}

func newFixture(tb testing.TB, powers []int64) *fixture {
	tb.Helper()
	storeService, ctx := colltest.MockStore()
	k := keeper.NewKeeper(storeService, codec.NewProtoCodec(codectypes.NewInterfaceRegistry()))
	sctx := sdk.NewContext(nil, cmtproto.Header{}, false, log.NewNopLogger()).WithContext(ctx).WithChainID(chainID)
	f := &fixture{k: k, ctx: sctx, powers: powers}
	for _, p := range powers {
		priv := ed25519.GenPrivKey()
		pub := priv.PubKey().(ed25519.PubKey)
		if err := k.ValidatorPubKeys.Set(sctx, string(pub.Address()), types.ValidatorInfo{PubKey: pub.Bytes(), Power: p}); err != nil {
			tb.Fatal(err)
		}
		f.keys = append(f.keys, priv)
	}
	return f
}

// cert signs the metrics of validator i; the same signature scheme as the live vote extension.
func (f *fixture) cert(tb testing.TB, i int, m *types.PeripheralMetrics) *types.SignedHealthCertificate {
	tb.Helper()
	ext, err := proto.Marshal(m)
	if err != nil {
		tb.Fatal(err)
	}
	sig, err := f.keys[i].Sign(cmttypes.VoteExtensionSignBytes(chainID, &cmtproto.Vote{Height: height, Round: 0, Extension: ext}))
	if err != nil {
		tb.Fatal(err)
	}
	return &types.SignedHealthCertificate{ValidatorAddress: f.keys[i].PubKey().Address(), Round: 0, MetricsExtension: ext, ExtensionSignature: sig}
}

func (f *fixture) aggregate(certs []*types.SignedHealthCertificate) (*types.PeripheralMetrics, bool) {
	return sovereignty.AggregateHealthCertificates(f.ctx, f.k, chainID, height, certs)
}

func total(p []int64) (t int64) {
	for _, x := range p {
		t += x
	}
	return
}

// refMedian is the reference weighted median: the element at index floor(T/2) of the power-expanded sorted multiset.
func refMedian(vals []uint64, powers []int64) uint64 {
	var exp []uint64
	for i, v := range vals {
		for j := int64(0); j < powers[i]; j++ {
			exp = append(exp, v)
		}
	}
	sort.Slice(exp, func(a, b int) bool { return exp[a] < exp[b] })
	return exp[len(exp)/2]
}

func refMajority(vals []bool, powers []int64) bool {
	var yes, all int64
	for i, v := range vals {
		all += powers[i]
		if v {
			yes += powers[i]
		}
	}
	return 2*yes > all
}

func weight(mask int, p []int64) (w int64) {
	for i := range p {
		if mask>>i&1 == 1 {
			w += p[i]
		}
	}
	return
}

func quorum(p []int64) int64 { return total(p)*2/3 + 1 }

// TestQuorumStraddle: a verified power of 2W/3+1 or more aggregates, one below it does not, for every subset.
func TestQuorumStraddle(t *testing.T) {
	for name, p := range distributions {
		f := newFixture(t, p)
		m := &types.PeripheralMetrics{BtcGap: 1}
		for mask := 1; mask < 1<<len(p); mask++ {
			var certs []*types.SignedHealthCertificate
			for i := range p {
				if mask>>i&1 == 1 {
					certs = append(certs, f.cert(t, i, m))
				}
			}
			_, ok := f.aggregate(certs)
			want := weight(mask, p) >= quorum(p)
			if ok != want {
				t.Fatalf("%s subset %b power %d quorum %d: ok=%v want %v", name, mask, weight(mask, p), quorum(p), ok, want)
			}
		}
	}
}

// TestAgainstReference: random values, random quorum-reaching subsets, every field against the reference.
func TestAgainstReference(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for name, p := range distributions {
		f := newFixture(t, p)
		for trial := 0; trial < 1500; trial++ {
			var mask int
			for mask == 0 || weight(mask, p) < quorum(p) {
				mask = rng.Intn(1 << len(p))
			}
			var certs []*types.SignedHealthCertificate
			var gap, da []uint64
			var das []bool
			var pw []int64
			for i := range p {
				if mask>>i&1 == 0 {
					continue
				}
				m := &types.PeripheralMetrics{BtcGap: uint64(rng.Intn(12)), DaGap: uint64(rng.Intn(40)), IsDasFailed: rng.Intn(2) == 0}
				certs = append(certs, f.cert(t, i, m))
				gap, da, das, pw = append(gap, m.BtcGap), append(da, m.DaGap), append(das, m.IsDasFailed), append(pw, p[i])
			}
			got, ok := f.aggregate(certs)
			if !ok {
				t.Fatalf("%s: quorum subset %b rejected", name, mask)
			}
			if got.BtcGap != refMedian(gap, pw) || got.DaGap != refMedian(da, pw) || got.IsDasFailed != refMajority(das, pw) {
				t.Fatalf("%s subset %b: got gap %d da %d das %v, reference %d %d %v", name, mask, got.BtcGap, got.DaGap, got.IsDasFailed,
					refMedian(gap, pw), refMedian(da, pw), refMajority(das, pw))
			}
		}
	}
}

// TestHonestRange enumerates every Byzantine coalition and every omission pattern the proposer can choose and checks
// Lemma 1: with Byzantine weight below W/3 the aggregate lies within the honest values of the included certificates.
// The adversary tries both extremes and a split. It also records the smallest Byzantine weight at which the range is
// escaped, which should sit at or above W/3 (the bound is tight only if some coalition just above it succeeds).
func TestHonestRange(t *testing.T) {
	for name, p := range distributions {
		f := newFixture(t, p)
		w := total(p)
		n := len(p)
		maxSafe, minBreak := int64(-1), int64(1<<62)
		for byz := 0; byz < 1<<n; byz++ {
			bw := weight(byz, p)
			for omit := 0; omit < 1<<n; omit++ {
				inc := (1<<n - 1) &^ omit
				if weight(inc, p) < quorum(p) {
					continue
				}
				for pattern := 0; pattern < 3; pattern++ { // 0 low, 1 high, 2 split by index parity
					var certs []*types.SignedHealthCertificate
					var hmin, hmax uint64 = 1 << 62, 0
					honestPresent := false
					for i := 0; i < n; i++ {
						if inc>>i&1 == 0 {
							continue
						}
						m := &types.PeripheralMetrics{}
						if byz>>i&1 == 1 {
							switch pattern {
							case 0:
								m.BtcGap = 0
							case 1:
								m.BtcGap = 1000
							default:
								m.BtcGap = uint64(i%2) * 1000
							}
							m.IsDasFailed = true
						} else {
							m.BtcGap = uint64(50 + 10*i) // honest readings differ
							if m.BtcGap < hmin {
								hmin = m.BtcGap
							}
							if m.BtcGap > hmax {
								hmax = m.BtcGap
							}
							honestPresent = true
						}
						certs = append(certs, f.cert(t, i, m))
					}
					if !honestPresent {
						continue
					}
					got, ok := f.aggregate(certs)
					if !ok {
						t.Fatalf("%s: quorum subset rejected", name)
					}
					inRange := got.BtcGap >= hmin && got.BtcGap <= hmax
					boolOK := !got.IsDasFailed // honest all report false
					if 3*bw < w && (!inRange || !boolOK) {
						t.Fatalf("%s: Byzantine weight %d < W/3 (W=%d), included %b, pattern %d: gap %d outside [%d,%d] or boolean flipped", name, bw, w, inc, pattern, got.BtcGap, hmin, hmax)
					}
					if 3*bw < w && bw > maxSafe {
						maxSafe = bw
					}
					if (!inRange || !boolOK) && bw < minBreak {
						minBreak = bw
					}
				}
			}
		}
		t.Logf("%s (W=%d, W/3=%.1f): largest Byzantine weight checked below W/3 = %d; smallest weight that escaped the honest range = %d", name, w, float64(w)/3, maxSafe, minBreak)
		if minBreak < 1<<62 && 3*minBreak < w {
			t.Fatalf("%s: a coalition below W/3 escaped", name)
		}
	}
}

// TestWeightNotCount: three light validators on one side and one heavy validator on the other; the median follows weight.
func TestWeightNotCount(t *testing.T) {
	f := newFixture(t, []int64{30, 20, 15, 10, 10, 10, 5})
	vals := []uint64{900, 100, 100, 100, 100, 100, 900} // powers 30 and 5 at 900, the rest (65) at 100
	var certs []*types.SignedHealthCertificate
	for i, v := range vals {
		certs = append(certs, f.cert(t, i, &types.PeripheralMetrics{BtcGap: v}))
	}
	got, ok := f.aggregate(certs)
	if !ok || got.BtcGap != 100 {
		t.Fatalf("median %d ok=%v, want 100 (65%% of the weight)", got.BtcGap, ok)
	}
	// a count majority on the other side: three of seven report 900 but hold 45% of the weight
	vals = []uint64{100, 900, 900, 900, 100, 100, 100} // power 30+... at 100 = 30+10+10+5=55 ; at 900 = 20+15+10=45
	certs = nil
	for i, v := range vals {
		certs = append(certs, f.cert(t, i, &types.PeripheralMetrics{BtcGap: v}))
	}
	got, _ = f.aggregate(certs)
	if got.BtcGap != 100 {
		t.Fatalf("median %d, want 100 (55% of the weight, 4 of 7 by count)", got.BtcGap)
	}
}
