package leansql

import (
	"context"
	"database/sql"

	"github.com/dop251/goja"
	"github.com/draganm/go-lean/common/globals"
)

func NewProvider(db *sql.DB) func(vm *goja.Runtime, ctx context.Context) globals.Values {
	return func(vm *goja.Runtime, ctx context.Context) globals.Values {
		return New(db, vm, ctx)
	}
}
