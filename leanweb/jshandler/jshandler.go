package jshandler

import (
	"fmt"
	"net/http"
	"path"
	"strings"
	"sync"

	"github.com/dop251/goja"
	"github.com/draganm/go-lean/leanweb/mustache"
	"github.com/go-chi/chi"
	"github.com/go-logr/logr"
)

func New(
	log logr.Logger,
	requestPath, code string,
	globals map[string]any,
	templatePartials map[string]string,
	libs map[string]string,
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

		rt.Set("require", func(libName string) (goja.Value, error) {
			libCode, found := libs[libName]
			if !found {
				return nil, fmt.Errorf("%s not found", libName)
			}
			return rt.RunScript(
				libName,
				fmt.Sprintf(`(() => { var exports = {}; var module = { exports: exports}; %s; return module.exports})()`, libCode),
			)

		})

		rt.SetFieldNameMapper(newSmartCapFieldNameMapper())
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
		log := logr.FromContextOrDiscard(r.Context()).WithValues("handler", requestPath)
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

		// add function to render mustache templates
		rt.Set("render", mustache.RenderTemplateForScope(templatePartials, path.Dir(requestPath), w))
		rt.Set("renderToString", mustache.RenderTemplateForScopeToString(templatePartials, path.Dir(requestPath)))

		// add function to send SSE

		type ServerEvent struct {
			ID    string
			Event string
			Data  string
		}

		rt.Set("sendServerEvents", func(nextEvent func() (*ServerEvent, error)) error {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")

			writeMessage := func(m *ServerEvent) error {
				if len(m.ID) > 0 {
					_, err := fmt.Fprintf(w, "id: %s\n", strings.Replace(m.ID, "\n", "", -1))
					if err != nil {
						return err
					}
				}
				if len(m.Event) > 0 {
					_, err := fmt.Fprintf(w, "event: %s\n", strings.Replace(m.Event, "\n", "", -1))
					if err != nil {
						return err
					}
				}
				if len(m.Data) > 0 {
					lines := strings.Split(m.Data, "\n")
					for _, line := range lines {
						_, err := fmt.Fprintf(w, "data: %s\n", line)
						if err != nil {
							return err
						}
					}
				}
				_, err := w.Write([]byte("\n"))
				if err != nil {
					return err
				}

				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}

				return nil

			}

			for {
				evt, err := nextEvent()
				if err != nil {
					return err
				}

				if evt == nil {
					return nil
				}

				err = writeMessage(evt)
				if err != nil {
					return err
				}

			}
		})

		_, err := fn(nil, rt.ToValue(w), rt.ToValue(r), rt.ToValue(params))
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			log.Error(err, "handler error")
			return
		}

	}), nil
}
