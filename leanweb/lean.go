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

	"github.com/draganm/go-lean/leanweb/jshandler"
	"github.com/draganm/go-lean/leanweb/mustache"
	"github.com/draganm/go-lean/leanweb/sse"
	"github.com/go-chi/chi"
	"github.com/go-logr/logr"
)

type Lean struct {
	http.Handler
}

var handlerRegexp = regexp.MustCompile(`^@([A-Z]+).js$`)

var libRegexp = regexp.MustCompile(`^/lib/(.+).js$`)

type RequestGlobalsProvider jshandler.RequestGlobalsProvider

func New(src fs.FS, root string, log logr.Logger, globals map[string]any) (*Lean, error) {
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

	globalsProviders := []jshandler.RequestGlobalsProvider{
		mp,
		sse.NewProvider(),
	}

	libs := map[string]string{}

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
				globalsProviders,
				libs,
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

		if libRegexp.MatchString(withoutPrefix) {
			libs[withoutPrefix] = string(data)
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
