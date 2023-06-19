package leanhttp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/dop251/goja"
	"github.com/draganm/go-lean/common/providers"
)

type HTTPOptions struct {
	Body   string
	Header map[string]string
}

type HTTPResponse struct {
	Status     string
	StatusCode int
	Body       string
	Header     http.Header
}

func NewProvider(client *http.Client) providers.ContextGlobalsProvider {

	return func(vm *goja.Runtime, ctx context.Context) (map[string]any, error) {
		return map[string]any{
			"requestHttp": func(method, url string, opts HTTPOptions) (*HTTPResponse, error) {
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
		}, nil
	}
}
