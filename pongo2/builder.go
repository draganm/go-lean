package pongo2

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/draganm/go-lean/common/globals"
	"github.com/draganm/go-lean/web/types"
	"github.com/flosch/pongo2/v6"
)

type Builder struct {
	files map[string]func() ([]byte, error)
}

func NewBuilder() *Builder {
	return &Builder{
		files: map[string]func() ([]byte, error){},
	}
}

func (b *Builder) Consume(pth string, getContent func() ([]byte, error)) bool {
	if !strings.HasPrefix(pth, "/web") {
		return false
	}

	pth = strings.TrimPrefix(pth, "/web")

	_, fileName := path.Split(pth)

	if templateRegexp.MatchString(fileName) {
		b.files[pth] = getContent
		return true
	}

	return false
}

type templateLoader struct {
	mu    *sync.RWMutex
	files map[string]func() ([]byte, error)
}

func (tl *templateLoader) Abs(base, name string) string {
	if path.IsAbs(name) {
		return name
	}

	if base == "" {
		return path.Clean(path.Join("/", name))
	}

	return path.Clean(path.Join(path.Dir(base), name))
}

func (tl *templateLoader) Get(path string) (io.Reader, error) {
	tl.mu.RLock()
	defer tl.mu.RUnlock()

	getData, found := tl.files[path]
	if !found {
		return nil, os.ErrNotExist
	}

	data, err := getData()
	if err != nil {
		return nil, fmt.Errorf("could not get pong2 template for %s: %w", path, err)
	}

	return bytes.NewReader(data), nil
}

func (b *Builder) Create(gl globals.Globals) (Pongo2Provider, error) {

	loader := &templateLoader{
		mu:    &sync.RWMutex{},
		files: b.files,
	}

	ts := pongo2.NewSet("lean", loader)

	for k, v := range gl {

		if strings.HasPrefix(k, "pongo2.filter.") {

			ff, isFilterFunction := v.(pongo2.FilterFunction)

			if !isFilterFunction {
				ff, isFilterFunction = v.(func(in *pongo2.Value, param *pongo2.Value) (out *pongo2.Value, err *pongo2.Error))
				if !isFilterFunction {
					continue
				}
			}

			name := strings.TrimPrefix(k, "pongo2.filter.")

			err := ts.RegisterFilter(name, ff)
			if err != nil {
				return nil, fmt.Errorf("could not register pongo2 filter %s: %w", name, err)
			}

		}
	}

	return func(ctx context.Context, handlerPath types.HandlerPath, w http.ResponseWriter) globals.Values {

		return map[string]any{
			"render": func(name string, vals pongo2.Context) error {

				if !strings.HasSuffix(name, ".pongo2") {
					name = name + ".pongo2"
				}

				if !strings.HasPrefix(name, "/") {
					name = path.Join(path.Dir(string(handlerPath)), name)
				}

				template, err := ts.FromCache(name)
				if err != nil {
					return fmt.Errorf("could not get template %s: %w", name, err)
				}

				return template.ExecuteWriter(vals, w)
			},
		}

	}, nil
}
