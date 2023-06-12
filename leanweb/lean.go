package leanweb

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/dop251/goja"
	"github.com/draganm/go-lean/common/providers"
	"github.com/draganm/go-lean/leanweb/jshandler"
	"github.com/draganm/go-lean/leanweb/mustache"
	"github.com/draganm/go-lean/leanweb/require"
	"github.com/draganm/go-lean/leanweb/sse"
	"github.com/go-chi/chi"
	"github.com/go-logr/logr"
)

type Lean struct {
	http.Handler
}

var handlerRegexp = regexp.MustCompile(`^@([A-Z]+).js$`)

type GlobalsProviders struct {
	Generic []providers.GenericGlobalsProvider
	Request []providers.RequestGlobalsProvider
	Context []providers.ContextGlobalsProvider
}

func New(
	src fs.FS,
	root string,
	log logr.Logger,
	globals map[string]any,
	globalsProviders *GlobalsProviders,
) (*Lean, error) {
	r := chi.NewRouter()

	root = path.Clean(root)

	staticGetHandlers := map[string]bool{}

	hasHandler := func(path string) bool {
		return staticGetHandlers[path]
	}

	setHandler := func(path string) {
		staticGetHandlers[path] = true
	}

	mp, err := mustache.NewProvider(src, root)
	if err != nil {
		return nil, fmt.Errorf("could not initialize mustache: %w", err)
	}

	require, err := require.NewProvider(src, root)
	if err != nil {
		return nil, fmt.Errorf("could not initialize require(libs): %w", err)
	}

	genericGlobalsProviders := []providers.GenericGlobalsProvider{
		require,
	}

	requestGlobalsProviders := []providers.RequestGlobalsProvider{
		mp,
		sse.NewProvider(),
	}

	if globalsProviders != nil {
		genericGlobalsProviders = append(genericGlobalsProviders, globalsProviders.Generic...)
		requestGlobalsProviders = append(requestGlobalsProviders, globalsProviders.Request...)
		for _, cgp := range globalsProviders.Context {
			cgp := cgp
			requestGlobalsProviders = append(requestGlobalsProviders, func(handlerPath string, vm *goja.Runtime, w http.ResponseWriter, r *http.Request) (map[string]any, error) {
				return cgp(vm, r.Context())
			})
		}
	}

	err = fs.WalkDir(src, root, func(pth string, d fs.DirEntry, err error) error {
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

		_, fileName := path.Split(withoutPrefix)

		handlerSubmatches := handlerRegexp.FindStringSubmatch(fileName)
		if len(handlerSubmatches) == 2 {
			method := handlerSubmatches[1]
			handler, err := jshandler.New(
				log, withoutPrefix,
				string(data),
				globals,
				genericGlobalsProviders,
				requestGlobalsProviders,
			)
			if err != nil {
				return fmt.Errorf("could not create js handler: %w", err)
			}

			if method == "GET" {
				if hasHandler(path.Dir(withoutPrefix)) {
					return fmt.Errorf("path %s has conflicting index handlers", path.Dir(withoutPrefix))
				}
				setHandler(path.Dir(withoutPrefix))
			}

			r.MethodFunc(method, path.Dir(withoutPrefix), func(w http.ResponseWriter, r *http.Request) {
				ctx := r.Context()
				r = r.WithContext(logr.NewContext(ctx, log))
				handler(w, r)
			})

			return nil
		}

		t := time.Now()

		var contentType string

		ext := filepath.Ext(fileName)
		if ext == "" {
			contentType = http.DetectContentType(data)
		} else {
			contentType = mime.TypeByExtension(ext)
		}

		sum := sha1.Sum(data)
		eTag := fmt.Sprintf(`"%x"`, sum[:])

		handlerFunc := func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("content-type", contentType)
			w.Header().Set("etag", eTag)
			http.ServeContent(w, r, r.URL.Path, t, bytes.NewReader(data))
		}

		if fileName == "index.html" {

			if hasHandler(path.Dir(withoutPrefix)) {
				return fmt.Errorf("path %s has conflicting index handlers", path.Dir(withoutPrefix))
			}

			setHandler(path.Dir(withoutPrefix))
			r.Get(path.Dir(withoutPrefix), handlerFunc)

		}

		r.Get(withoutPrefix, handlerFunc)

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &Lean{Handler: r}, nil

}
