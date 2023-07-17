package mustache

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"sync"

	"github.com/cbroglie/mustache"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func renderTemplateForScope(ctx context.Context, tc *scopedTemplateCache, w io.Writer) func(name string, data interface{}) error {

	return func(name string, data any) error {
		_, span := tracer.Start(ctx, fmt.Sprintf("mustache.RenderTemplate %s", name),
			trace.WithAttributes(
				attribute.String("template", name),
			),
		)

		defer span.End()

		template, err := tc.getTemplate(name)
		if err != nil {
			span.RecordError(err)
			return fmt.Errorf("could not get/parse template %s in scope %s: %w", name, tc.sp.scope, err)
		}

		return template.FRender(w, data)
	}

}

func renderTemplateForScopeToString(ctx context.Context, tc *scopedTemplateCache) func(name string, data interface{}) (string, error) {

	return func(name string, data any) (string, error) {
		_, span := tracer.Start(ctx, fmt.Sprintf("mustache.RenderTemplateToString %s", name),
			trace.WithAttributes(
				attribute.String("template", name),
			),
		)

		defer span.End()
		template, err := tc.getTemplate(name)
		if err != nil {
			span.RecordError(err)
			return "", fmt.Errorf("could not get/parse template %s in scope %s: %w", name, tc.sp.scope, err)
		}

		return template.Render(data)
	}

}

type templateCacheForPathFactory struct {
	partials      map[string]string
	cachesForPath map[string]*scopedTemplateCache
	mu            *sync.Mutex
}

func (tf *templateCacheForPathFactory) getTemplateCacheForPath(pth string) *scopedTemplateCache {
	tf.mu.Lock()
	defer tf.mu.Unlock()
	tc, found := tf.cachesForPath[pth]
	if !found {
		tc = &scopedTemplateCache{
			cached: make(map[string]*mustache.Template),
			sp:     scopedPartialProvider{partials: tf.partials, scope: pth},
			mu:     &sync.RWMutex{},
		}
		tf.cachesForPath[pth] = tc
	}
	return tc
}

type scopedTemplateCache struct {
	cached map[string]*mustache.Template
	sp     scopedPartialProvider
	mu     *sync.RWMutex
}

func (tc *scopedTemplateCache) getTemplate(name string) (*mustache.Template, error) {
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
