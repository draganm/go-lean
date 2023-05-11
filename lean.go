package lean

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

	"github.com/draganm/go-lean/jshandler"
	"github.com/go-chi/chi"
	"github.com/go-logr/logr"
)

type Lean struct {
	http.Handler
}

var handlerRegexp = regexp.MustCompile(`^@([A-Z]+).js$`)
var templateRegexp = regexp.MustCompile(`^(.+).mustache$`)
var libRegexp = regexp.MustCompile(`^/lib/(.+).js$`)

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

	templates := map[string]string{}
	libs := map[string]string{}

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

		handlerSubmatches := handlerRegexp.FindStringSubmatch(fileName)
		if len(handlerSubmatches) == 2 {
			method := handlerSubmatches[1]
			handler, err := jshandler.New(log, withoutPrefix, string(data), globals, templates, libs)
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

		templateSubmatches := templateRegexp.FindStringSubmatch(fileName)
		if len(templateSubmatches) == 2 {
			name := templateSubmatches[1]
			templates[path.Join(fileDir, name)] = string(data)
			return nil
		}

		if libRegexp.MatchString(withoutPrefix) {
			libs[withoutPrefix] = string(data)
		}

		// handling of the static content

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
