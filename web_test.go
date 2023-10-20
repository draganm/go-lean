package lean_test

import (
	"context"
	"embed"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/draganm/go-lean"
	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
)

//go:embed fixtures
var simple embed.FS

func TestServingStaticFiles(t *testing.T) {

	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w, err := lean.Construct(ctx, simple, "fixtures/simple", testr.New(t), map[string]any{})
	require.NoError(err)

	require.HTTPStatusCode(w.ServeHTTP, "GET", "/", nil, 200)

}

func TestServingDynamicContent(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w, err := lean.Construct(ctx, simple, "fixtures/simple", testr.New(t), map[string]any{})
	require.NoError(err)

	require.HTTPStatusCode(w.ServeHTTP, "GET", "/123", nil, 200)

}

func TestServingDynamicContentWithParams(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w, err := lean.Construct(ctx, simple, "fixtures/simple", logr.Discard(), map[string]any{})
	require.NoError(err)

	require.HTTPStatusCode(w.ServeHTTP, "GET", "/vars/123", nil, 200)
	require.HTTPBodyContains(w.ServeHTTP, "GET", "/vars/foo-bar", nil, "foo-bar")

}

func TestServingDynamicContentWithTemplates(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w, err := lean.Construct(ctx, simple, "fixtures/simple", testr.New(t), map[string]any{})
	require.NoError(err)

	require.HTTPStatusCode(w.ServeHTTP, "GET", "/template", nil, 200)
	require.HTTPBodyContains(w.ServeHTTP, "GET", "/template", nil, "this is index foo=bar and root")

}

func TestUsingLibraries(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w, err := lean.Construct(ctx, simple, "fixtures/simple", testr.New(t), map[string]any{})
	require.NoError(err)

	require.HTTPStatusCode(w.ServeHTTP, "GET", "/libuser", nil, 200)
	require.HTTPBodyContains(w.ServeHTTP, "GET", "/libuser", nil, "called")

}

func BenchmarkSimpleTemplate(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handler, err := lean.Construct(ctx, simple, "fixtures/simple", logr.Discard(), map[string]any{})
	if err != nil {
		b.Error(err)
	}
	for n := 0; n < b.N; n++ {
		w := httptest.NewRecorder()
		req, err := http.NewRequest("GET", "/template", nil)
		if err != nil {
			b.Error(err)
		}
		handler.ServeHTTP(w, req)

	}
}

func BenchmarkUsingLibraries(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handler, err := lean.Construct(ctx, simple, "fixtures/simple", logr.Discard(), map[string]any{})
	if err != nil {
		b.Error(err)
	}
	for n := 0; n < b.N; n++ {
		w := httptest.NewRecorder()
		req, err := http.NewRequest("GET", "/libuser", nil)
		if err != nil {
			b.Error(err)
		}
		handler.ServeHTTP(w, req)

	}
}

func TestSSE(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w, err := lean.Construct(ctx, simple, "fixtures/simple", testr.New(t), map[string]any{})
	require.NoError(err)
	require.HTTPStatusCode(w.ServeHTTP, "GET", "/sse", nil, 200)
	require.HTTPBodyContains(w.ServeHTTP, "GET", "/sse", nil, "event: foo\ndata: bar\n\n")
}

func TestRenderToString(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w, err := lean.Construct(ctx, simple, "fixtures/simple", testr.New(t), map[string]any{})
	require.NoError(err)

	require.HTTPStatusCode(w.ServeHTTP, "GET", "/templateToString", nil, 200)

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

func TestMetrics(t *testing.T) {

	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w, err := lean.Construct(ctx, simple, "fixtures/simple", logr.Discard(), map[string]any{})
	require.NoError(err)

	require.HTTPStatusCode(w.ServeHTTP, "GET", "/123", nil, 200)
	durationMetrics := findMetrics(t, "leanweb_response_duration", dto.MetricType_SUMMARY)
	require.NotEmpty(durationMetrics)

	statusCounterMetrics := findMetrics(t, "leanweb_response_status_count", dto.MetricType_COUNTER)
	require.NotEmpty(statusCounterMetrics)

}

func TestReturningStatus(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w, err := lean.Construct(ctx, simple, "fixtures/simple", testr.New(t), map[string]any{})
	require.NoError(err)

	require.HTTPStatusCode(w.ServeHTTP, "GET", "/status", nil, 401)

}
