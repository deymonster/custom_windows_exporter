package collector

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
)

type Interface interface {
	RegisterMetrics(reg prometheus.Registerer) error
	Start(ctx context.Context) error
}

func New(os string) (Interface, error) {
	return newForOS(os)
}
