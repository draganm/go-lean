package require

import (
	"fmt"

	"github.com/dop251/goja"
)

type Builder struct {
	files map[string]func() ([]byte, error)
}

func NewBuilder() *Builder {
	return &Builder{
		files: map[string]func() ([]byte, error){},
	}
}

func (b *Builder) Consume(pth string, getContent func() ([]byte, error)) bool {

	if libRegexp.MatchString(pth) {
		b.files[pth] = getContent
		return true
	}

	return false
}

type RequireProvider func(rt *goja.Runtime, libName string) (goja.Value, error)

func (b *Builder) Build() RequireProvider {
	return func(rt *goja.Runtime, libName string) (goja.Value, error) {
		libCodeGet, found := b.files[libName]
		if !found {
			return nil, fmt.Errorf("%s not found", libName)
		}

		libCode, err := libCodeGet()
		if err != nil {
			return nil, fmt.Errorf("could not get code: %w", err)
		}
		return rt.RunScript(
			libName,
			fmt.Sprintf(`(() => { var exports = {}; var module = { exports: exports}; %s; return module.exports})()`, libCode),
		)

	}
}
