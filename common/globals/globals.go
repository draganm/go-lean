package globals

import (
	"errors"
	"fmt"
	"reflect"
)

type Globals map[string]any

func (g Globals) Merge(other Globals) (res Globals, err error) {
	res = Globals{}

	for k, v := range g {
		res[k] = v
	}

	errs := []error{}

	for k, v := range other {
		_, contains := res[k]
		if contains {
			errs = append(errs, fmt.Errorf("could not merge globals, %s is set in both globals", k))
			continue
		}
		res[k] = v
	}

	if len(errs) != 0 {
		return nil, errors.Join(errs...)
	}

	return res, nil

}

func autoWireFunction(v any, values ...any) (any, error) {
	rv := reflect.ValueOf(v)

	if rv.Kind() != reflect.Func {
		return v, nil
	}

	t := rv.Type()

	argCount := t.NumIn()
	bound := []reflect.Value{}
outer:
	for i := 0; i < argCount; i++ {
		it := t.In(i)

		for _, av := range values {
			avVal := reflect.ValueOf(av)
			if avVal.Type().AssignableTo(it) {
				bound = append(bound, avVal)
				continue outer
			}
		}

		break
	}

	// if len(bound) == 0 {
	// 	return v
	// }

	in := []reflect.Type{}
	for i := len(bound); i < t.NumIn(); i++ {
		in = append(in, t.In(i))
	}

	out := []reflect.Type{}
	for i := 0; i < t.NumOut(); i++ {
		out = append(out, t.Out(i))
	}

	ft := reflect.FuncOf(in, out, t.IsVariadic())

	if len(in) == 0 && len(out) > 0 && out[0] == valuesType {
		res := rv.Call(bound)
		if len(out) > 1 {
			// check for error
			lastType := out[len(out)-1]
			if lastType == errorType {
				lv := res[len(out)-1]
				if !lv.IsNil() {
					return nil, lv.Interface().(error)
				}
			}
		}

		return res[0].Interface().(Values), nil

	}

	return reflect.MakeFunc(ft, func(args []reflect.Value) (results []reflect.Value) {
		realArgs := make([]reflect.Value, len(args)+len(bound))
		copy(realArgs, bound)
		copy(realArgs[len(bound):], args)
		return rv.Call(realArgs)
	}).Interface(), nil

}

func (g Globals) Autowire(vals ...any) (Globals, error) {
	res := Globals{}
	for k, v := range g {
		wv, err := autoWireFunction(v, vals...)
		if err != nil {
			return nil, fmt.Errorf("could not autowire %s: %w", k, err)
		}
		res[k] = wv
	}

	return res, nil
}

type Values map[string]any

var valuesType = reflect.TypeOf(Values{})
var errorType = reflect.TypeOf(errors.New(""))
