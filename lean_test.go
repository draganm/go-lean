package lean_test

import (
	"embed"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dop251/goja"
	"github.com/draganm/go-lean"
	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/require"
)

//go:embed fixtures
var simple embed.FS

func TestServingStaticFiles(t *testing.T) {
	require := require.New(t)
	w, err := lean.New(simple, "fixtures/simple", logr.Discard(), map[string]any{})
	require.NoError(err)

	require.HTTPStatusCode(w.ServeHTTP, "GET", "/", nil, 200)

}

func TestServingDynamicContent(t *testing.T) {
	require := require.New(t)
	w, err := lean.New(simple, "fixtures/simple", logr.Discard(), map[string]any{})
	require.NoError(err)

	require.HTTPStatusCode(w.ServeHTTP, "GET", "/123", nil, 200)

}

func TestServingDynamicContentWithParams(t *testing.T) {
	require := require.New(t)
	w, err := lean.New(simple, "fixtures/simple", logr.Discard(), map[string]any{})
	require.NoError(err)

	require.HTTPStatusCode(w.ServeHTTP, "GET", "/vars/123", nil, 200)
	require.HTTPBodyContains(w.ServeHTTP, "GET", "/vars/foo-bar", nil, "foo-bar")

}

func TestServingDynamicContentWithTemplates(t *testing.T) {
	require := require.New(t)
	w, err := lean.New(simple, "fixtures/simple", testr.New(t), map[string]any{})
	require.NoError(err)

	require.HTTPStatusCode(w.ServeHTTP, "GET", "/template", nil, 200)
	require.HTTPBodyContains(w.ServeHTTP, "GET", "/template", nil, "this is index foo=bar and root")

}

func TestUsingLibraries(t *testing.T) {
	require := require.New(t)
	w, err := lean.New(simple, "fixtures/simple", testr.New(t), map[string]any{})
	require.NoError(err)

	require.HTTPStatusCode(w.ServeHTTP, "GET", "/libuser", nil, 200)
	require.HTTPBodyContains(w.ServeHTTP, "GET", "/libuser", nil, "called")

}

func BenchmarkSimpleTemplate(b *testing.B) {
	handler, err := lean.New(simple, "fixtures/simple", logr.Discard(), map[string]any{})
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
	handler, err := lean.New(simple, "fixtures/simple", logr.Discard(), map[string]any{})
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
	w, err := lean.New(simple, "fixtures/simple", testr.New(t), map[string]any{})
	require.NoError(err)
	require.HTTPStatusCode(w.ServeHTTP, "GET", "/sse", nil, 200)
	require.HTTPBodyContains(w.ServeHTTP, "GET", "/sse", nil, "event: foo\ndata: bar\n\n")
}

func TestRuntimeValueFactory(t *testing.T) {
	require := require.New(t)
	w, err := lean.New(simple, "fixtures/simple", testr.New(t), map[string]any{
		"myValue": func(rt *goja.Runtime) (any, error) {
			return "bar", nil
		},
	})
	require.NoError(err)
	require.HTTPStatusCode(w.ServeHTTP, "GET", "/valueFactory", nil, 200)
	require.HTTPBodyContains(w.ServeHTTP, "GET", "/valueFactory", nil, "bar")
}
