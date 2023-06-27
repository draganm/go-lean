package leancron_test

import (
	"context"
	"embed"
	"testing"
	"time"

	"github.com/draganm/go-lean/leancron"
	"github.com/go-logr/logr/testr"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
)

//go:embed fixtures
var simple embed.FS

func TestCron(t *testing.T) {

	require := require.New(t)
	ch := make(chan bool)

	ctx, cancel := context.WithCancel(context.Background())

	defer cancel()

	err := leancron.Start(
		ctx,
		simple,
		"fixtures",
		testr.New(t),
		time.Local,
		map[string]any{
			"close": func() { close(ch) },
		},
	)

	require.NoError(err)

	select {
	case <-time.After(2 * time.Second):
		require.Fail("timed out")
	case <-ch:
		// all good
	}

	durations := findMetrics(t, "leancron_execution_duration", dto.MetricType_SUMMARY)
	require.NotEmpty(durations)

	successes := findMetrics(t, "leancron_execution_successful_count", dto.MetricType_COUNTER)
	require.NotEmpty(successes)

}

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
