package require

import (
	"fmt"
	"io"
	"io/fs"
	"regexp"
	"strings"

	"github.com/dop251/goja"
	"github.com/draganm/go-lean/common/globals"
)

var libRegexp = regexp.MustCompile(`^/lib/(.+).js$`)

func NewProvider(src fs.FS, root string) (globals.VMGlobalProvider, error) {

	libs := map[string]string{}

	err := fs.WalkDir(src, root, func(pth string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		withoutPrefix := strings.TrimPrefix(pth, root)

		if d.IsDir() {
			return nil
		}

		f, err := src.Open(pth)
		if err != nil {
			return fmt.Errorf("could not open %s: %w", pth, err)
		}

		defer f.Close()
		data, err := io.ReadAll(f)
		if err != nil {
			return fmt.Errorf("could not read %s: %w", pth, err)
		}

		if libRegexp.MatchString(withoutPrefix) {
			libs[withoutPrefix] = string(data)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("could not get libs: %w", err)
	}

	return func(rt *goja.Runtime) (any, error) {
		return func(libName string) (goja.Value, error) {
			libCode, found := libs[libName]
			if !found {
				return nil, fmt.Errorf("%s not found", libName)
			}
			return rt.RunScript(
				libName,
				fmt.Sprintf(`(() => { var exports = {}; var module = { exports: exports}; %s; return module.exports})()`, libCode),
			)

		}, nil
	}, nil

}
