//go:build !windows

package collector

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

type noopCollector struct{}

func (noopCollector) RegisterMetrics(reg prometheus.Registerer) error { return nil }
func (noopCollector) Start(ctx context.Context) error                 { <-ctx.Done(); return nil }

func newForOS(os string) (Interface, error) {
	return noopCollector{}, fmt.Errorf("collector not implemented for %s", os)
}
