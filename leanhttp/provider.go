package leanhttp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/draganm/go-lean/common/globals"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type HTTPOptions struct {
	Body   string            `lean:"body"`
	Header map[string]string `lean:"header"`
}

type HTTPResponse struct {
	Status     string      `lean:"status"`
	StatusCode int         `lean:"statusCode"`
	Body       string      `lean:"body"`
	Header     http.Header `lean:"header"`
}

func NewProvider(client *http.Client) func(ctx context.Context) globals.Values {

	transport := client.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	newTransport := otelhttp.NewTransport(transport)

	client.Transport = newTransport

	return func(ctx context.Context) globals.Values {

		return map[string]any{
			"request": func(method, url string, opts HTTPOptions) (*HTTPResponse, error) {
				req, err := http.NewRequestWithContext(ctx, method, url, strings.NewReader(opts.Body))
				if err != nil {
					return nil, fmt.Errorf("could not create request: %w", err)
				}

				for k, v := range opts.Header {
					req.Header.Set(k, v)
				}

				res, err := client.Do(req)
				if err != nil {
					return nil, fmt.Errorf("could not perform request: %w", err)
				}

				defer res.Body.Close()

				d, err := io.ReadAll(res.Body)
				if err != nil {
					return nil, fmt.Errorf("could not read request body: %w", err)
				}

				copyOfHeader := http.Header{}

				for k, v := range res.Header {
					vc := make([]string, len(v))
					copy(vc, v)
					copyOfHeader[k] = vc
				}

				return &HTTPResponse{
					Status:     res.Status,
					StatusCode: res.StatusCode,
					Body:       string(d),
					Header:     copyOfHeader,
				}, nil

			},
		}
	}
}
