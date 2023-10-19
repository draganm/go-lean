package globals

import (
	"context"
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/require"
)

func TestAutoWireFunction(t *testing.T) {
	require := require.New(t)

	ctx := context.Background()
	rt := goja.New()

	var passedContext context.Context
	var passedRT *goja.Runtime

	fn := func(ctx context.Context, rt *goja.Runtime) {
		passedContext = ctx
		passedRT = rt
	}

	wired, err := autoWireFunction(fn, ctx, rt)
	require.NoError(err)

	rt.Set("foo", wired)

	_, err = rt.RunString("foo()")
	require.NoError(err)
	require.Equal(ctx, passedContext)
	require.Equal(rt, passedRT)

}

func TestAutoWireFunctionProvidingValuesObject(t *testing.T) {
	require := require.New(t)

	ctx := context.Background()
	rt := goja.New()

	fn := func(ctx context.Context, rt *goja.Runtime) Values {
		return Values{
			"a": 1,
			"b": 2,
		}
	}

	wired, err := autoWireFunction(fn, ctx, rt)
	require.NoError(err)

	rt.Set("foo", wired)

	res, err := rt.RunString("foo.a+foo.b")
	require.NoError(err)

	require.Equal(res.ToInteger(), int64(3))

}
