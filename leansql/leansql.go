package leansql

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"

	"github.com/dop251/goja"
	"github.com/draganm/go-lean/common/globals"
)

type LeanSQL struct {
	db  *sql.DB
	vm  *goja.Runtime
	ctx context.Context
}

func New(db *sql.DB, vm *goja.Runtime, ctx context.Context) globals.Values {
	ls := &LeanSQL{db: db, vm: vm, ctx: ctx}

	return map[string]any{
		"query": ls.Query,
	}
}

func (ls *LeanSQL) Query(q string, args []any, fn goja.Callable) (goja.Value, error) {
	res, err := ls.db.QueryContext(ls.ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("could not execute query: %w", err)
	}
	defer res.Close()

	cts, err := res.ColumnTypes()
	if err != nil {
		return nil, fmt.Errorf("could not get column types: %w", err)
	}

	o := ls.vm.NewObject()

	toGojaObject := func(vals map[string]any) goja.Value {
		obj := ls.vm.NewObject()
		for k, v := range vals {
			obj.Set(k, v)
		}
		return obj
	}
	o.SetSymbol(goja.SymIterator, func() (*goja.Object, error) {
		iter := ls.vm.NewObject()

		iter.Set("next", goja.Callable(func(this goja.Value, args ...goja.Value) (goja.Value, error) {

			isDone := !res.Next()

			if res.Err() != nil {
				return nil, err
			}

			if isDone {
				return toGojaObject(map[string]any{
					"done": true,
				}), nil
			}

			vals := make([]interface{}, len(cts))
			for i, ct := range cts {
				vals[i] = reflect.New(ct.ScanType()).Interface()
			}

			err = res.Scan(vals...)
			if err != nil {
				return nil, fmt.Errorf("could not scan: %w", err)
			}

			for i, v := range vals {
				vals[i] = sqlValueToGoValue(v, ls.vm)
			}

			return toGojaObject(map[string]any{
				"value": vals,
				"done":  false,
			}), nil
		}))
		return iter, nil
	})

	return fn(nil, o)

}

func sqlValueToGoValue(v any, vm *goja.Runtime) any {

	switch tv := v.(type) {
	case *sql.NullInt64:
		if tv.Valid {
			return tv.Int64
		}
		return nil
	case *sql.NullTime:
		if tv.Valid {
			o, _ := vm.New(vm.GlobalObject().Get("Date"), vm.ToValue(tv.Time.Unix()*1000))
			return o
			// return tv.Time
		}
		return nil
	case *sql.NullString:
		if tv.Valid {
			return tv.String
		}
		return nil
	case *sql.NullBool:
		if tv.Valid {
			return tv.Bool
		}
		return nil

	case *sql.NullByte:
		if tv.Valid {
			return tv.Byte
		}
		return nil

	case *sql.NullFloat64:
		if tv.Valid {
			return tv.Float64
		}
		return nil

	case *sql.NullInt16:
		if tv.Valid {
			return tv.Int16
		}
		return nil

	case *sql.NullInt32:
		if tv.Valid {
			return tv.Int32
		}
		return nil

	}

	val := reflect.ValueOf(v)
	for val.Kind() == reflect.Pointer {
		val = val.Elem()
	}
	return val.Interface()
}
