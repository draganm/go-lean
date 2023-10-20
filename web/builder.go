package web

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/draganm/go-lean/common/globals"
	"github.com/draganm/go-lean/web/jshandler"
	"github.com/draganm/go-lean/web/sse"
	"github.com/go-chi/chi/v5"
	"github.com/go-logr/logr"
)

type jsHandlerInfo struct {
	method     string
	path       string
	getContent func() ([]byte, error)
}

type Builder struct {
	jsHandlers  []jsHandlerInfo
	staticFiles map[string]func() ([]byte, error)
}

func NewBuilder() *Builder {
	return &Builder{
		staticFiles: map[string]func() ([]byte, error){},
	}
}

func (b *Builder) Consume(pth string, getContent func() ([]byte, error)) bool {

	if !strings.HasPrefix(pth, "/web") {
		return false
	}

	pth = strings.TrimPrefix(pth, "/web")

	_, fileName := path.Split(pth)

	handlerSubmatches := handlerRegexp.FindStringSubmatch(fileName)
	if len(handlerSubmatches) == 2 {
		method := handlerSubmatches[1]
		b.jsHandlers = append(b.jsHandlers, jsHandlerInfo{
			method:     method,
			path:       pth,
			getContent: getContent,
		})

		return true
	}

	b.staticFiles[pth] = getContent
	return true
}

func (b *Builder) Create(
	log logr.Logger,
	globs map[string]any,
) (*chi.Mux, error) {
	r := chi.NewMux()

	gl := globals.Globals{
		"sendServerEvents": sse.SSEProvider,
	}

	var err error
	gl, err = gl.Merge(globs)
	if err != nil {
		return nil, fmt.Errorf("could not merge globals: %w", err)
	}

	// TODO: check for overlapping handlers

	for _, jh := range b.jsHandlers {

		jh := jh

		data, err := jh.getContent()
		if err != nil {
			return nil, fmt.Errorf("could not read data: %w", err)
		}

		handler, err := jshandler.New(
			log,
			jh.path,
			string(data),
			gl,
		)
		if err != nil {
			return nil, fmt.Errorf("could not create js handler: %w", err)
		}

		requestPath := path.Dir(jh.path)

		r.MethodFunc(jh.method, requestPath, func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			log := log.WithValues("method", jh.method, "handlerPath", requestPath)
			r = r.WithContext(logr.NewContext(ctx, log))
			startTime := time.Now()
			crw := newCapturingResponseWriter(w)

			defer func() {
				duration := time.Since(startTime)
				durationMetric, err := responseDurations.GetMetricWithLabelValues(r.Method, requestPath)
				if err != nil {
					log.Error(err, "could not find duration metric")

				} else {
					durationMetric.Observe(duration.Seconds())
				}

				statusString := fmt.Sprintf("%d", crw.status)
				statusMetric, err := responseStatusCount.GetMetricWithLabelValues(statusString, r.Method, requestPath)
				if err != nil {
					log.Error(err, "could not find status metric")

				} else {
					statusMetric.Add(1)
				}
			}()
			handler(crw, r)
		})
	}

	for pth, getDataFunc := range b.staticFiles {
		t := time.Now()

		data, err := getDataFunc()

		if err != nil {
			return nil, fmt.Errorf("could not read data for path %s: %w", pth, err)
		}

		var contentType string

		_, fileName := path.Split(pth)

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
			r.Get(path.Dir(pth), handlerFunc)

		}

		r.Get(pth, handlerFunc)

	}

	return r, nil

}
