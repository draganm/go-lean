package mustache

import (
	"context"
	"net/http"
	"regexp"

	"github.com/draganm/go-lean/common/globals"
	"github.com/draganm/go-lean/web/types"
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("github.com/draganm/go-lean/leanweb/mustache")

var templateRegexp = regexp.MustCompile(`^(.+).mustache$`)

type MustacheProvider func(ctx context.Context, handlerPath types.HandlerPath, w http.ResponseWriter) globals.Values
