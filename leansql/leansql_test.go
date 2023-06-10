package leansql_test

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/dop251/goja"
	"github.com/draganm/go-lean/common/providers"
	"github.com/draganm/go-lean/leansql"
	"github.com/draganm/go-lean/leanweb"
	"github.com/go-logr/logr/testr"
	"github.com/golang-migrate/migrate/v4"
	three "github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/httpfs"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
)

//go:embed fixtures
var simple embed.FS

func XTestQuery(t *testing.T) {

	require := require.New(t)
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name()))
	require.NoError(err)
	defer db.Close()

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS blog (
		id INTEGER NOT NULL PRIMARY KEY,
		time DATETIME NOT NULL,
		description TEXT
		); 
	`)

	require.NoError(err)

	_, err = db.Exec(`
	INSERT INTO blog VALUES (1,?,?); 
	`, time.Now(), "foo")
	require.NoError(err)

	_, err = db.Exec(`
	INSERT INTO blog VALUES (2,?,?); 
	`, time.Now(), "bar")
	require.NoError(err)

	vm := goja.New()

	vm.Set("sql", leansql.New(db, vm, context.Background()))
	vm.Set("print", func(s *string) { fmt.Println(*s) })

	res, err := vm.RunString(`
		sql.Query("select * from blog order by id",[],(x) => Array.from(x, ([a]) => a))
	`)
	require.NoError(err)

	js, err := res.ToObject(vm).MarshalJSON()
	require.NoError(err)

	require.JSONEq(`[1,2]`, string(js))
}

func TestWithWeb(t *testing.T) {
	require := require.New(t)

	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name()))
	require.NoError(err)

	defer db.Close()

	source, err := httpfs.New(http.FS(simple), "fixtures/migrations")
	require.NoError(err)

	migDriver, err := three.WithInstance(db, &three.Config{
		MigrationsTable: three.DefaultMigrationsTable,
		DatabaseName:    t.Name(),
	})
	require.NoError(err)

	mig, err := migrate.NewWithInstance("httpfs", source, "sqlite3", migDriver)
	require.NoError(err)

	err = mig.Up()
	if err == migrate.ErrNoChange {
		// ignore it
	} else {
		require.NoError(err)
	}

	lw, err := leanweb.New(simple, "fixtures/html", testr.New(t), map[string]any{}, &leanweb.GlobalsProviders{
		Context: []providers.ContextGlobalsProvider{leansql.NewProvider(db, "sql")},
	})

	require.NoError(err)

	require.HTTPStatusCode(lw.ServeHTTP, "GET", "/", nil, 200)
	require.HTTPBodyContains(lw.ServeHTTP, "GET", "/", nil, `[[2,"abc"],[3,"def"]]`)
}
