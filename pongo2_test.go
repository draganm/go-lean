package lean_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/draganm/go-lean"
	"github.com/flosch/pongo2/v6"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/require"
)

func TestRenderingPongo2Templates(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w, err := lean.Construct(ctx, simple, "fixtures/simple", testr.New(t), map[string]any{})
	require.NoError(err)

	require.HTTPStatusCode(w.ServeHTTP, "GET", "/pongo2", nil, 200)
	require.HTTPBodyContains(w.ServeHTTP, "GET", "/pongo2", nil, "this is a pongo template I'm included bar")

}

func TestRenderingPongo2TemplatesWithCustomFilters(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w, err := lean.Construct(ctx, simple, "fixtures/simple", testr.New(t), map[string]any{
		"pongo2.filter.twodigit": func(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
			return pongo2.AsValue(fmt.Sprintf("%05.1f", in.Float())), nil
		},
	})
	require.NoError(err)

	require.HTTPStatusCode(w.ServeHTTP, "GET", "/pongo2-filters", nil, 200)
	require.HTTPBodyContains(w.ServeHTTP, "GET", "/pongo2-filters", nil, "this is a pongo template 003.3")

}
