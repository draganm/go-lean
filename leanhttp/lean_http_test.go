package leanhttp_test

import (
	"embed"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/draganm/go-lean/common/providers"
	"github.com/draganm/go-lean/leanhttp"
	"github.com/draganm/go-lean/leanweb"
	"github.com/go-chi/chi"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/require"
)

//go:embed fixtures
var simple embed.FS

func TestLeanHTTP(t *testing.T) {

	require := require.New(t)

	httpProvider := leanhttp.NewProvider(http.DefaultClient)

	r := chi.NewMux()
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("bar"))
	})
	server := httptest.NewServer(r)
	defer server.Close()

	wh, err := leanweb.New(simple, "fixtures/html", testr.New(t), map[string]any{
		"testServerUrl": server.URL,
	}, &leanweb.GlobalsProviders{
		Context: []providers.ContextGlobalsProvider{httpProvider},
	})

	require.NoError(err)

	require.HTTPStatusCode(wh.ServeHTTP, "GET", "/", nil, 200)
	require.HTTPBodyContains(wh.ServeHTTP, "GET", "/", nil, "bar")
}
