package mustache

import (
	"context"
	"fmt"
	"net/http"
	"path"
	"strings"
	"sync"

	"github.com/draganm/go-lean/common/globals"
	"github.com/draganm/go-lean/web/types"
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

	templateSubmatches := templateRegexp.FindStringSubmatch(fileName)
	if len(templateSubmatches) == 2 {
		b.files[pth] = getContent
		return true
	}

	return false
}

func (b *Builder) Create() (MustacheProvider, error) {
	templates := map[string]string{}

	for pth, getContent := range b.files {
		data, err := getContent()
		if err != nil {
			return nil, fmt.Errorf("could not get content of %s: %w", pth, err)
		}

		fileDir, fileName := path.Split(pth)

		templateSubmatches := templateRegexp.FindStringSubmatch(fileName)
		if len(templateSubmatches) == 2 {
			name := templateSubmatches[1]
			templates[path.Join(fileDir, name)] = string(data)
		}

	}

	tcf := &templateCacheForPathFactory{
		partials:      templates,
		cachesForPath: make(map[string]*scopedTemplateCache),
		mu:            &sync.Mutex{},
	}

	return func(ctx context.Context, handlerPath types.HandlerPath, w http.ResponseWriter) globals.Values {

		tc := tcf.getTemplateCacheForPath(path.Dir(string(handlerPath)))
		return map[string]any{
			"render":         renderTemplateForScope(ctx, tc, w),
			"renderToString": renderTemplateForScopeToString(ctx, tc),
		}
	}, nil

}
