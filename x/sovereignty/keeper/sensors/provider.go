package sensors

import "context"

// SensorProvider is the legacy single-metric interface, kept for BTC/DA sensors
// whose spec predicates reduce to a single scalar comparison.
type SensorProvider interface {
	GetMetric(ctx context.Context) (uint64, error)
}
