package jshandler

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/dop251/goja"
	"github.com/draganm/go-lean/common/providers"
	"github.com/draganm/go-lean/gojautils"
	"github.com/go-chi/chi"
	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func New(
	log logr.Logger,
	requestPath, code string,
	globals map[string]any,
	globalsProviders []providers.GenericGlobalsProvider,
	requestGlobalsProviders []providers.RequestGlobalsProvider,
) (http.HandlerFunc, error) {

	prog, err := goja.Compile(requestPath, code, true)
	if err != nil {
		return nil, fmt.Errorf("could not compile %s: %w", requestPath, err)
	}

	createInstance := func() (*goja.Runtime, error) {
		rt := goja.New()

		for k, v := range globals {
			vf, isRuntimeValueFactory := v.(func(rt *goja.Runtime) (any, error))
			if isRuntimeValueFactory {
				var err error
				v, err = vf(rt)
				if err != nil {
					return nil, fmt.Errorf("runtime value creation for %s failed: %w", k, err)
				}
			}

			rt.Set(k, v)
		}

		for _, gp := range globalsProviders {
			globs, err := gp(rt)
			if err != nil {
				return nil, fmt.Errorf("could not create globals from %v: %w", gp, err)
			}
			for k, v := range globs {
				err = rt.GlobalObject().Set(k, v)
				if err != nil {
					return nil, fmt.Errorf("could not set global %s: %w", k, err)
				}
			}
		}

		rt.SetFieldNameMapper(gojautils.SmartCapFieldNameMapper)
		_, err := rt.RunProgram(prog)
		if err != nil {
			return nil, fmt.Errorf("could not eval handler script: %w", err)
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

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		span := trace.SpanFromContext(r.Context())
		span.AddEvent("handling http request",
			trace.WithAttributes(
				attribute.String("method", r.Method),
				attribute.String("path", r.URL.RawPath),
			),
		)

		defer span.End()

		log := logr.FromContextOrDiscard(r.Context())
		rt := rtPool.Get().(*goja.Runtime)
		defer rtPool.Put(rt)
		v := rt.Get("handler")

		routeContext := chi.RouteContext(r.Context())

		params := map[string]string{}
		urlParams := routeContext.URLParams
		for i, pn := range urlParams.Keys {
			params[pn] = urlParams.Values[i]
		}
		fn, isFunction := goja.AssertFunction(v)
		if !isFunction {
			http.Error(w, "internal error", http.StatusInternalServerError)
			log.Error(err, "could not find handler function")
			return
		}

		addedGlobals := []string{}

		// remove globals at the end of the request before it's returned to the pool
		defer func() {
			for _, g := range addedGlobals {
				rt.GlobalObject().Delete(g)
			}
		}()

		for _, gp := range requestGlobalsProviders {
			globals, err := gp(requestPath, rt, w, r)
			if err != nil {
				span.RecordError(err)
				log.Error(err, "could not provide globals")
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			for k, v := range globals {
				err = rt.GlobalObject().Set(k, v)
				if err != nil {
					span.RecordError(err)
					log.Error(err, "could not provide global", "name", k)
					http.Error(w, "internal error", http.StatusInternalServerError)
					return
				}
				addedGlobals = append(addedGlobals, k)
			}
		}

		_, err := fn(nil, rt.ToValue(w), rt.ToValue(r), rt.ToValue(params))
		if err != nil {
			span.RecordError(err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			log.Error(err, "handler error")
			return
		}

	}), nil
}
