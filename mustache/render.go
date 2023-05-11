package mustache

import (
	"fmt"
	"io"
	"path"
	"strings"
	"sync"

	"github.com/cbroglie/mustache"
)

type scopedPartialProvider struct {
	partials map[string]string
	scope    string
}

func (sp scopedPartialProvider) Get(name string) (string, error) {
	if !strings.HasPrefix(name, "/") {
		name = path.Join(sp.scope, name)
	}
	name = path.Clean(name)
	partial, found := sp.partials[name]
	if !found {
		return "", fmt.Errorf("could not find mustache partial %s", name)
	}
	return partial, nil
}

type templateCache struct {
	cached map[string]*mustache.Template
	sp     scopedPartialProvider
	mu     *sync.RWMutex
}

func (tc *templateCache) getTemplate(name string) (*mustache.Template, error) {
	tc.mu.RLock()
	template, found := tc.cached[name]
	if found {
		tc.mu.RUnlock()
		return template, nil
	}
	tc.mu.RUnlock()

	templateString, err := tc.sp.Get(name)
	if err != nil {
		return nil, err
	}

	template, err = mustache.ParseStringPartials(templateString, tc.sp)
	if err != nil {
		return nil, fmt.Errorf("could not parse template: %w", err)
	}

	tc.mu.Lock()
	tc.cached[name] = template
	tc.mu.Unlock()

	return template, nil
}

func RenderTemplateForScope(partials map[string]string, currentPath string, w io.Writer) func(name string, data interface{}) error {
	tc := &templateCache{
		cached: map[string]*mustache.Template{},
		sp:     scopedPartialProvider{partials: partials, scope: currentPath},
		mu:     &sync.RWMutex{},
	}

	return func(name string, data any) error {
		template, err := tc.getTemplate(name)
		if err != nil {
			return fmt.Errorf("could not get/parse template %s in scope %s: %w", name, currentPath, err)
		}

		return template.FRender(w, data)
	}

}
