package globals

import (
	"context"
	"fmt"
	"net/http"

	"github.com/dop251/goja"
)

type RequestGlobalsProvider func(handlerPath string, vm *goja.Runtime, w http.ResponseWriter, r *http.Request) (any, error)
type ContextGlobalProvider func(ctx context.Context) (any, error)
type VMGlobalProvider func(vm *goja.Runtime) (any, error)
type ContextAndVMGlobalProvider func(vm *goja.Runtime, ctx context.Context) (any, error)

func IsRequestGlobalProvider(val any) bool {
	_, res := val.(RequestGlobalsProvider)
	return res
}

func IsContextGlobalProvider(val any) bool {
	_, res := val.(ContextGlobalProvider)
	return res
}

func IsVMGlobalProvider(val any) bool {
	_, res := val.(VMGlobalProvider)
	return res
}

func IsContextAndVMGlobalProvider(val any) bool {
	_, res := val.(ContextAndVMGlobalProvider)
	return res
}

func IsPlainValue(val any) bool {
	return !IsContextGlobalProvider(val) && !IsRequestGlobalProvider(val) && !IsVMGlobalProvider(val) && !IsContextAndVMGlobalProvider(val)
}

func ProvideGlobalValue(vm *goja.Runtime, name string, value any) error {

	if IsRequestGlobalProvider(value) {
		return fmt.Errorf("%s is unresolved request global provider", name)
	}

	if IsContextGlobalProvider(value) {
		return fmt.Errorf("%s is unresolved context global provider", name)
	}

	if IsVMGlobalProvider(value) {
		return fmt.Errorf("%s is unresolved VM global provider", name)
	}

	err := vm.GlobalObject().Set(name, value)
	if err != nil {
		return fmt.Errorf("counld not set global value %s: %w", name, err)
	}

	return nil
}

func ProvideVMGlobalValue(vm *goja.Runtime, name string, value any) error {
	ctxAndVm, isVMGlobal := value.(VMGlobalProvider)
	if isVMGlobal {
		var err error
		value, err = ctxAndVm(vm)
		if err != nil {
			return fmt.Errorf("could not create vm global value %s: %w", name, err)
		}
	}

	return ProvideGlobalValue(vm, name, value)
}
func ProvideContextGlobalValue(ctx context.Context, vm *goja.Runtime, name string, value any) error {
	cg, isContextGlobal := value.(ContextGlobalProvider)
	if isContextGlobal {
		var err error
		value, err = cg(ctx)
		if err != nil {
			return fmt.Errorf("could not create context global value %s: %w", name, err)
		}
	}

	return ProvideVMGlobalValue(vm, name, value)
}

func ProvideContextAndVMGlobalValue(ctx context.Context, vm *goja.Runtime, name string, value any) error {
	ctxAndVm, isContextAndVMGlobal := value.(ContextAndVMGlobalProvider)
	if isContextAndVMGlobal {
		var err error
		value, err = ctxAndVm(vm, ctx)
		if err != nil {
			return fmt.Errorf("could not create context global value %s: %w", name, err)
		}
	}

	return ProvideContextGlobalValue(ctx, vm, name, value)
}

func ProvideRequestGlobalValue(handlerPath string, vm *goja.Runtime, w http.ResponseWriter, r *http.Request, name string, value any) error {
	rg, isRequestGlobal := value.(RequestGlobalsProvider)
	if isRequestGlobal {
		var err error
		value, err = rg(handlerPath, vm, w, r)
		if err != nil {
			return fmt.Errorf("could not create context global value %s: %w", name, err)
		}
	}

	return ProvideContextAndVMGlobalValue(r.Context(), vm, name, value)

}
