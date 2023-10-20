package lean_test

import (
	"context"
	"testing"
	"time"

	"github.com/draganm/go-lean"
	"github.com/go-logr/logr/testr"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
)

func TestCron(t *testing.T) {

	require := require.New(t)
	ch := make(chan bool)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, err := lean.Construct(ctx, simple, "fixtures/simple", testr.New(t), map[string]any{
		"close": func() { close(ch) },
	})

	require.NoError(err)

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
