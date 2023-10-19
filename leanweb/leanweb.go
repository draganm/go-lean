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

	"github.com/draganm/go-lean/common/globals"
	"github.com/draganm/go-lean/leanweb/jshandler"
	"github.com/draganm/go-lean/leanweb/mustache"
	"github.com/draganm/go-lean/leanweb/require"
	"github.com/draganm/go-lean/leanweb/sse"
	"github.com/go-chi/chi"
	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Lean struct {
	http.Handler
}

var (
	responseDurations = promauto.NewSummaryVec(prometheus.SummaryOpts{
		Name: "leanweb_response_duration",
		Help: "HTTP Response Duration",
	}, []string{"method", "path"})

	responseStatusCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "leanweb_response_status_count",
		Help: "HTTP Status per response",
	}, []string{"status", "method", "path"})
)

var handlerRegexp = regexp.MustCompile(`^@([A-Z]+).js$`)

func New(
	src fs.FS,
	root string,
	log logr.Logger,
	gl globals.Globals,
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

	gl, err = gl.Merge(globals.Globals{
		"mustache":         mp,
		"sendServerEvents": sse.SSEProvider,
	})
	if err != nil {
		return nil, fmt.Errorf("could not merge global values: %w", err)
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
				gl,
				require,
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
				log := log.WithValues("method", method, "handlerPath", path.Dir(withoutPrefix))
				r = r.WithContext(logr.NewContext(ctx, log))
				startTime := time.Now()
				crw := newCapturingResponseWriter(w)

				defer func() {
					duration := time.Since(startTime)
					durationMetric, err := responseDurations.GetMetricWithLabelValues(r.Method, path.Dir(withoutPrefix))
					if err != nil {
						log.Error(err, "could not find duration metric")

					} else {
						durationMetric.Observe(duration.Seconds())
					}

					statusString := fmt.Sprintf("%d", crw.status)
					statusMetric, err := responseStatusCount.GetMetricWithLabelValues(statusString, r.Method, path.Dir(withoutPrefix))
					if err != nil {
						log.Error(err, "could not find status metric")

					} else {
						statusMetric.Add(1)
					}
					//  crw.status
				}()
				handler(crw, r)
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
