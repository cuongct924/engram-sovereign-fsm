package types

import "testing"

// healthyMetricsForParams returns a PeripheralMetrics snapshot that satisfies
// IsHealthyCondition under p -- mirrors keeper/circuit_breaker_test.go's
// healthyMetrics() helper, duplicated here since that one lives in package
// keeper and imports this package, not the other way around.
func healthyMetricsForParams(p Params) *PeripheralMetrics {
	return &PeripheralMetrics{
		BtcGap:          0,
		DaGap:           0,
		SubnetDiversity: p.MinSubnetDiversity,
		ActiveAnchors:   p.MinAnchorPeers,
		CleanPeers:      p.MinPeers,
		PeerChurnRate:   0,
		AvgPeerTenure:   p.MinAvgTenure,
		PeerLatency:     0,
	}
}

// spec/core/EngramFSM.tla:94-96:
//
//	IsBTCGapSuspicious == SUSPICIOUS_THRESHOLD <= btc_gap /\ btc_gap < SOVEREIGN_THRESHOLD
func TestIsBTCGapSuspicious(t *testing.T) {
	p := DefaultParams()

	cases := []struct {
		name   string
		btcGap uint64
		want   bool
	}{
		{"below suspicious threshold", p.SuspiciousThreshold - 1, false},
		{"exactly at suspicious threshold", p.SuspiciousThreshold, true},
		{"between suspicious and sovereign", p.SovereignThreshold - 1, true},
		{"exactly at sovereign threshold", p.SovereignThreshold, false},
		{"above sovereign threshold", p.SovereignThreshold + 1, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := &PeripheralMetrics{BtcGap: tc.btcGap}
			if got := IsBTCGapSuspicious(m, p); got != tc.want {
				t.Errorf("BtcGap=%d: got %v, want %v", tc.btcGap, got, tc.want)
			}
		})
	}
}

// spec/core/EngramFSM.tla:98: IsBTCGapSovereign == btc_gap >= SOVEREIGN_THRESHOLD
func TestIsBTCGapSovereign(t *testing.T) {
	p := DefaultParams()

	cases := []struct {
		name   string
		btcGap uint64
		want   bool
	}{
		{"below sovereign threshold", p.SovereignThreshold - 1, false},
		{"exactly at sovereign threshold", p.SovereignThreshold, true},
		{"above sovereign threshold", p.SovereignThreshold + 1, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := &PeripheralMetrics{BtcGap: tc.btcGap}
			if got := IsBTCGapSovereign(m, p); got != tc.want {
				t.Errorf("BtcGap=%d: got %v, want %v", tc.btcGap, got, tc.want)
			}
		})
	}
}

