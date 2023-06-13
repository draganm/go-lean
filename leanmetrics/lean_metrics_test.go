package leanmetrics_test

import (
	"context"
	"embed"
	"testing"

	"github.com/draganm/go-lean/leanmetrics"
	"github.com/go-logr/logr/testr"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
)

//go:embed fixtures
var simple embed.FS

func findMetrics(t *testing.T, name string, mt dto.MetricType) []*dto.Metric {
	require := require.New(t)
	families, err := prometheus.DefaultGatherer.Gather()
	require.NoError(err)
	for _, f := range families {

		if *f.Name == name && *f.Type == mt {
			return f.Metric
		}
	}
	return nil
}

func TestMetricsProviders(t *testing.T) {
	require := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := leanmetrics.Start(ctx, simple, "fixtures", testr.New(t), map[string]any{}, nil)
	require.NoError(err)

	metrics := findMetrics(t, "test", dto.MetricType_COUNTER)
	require.NotEmpty(metrics)
}
