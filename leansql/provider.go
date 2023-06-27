package leansql

import (
	"context"
	"database/sql"

	"github.com/dop251/goja"
	"github.com/draganm/go-lean/common/globals"
)

func NewProvider(db *sql.DB) globals.ContextAndVMGlobalProvider {

	return func(vm *goja.Runtime, ctx context.Context) (any, error) {
		return New(db, vm, ctx), nil
	}
}
