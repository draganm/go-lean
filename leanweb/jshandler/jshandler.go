package jshandler

import (
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/dop251/goja"
	"github.com/draganm/go-lean/common/globals"
	"github.com/draganm/go-lean/gojautils"
	"github.com/go-chi/chi"
	"github.com/go-logr/logr"
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

func New(
	log logr.Logger,
	requestPath, code string,
	gl map[string]any,
) (http.HandlerFunc, error) {

	prog, err := goja.Compile(requestPath, code, true)
	if err != nil {
		return nil, fmt.Errorf("could not compile %s: %w", requestPath, err)
	}
	requestTimeGlobals := globals.Globals{}

	for k, v := range gl {
		if globals.IsVMGlobalProvider(v) || globals.IsPlainValue(v) {

			continue
		}
		requestTimeGlobals[k] = v
	}

	createInstance := func() (*goja.Runtime, error) {
		rt := goja.New()

		for k, v := range gl {
			if globals.IsVMGlobalProvider(v) || globals.IsPlainValue(v) {
				err = globals.ProvideVMGlobalValue(rt, k, v)
				if err != nil {
					return nil, err
				}
				continue
			}
			requestTimeGlobals[k] = v

		}

		rt.GlobalObject().Set("returnStatus", func(code int, message string) error {
			return &statusError{code: code, message: message}
		})

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

		for k, v := range requestTimeGlobals {
			err = globals.ProvideRequestGlobalValue(requestPath, rt, w, r, k, v)
			if err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				log.Error(err, "could set request time globals")
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
		fn, isFunction := goja.AssertFunction(v)
		if !isFunction {
			http.Error(w, "internal error", http.StatusInternalServerError)
			log.Error(err, "could not find handler function")
			return
		}

		// remove globals at the end of the request before it's returned to the pool
		defer func() {
			for g := range requestTimeGlobals {
				rt.GlobalObject().Delete(g)
			}
		}()

		_, err := fn(nil, rt.ToValue(w), rt.ToValue(r), rt.ToValue(params))

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

	}), nil
}
