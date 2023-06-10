package providers

import (
	"context"
	"net/http"

	"github.com/dop251/goja"
)

type RequestGlobalsProvider func(handlerPath string, vm *goja.Runtime, w http.ResponseWriter, r *http.Request) (map[string]any, error)
type ContextGlobalsProvider func(vm *goja.Runtime, ctx context.Context) (map[string]any, error)
type GenericGlobalsProvider func(vm *goja.Runtime) (map[string]any, error)
