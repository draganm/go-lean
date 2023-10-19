package mustache

import (
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"path"
	"regexp"
	"strings"
	"sync"

	"github.com/dop251/goja"
	"github.com/draganm/go-lean/common/globals"
	"github.com/draganm/go-lean/leanweb/types"
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("github.com/draganm/go-lean/leanweb/mustache")

var templateRegexp = regexp.MustCompile(`^(.+).mustache$`)

func NewProvider(src fs.FS, root string) (func(handlerPath types.HandlerPath, vm *goja.Runtime, w http.ResponseWriter, r *http.Request) globals.Values, error) {

	templates := map[string]string{}

	err := fs.WalkDir(src, root, func(pth string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		withoutPrefix := strings.TrimPrefix(pth, root)

		if d.IsDir() {
			return nil
		}

		f, err := src.Open(pth)
		if err != nil {
			return fmt.Errorf("could not open %s: %w", pth, err)
		}

		defer f.Close()
		data, err := io.ReadAll(f)
		if err != nil {
			return fmt.Errorf("could not read %s: %w", pth, err)
		}

		fileDir, fileName := path.Split(withoutPrefix)

		templateSubmatches := templateRegexp.FindStringSubmatch(fileName)
		if len(templateSubmatches) == 2 {
			name := templateSubmatches[1]
			templates[path.Join(fileDir, name)] = string(data)
		}

		return nil

	})

	if err != nil {
		return nil, fmt.Errorf("could not parse templates: %w", err)
	}

	tcf := &templateCacheForPathFactory{
		partials:      templates,
		cachesForPath: make(map[string]*scopedTemplateCache),
		mu:            &sync.Mutex{},
	}

	return func(handlerPath types.HandlerPath, vm *goja.Runtime, w http.ResponseWriter, r *http.Request) globals.Values {

		tc := tcf.getTemplateCacheForPath(path.Dir(string(handlerPath)))
		return map[string]any{
			"render":         renderTemplateForScope(r.Context(), tc, w),
			"renderToString": renderTemplateForScopeToString(r.Context(), tc),
		}
	}, nil

}
