package jshandler

import (
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/dop251/goja"
	"github.com/draganm/go-lean/common/globals"
	"github.com/draganm/go-lean/web/types"
	"github.com/go-chi/chi/v5"
	"github.com/go-logr/logr"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type statusError struct {
	code    int
	message string
}

func (s *statusError) Error() string {
	return fmt.Sprintf("%d: %s", s.code, s.message)
}

var tracer = otel.Tracer("github.com/draganm/go-lean/leanweb/jshandler")

func New(
	log logr.Logger,
	requestPath string,
	code string,
	gl globals.Globals,
) (http.HandlerFunc, error) {

	prog, err := goja.Compile(requestPath, code, true)
	if err != nil {
		return nil, fmt.Errorf("could not compile %s: %w", requestPath, err)
	}

	createInstance := func() (*goja.Runtime, error) {
		rt := goja.New()
		rt.SetFieldNameMapper(goja.TagFieldNameMapper("lean", false))

		// I'm aware that not everything here will be wired properly, but
		// this is necessary in order not to have to treat require()
		// as a special case
		wired, err := gl.AutoWire(rt)
		if err != nil {
			return nil, fmt.Errorf("could not autowire globals: %w", err)
		}

		for k, v := range wired {
			err = rt.GlobalObject().Set(k, v)
			if err != nil {
				return nil, fmt.Errorf("could not set global %s: %w", k, err)
			}
		}

		rt.GlobalObject().Set("returnStatus", func(code int, message string) error {
			return &statusError{code: code, message: message}
		})

		_, err = rt.RunProgram(prog)
		if err != nil {
			return nil, fmt.Errorf("could not eval handler script: %w", err)
		}

		// delete autowired globals, they'll be provided again at request time
		for k := range wired {
			rt.GlobalObject().Delete(k)
		}

		v := rt.Get("handler")

		_, isFunction := goja.AssertFunction(v)
		if !isFunction {
			return nil, fmt.Errorf("could not find handler() function")
		}

		return rt, nil
	}

	canary, err := createInstance()
	if err != nil {
		return nil, fmt.Errorf("invalid handler %s: %w", requestPath, err)
	}

	rtPool := &sync.Pool{
		New: func() any {
			v, err := createInstance()
			if err != nil {
				panic(fmt.Errorf("could not create handler instance for %s: %w", requestPath, err))
			}
			return v
		},
	}

	rtPool.Put(canary)

	return otelhttp.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		ctx, span := tracer.Start(r.Context(), fmt.Sprintf("%s %s", r.Method, requestPath),
			trace.WithAttributes(
				attribute.String("method", r.Method),
				attribute.String("path", r.URL.RawPath),
			),
		)
		defer span.End()
		r = r.WithContext(ctx)

		log := logr.FromContextOrDiscard(r.Context())
		rt := rtPool.Get().(*goja.Runtime)
		defer rtPool.Put(rt)

		autowired, err := gl.AutoWire(rt, r.Context(), r, w, types.HandlerPath(requestPath))
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			log.Error(err, "could not autowire globals")
			return
		}

		for k, v := range autowired {
			err = rt.GlobalObject().Set(k, v)
			if err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				log.Error(err, "could not set global %s: %w", k, err)
				return
			}
		}

		v := rt.Get("handler")

		routeContext := chi.RouteContext(r.Context())

		params := map[string]string{}
		urlParams := routeContext.URLParams
		for i, pn := range urlParams.Keys {
			params[pn] = urlParams.Values[i]
		}

		err = rt.GlobalObject().Set("log", log.WithValues("params", params))
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			log.Error(err, "could set log global")
			return
		}

		fn, isFunction := goja.AssertFunction(v)
		if !isFunction {
			http.Error(w, "internal error", http.StatusInternalServerError)
			log.Error(err, "could not find handler function")
			return
		}

		// remove globals at the end of the request before it's returned to the pool
		defer func() {
			for g := range gl {
				rt.GlobalObject().Delete(g)
			}
		}()

		_, err = fn(nil, rt.ToValue(w), rt.ToValue(r), rt.ToValue(params))

		// check for statusError exception being thrown
		exc := &goja.Exception{}
		if errors.As(err, &exc) {
			exported := exc.Value().Export()
			m, ok := exported.(map[string]any)
			if ok {
				v, found := m["value"]
				if found && v != nil {
					se, ok := v.(*statusError)
					if ok {
						http.Error(w, se.message, se.code)
						return
					}
				}
			}
		}
		if err != nil {
			span.RecordError(err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			log.Error(err, "handler error")
			return
		}

	}), "golean").ServeHTTP, nil
}
