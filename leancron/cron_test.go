package leancron_test

import (
	"context"
	"embed"
	"testing"
	"time"

	"github.com/draganm/go-lean/leancron"
	"github.com/go-logr/logr/testr"
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
}
