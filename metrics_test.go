package lean_test

import (
	"context"
	"testing"

	"github.com/draganm/go-lean"
	"github.com/go-logr/logr/testr"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
)

func TestMetricsProviders(t *testing.T) {
	require := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, err := lean.Construct(ctx, simple, "fixtures/simple", testr.New(t), map[string]any{})
	require.NoError(err)

	metrics := findMetrics(t, "test", dto.MetricType_COUNTER)
	require.NotEmpty(metrics)
}
