package lean_test

import (
	"context"
	"io/fs"
	"testing"

	"github.com/draganm/go-lean"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/require"
)

func TestRenderingPongo2Templates(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sfs, err := fs.Sub(simple, "fixtures/simple")
	require.NoError(err)

	w, err := lean.Construct(ctx, sfs, testr.New(t), map[string]any{})
	require.NoError(err)

	require.HTTPStatusCode(w.ServeHTTP, "GET", "/pongo2", nil, 200)
	require.HTTPBodyContains(w.ServeHTTP, "GET", "/pongo2", nil, "this is a pongo template I'm included bar")

}
