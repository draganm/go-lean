package leansql

import (
	"context"
	"database/sql"

	"github.com/dop251/goja"
	"github.com/draganm/go-lean/common/providers"
)

func NewProvider(db *sql.DB, name string) providers.ContextGlobalsProvider {

	return func(vm *goja.Runtime, ctx context.Context) (map[string]any, error) {
		return map[string]any{
			name: New(db, vm, ctx),
		}, nil
	}
}