// spec/core/EngramFSM.tla:87: IsDAHealthy == (da_gap < DA_THRESHOLD) /\ ~is_das_failed /\ ~is_attestation_failed
func TestIsDAHealthy(t *testing.T) {
	p := DefaultParams()

	cases := []struct {
		name                string
		daGap               uint64
		isDasFailed         bool
		isAttestationFailed bool
		want                bool
	}{
		{"below threshold, both flags clear", p.DAThreshold - 1, false, false, true},
		{"exactly at threshold is unhealthy", p.DAThreshold, false, false, false},
		{"below threshold but das failed", p.DAThreshold - 1, true, false, false},
		{"below threshold but attestation failed", p.DAThreshold - 1, false, true, false},
		{"below threshold, both flags set", p.DAThreshold - 1, true, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := &PeripheralMetrics{
				DaGap:               tc.daGap,
				IsDasFailed:         tc.isDasFailed,
				IsAttestationFailed: tc.isAttestationFailed,
			}
			if got := IsDAHealthy(m, p); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// spec/core/EngramFSM.tla:76-82: IsP2PQualityHealthy is a 6-way conjunction --
// each clause must independently be able to fail the whole predicate.
func TestIsP2PQualityHealthy(t *testing.T) {
	p := DefaultParams()
	healthy := func() *PeripheralMetrics { return healthyMetricsForParams(p) }

	cases := []struct {
		name   string
		mutate func(m *PeripheralMetrics)
		want   bool
	}{
		{"all clauses satisfied", func(m *PeripheralMetrics) {}, true},
		{"subnet diversity below minimum", func(m *PeripheralMetrics) { m.SubnetDiversity = p.MinSubnetDiversity - 1 }, false},
		{"active anchors below minimum", func(m *PeripheralMetrics) { m.ActiveAnchors = p.MinAnchorPeers - 1 }, false},
		{"clean peers below minimum", func(m *PeripheralMetrics) { m.CleanPeers = p.MinPeers - 1 }, false},
		{"churn rate above maximum", func(m *PeripheralMetrics) { m.PeerChurnRate = p.MaxChurnRate + 1 }, false},
		{"avg tenure below minimum", func(m *PeripheralMetrics) { m.AvgPeerTenure = p.MinAvgTenure - 1 }, false},
		{"peer latency above maximum", func(m *PeripheralMetrics) { m.PeerLatency = p.MaxPeerLatency + 1 }, false},
		{"churn rate exactly at maximum is still healthy", func(m *PeripheralMetrics) { m.PeerChurnRate = p.MaxChurnRate }, true},
		{"peer latency exactly at maximum is still healthy", func(m *PeripheralMetrics) { m.PeerLatency = p.MaxPeerLatency }, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := healthy()
			tc.mutate(m)
			if got := IsP2PQualityHealthy(m, p); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// spec/core/EngramFSM.tla:101-104: IsCriticalCondition is a 3-way disjunction --
// any single arm must independently be able to trigger it.
func TestIsCriticalCondition(t *testing.T) {
	p := DefaultParams()

	t.Run("healthy metrics, zero suspicious duration -> not critical", func(t *testing.T) {
		m := healthyMetricsForParams(p)
		if IsCriticalCondition(m, p, 0) {
			t.Error("expected not critical")
		}
	})

	t.Run("btc gap sovereign alone triggers critical", func(t *testing.T) {
		m := healthyMetricsForParams(p)
		m.BtcGap = p.SovereignThreshold
		if !IsCriticalCondition(m, p, 0) {
			t.Error("expected critical via IsBTCGapSovereign")
		}
	})

	t.Run("zero active anchors alone triggers critical (complete eclipse)", func(t *testing.T) {
		m := healthyMetricsForParams(p)
		m.ActiveAnchors = 0
		if !IsCriticalCondition(m, p, 0) {
			t.Error("expected critical via total anchor-peer loss")
		}
	})

	t.Run("suspicious duration at max alone triggers critical", func(t *testing.T) {
		m := healthyMetricsForParams(p)
		if !IsCriticalCondition(m, p, p.MaxSuspiciousTime) {
			t.Error("expected critical via gray-failure timeout")
		}
	})

	t.Run("suspicious duration just below max does not trigger critical", func(t *testing.T) {
		m := healthyMetricsForParams(p)
		if IsCriticalCondition(m, p, p.MaxSuspiciousTime-1) {
			t.Error("expected not critical")
		}
	})
}

// spec/core/EngramFSM.tla:107-110: IsWarningCondition is a 3-way disjunction.
func TestIsWarningCondition(t *testing.T) {
	p := DefaultParams()

	t.Run("healthy metrics -> not warning", func(t *testing.T) {
		m := healthyMetricsForParams(p)
		if IsWarningCondition(m, p) {
			t.Error("expected not warning")
		}
	})

	t.Run("btc gap suspicious alone triggers warning", func(t *testing.T) {
		m := healthyMetricsForParams(p)
		m.BtcGap = p.SuspiciousThreshold
		if !IsWarningCondition(m, p) {
			t.Error("expected warning via IsBTCGapSuspicious")
		}
	})

	t.Run("unhealthy DA alone triggers warning", func(t *testing.T) {
		m := healthyMetricsForParams(p)
		m.DaGap = p.DAThreshold
		if !IsWarningCondition(m, p) {
			t.Error("expected warning via ~IsDAHealthy")
		}
	})

	t.Run("unhealthy P2P alone triggers warning", func(t *testing.T) {
		m := healthyMetricsForParams(p)
		m.CleanPeers = p.MinPeers - 1
		if !IsWarningCondition(m, p) {
			t.Error("expected warning via ~IsP2PQualityHealthy")
		}
	})
}

// spec/core/EngramFSM.tla:113-117: IsHealthyCondition is a 4-way conjunction.
func TestIsHealthyCondition(t *testing.T) {
	p := DefaultParams()

	t.Run("all sensors green -> healthy", func(t *testing.T) {
		m := healthyMetricsForParams(p)
		if !IsHealthyCondition(m, p) {
			t.Error("expected healthy")
		}
	})

	t.Run("btc gap sovereign alone breaks healthy", func(t *testing.T) {
		m := healthyMetricsForParams(p)
		m.BtcGap = p.SovereignThreshold
		if IsHealthyCondition(m, p) {
			t.Error("expected not healthy")
		}
	})

	t.Run("btc gap suspicious alone breaks healthy", func(t *testing.T) {
		m := healthyMetricsForParams(p)
		m.BtcGap = p.SuspiciousThreshold
		if IsHealthyCondition(m, p) {
			t.Error("expected not healthy")
		}
	})

	t.Run("unhealthy DA alone breaks healthy", func(t *testing.T) {
		m := healthyMetricsForParams(p)
		m.IsDasFailed = true
		if IsHealthyCondition(m, p) {
			t.Error("expected not healthy")
		}
	})

	t.Run("unhealthy P2P alone breaks healthy", func(t *testing.T) {
		m := healthyMetricsForParams(p)
		m.PeerLatency = p.MaxPeerLatency + 1
		if IsHealthyCondition(m, p) {
			t.Error("expected not healthy")
		}
	})

	t.Run("IsWarningCondition and IsHealthyCondition are mutually exclusive on healthy metrics", func(t *testing.T) {
		m := healthyMetricsForParams(p)
		if IsHealthyCondition(m, p) == IsWarningCondition(m, p) {
			t.Error("IsHealthyCondition and IsWarningCondition must never agree")
		}
	})
}
