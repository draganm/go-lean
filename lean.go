package lean

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"strings"

	"github.com/draganm/go-lean/common/globals"
	"github.com/draganm/go-lean/cron"
	"github.com/draganm/go-lean/metrics"
	"github.com/draganm/go-lean/mustache"
	"github.com/draganm/go-lean/require"
	"github.com/draganm/go-lean/web"
	"github.com/go-chi/chi/v5"
	"github.com/go-logr/logr"
)

type Lean struct {
	files           map[string]func() ([]byte, error)
	webBuilder      *web.Builder
	mustacheBuilder *mustache.Builder
	requireBuilder  *require.Builder
}

func Construct(ctx context.Context, src fs.FS, rootPath string, log logr.Logger, globs map[string]any) (*chi.Mux, error) {
	files := map[string](func() ([]byte, error)){}

	err := fs.WalkDir(src, rootPath, func(pth string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		withoutPrefix := strings.TrimPrefix(pth, rootPath)

		if d.IsDir() {
			return nil
		}

		files[withoutPrefix] = func() ([]byte, error) {
			f, err := src.Open(pth)
			if err != nil {
				return nil, fmt.Errorf("could not open %s: %w", pth, err)
			}
			defer f.Close()
			return io.ReadAll(f)
		}

		return nil

	})

	if err != nil {
		return nil, fmt.Errorf("could not read the lean fs: %w", err)
	}

	consumeFiles := func(fn func(string, func() ([]byte, error)) bool) {

		consumed := []string{}
		for k, v := range files {
			wasConsumed := fn(k, v)
			if wasConsumed {
				consumed = append(consumed, k)
			}
		}

		for _, c := range consumed {
			delete(files, c)
		}

	}

	metricsBuilder := metrics.NewBuilder()
	cronBuilder := cron.NewBuilder()
	webBuilder := web.NewBuilder()
	requireBuilder := require.NewBuilder()
	mustacheBuilder := mustache.NewBuilder()

	cc := chainedConsume{
		metricsBuilder.Consume,
		cronBuilder.Consume,
		requireBuilder.Consume,
		mustacheBuilder.Consume,
		webBuilder.Consume,
	}

	consumeFiles(cc.Consume)

	req := requireBuilder.Build()

	mst, err := mustacheBuilder.Create()
	if err != nil {
		return nil, fmt.Errorf("could not build mustache provider: %w", err)
	}

	finalGlobs := globals.Globals{
		"require":  req,
		"mustache": mst,
	}

	finalGlobs, err = finalGlobs.Merge(globs)
	if err != nil {
		return nil, fmt.Errorf("could not merge globals: %w", err)
	}

	mux, err := webBuilder.Create(log, finalGlobs)
	if err != nil {
		return nil, fmt.Errorf("could not create web hadlder: %w", err)
	}

	cronBuilder.Start(ctx, log, finalGlobs)

	metricsBuilder.Start(ctx, log, globs)

	return mux, nil

}

type chainedConsume []func(string, func() ([]byte, error)) bool

func (cc chainedConsume) Consume(pth string, getContent func() ([]byte, error)) bool {
	for _, c := range cc {
		if c(pth, getContent) {
			return true
		}
	}

	return false
}
